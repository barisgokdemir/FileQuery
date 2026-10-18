package web

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testServer(t *testing.T) (*Server, *httptest.Server, string) {
	t.Helper()
	root := t.TempDir()
	for name, size := range map[string]int{"rapor.pdf": 3 << 20, "not.txt": 10, "alt/sunum.pptx": 1 << 20} {
		p := filepath.Join(root, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, make([]byte, size), 0o644)
	}
	s := New("test")
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return s, ts, root
}

func do(t *testing.T, s *Server, method, url, body string, withToken bool) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	if withToken {
		req.AddCookie(&http.Cookie{Name: cookieName, Value: s.Token()})
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func readStream(t *testing.T, resp *http.Response) []map[string]json.RawMessage {
	t.Helper()
	var msgs []map[string]json.RawMessage
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("geçersiz NDJSON satırı %q: %v", sc.Text(), err)
		}
		msgs = append(msgs, m)
	}
	return msgs
}

func TestTokenExchangeSetsCookieAndRedirects(t *testing.T) {
	s, ts, _ := testServer(t)
	resp := do(t, s, "GET", ts.URL+"/?t="+s.Token(), "", false)
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" {
		t.Fatalf("yönlendirme bekleniyordu: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	c := resp.Cookies()
	if len(c) != 1 || c[0].Value != s.Token() || !c[0].HttpOnly || c[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("çerez hatalı: %+v", c)
	}

	if resp := do(t, s, "GET", ts.URL+"/?t=yanlis", "", false); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("yanlış anahtar kabul edildi: %d", resp.StatusCode)
	}
	if resp := do(t, s, "GET", ts.URL+"/", "", false); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("çerezsiz ana sayfa açıldı: %d", resp.StatusCode)
	}
	resp = do(t, s, "GET", ts.URL+"/", "", true)
	if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Content-Security-Policy"), "default-src 'self'") {
		t.Errorf("ana sayfa: %d %v", resp.StatusCode, resp.Header)
	}
}

func TestAPIRequiresSession(t *testing.T) {
	s, ts, root := testServer(t)
	body := `{"query":"SELECT name FROM '` + jsonPath(root) + `'"}`
	if resp := do(t, s, "POST", ts.URL+"/api/query", body, false); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("oturumsuz sorgu: %d", resp.StatusCode)
	}
	if resp := do(t, s, "GET", ts.URL+"/api/schema", "", false); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("oturumsuz şema: %d", resp.StatusCode)
	}
	if resp := do(t, s, "GET", ts.URL+"/api/export?format=csv&q=x", "", false); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("oturumsuz dışa aktarma: %d", resp.StatusCode)
	}
}

func TestRejectsForeignHostAndOrigin(t *testing.T) {
	s, ts, root := testServer(t)
	req, _ := http.NewRequest("GET", ts.URL+"/api/schema", nil)
	req.Host = "kotu-site.example:80"
	req.AddCookie(&http.Cookie{Name: cookieName, Value: s.Token()})
	resp, _ := http.DefaultTransport.RoundTrip(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("DNS rebinding Host başlığı kabul edildi: %d", resp.StatusCode)
	}

	body := `{"query":"SELECT name FROM '` + jsonPath(root) + `'"}`
	req, _ = http.NewRequest("POST", ts.URL+"/api/query", strings.NewReader(body))
	req.Header.Set("Origin", "https://kotu-site.example")
	req.AddCookie(&http.Cookie{Name: cookieName, Value: s.Token()})
	resp, _ = http.DefaultTransport.RoundTrip(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("yabancı Origin kabul edildi: %d", resp.StatusCode)
	}
}

