//go:build !windows && !darwin

package web

import (
	"os"
	"path/filepath"
	"testing"
)

func TestXDGUserDirsTurkish(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, ".config")
	for _, d := range []string{".config", "Masaüstü", "Belgeler", "İndirilenler"} {
		os.MkdirAll(filepath.Join(home, d), 0o755)
	}
	os.WriteFile(filepath.Join(config, "user-dirs.dirs"), []byte(`# xdg-user-dirs-update tarafından yazıldı
XDG_DESKTOP_DIR="$HOME/Masaüstü"
XDG_DOCUMENTS_DIR="$HOME/Belgeler"
XDG_DOWNLOAD_DIR="$HOME/İndirilenler"
XDG_MUSIC_DIR="$HOME/"
XDG_TEMPLATES_DIR="$HOME/Şablonlar"
`), 0o644)

	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	got := userFolders(home)
	want := map[string]string{
		"desktop":   filepath.Join(home, "Masaüstü"),
		"documents": filepath.Join(home, "Belgeler"),
		"downloads": filepath.Join(home, "İndirilenler"),
		"music":     filepath.Join(home, "Music"), // $HOME/ means disabled -> default
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, beklenen %q", k, got[k], v)
		}
	}

	byKey := map[string]Place{}
	for _, p := range Places() {
		byKey[p.Key] = p
	}
	if byKey["downloads"].Path != want["downloads"] || byKey["downloads"].Name != "İndirilenler" {
		t.Errorf("Places indirilenler: %+v", byKey["downloads"])
	}
	if _, ok := byKey["music"]; ok {
		t.Errorf("olmayan klasör listelenmemeli: %+v", byKey["music"])
	}
}
