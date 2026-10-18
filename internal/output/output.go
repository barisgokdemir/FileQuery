// Package output renders query results as an aligned table, CSV or JSON.
package output

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/barisgokdemir/FileQuery/internal/engine"
	"github.com/barisgokdemir/FileQuery/internal/query"
)

// Writer receives result rows.
type Writer interface {
	Begin(cols []engine.Column) error
	Row(values []query.Value) error
	End() error
}

// New returns a writer for the given format: "table", "csv" or "json".
func New(format string, w io.Writer) (Writer, error) {
	switch strings.ToLower(format) {
	case "table", "tablo", "":
		return &tableWriter{w: w}, nil
	case "csv":
		return &csvWriter{w: csv.NewWriter(w)}, nil
	case "json":
		return &jsonWriter{w: bufio.NewWriter(w)}, nil
	}
	return nil, fmt.Errorf("bilinmeyen çıktı biçimi %q; tablo, csv veya json kullanın", format)
}

// ---------- Table ----------

type tableWriter struct {
	w    io.Writer
	cols []engine.Column
	rows [][]string
}

func (t *tableWriter) Begin(cols []engine.Column) error {
	t.cols = cols
	return nil
}

func (t *tableWriter) Row(values []query.Value) error {
	cells := make([]string, len(values))
	for i, v := range values {
		cells[i] = display(v, t.cols[i].IsSize)
	}
	t.rows = append(t.rows, cells)
	return nil
}

func (t *tableWriter) End() error {
	if len(t.rows) == 0 {
		return nil
	}
	widths := make([]int, len(t.cols))
	rightAlign := make([]bool, len(t.cols))
	for i, c := range t.cols {
		widths[i] = utf8.RuneCountInString(c.Name)
		rightAlign[i] = c.IsSize
	}
	for _, r := range t.rows {
		for i, cell := range r {
			widths[i] = max(widths[i], utf8.RuneCountInString(cell))
		}
	}

	bw := bufio.NewWriter(t.w)
	writeLine := func(cells []string) {
		for i, cell := range cells {
			if i > 0 {
				bw.WriteString("  ")
			}
			pad := strings.Repeat(" ", widths[i]-utf8.RuneCountInString(cell))
			last := i == len(cells)-1
			switch {
			case rightAlign[i]:
				bw.WriteString(pad + cell)
			case last:
				bw.WriteString(cell)
			default:
				bw.WriteString(cell + pad)
			}
		}
		bw.WriteByte('\n')
	}

	header := make([]string, len(t.cols))
	rule := make([]string, len(t.cols))
	for i, c := range t.cols {
		header[i] = c.Name
		rule[i] = strings.Repeat("─", widths[i])
	}
	writeLine(header)
	writeLine(rule)
	for _, r := range t.rows {
		writeLine(r)
	}
	return bw.Flush()
}

// display formats a value for people: sizes as KB/MB/GB, local dates.
func display(v query.Value, isSize bool) string {
	switch v.Kind {
	case query.KindNull:
		return ""
	case query.KindBool:
		if v.B {
			return "evet"
		}
		return "hayır"
	case query.KindInt:
		if isSize {
			return FormatSize(v.I)
		}
		return strconv.FormatInt(v.I, 10)
	case query.KindFloat:
		if isSize {
			return FormatSize(int64(v.F))
		}
		return strings.Replace(strconv.FormatFloat(v.F, 'f', 2, 64), ".", ",", 1)
	case query.KindString:
		return v.S
	case query.KindTime:
		return v.T.Local().Format("2006-01-02 15:04")
	case query.KindDuration:
		return FormatDuration(time.Duration(v.I))
	}
	return v.String()
}

// FormatSize renders bytes the way Windows Explorer does (1024-based) with a
// Turkish decimal comma: 3,4 MB.
func FormatSize(b int64) string {
	if b < 1024 {
		return strconv.FormatInt(b, 10) + " B"
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	f := float64(b)
	i := -1
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	s := strconv.FormatFloat(f, 'f', 1, 64)
	return strings.Replace(s, ".", ",", 1) + " " + units[i]
}

// FormatDuration renders a duration in days, hours and minutes: 3g 4sa 10dk.
func FormatDuration(d time.Duration) string {
	neg := d < 0
	if neg {
		d = -d
	}
	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hours := d / time.Hour
	d -= hours * time.Hour
	mins := d / time.Minute
	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dg", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dsa", hours))
	}
	if mins > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ddk", mins))
	}
	s := strings.Join(parts, " ")
	if neg {
		s = "-" + s
	}
	return s
}

// ---------- CSV ----------

type csvWriter struct {
	w    *csv.Writer
	cols []engine.Column
}

func (c *csvWriter) Begin(cols []engine.Column) error {
	c.cols = cols
	header := make([]string, len(cols))
	for i, col := range cols {
		header[i] = col.Name
	}
	return c.w.Write(header)
}

func (c *csvWriter) Row(values []query.Value) error {
	rec := make([]string, len(values))
	for i, v := range values {
		rec[i] = raw(v)
	}
	return c.w.Write(rec)
}

func (c *csvWriter) End() error {
	c.w.Flush()
	return c.w.Error()
}

// raw formats a value for machines: bytes, RFC 3339 dates, seconds.
func raw(v query.Value) string {
	switch v.Kind {
	case query.KindNull:
		return ""
	case query.KindBool:
		return strconv.FormatBool(v.B)
	case query.KindInt:
		return strconv.FormatInt(v.I, 10)
	case query.KindFloat:
		return strconv.FormatFloat(v.F, 'f', -1, 64)
	case query.KindString:
		return v.S
	case query.KindTime:
		return v.T.Format(time.RFC3339)
	case query.KindDuration:
		return strconv.FormatFloat(time.Duration(v.I).Seconds(), 'f', -1, 64)
	}
	return v.String()
}

// ---------- JSON ----------

type jsonWriter struct {
	w     *bufio.Writer
	cols  []engine.Column
	count int
}

func (j *jsonWriter) Begin(cols []engine.Column) error {
	j.cols = cols
	_, err := j.w.WriteString("[")
	return err
}

func (j *jsonWriter) Row(values []query.Value) error {
	if j.count > 0 {
		j.w.WriteString(",")
	}
	j.count++
	j.w.WriteString("\n  {")
	for i, v := range values {
		if i > 0 {
			j.w.WriteString(", ")
		}
		key, _ := json.Marshal(j.cols[i].Name)
		j.w.Write(key)
		j.w.WriteString(": ")
		val, err := json.Marshal(jsonValue(v))
		if err != nil {
			return err
		}
		j.w.Write(val)
	}
	_, err := j.w.WriteString("}")
	return err
}

func (j *jsonWriter) End() error {
	if j.count > 0 {
		j.w.WriteString("\n")
	}
	j.w.WriteString("]\n")
	return j.w.Flush()
}

func jsonValue(v query.Value) any {
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
		return v.T.Format(time.RFC3339)
	case query.KindDuration:
		return time.Duration(v.I).Seconds()
	}
	return nil
}