func TestSchema(t *testing.T) {
	s, ts, _ := testServer(t)
	resp := do(t, s, "GET", ts.URL+"/api/schema", "", true)
	var sch struct {
		Version   string      `json:"version"`
		Fields    []fieldJSON `json:"fields"`
		Functions []string    `json:"functions"`
		OS        string      `json:"os"`
		Places    []Place     `json:"places"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sch); err != nil {
		t.Fatal(err)
	}
	if sch.OS != runtime.GOOS || len(sch.Places) == 0 || sch.Places[0].Key != "home" {
		t.Errorf("sistem bilgisi: os=%q yerler=%+v", sch.OS, sch.Places)
	}
	if sch.Version != "test" || len(sch.Fields) < 10 || len(sch.Functions) < 5 {
		t.Errorf("şema: %+v", sch)
	}
}

func jsonPath(p string) string { return strings.ReplaceAll(p, `\`, `\\`) }

func TestQueryStream(t *testing.T) {
	s, ts, root := testServer(t)
	body := `{"query":"SELECT name, size, modified FROM '` + jsonPath(root) + `' WHERE type = 'file' ORDER BY size DESC"}`
	msgs := readStream(t, do(t, s, "POST", ts.URL+"/api/query", body, true))

	var cols []columnJSON
	json.Unmarshal(msgs[0]["columns"], &cols)
	if len(cols) != 3 || cols[1].Name != "size" || !cols[1].IsSize {
		t.Fatalf("sütunlar: %s", msgs[0]["columns"])
	}
	var rows [][]any
	var done map[string]any
	for _, m := range msgs[1:] {
		if raw, ok := m["rows"]; ok {
			var r [][]any
			json.Unmarshal(raw, &r)
			rows = append(rows, r...)
		}
		if raw, ok := m["done"]; ok {
			json.Unmarshal(raw, &done)
		}
	}
	if len(rows) != 3 || rows[0][0] != "rapor.pdf" || rows[0][1] != float64(3<<20) {
		t.Fatalf("satırlar: %v", rows)
	}
	tm, ok := rows[0][2].(map[string]any)
	if !ok || tm["t"] == "" {
		t.Errorf("tarih etiketli gelmeli: %v", rows[0][2])
	}
	if done == nil || done["rows"] != float64(3) || done["truncated"] != false {
		t.Errorf("done: %v", done)
	}
}

func TestQueryErrorHasRunePosition(t *testing.T) {
	s, ts, _ := testServer(t)
	// "ş" is two bytes; the position must be counted in characters.
	body := `{"query":"SELECT name FROM 'ş' WHERE sise > 1"}`
	msgs := readStream(t, do(t, s, "POST", ts.URL+"/api/query", body, true))
	var e errorJSON
	if err := json.Unmarshal(msgs[0]["error"], &e); err != nil {
		t.Fatalf("hata mesajı bekleniyordu: %v", msgs)
	}
	if e.Pos != strings.Index("SELECT name FROM 'x' WHERE sise > 1", "sise") || !strings.Contains(e.Message, "size mi") {
		t.Errorf("hata: %+v", e)
	}

	// Runtime errors (missing folder) have no position
	body = `{"query":"SELECT name FROM '/kesinlikle/olmayan'"}`
	msgs = readStream(t, do(t, s, "POST", ts.URL+"/api/query", body, true))
	var last errorJSON
	json.Unmarshal(msgs[len(msgs)-1]["error"], &last)
	if last.Pos != -1 || !strings.Contains(last.Message, "bulunamadı") {
		t.Errorf("çalışma hatası: %+v", last)
	}
}

func TestRowCap(t *testing.T) {
	s, ts, root := testServer(t)
	s.maxRows = 2
	body := `{"query":"SELECT name FROM '` + jsonPath(root) + `'"}`
	msgs := readStream(t, do(t, s, "POST", ts.URL+"/api/query", body, true))
	var done map[string]any
	total := 0
	for _, m := range msgs {
		if raw, ok := m["rows"]; ok {
			var r [][]any
			json.Unmarshal(raw, &r)
			total += len(r)
		}
		if raw, ok := m["done"]; ok {
			json.Unmarshal(raw, &done)
		}
	}
	if total != 2 || done["truncated"] != true {
		t.Errorf("sınır: %d satır, done=%v", total, done)
	}
}

func TestExport(t *testing.T) {
	s, ts, root := testServer(t)
	q := "SELECT name, size FROM '" + root + "' WHERE ext = 'pdf'"
	resp := do(t, s, "GET", ts.URL+"/api/export?format=excel&q="+url.QueryEscape(q), "", true)
	if !strings.Contains(resp.Header.Get("Content-Disposition"), "attachment") {
		t.Errorf("indirme başlığı yok: %v", resp.Header)
	}
	buf := new(strings.Builder)
	bufio.NewReader(resp.Body).WriteTo(buf)
	if got := buf.String(); got != "\xEF\xBB\xBFname;size\r\nrapor.pdf;3145728\r\n" {
		t.Errorf("excel: %q", got)
	}
	if resp := do(t, s, "GET", ts.URL+"/api/export?format=xml&q=x", "", true); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("geçersiz biçim: %d", resp.StatusCode)
	}
}

func TestReveal(t *testing.T) {
	s, ts, root := testServer(t)
	var revealed string
	s.reveal = func(p string) error { revealed = p; return nil }

	target := filepath.Join(root, "rapor.pdf")
	body, _ := json.Marshal(map[string]string{"path": target})
	if resp := do(t, s, "POST", ts.URL+"/api/reveal", string(body), true); resp.StatusCode != http.StatusOK || revealed != target {
		t.Errorf("reveal: %d %q", resp.StatusCode, revealed)
	}
	body, _ = json.Marshal(map[string]string{"path": filepath.Join(root, "yok.txt")})
	if resp := do(t, s, "POST", ts.URL+"/api/reveal", string(body), true); resp.StatusCode != http.StatusNotFound {
		t.Errorf("olmayan dosya: %d", resp.StatusCode)
	}
}

func TestStaticFilesAreEmbedded(t *testing.T) {
	s, ts, _ := testServer(t)
	for _, f := range []string{"app.js", "style.css", "icon.svg"} {
		resp := do(t, s, "GET", ts.URL+"/static/"+f, "", false)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: %d", f, resp.StatusCode)
		}
	}
}

func TestCheck(t *testing.T) {
	s, ts, root := testServer(t)
	check := func(q string) map[string]any {
		body, _ := json.Marshal(map[string]string{"query": q})
		resp := do(t, s, "POST", ts.URL+"/api/check", string(body), true)
		var out map[string]any
		json.NewDecoder(resp.Body).Decode(&out)
		return out
	}
	if out := check("SELECT name, size FROM '" + root + "' WHERE size > 1 MB"); out["ok"] != true {
		t.Errorf("geçerli sorgu: %v", out)
	}
	out := check("SELECT nmae FROM '" + root + "'")
	if e, _ := out["error"].(map[string]any); e == nil || e["pos"] != float64(7) {
		t.Errorf("hata konumu: %v", out)
	}
	out = check("SELECT name FROM '" + filepath.Join(root, "yok") + "'")
	if e, _ := out["error"].(map[string]any); e == nil || !strings.Contains(e["message"].(string), "bulunamadı") {
		t.Errorf("olmayan klasör: %v", out)
	}
	if resp := do(t, s, "POST", ts.URL+"/api/check", `{"query":"x"}`, false); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("oturumsuz: %d", resp.StatusCode)
	}
}
