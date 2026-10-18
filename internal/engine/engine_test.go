package engine

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/barisgokdemir/FileQuery/internal/query"
)

// testTree builds:
//
//	root/
//	  rapor.pdf          3 MB   2024-01-10
//	  Fatura-2024.PDF    100 KB 2024-06-01
//	  notlar.txt         1 KB   2023-05-05
//	  .gizli             10 B
//	  belgeler/
//	    sunum.pptx       5 MB   2024-03-01
//	    eski/
//	      arsiv.zip      20 MB  2022-01-01
//	      eski.txt       2 KB   2021-01-01
func testTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := []struct {
		path string
		size int
		date string
	}{
		{"rapor.pdf", 3 << 20, "2024-01-10"},
		{"Fatura-2024.PDF", 100 << 10, "2024-06-01"},
		{"notlar.txt", 1 << 10, "2023-05-05"},
		{".gizli", 10, "2024-01-01"},
		{"belgeler/sunum.pptx", 5 << 20, "2024-03-01"},
		{"belgeler/eski/arsiv.zip", 20 << 20, "2022-01-01"},
		{"belgeler/eski/eski.txt", 2 << 10, "2021-01-01"},
	}
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f.path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, make([]byte, f.size), 0o644); err != nil {
			t.Fatal(err)
		}
		tm, _ := query.ParseTime(f.date)
		if err := os.Chtimes(p, tm, tm); err != nil {
			t.Fatal(err)
		}
	}
	if runtime.GOOS == "windows" {
		hideOnWindows(t, filepath.Join(root, ".gizli"))
	}
	return root
}

func runQuery(t *testing.T, src string, opts Options) ([][]query.Value, Stats) {
	t.Helper()
	q, err := query.Parse(src)
	if err != nil {
		if qe, ok := err.(*query.Error); ok {
			t.Fatalf("\n%s", qe.Format(src))
		}
		t.Fatal(err)
	}
	var rows [][]query.Value
	stats, err := Run(context.Background(), q, opts, func(r []query.Value) error {
		rows = append(rows, r)
		return nil
	})
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	return rows, stats
}

func names(rows [][]query.Value) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r[0].S
	}
	return out
}

func sorted(s []string) []string {
	c := append([]string(nil), s...)
	sort.Strings(c)
	return c
}

func q(root, rest string) string {
	return "SELECT name FROM '" + root + "' " + rest
}

func TestFilters(t *testing.T) {
	root := testTree(t)
	cases := []struct {
		where string
		want  []string
	}{
		{"WHERE ext = 'pdf'", []string{"Fatura-2024.PDF", "rapor.pdf"}},
		{"WHERE ext IN ('pdf', 'pptx')", []string{"Fatura-2024.PDF", "rapor.pdf", "sunum.pptx"}},
		{"WHERE size > 2 MB", []string{"arsiv.zip", "rapor.pdf", "sunum.pptx"}},
		{"WHERE size BETWEEN 1 KB AND 100KB", []string{"Fatura-2024.PDF", "eski.txt", "notlar.txt"}},
		{"WHERE name LIKE 'fatura%'", []string{"Fatura-2024.PDF"}},
		{"WHERE name LIKE '%.txt'", []string{"eski.txt", "notlar.txt"}},
		{"WHERE name LIKE 'e___.txt'", []string{"eski.txt"}},
		{"WHERE name MATCHES '^[a-z]+\\.(pdf|zip)$'", []string{"arsiv.zip", "rapor.pdf"}},
		{"WHERE modified >= '2024-01-01' AND type = 'file' AND NOT hidden", []string{"Fatura-2024.PDF", "rapor.pdf", "sunum.pptx"}},
		{"WHERE modified < '01.01.2023'", []string{"arsiv.zip", "eski.txt"}},
		{"WHERE type = 'dir'", []string{"belgeler", "eski"}},
		{"WHERE size IS NULL", []string{"belgeler", "eski"}},
		{"WHERE hidden", []string{".gizli"}},
		{"WHERE depth = 3", []string{"arsiv.zip", "eski.txt"}},
		{"WHERE ext = 'txt' OR size > 10 MB", []string{"arsiv.zip", "eski.txt", "notlar.txt"}},
		{"WHERE LOWER(name) = 'fatura-2024.pdf'", []string{"Fatura-2024.PDF"}},
		{"WHERE YEAR(modified) = 2022", []string{"arsiv.zip"}},
		{"WHERE dir LIKE '%eski'", []string{"arsiv.zip", "eski.txt"}},
	}
	for _, c := range cases {
		rows, _ := runQuery(t, q(root, c.where), Options{})
		got := sorted(names(rows))
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("%s\n got: %v\nwant: %v", c.where, got, c.want)
		}
	}
}

