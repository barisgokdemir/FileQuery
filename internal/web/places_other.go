//go:build !windows

package web

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// xdgKeys maps XDG user-dirs variables to place keys.
var xdgKeys = map[string]string{
	"XDG_DESKTOP_DIR":   "desktop",
	"XDG_DOCUMENTS_DIR": "documents",
	"XDG_DOWNLOAD_DIR":  "downloads",
	"XDG_PICTURES_DIR":  "pictures",
	"XDG_MUSIC_DIR":     "music",
	"XDG_VIDEOS_DIR":    "videos",
}

// userFolders reads ~/.config/user-dirs.dirs on Linux, where localized
// desktops use translated names (İndirilenler, Belgeler, Masaüstü...).
func userFolders(home string) map[string]string {
	out := defaultUserFolders(home)
	if runtime.GOOS == "darwin" {
		return out
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	for k, v := range parseUserDirs(filepath.Join(config, "user-dirs.dirs"), home) {
		out[k] = v
	}
	return out
}

// parseUserDirs reads lines like XDG_DOWNLOAD_DIR="$HOME/İndirilenler".
func parseUserDirs(file, home string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(file)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		key, known := xdgKeys[strings.TrimSpace(name)]
		if !ok || !known {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"`)
		value = strings.Replace(value, "$HOME", home, 1)
		if !filepath.IsAbs(value) {
			continue
		}
		// XDG convention: a directory equal to $HOME means "disabled".
		if filepath.Clean(value) == filepath.Clean(home) {
			continue
		}
		out[key] = filepath.Clean(value)
	}
	return out
}

func drives() []string { return nil }
