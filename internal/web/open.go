package web

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// OpenBrowser opens url in the user's default browser.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// Reveal shows a file in the system file manager, selected when the
// platform supports it. Files are never opened or executed.
func Reveal(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// explorer.exe exits with code 1 even on success, so only Start errors count.
		cmd = exec.Command("explorer.exe", "/select,", path)
	case "darwin":
		cmd = exec.Command("open", "-R", path)
	default:
		cmd = exec.Command("xdg-open", filepath.Dir(path))
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
