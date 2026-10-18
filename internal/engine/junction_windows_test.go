//go:build windows

package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// A junction pointing back at its own parent would make a naive scan loop
// forever. It must be listed as a link and never descended into.
func TestJunctionsAreNotFollowed(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "a"), 0o755)
	os.WriteFile(filepath.Join(root, "a", "x.txt"), nil, 0o644)

	junction := filepath.Join(root, "a", "dongu")
	out, err := exec.Command("cmd", "/c", "mklink", "/J", junction, root).CombinedOutput()
	if err != nil {
		t.Skipf("junction oluşturulamadı: %v %s", err, out)
	}

	rows, _ := runQuery(t, q(root, "WHERE name = 'x.txt'"), Options{})
	if len(rows) != 1 {
		t.Errorf("x.txt bir kez bulunmalı, %d kez bulundu (junction takip edildi)", len(rows))
	}
	rows, _ = runQuery(t, q(root, "WHERE name = 'dongu'"), Options{})
	if len(rows) != 1 {
		t.Fatalf("junction listelenmeli: %d", len(rows))
	}
	rows, _ = runQuery(t, "SELECT type FROM '"+root+"' WHERE name = 'dongu'", Options{})
	if rows[0][0].S != "link" {
		t.Errorf("junction type = %q, beklenen 'link'", rows[0][0].S)
	}
}
