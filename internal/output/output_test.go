package output

import (
	"testing"
	"time"
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