func TestDepth(t *testing.T) {
	root := testTree(t)
	rows, _ := runQuery(t, q(root, "DEPTH 1 WHERE type = 'file' AND NOT hidden"), Options{})
	want := []string{"Fatura-2024.PDF", "notlar.txt", "rapor.pdf"}
	if got := sorted(names(rows)); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("DEPTH 1: %v", got)
	}
	rows, _ = runQuery(t, q(root, "DEPTH 2 WHERE name = 'arsiv.zip'"), Options{})
	if len(rows) != 0 {
		t.Errorf("DEPTH 2 üçüncü seviyeye inmemeli: %v", names(rows))
	}
}

func TestOrderAndLimit(t *testing.T) {
	root := testTree(t)
	rows, _ := runQuery(t, "SELECT name, size FROM '"+root+"' WHERE type = 'file' ORDER BY size DESC LIMIT 3", Options{})
	got := names(rows)
	if strings.Join(got, ",") != "arsiv.zip,sunum.pptx,rapor.pdf" {
		t.Errorf("ORDER BY size DESC LIMIT 3: %v", got)
	}
	if rows[0][1] != query.Int(20<<20) {
		t.Errorf("boyut: %v", rows[0][1])
	}

	rows, _ = runQuery(t, "SELECT name, modified AS tarih FROM '"+root+"' WHERE ext = 'txt' ORDER BY tarih", Options{})
	if strings.Join(names(rows), ",") != "eski.txt,notlar.txt" {
		t.Errorf("ORDER BY alias: %v", names(rows))
	}

	// NULL (directories have no size) sorts last
	rows, _ = runQuery(t, "SELECT name FROM '"+root+"' ORDER BY size", Options{})
	last := names(rows)[len(rows)-2:]
	if strings.Join(sorted(last), ",") != "belgeler,eski" {
		t.Errorf("NULL sona gelmeli: %v", names(rows))
	}
}

func TestLimitStopsEarly(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 50; i++ {
		d := filepath.Join(root, "k"+strings.Repeat("x", i%5), "alt"+string(rune('a'+i%26)))
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "f.txt"), nil, 0o644)
	}
	rows, stats := runQuery(t, q(root, "WHERE ext = 'txt' LIMIT 3"), Options{})
	if len(rows) != 3 || stats.Rows != 3 {
		t.Errorf("LIMIT 3 -> %d satır", len(rows))
	}
}

func TestStreamingAndSortByPathReturnSameRows(t *testing.T) {
	root := testTree(t)
	streamed, _ := runQuery(t, "SELECT path FROM '"+root+"'", Options{})
	ordered, _ := runQuery(t, "SELECT path FROM '"+root+"'", Options{SortByPath: true})
	if len(streamed) != 9 || len(ordered) != 9 {
		t.Fatalf("9 giriş bekleniyordu: %d / %d", len(streamed), len(ordered))
	}
	o := names(ordered)
	if !sort.StringsAreSorted(o) {
		t.Errorf("SortByPath sıralı değil: %v", o)
	}
	if strings.Join(sorted(names(streamed)), ",") != strings.Join(o, ",") {
		t.Error("akış ve sıralı çıktı farklı satırlar döndürdü")
	}
}

func TestAggregates(t *testing.T) {
	root := testTree(t)
	rows, _ := runQuery(t, "SELECT COUNT(*), SUM(size), MAX(size), MIN(modified) FROM '"+root+"' WHERE type = 'file'", Options{})
	if len(rows) != 1 {
		t.Fatalf("tek satır bekleniyordu: %v", rows)
	}
	r := rows[0]
	total := int64(3<<20 + 100<<10 + 1<<10 + 10 + 5<<20 + 20<<20 + 2<<10)
	if r[0] != query.Int(7) || r[1] != query.Int(total) || r[2] != query.Int(20<<20) {
		t.Errorf("toplamlar: %v", r)
	}
	if r[3].T.Year() != 2021 {
		t.Errorf("MIN(modified): %v", r[3])
	}

	rows, _ = runQuery(t, "SELECT ext, COUNT(*) AS adet, SUM(size) FROM '"+root+"' WHERE type = 'file' AND ext != '' GROUP BY ext ORDER BY adet DESC, ext", Options{})
	var got []string
	for _, r := range rows {
		got = append(got, query.Fold(r[0].S)+"="+r[1].String())
	}
	// PDF and pdf fall into the same group
	want := "pdf=2,txt=2,gizli=1,pptx=1,zip=1"
	if strings.Join(got, ",") != want {
		t.Errorf("GROUP BY ext\n got: %s\nwant: %s", strings.Join(got, ","), want)
	}

	rows, _ = runQuery(t, "SELECT YEAR(modified), COUNT(*) FROM '"+root+"' WHERE type = 'file' GROUP BY YEAR(modified) ORDER BY YEAR(modified)", Options{})
	if len(rows) != 4 || rows[0][0] != query.Int(2021) || rows[3][0] != query.Int(2024) || rows[3][1] != query.Int(4) {
		t.Errorf("GROUP BY YEAR: %v", rows)
	}

	// Aggregates with no matching rows still return one row
	rows, _ = runQuery(t, "SELECT COUNT(*), SUM(size) FROM '"+root+"' WHERE ext = 'yok'", Options{})
	if len(rows) != 1 || rows[0][0] != query.Int(0) || !rows[0][1].IsNull() {
		t.Errorf("boş toplama: %v", rows)
	}
}

