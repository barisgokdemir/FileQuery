// Package web serves FileQuery's browser interface on the local machine.
//
// Security model: the server listens on 127.0.0.1 only. Every session starts
// from a URL containing a random token, which is exchanged for a SameSite=Strict
// cookie; API calls without the cookie (or matching header) are rejected, and
// requests whose Host header is not the loopback address are refused to defeat
// DNS rebinding. Other web sites open in the same browser therefore cannot run
// queries against the file system. The API is read-only.
package web

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/barisgokdemir/FileQuery/internal/engine"
	"github.com/barisgokdemir/FileQuery/internal/output"
	"github.com/barisgokdemir/FileQuery/internal/query"
)

//go:embed static
var staticFiles embed.FS

const (
	cookieName = "fq_oturum"
	// DefaultMaxRows caps how many rows are sent to the browser for one query;
	// larger results can still be exported in full.
	DefaultMaxRows = 10000
)

// Server is the HTTP handler for the browser interface.
type Server struct {
	token   string
	version string
	maxRows int
	reveal  func(path string) error // "show in folder"; replaced in tests
	mux     *http.ServeMux
	host    string // expected Host header, set by Listen
}

// New creates a server with a fresh random session token.
func New(version string) *Server {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	s := &Server{token: hex.EncodeToString(b), version: version, maxRows: DefaultMaxRows, reveal: Reveal}
	s.routes()
	return s
}

// Token returns the session token embedded in the start URL.
func (s *Server) Token() string { return s.token }

func (s *Server) routes() {
	static, _ := fs.Sub(staticFiles, "static")
	files := http.FileServer(http.FS(static))

	s.mux = http.NewServeMux()
	s.mux.HandleFunc("GET /{$}", s.handleIndex)
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", files))
	s.mux.HandleFunc("GET /api/schema", s.auth(s.handleSchema))
	s.mux.HandleFunc("POST /api/query", s.auth(s.handleQuery))
	s.mux.HandleFunc("POST /api/check", s.auth(s.handleCheck))
	s.mux.HandleFunc("GET /api/export", s.auth(s.handleExport))
	s.mux.HandleFunc("POST /api/reveal", s.auth(s.handleReveal))
}

// Listen opens a loopback listener. port 0 picks a free port.
func (s *Server) Listen(port int) (net.Listener, string, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, "", err
	}
	s.host = ln.Addr().String()
	url := fmt.Sprintf("http://%s/?t=%s", s.host, s.token)
	return ln, url, nil
}

// Serve runs the HTTP server until ctx is cancelled.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}()
	err := srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.hostAllowed(r.Host) {
		http.Error(w, "geçersiz Host başlığı", http.StatusForbidden)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; img-src 'self' data:; frame-ancestors 'none'")
	s.mux.ServeHTTP(w, r)
}

// hostAllowed accepts only loopback hosts, which defeats DNS rebinding.
func (s *Server) hostAllowed(host string) bool {
	if s.host != "" && host == s.host {
		return true
	}
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	return h == "127.0.0.1" || h == "localhost" || h == "[::1]" || h == "::1"
}

