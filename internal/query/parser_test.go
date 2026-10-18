package query

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func mustParse(t *testing.T, src string) *Query {
	t.Helper()
	q, err := Parse(src)
	if err != nil {
		var qe *Error
		if errors.As(err, &qe) {
			t.Fatalf("Parse(%q):\n%s", src, qe.Format(src))
		}
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return q
}

func TestParseFullQuery(t *testing.T) {
	q := mustParse(t, `SELECT name, size AS boyut, modified FROM 'C:\Users\baris\Documents', 'D:\Yedek' DEPTH 3
		WHERE ext IN ('pdf', 'docx') AND size > 2 MB AND modified >= '2024-01-01'
		ORDER BY size DESC, name LIMIT 20;`)

	if len(q.Items) != 3 || q.Items[1].Alias != "boyut" || q.Items[1].Name() != "boyut" {
		t.Errorf("select listesi hatalı: %+v", q.Items)
	}
	if got := strings.Join(q.Sources, "|"); got != `C:\Users\baris\Documents|D:\Yedek` {
		t.Errorf("kaynaklar: %s", got)
	}
	if q.Depth != 3 || q.Limit != 20 {
		t.Errorf("depth=%d limit=%d", q.Depth, q.Limit)
	}
	if len(q.OrderBy) != 2 || !q.OrderBy[0].Desc || q.OrderBy[1].Desc {
		t.Errorf("order by: %+v", q.OrderBy)
	}
	want := "((ext IN ('pdf', 'docx') AND (size > 2 MB)) AND (modified >= '2024-01-01'))"
	if got := q.Where.String(); got != want {
		t.Errorf("where\n got: %s\nwant: %s", got, want)
	}
}

func TestKeywordsAreCaseInsensitive(t *testing.T) {
	q := mustParse(t, `select Name from '/tmp' where EXT = 'txt' order by NAME desc limit 5`)
	if q.Items[0].Expr.(*Field).Name != "name" || q.Limit != 5 || !q.OrderBy[0].Desc {
		t.Errorf("beklenmeyen sonuç: %+v", q)
	}
}

func TestStarSelect(t *testing.T) {
	q := mustParse(t, `SELECT * FROM '.'`)
	if !q.Star || q.Where != nil || q.Depth != 0 {
		t.Errorf("%+v", q)
	}
}

func TestUnitLiterals(t *testing.T) {
	cases := []struct {
		src  string
		want Value
	}{
		{"500 KB", Int(500 * 1024)},
		{"500KB", Int(500 * 1024)},
		{"1.5 GB", Int(1610612736)},
		{"2mb", Int(2 * 1024 * 1024)},
		{"10 B", Int(10)},
		{"7d", Duration(7 * 24 * time.Hour)},
		{"12h", Duration(12 * time.Hour)},
		{"30m", Duration(30 * time.Minute)},
		{"2w", Duration(14 * 24 * time.Hour)},
		{"3g", Duration(3 * 24 * time.Hour)},
		{"42", Int(42)},
		{"0.5", Float(0.5)},
		{"-1", Int(-1)},
	}
	for _, c := range cases {
		q := mustParse(t, "SELECT name FROM '.' WHERE size > "+c.src)
		lit := q.Where.(*Binary).R.(*Literal)
		if lit.Value != c.want {
			t.Errorf("%s: got %v (%s), want %v", c.src, lit.Value, lit.Value.Kind, c.want)
		}
	}
}

func TestPredicates(t *testing.T) {
	cases := map[string]string{
		`name LIKE 'fatura%'`:                         "name LIKE 'fatura%'",
		`name NOT LIKE '%.tmp'`:                       "name NOT LIKE '%.tmp'",
		`name MATCHES '^app-\d+\.log$'`:               `name MATCHES '^app-\d+\.log$'`,
		`ext NOT IN ('tmp', 'log')`:                   "ext NOT IN ('tmp', 'log')",
		`size BETWEEN 1 MB AND 10 MB`:                 "size BETWEEN 1 MB AND 10 MB",
		`size NOT BETWEEN 1 MB AND 10 MB`:             "size NOT BETWEEN 1 MB AND 10 MB",
		`NOT hidden AND readonly`:                     "(NOT hidden AND readonly)",
		`a_or_b`:                                      "",
		`modified > NOW() - 7d`:                       "(modified > (NOW() - 7d))",
		`size IS NULL`:                                "size IS NULL",
		`created IS NOT NULL`:                         "created IS NOT NULL",
		`(ext = 'jpg' OR ext = 'png') AND size > 1MB`: "(((ext = 'jpg') OR (ext = 'png')) AND (size > 1MB))",
		`LOWER(name) = 'readme.md'`:                   "(LOWER(name) = 'readme.md')",
		`group = 'staff'`:                             "(group = 'staff')",
		`depth <= 2`:                                  "(depth <= 2)",
		`name <> 'x'`:                                 "(name != 'x')",
	}
	for where, want := range cases {
		if want == "" {
			continue
		}
		q := mustParse(t, "SELECT name FROM '.' WHERE "+where)
		if got := q.Where.String(); got != want {
			t.Errorf("WHERE %s\n got: %s\nwant: %s", where, got, want)
		}
	}
}

func TestAndBindsTighterThanOr(t *testing.T) {
	q := mustParse(t, `SELECT name FROM '.' WHERE ext = 'a' OR ext = 'b' AND size > 1`)
	or, ok := q.Where.(*Binary)
	if !ok || or.Op != "OR" {
		t.Fatalf("en dıştaki operatör OR olmalı: %s", q.Where)
	}
	if and, ok := or.R.(*Binary); !ok || and.Op != "AND" {
		t.Fatalf("AND sağ tarafta gruplanmalı: %s", q.Where)
	}
}

func TestQuotedStrings(t *testing.T) {
	q := mustParse(t, `SELECT name FROM 'C:\Program Files' WHERE name = 'O''Brien.txt' OR name = "çift tırnak"`)
	if q.Sources[0] != `C:\Program Files` {
		t.Errorf("yol: %q", q.Sources[0])
	}
	if !strings.Contains(q.Where.String(), "'O''Brien.txt'") || !strings.Contains(q.Where.String(), "'çift tırnak'") {
		t.Errorf("metin: %s", q.Where)
	}
}

func TestComments(t *testing.T) {
	mustParse(t, "SELECT name -- sadece ad\nFROM '.' -- geçerli klasör\nWHERE size > 1 MB")
}

func TestGroupBy(t *testing.T) {
	q := mustParse(t, `SELECT ext, COUNT(*), SUM(size) AS toplam FROM '.' GROUP BY ext ORDER BY toplam DESC`)
	if len(q.GroupBy) != 1 || q.GroupBy[0].String() != "ext" {
		t.Errorf("group by: %v", q.GroupBy)
	}
	if q.Items[1].Name() != "count(*)" {
		t.Errorf("sütun adı: %s", q.Items[1].Name())
	}
	mustParse(t, `SELECT COUNT(*), SUM(size), MAX(modified) FROM '.'`)
	mustParse(t, `SELECT YEAR(modified), COUNT(*) FROM '.' GROUP BY YEAR(modified) ORDER BY COUNT(*) DESC`)
	mustParse(t, `SELECT ext, SUM(size) FROM '.' GROUP BY ext ORDER BY SUM(size)`)
}

func TestErrors(t *testing.T) {
	cases := []struct {
		src      string
		wantPos  int
		contains string
	}{
		{`SELEC name FROM '.'`, 0, "SELECT ile başlamalı"},
		{`SELECT nmae FROM '.'`, 7, "name mi demek istediniz"},
		{`SELECT name FROM C:\`, 17, "tırnak içinde"},
		{`SELECT name FROM '.' WHERE size > > 1`, 34, "beklenmeyen"},
		{`SELECT name FROM '.' WHERE name = 'abc`, 34, "kapanmamış"},
		{`SELECT name FROM '.' WHERE size > 5 XB`, 36, "beklenmeyen"},
		{`SELECT name FROM '.' WHERE size > 5XB`, 34, "bilinmeyen birim"},
		{`SELECT name FROM '.' LIMIT 0`, 27, "pozitif"},
		{`SELECT name FROM '.' WHERE COUNT(*) > 1`, 27, "WHERE içinde"},
		{`SELECT name, COUNT(*) FROM '.'`, 7, "GROUP BY'da yok"},
		{`SELECT ext, size FROM '.' GROUP BY ext`, 12, "GROUP BY'da yok"},
		{`SELECT * FROM '.' GROUP BY ext`, 0, "SELECT *"},
		{`SELECT name FROM '.' WHERE name MATCHES '(['`, 40, "düzenli ifade"},
		{`SELECT name FROM '.' WHERE LOWR(name) = 'a'`, 27, "LOWER mi demek istediniz"},
		{`SELECT name FROM '.' WHERE LOWER() = 'a'`, 27, "1 argüman"},
		{`SELECT name FROM '.' ORDER BY name extra`, 35, "sorgu burada bitmeliydi"},
		{`SELECT name FROM '.' WHERE ext IN 'pdf'`, 34, "( veya hazır liste bekleniyordu"},
		{`SELECT name FROM ''`, 17, "boş olamaz"},
	}
	for _, c := range cases {
		_, err := Parse(c.src)
		var qe *Error
		if !errors.As(err, &qe) {
			t.Errorf("%s: konumlu hata bekleniyordu, alınan: %v", c.src, err)
			continue
		}
		if qe.Pos != c.wantPos || !strings.Contains(qe.Msg, c.contains) {
			t.Errorf("%s\n got: pos=%d %q\nwant: pos=%d ...%q...\n%s", c.src, qe.Pos, qe.Msg, c.wantPos, c.contains, qe.Format(c.src))
		}
	}
}

func TestErrorFormatShowsCaret(t *testing.T) {
	src := `SELECT name FROM '.' WHERE sise > 1`
	_, err := Parse(src)
	var qe *Error
	if !errors.As(err, &qe) {
		t.Fatal(err)
	}
	got := qe.Format(src)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 || lines[0] != src || strings.Index(lines[1], "^") != strings.Index(src, "sise") {
		t.Errorf("biçim hatalı:\n%s", got)
	}
	if !strings.Contains(lines[2], "size mi demek istediniz") {
		t.Errorf("öneri yok: %s", lines[2])
	}
}

func TestParseTime(t *testing.T) {
	for _, s := range []string{"2024-03-15", "2024-03-15 14:30", "15.03.2024", "15.03.2024 14:30:00", "2024-03-15T14:30:00"} {
		tm, ok := ParseTime(s)
		if !ok || tm.Year() != 2024 || tm.Month() != 3 || tm.Day() != 15 {
			t.Errorf("%s -> %v %v", s, tm, ok)
		}
	}
	if _, ok := ParseTime("15/03/2024"); ok {
		t.Error("desteklenmeyen biçim kabul edildi")
	}
}

func TestCompare(t *testing.T) {
	mar15, _ := ParseTime("2024-03-15")
	cases := []struct {
		a, b Value
		cmp  int
		ok   bool
	}{
		{Int(5), Int(3), 1, true},
		{Int(5), Float(5.0), 0, true},
		{String("Rapor.PDF"), String("rapor.pdf"), 0, true},
		{String("İŞ.txt"), String("iş.txt"), 0, true},
		{Time(mar15), String("2024-03-01"), 1, true},
		{String("15.03.2024"), Time(mar15), 0, true},
		{Time(mar15), String("dün"), 0, false},
		{Int(1), String("1"), 0, false},
		{Null, Int(1), 0, false},
		{Bool(false), Bool(true), -1, true},
	}
	for _, c := range cases {
		cmp, ok := Compare(c.a, c.b)
		if cmp != c.cmp || ok != c.ok {
			t.Errorf("Compare(%v, %v) = %d,%v; want %d,%v", c.a, c.b, cmp, ok, c.cmp, c.ok)
		}
	}
}

func TestArith(t *testing.T) {
	base, _ := ParseTime("2024-03-15")
	v, err := Arith("-", Time(base), Duration(7*24*time.Hour))
	if err != nil || !v.T.Equal(base.AddDate(0, 0, -7)) {
		t.Errorf("tarih - süre: %v %v", v, err)
	}
	if _, err := Arith("+", String("a"), Int(1)); err == nil {
		t.Error("metin + sayı hata vermeliydi")
	}
	if v, _ := Arith("+", Int(2), Int(3)); v != Int(5) {
		t.Errorf("2+3 = %v", v)
	}
}

func TestExtSets(t *testing.T) {
	q := mustParse(t, `SELECT name FROM '.' WHERE ext IN @resim AND ext NOT IN ('ico', 'svg')`)
	and := q.Where.(*Binary)
	in := and.L.(*In)
	if len(in.List) != len(ExtSets[0].Exts) || in.Not {
		t.Fatalf("@resim açılmadı: %d öğe", len(in.List))
	}
	has := map[string]bool{}
	for _, e := range in.List {
		has[e.(*Literal).Value.S] = true
	}
	for _, ext := range []string{"jpg", "png", "gif", "heic", "ico", "cr2"} {
		if !has[ext] {
			t.Errorf("@resim içinde %s yok", ext)
		}
	}

	q = mustParse(t, `SELECT name FROM '.' WHERE ext NOT IN @VIDEO`)
	if in := q.Where.(*In); !in.Not || len(in.List) != len(ExtSets[1].Exts) {
		t.Errorf("NOT IN @VIDEO: %v", q.Where)
	}

	q = mustParse(t, `SELECT name FROM '.' WHERE ext IN (@ses, @image, 'pdf')`)
	if in := q.Where.(*In); len(in.List) != len(ExtSets[2].Exts)+len(ExtSets[0].Exts)+1 {
		t.Errorf("karma liste: %d", len(in.List))
	}
	for _, alias := range []string{"@arşiv", "@archive", "@Belge", "@documents", "@müzik"} {
		mustParse(t, `SELECT name FROM '.' WHERE ext IN `+alias)
	}
}

func TestExtSetsHaveNoDuplicates(t *testing.T) {
	for _, s := range ExtSets {
		seen := map[string]bool{}
		for _, e := range s.Exts {
			if seen[e] || e != strings.ToLower(e) || strings.HasPrefix(e, ".") {
				t.Errorf("@%s: hatalı veya yinelenen uzantı %q", s.Name, e)
			}
			seen[e] = true
		}
	}
}

func TestExtSetErrors(t *testing.T) {
	cases := []struct{ src, contains string }{
		{`SELECT name FROM '.' WHERE ext IN @resm`, "@resim mi demek istediniz"},
		{`SELECT name FROM '.' WHERE ext IN @xyzabc`, "kullanılabilenler: @resim, @video"},
		{`SELECT name FROM '.' WHERE ext = @resim`, "sadece IN ile"},
		{`SELECT name FROM '.' WHERE ext IN @`, "liste adı bekleniyordu"},
	}
	for _, c := range cases {
		_, err := Parse(c.src)
		if err == nil || !strings.Contains(err.Error(), c.contains) {
			t.Errorf("%s: %v", c.src, err)
		}
	}
}
