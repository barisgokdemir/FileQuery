package web

import (
	"os"
	"runtime"
	"testing"
)

func TestPlacesExistAndStartWithHome(t *testing.T) {
	places := Places()
	if len(places) == 0 || places[0].Key != "home" {
		t.Fatalf("ilk yer ev klasörü olmalı: %+v", places)
	}
	seen := map[string]bool{}
	for _, p := range places {
		info, err := os.Stat(p.Path)
		if err != nil || !info.IsDir() {
			t.Errorf("%s (%s) klasör değil: %v", p.Name, p.Path, err)
		}
		if seen[p.Path] {
			t.Errorf("yinelenen yer: %s", p.Path)
		}
		seen[p.Path] = true
		if p.Name == "" {
			t.Errorf("adsız yer: %+v", p)
		}
	}
	hasRoot := false
	for _, p := range places {
		if p.Key == "drive" || p.Key == "root" {
			hasRoot = true
		}
	}
	if !hasRoot {
		t.Errorf("sürücü veya kök dizin yok: %+v", places)
	}
	if runtime.GOOS == "windows" {
		t.Logf("Windows yerleri: %+v", places)
	}
}
