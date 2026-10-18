package engine

import (
	"io/fs"
	"testing"
	"time"
)

// fakeDirEntry lets us test how file modes reported by Go on Windows are
// classified without needing OneDrive or junctions on the test machine.
type fakeDirEntry struct {
	name string
	mode fs.FileMode
}

func (f fakeDirEntry) Name() string               { return f.name }
func (f fakeDirEntry) IsDir() bool                { return f.mode.IsDir() }
func (f fakeDirEntry) Type() fs.FileMode          { return f.mode.Type() }
func (f fakeDirEntry) Info() (fs.FileInfo, error) { return fakeInfo(f), nil }

type fakeInfo fakeDirEntry

func (f fakeInfo) Name() string       { return f.name }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeInfo) Sys() any           { return nil }

func TestDirectoryClassification(t *testing.T) {
	cases := []struct {
		desc     string
		mode     fs.FileMode
		descend  bool
		wantType string
	}{
		{"normal klasör", fs.ModeDir, true, "dir"},
		// Go on Windows: OneDrive / cloud placeholder directory
		{"OneDrive klasörü", fs.ModeDir | fs.ModeIrregular, true, "dir"},
		{"sembolik bağlantı", fs.ModeSymlink, false, "link"},
		// Go never sets ModeDir on junctions; it reports them as irregular
		{"junction (Windows)", fs.ModeIrregular, false, ""},
		{"normal dosya", 0, false, "file"},
	}
	for _, c := range cases {
		e := newEntry("kok", fakeDirEntry{name: "x", mode: c.mode}, 1)
		if got := e.IsDir(); got != c.descend {
			t.Errorf("%s: içine girilmeli=%v, alınan %v", c.desc, c.descend, got)
		}
		if c.wantType != "" {
			if got := e.Get("type").S; got != c.wantType {
				t.Errorf("%s: type=%q, beklenen %q", c.desc, got, c.wantType)
			}
		}
	}
}