func (s *Server) validToken(t string) bool {
	return subtle.ConstantTimeCompare([]byte(t), []byte(s.token)) == 1
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ok := false
		if c, err := r.Cookie(cookieName); err == nil && s.validToken(c.Value) {
			ok = true
		}
		if h := r.Header.Get("X-FQ-Token"); h != "" && s.validToken(h) {
			ok = true
		}
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"hata": "Oturum geçersiz. Arayüzü fq penceresinde yazan adresten yeniden açın.",
			})
			return
		}
		// State-changing requests must come from our own page.
		if r.Method == http.MethodPost {
			if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
				writeJSON(w, http.StatusForbidden, map[string]string{"hata": "farklı kaynaktan istek reddedildi"})
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if t := r.URL.Query().Get("t"); t != "" {
		if !s.validToken(t) {
			http.Error(w, "Geçersiz oturum anahtarı. fq penceresindeki adresi kullanın.", http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name: cookieName, Value: s.token, Path: "/",
			HttpOnly: true, SameSite: http.SameSiteStrictMode,
		})
		// Drop the token from the address bar and history.
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if c, err := r.Cookie(cookieName); err != nil || !s.validToken(c.Value) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><title>FileQuery</title>
<link rel="stylesheet" href="/static/style.css">
<main class="bos"><h1>Oturum bulunamadı</h1>
<p>Arayüzü açmak için <b>fq</b> penceresinde yazan adresi kullanın. fq kapatıldıysa yeniden başlatın.</p></main>`)
		return
	}
	page, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(page)
}

type fieldJSON struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Desc string `json:"desc"`
}

func (s *Server) handleSchema(w http.ResponseWriter, r *http.Request) {
	fields := make([]fieldJSON, len(query.Schema))
	for i, f := range query.Schema {
		fields[i] = fieldJSON{f.Name, f.Kind.String(), f.Desc}
	}
	home, _ := os.UserHomeDir()
	writeJSON(w, http.StatusOK, map[string]any{
		"version":   s.version,
		"fields":    fields,
		"functions": query.FunctionNames(),
		"home":      home,
		"separator": string(os.PathSeparator),
		"maxRows":   s.maxRows,
		// The server runs on the user's own machine, so this is their real OS.
		"os":     runtime.GOOS,
		"places": Places(),
		"sets":   extSetsJSON(),
	})
}

// cell encodes a value for the browser. Numbers, text and booleans map to
// JSON directly; dates and durations are tagged so the page can format them.
func cell(v query.Value) any {
	switch v.Kind {
	case query.KindBool:
		return v.B
	case query.KindInt:
		return v.I
	case query.KindFloat:
		return v.F
	case query.KindString:
		return v.S
	case query.KindTime:
		return map[string]string{"t": v.T.Format(time.RFC3339)}
	case query.KindDuration:
		return map[string]float64{"d": time.Duration(v.I).Seconds()}
	}
	return nil
}

type queryRequest struct {
	Query string `json:"query"`
}

type columnJSON struct {
	Name   string `json:"name"`
	IsSize bool   `json:"isSize"`
}

type errorJSON struct {
	Message string `json:"message"`
	Pos     int    `json:"pos"` // character (rune) offset; -1 when unknown
}

var errRowLimit = errors.New("satır sınırına ulaşıldı")

// handleQuery runs a query and streams the result as newline-delimited JSON:
//
//	{"columns": [...]}
//	{"rows": [[...], ...]}          (batched)
//	{"progress": {"scanned": N, "rows": M}}
//	{"done": {...stats}} or {"error": {...}}
//
// Closing the connection (the Stop button) cancels the scan.
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	var req queryRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"hata": "geçersiz istek"})
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	sw := newStreamWriter(w)
	defer sw.close()

	q, err := query.Parse(req.Query)
	if err != nil {
		sw.send(map[string]any{"error": queryError(req.Query, err)})
		return
	}

	cols := engine.Columns(q)
	colsJSON := make([]columnJSON, len(cols))
	for i, c := range cols {
		colsJSON[i] = columnJSON{c.Name, c.IsSize}
	}
	sw.send(map[string]any{"columns": colsJSON})

	var (
		progress atomic.Int64
		rows     atomic.Int64
		mu       sync.Mutex
		batch    [][]any
	)
	flushBatch := func() {
		mu.Lock()
		b := batch
		batch = nil
		mu.Unlock()
		if len(b) > 0 {
			sw.send(map[string]any{"rows": b})
		}
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Heartbeat: push batched rows and the live counters a few times a second.
	tickerDone := make(chan struct{})
	go func() {
		defer close(tickerDone)
		t := time.NewTicker(200 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				flushBatch()
				sw.send(map[string]any{"progress": map[string]int64{"scanned": progress.Load(), "rows": rows.Load()}})
			}
		}
	}()

	truncated := false
	stats, runErr := engine.Run(ctx, q, engine.Options{Progress: &progress}, func(vals []query.Value) error {
		if int(rows.Load()) >= s.maxRows {
			truncated = true
			return errRowLimit
		}
		row := make([]any, len(vals))
		for i, v := range vals {
			row[i] = cell(v)
		}
		mu.Lock()
		batch = append(batch, row)
		full := len(batch) >= 500
		mu.Unlock()
		rows.Add(1)
		if full {
			flushBatch()
		}
		return nil
	})
	cancel()
	<-tickerDone
	flushBatch()

	if runErr != nil && !errors.Is(runErr, errRowLimit) {
		if r.Context().Err() != nil {
			return // the browser stopped the query
		}
		sw.send(map[string]any{"error": queryError(req.Query, runErr)})
		return
	}
	sw.send(map[string]any{"done": map[string]any{
		"rows":        rows.Load(),
		"scanned":     stats.Entries,
		"denied":      stats.Denied,
		"deniedPaths": stats.DeniedPaths,
		"elapsedMs":   stats.Elapsed.Milliseconds(),
		"truncated":   truncated,
		"maxRows":     s.maxRows,
	}})
}

// handleCheck validates a query without scanning anything: it parses it and
// checks that the folders exist. The query builder uses it for live feedback.
func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	var req queryRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"hata": "geçersiz istek"})
		return
	}
	q, err := query.Parse(req.Query)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": queryError(req.Query, err)})
		return
	}
	for _, src := range q.Sources {
		if _, err := engine.ResolveRoot(src); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"error": errorJSON{Message: err.Error(), Pos: -1}})
			return
		}
	}
	cols := engine.Columns(q)
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "columns": names})
}

func queryError(src string, err error) errorJSON {
	var qe *query.Error
	if errors.As(err, &qe) {
		pos := qe.Pos
		if pos > len(src) {
			pos = len(src)
		}
		return errorJSON{Message: qe.Msg, Pos: len([]rune(src[:pos]))}
	}
	return errorJSON{Message: err.Error(), Pos: -1}
}

// handleExport runs the query without the row cap and returns a file.
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	src := r.URL.Query().Get("q")
	format := r.URL.Query().Get("format")
	ext := map[string]string{"csv": "csv", "excel": "csv", "json": "json"}[format]
	if ext == "" {
		http.Error(w, "biçim csv, excel veya json olmalı", http.StatusBadRequest)
		return
	}
	q, err := query.Parse(src)
	if err != nil {
		http.Error(w, queryError(src, err).Message, http.StatusBadRequest)
		return
	}
	name := "filequery-" + time.Now().Format("20060102-1504") + "." + ext
	ctype := map[string]string{"csv": "text/csv; charset=utf-8", "json": "application/json; charset=utf-8"}[ext]
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)

	bw := bufio.NewWriterSize(w, 64<<10)
	out, _ := output.New(format, bw)
	out.Begin(engine.Columns(q))
	_, runErr := engine.Run(r.Context(), q, engine.Options{SortByPath: true}, out.Row)
	out.End()
	if runErr != nil {
		fmt.Fprintf(bw, "\n# hata: %s\n", runErr)
	}
	bw.Flush()
}

func (s *Server) handleReveal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil || req.Path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"hata": "yol eksik"})
		return
	}
	if _, err := os.Lstat(req.Path); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"hata": "dosya bulunamadı: " + req.Path})
		return
	}
	if err := s.reveal(req.Path); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"hata": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// streamWriter writes NDJSON lines from several goroutines and flushes each
// line so the browser sees results immediately.
type streamWriter struct {
	mu      sync.Mutex
	w       http.ResponseWriter
	flusher http.Flusher
	enc     *json.Encoder
	closed  bool
}

func newStreamWriter(w http.ResponseWriter) *streamWriter {
	f, _ := w.(http.Flusher)
	return &streamWriter{w: w, flusher: f, enc: json.NewEncoder(w)}
}

func (s *streamWriter) send(v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if err := s.enc.Encode(v); err != nil {
		s.closed = true
		return
	}
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

func (s *streamWriter) close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}

type extSetJSON struct {
	Name string   `json:"name"`
	Desc string   `json:"desc"`
	Exts []string `json:"exts"`
}

func extSetsJSON() []extSetJSON {
	out := make([]extSetJSON, len(query.ExtSets))
	for i, s := range query.ExtSets {
		out[i] = extSetJSON{s.Name, s.Desc, s.Exts}
	}
	return out
}
