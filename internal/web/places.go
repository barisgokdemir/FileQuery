package web

import (
	"os"
	"path/filepath"
	"runtime"
)

// Place is a well-known folder offered by the interface: in example queries,
// and as a quick pick in the query builder.
type Place struct {
	Key  string `json:"key"`  // home, desktop, documents, downloads, pictures, music, videos, temp, drive, root
	Name string `json:"name"` // label shown to the user
	Path string `json:"path"`
}

var placeLabels = map[string]string{
	"home":      "Ev klasörü",
	"desktop":   "Masaüstü",
	"documents": "Belgeler",
	"downloads": "İndirilenler",
	"pictures":  "Resimler",
	"music":     "Müzik",
	"videos":    "Videolar",
	"temp":      "Geçici dosyalar",
	"root":      "Kök dizin",
}

// userFolderKeys is the display order of per-user folders.
var userFolderKeys = []string{"desktop", "documents", "downloads", "pictures", "music", "videos"}

// Places returns the user's real well-known folders that exist on this
// machine. On Windows the paths come from the shell, so OneDrive redirection
// and localized folder names are handled; on Linux the XDG user-dirs file is
// read (Turkish desktops use "İndirilenler", "Belgeler", ...).
func Places() []Place {
	home, _ := os.UserHomeDir()
	var out []Place
	add := func(key, name, path string) {
		if path == "" {
			return
		}
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			return
		}
		for _, p := range out {
			if p.Path == path {
				return
			}
		}
		if name == "" {
			name = placeLabels[key]
		}
		out = append(out, Place{Key: key, Name: name, Path: path})
	}

	add("home", "", home)
	folders := userFolders(home)
	for _, key := range userFolderKeys {
		add(key, "", folders[key])
	}
	add("temp", "", filepath.Clean(os.TempDir()))

	if runtime.GOOS == "windows" {
		for _, d := range drives() {
			add("drive", d[:2], d)
		}
	} else {
		add("root", "", "/")
	}
	return out
}

// defaultUserFolders is the fallback layout (macOS, and Linux without a
// user-dirs file).
func defaultUserFolders(home string) map[string]string {
	return map[string]string{
		"desktop":   filepath.Join(home, "Desktop"),
		"documents": filepath.Join(home, "Documents"),
		"downloads": filepath.Join(home, "Downloads"),
		"pictures":  filepath.Join(home, "Pictures"),
		"music":     filepath.Join(home, "Music"),
		"videos":    filepath.Join(home, videosDirName()),
	}
}

func videosDirName() string {
	if runtime.GOOS == "darwin" {
		return "Movies"
	}
	return "Videos"
}
