package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, size := range map[string]int{"a.pdf": 2048, "b.txt": 10, "alt/c.pdf": 3 << 20} {
		p := filepath.Join(root, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, make([]byte, size), 0o644)
	}
	return root
}

func fq(t *testing.T, stdin string, interactive bool, args ...string) (code int, out, errOut string) {
	t.Helper()
	var o, e bytes.Buffer
	code = run(args, strings.NewReader(stdin), &o, &e, interactive)
	return code, o.String(), e.String()
}

func TestTableOutput(t *testing.T) {
	root := tree(t)
	code, out, errOut := fq(t, "", false, "SELECT name, size FROM '"+root+"' WHERE ext = 'pdf' ORDER BY size DESC")
	if code != 0 {
		t.Fatalf("çıkış kodu %d: %s", code, errOut)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[0], "name") || !strings.HasPrefix(lines[1], "─") {
		t.Fatalf("tablo:\n%s", out)
	}
	if !strings.HasPrefix(lines[2], "c.pdf") || !strings.HasSuffix(lines[2], "3,0 MB") || !strings.HasSuffix(lines[3], "2,0 KB") {
		t.Errorf("satırlar:\n%s", out)
	}
	if !strings.Contains(errOut, "2 sonuç") {
		t.Errorf("özet: %q", errOut)
	}
}

func TestCSVAndJSON(t *testing.T) {
	root := tree(t)
	code, out, errOut := fq(t, "", false, "--csv", "SELECT name, size FROM '"+root+"' WHERE ext = 'pdf' ORDER BY name")
	if code != 0 {
		t.Fatal(errOut)
	}
	recs, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil || len(recs) != 3 || recs[1][0] != "a.pdf" || recs[1][1] != "2048" {
		t.Errorf("csv: %v %v\n%s", recs, err, out)
	}
	if errOut != "" {
		t.Errorf("csv modunda stderr sessiz olmalı: %q", errOut)
	}

	code, out, _ = fq(t, "", false, "--json", "SELECT name, size, type = 'file' AS dosya FROM '"+root+"' WHERE ext = 'txt'")
	var rows []map[string]any
	if code != 0 || json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 1 {
		t.Fatalf("json: %d\n%s", code, out)
	}
	if rows[0]["name"] != "b.txt" || rows[0]["size"] != float64(10) || rows[0]["dosya"] != true {
		t.Errorf("json satırı: %v", rows[0])
	}

	_, out, _ = fq(t, "", false, "--json", "SELECT name FROM '"+root+"' WHERE ext = 'yok'")
	if strings.TrimSpace(out) != "[\n]" && strings.TrimSpace(out) != "[]" {
		t.Errorf("boş json dizisi bekleniyordu: %q", out)
	}
}

func TestParseErrorShowsCaret(t *testing.T) {
	code, out, errOut := fq(t, "", false, "SELECT nmae FROM '.'")
	if code != 1 || out != "" {
		t.Errorf("çıkış kodu %d, çıktı %q", code, out)
	}
	lines := strings.Split(errOut, "\n")
	if len(lines) < 3 || lines[1] != "       ^" || !strings.Contains(lines[2], "name mi demek istediniz") {
		t.Errorf("hata biçimi:\n%s", errOut)
	}
}

func TestMissingFolder(t *testing.T) {
	code, _, errOut := fq(t, "", false, "SELECT name FROM '/kesinlikle/olmayan/klasor'")
	if code != 1 || !strings.Contains(errOut, "bulunamadı") {
		t.Errorf("%d %q", code, errOut)
	}
}

func TestQueryFromStdin(t *testing.T) {
	root := tree(t)
	code, out, errOut := fq(t, "SELECT name FROM '"+root+"' WHERE name = 'b.txt'", false, "--csv")
	if code != 0 || out != "name\nb.txt\n" {
		t.Errorf("%d %q %q", code, out, errOut)
	}
}

func TestREPL(t *testing.T) {
	root := tree(t)
	script := strings.Join([]string{
		".bicim csv",
		"SELECT name FROM '" + root + "' \\",
		"  WHERE ext = 'txt'",
		"SELECT nme FROM '.'",
		".bilinmeyen",
		".cikis",
		"SELECT name FROM '" + root + "'", // .cikis sonrasında çalışmamalı
	}, "\n")
	code, out, errOut := fq(t, script, true)
	if code != 0 {
		t.Fatalf("çıkış kodu %d", code)
	}
	if !strings.Contains(out, "çıktı biçimi: csv") || !strings.Contains(out, "name\nb.txt\n") {
		t.Errorf("repl çıktısı:\n%s", out)
	}
	if strings.Contains(out, "a.pdf") {
		t.Error(".cikis sonrasındaki sorgu çalıştı")
	}
	if !strings.Contains(errOut, "name mi demek istediniz") || !strings.Contains(errOut, "bilinmeyen komut") {
		t.Errorf("repl hataları:\n%s", errOut)
	}
}

func TestFlags(t *testing.T) {
	if code, out, _ := fq(t, "", false, "--version"); code != 0 || !strings.HasPrefix(out, "fq ") {
		t.Errorf("--version: %d %q", code, out)
	}
	if code, out, _ := fq(t, "", false, "--alanlar"); code != 0 || !strings.Contains(out, "modified") || !strings.Contains(out, "@resim") || !strings.Contains(out, "gif") {
		t.Errorf("--alanlar: %d", code)
	}
	if code, _, errOut := fq(t, "", false, "--format", "xml", "SELECT name FROM '.'"); code != 2 || !strings.Contains(errOut, "xml") {
		t.Errorf("geçersiz biçim: %d %q", code, errOut)
	}
	if code, _, errOut := fq(t, "", false, "--web", "SELECT name FROM '.'"); code != 2 || !strings.Contains(errOut, "arayüzde") {
		t.Errorf("--web ile sorgu: %d %q", code, errOut)
	}
	if code, _, errOut := fq(t, "", false, "-h"); code != 0 || !strings.Contains(errOut, "Kullanım") {
		t.Errorf("-h: %d", code)
	}
}

func TestGroupThousands(t *testing.T) {
	for n, want := range map[int64]string{0: "0", 999: "999", 1000: "1.000", 1234567: "1.234.567"} {
		if got := group(n); got != want {
			t.Errorf("group(%d) = %s", n, got)
		}
	}
}
