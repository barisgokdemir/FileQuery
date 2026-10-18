package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/barisgokdemir/FileQuery/internal/engine"
	"github.com/barisgokdemir/FileQuery/internal/query"
)

func TestFormatSize(t *testing.T) {
	cases := map[int64]string{
		0:               "0 B",
		1023:            "1023 B",
		1024:            "1,0 KB",
		1536:            "1,5 KB",
		3 << 20:         "3,0 MB",
		5*1<<30 + 1<<29: "5,5 GB",
		2 << 40:         "2,0 TB",
	}
	for in, want := range cases {
		if got := FormatSize(in); got != want {
			t.Errorf("FormatSize(%d) = %s, want %s", in, got, want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{
		0:                          "0dk",
		90 * time.Minute:           "1sa 30dk",
		3*24*time.Hour + time.Hour: "3g 1sa",
		-2 * time.Hour:             "-2sa",
	}
	for in, want := range cases {
		if got := FormatDuration(in); got != want {
			t.Errorf("FormatDuration(%v) = %s, want %s", in, got, want)
		}
	}
}

func TestExcelCSV(t *testing.T) {
	var buf bytes.Buffer
	w, err := New("excel", &buf)
	if err != nil {
		t.Fatal(err)
	}
	tm, _ := query.ParseTime("2024-03-15 14:30")
	w.Begin([]engine.Column{{Name: "ad"}, {Name: "boyut", IsSize: true}, {Name: "tarih"}, {Name: "oran"}})
	w.Row([]query.Value{query.String("şeker;tuz.txt"), query.Int(2048), query.Time(tm), query.Float(1.5)})
	w.End()
	got := buf.String()
	if !strings.HasPrefix(got, "\xEF\xBB\xBF") {
		t.Fatal("BOM yok")
	}
	want := "ad;boyut;tarih;oran\r\n\"şeker;tuz.txt\";2048;15.03.2024 14:30:00;1,5\r\n"
	if strings.TrimPrefix(got, "\xEF\xBB\xBF") != want {
		t.Errorf("excel csv\n got: %q\nwant: %q", got, want)
	}
}