func TestRelativeDates(t *testing.T) {
	root := t.TempDir()
	recent := filepath.Join(root, "yeni.log")
	old := filepath.Join(root, "eski.log")
	os.WriteFile(recent, nil, 0o644)
	os.WriteFile(old, nil, 0o644)
	past := time.Now().Add(-30 * 24 * time.Hour)
	os.Chtimes(old, past, past)

	rows, _ := runQuery(t, q(root, "WHERE modified > NOW() - 7d"), Options{})
	if strings.Join(names(rows), ",") != "yeni.log" {
		t.Errorf("son 7 gün: %v", names(rows))
	}
	rows, _ = runQuery(t, q(root, "WHERE modified < TODAY() - 2w"), Options{})
	if strings.Join(names(rows), ",") != "eski.log" {
		t.Errorf("2 haftadan eski: %v", names(rows))
	}
}

func TestPlatformFields(t *testing.T) {
	root := testTree(t)
	rows, _ := runQuery(t, "SELECT name, owner, mode, links, accessed, readonly FROM '"+root+"' WHERE name = 'rapor.pdf'", Options{})
	if len(rows) != 1 {
		t.Fatal("rapor.pdf bulunamadı")
	}
	r := rows[0]
	if r[1].Kind != query.KindString || r[1].S == "" {
		t.Errorf("owner boş: %v", r[1])
	}
	if r[2].Kind != query.KindString || len(r[2].S) != 9 {
		t.Errorf("mode: %v", r[2])
	}
	if r[3] != query.Int(1) {
		t.Errorf("links: %v", r[3])
	}
	if r[4].Kind != query.KindTime {
		t.Errorf("accessed: %v", r[4])
	}
	if r[5] != query.Bool(false) {
		t.Errorf("readonly: %v", r[5])
	}
}

func TestRuntimeErrors(t *testing.T) {
	root := testTree(t)
	src := "SELECT name FROM '" + root + "' WHERE name + 1 > 2"
	parsed, err := query.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Run(context.Background(), parsed, Options{}, func([]query.Value) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "metin") {
		t.Errorf("tip hatası bekleniyordu: %v", err)
	}
}

func TestResolveRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("FQ_TEST_DIR", root)
	got, err := ResolveRoot("%FQ_TEST_DIR%")
	if err != nil || got != root {
		t.Errorf("%%VAR%% açılımı: %q %v", got, err)
	}
	if _, err := ResolveRoot(filepath.Join(root, "yok")); err == nil || !strings.Contains(err.Error(), "bulunamadı") {
		t.Errorf("olmayan klasör: %v", err)
	}
	f := filepath.Join(root, "dosya.txt")
	os.WriteFile(f, nil, 0o644)
	if _, err := ResolveRoot(f); err == nil || !strings.Contains(err.Error(), "klasör değil") {
		t.Errorf("dosya: %v", err)
	}
	home, _ := os.UserHomeDir()
	if got, err := ResolveRoot("~"); err != nil || got != home {
		t.Errorf("~: %q %v", got, err)
	}
}

func TestSymlinksAreNotFollowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows'ta sembolik bağlantı oluşturmak yönetici izni gerektirebilir")
	}
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "a"), 0o755)
	os.WriteFile(filepath.Join(root, "a", "x.txt"), nil, 0o644)
	// a/dongu -> root: following it would loop forever
	if err := os.Symlink(root, filepath.Join(root, "a", "dongu")); err != nil {
		t.Skip(err)
	}
	rows, _ := runQuery(t, q(root, "WHERE name = 'x.txt'"), Options{})
	if len(rows) != 1 {
		t.Errorf("x.txt bir kez bulunmalı: %d", len(rows))
	}
	rows, _ = runQuery(t, q(root, "WHERE type = 'link'"), Options{})
	if len(rows) != 1 || rows[0][0].S != "dongu" {
		t.Errorf("link: %v", names(rows))
	}
}

func TestUnreadableDirectoriesAreCounted(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("izin testi Windows'ta ve root olarak anlamsız")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "kilitli")
	os.MkdirAll(locked, 0o755)
	os.WriteFile(filepath.Join(locked, "gizli.txt"), nil, 0o644)
	os.Chmod(locked, 0)
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	_, stats := runQuery(t, q(root, ""), Options{})
	if stats.Denied != 1 || len(stats.DeniedPaths) != 1 {
		t.Errorf("erişilemeyen klasör sayılmadı: %+v", stats.ScanStats)
	}
}
