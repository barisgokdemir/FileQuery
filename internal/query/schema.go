package query

import (
	"sort"
	"strings"
)

// FieldInfo describes a column that queries can read.
type FieldInfo struct {
	Name string
	Kind Kind
	Desc string
}

// Schema lists every field in display order.
var Schema = []FieldInfo{
	{"name", KindString, "dosya veya klasör adı (uzantı dahil)"},
	{"ext", KindString, "uzantı, noktasız ve küçük harf: pdf, docx"},
	{"path", KindString, "tam yol"},
	{"dir", KindString, "bulunduğu klasör"},
	{"type", KindString, "'file', 'dir' veya 'link'"},
	{"size", KindInt, "boyut (bayt); birimle yazılabilir: 500 KB, 2 MB, 1.5 GB. Klasörlerde boş"},
	{"modified", KindTime, "içeriğin son değiştiği tarih; kopyalanan, indirilen veya zipten çıkan dosyalarda orijinal tarih korunur"},
	{"created", KindTime, "dosyanın bu bilgisayarda oluştuğu tarih; indirilen/kopyalanan dosyalarda geliş tarihi (Windows, macOS)"},
	{"accessed", KindTime, "son erişim tarihi"},
	{"depth", KindInt, "aranan klasöre göre derinlik; doğrudan içindekiler 1"},
	{"hidden", KindBool, "gizli mi (Windows özniteliği; Linux/macOS'ta adı '.' ile başlayanlar)"},
	{"readonly", KindBool, "salt okunur mu"},
	{"system", KindBool, "sistem dosyası mı (Windows)"},
	{"owner", KindString, "sahibi (Windows: DOMAIN\\kullanıcı, Linux/macOS: kullanıcı)"},
	{"group", KindString, "grubu (Linux/macOS)"},
	{"mode", KindString, "izinler, ör. rw-r--r--"},
	{"links", KindInt, "sabit bağlantı (hardlink) sayısı"},
}

// DefaultColumns are shown for SELECT *.
var DefaultColumns = []string{"name", "size", "modified", "dir"}

var fieldSet = func() map[string]FieldInfo {
	m := map[string]FieldInfo{}
	for _, f := range Schema {
		m[f.Name] = f
	}
	return m
}()

// LookupField returns the field definition for a (case-insensitive) name.
func LookupField(name string) (FieldInfo, bool) {
	f, ok := fieldSet[strings.ToLower(name)]
	return f, ok
}

var aggregates = map[string]bool{"COUNT": true, "SUM": true, "MIN": true, "MAX": true, "AVG": true}

// functions maps scalar function names to their argument count.
var functions = map[string]int{
	"NOW": 0, "TODAY": 0, "LOWER": 1, "UPPER": 1, "LEN": 1, "YEAR": 1, "MONTH": 1, "DAY": 1,
}

// IsAggregateName reports whether name is an aggregate function.
func IsAggregateName(name string) bool { return aggregates[strings.ToUpper(name)] }

// FunctionNames lists the scalar and aggregate functions, sorted.
func FunctionNames() []string {
	var out []string
	for n := range functions {
		out = append(out, n)
	}
	for n := range aggregates {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// suggest returns the closest known name, if it is close enough to be a typo.
func suggest(word string, candidates []string) string {
	best, bestDist := "", 3
	for _, c := range candidates {
		if d := levenshtein(strings.ToLower(word), strings.ToLower(c)); d < bestDist {
			best, bestDist = c, d
		}
	}
	return best
}

func fieldNames() []string {
	out := make([]string, len(Schema))
	for i, f := range Schema {
		out[i] = f.Name
	}
	return out
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
