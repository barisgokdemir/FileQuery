//go:build windows

package web

import (
	"syscall"
	"unsafe"
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// Known folder IDs (KNOWNFOLDERID) from the Windows SDK, ShlObj_core.h.
var knownFolders = map[string]guid{
	"desktop":   {0xB4BFCC3A, 0xDB2C, 0x424C, [8]byte{0xB0, 0x29, 0x7F, 0xE9, 0x9A, 0x87, 0xC6, 0x41}},
	"documents": {0xFDD39AD0, 0x238F, 0x46AF, [8]byte{0xAD, 0xB4, 0x6C, 0x85, 0x48, 0x03, 0x69, 0xC7}},
	"downloads": {0x374DE290, 0x123F, 0x4565, [8]byte{0x91, 0x64, 0x39, 0xC4, 0x92, 0x5E, 0x46, 0x7B}},
	"pictures":  {0x33E28130, 0x4E1E, 0x4676, [8]byte{0x83, 0x5A, 0x98, 0x39, 0x5C, 0x3B, 0xC3, 0xBB}},
	"music":     {0x4BD8D571, 0x6D19, 0x48D3, [8]byte{0xBE, 0x97, 0x42, 0x22, 0x20, 0x08, 0x0E, 0x43}},
	"videos":    {0x18989B1D, 0x99B5, 0x455B, [8]byte{0x84, 0x1C, 0xAB, 0x7C, 0x74, 0xE4, 0xDD, 0xFC}},
}

var (
	procSHGetKnownFolderPath = syscall.NewLazyDLL("shell32.dll").NewProc("SHGetKnownFolderPath")
	procCoTaskMemFree        = syscall.NewLazyDLL("ole32.dll").NewProc("CoTaskMemFree")
	procGetLogicalDrives     = syscall.NewLazyDLL("kernel32.dll").NewProc("GetLogicalDrives")
	procGetDriveTypeW        = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDriveTypeW")
)

const (
	driveRemovable = 2
	driveFixed     = 3
	driveRemote    = 4
)

// userFolders asks the Windows shell where the user's folders really are.
// This follows OneDrive "Known Folder Move" and folders moved by the user.
func userFolders(home string) map[string]string {
	out := defaultUserFolders(home)
	for key, id := range knownFolders {
		if p := knownFolderPath(id); p != "" {
			out[key] = p
		}
	}
	return out
}

func knownFolderPath(id guid) string {
	var ptr *uint16
	hr, _, _ := procSHGetKnownFolderPath.Call(uintptr(unsafe.Pointer(&id)), 0, 0, uintptr(unsafe.Pointer(&ptr)))
	if ptr != nil {
		defer procCoTaskMemFree.Call(uintptr(unsafe.Pointer(ptr)))
	}
	if hr != 0 || ptr == nil {
		return ""
	}
	n := 0
	for p := unsafe.Pointer(ptr); *(*uint16)(p) != 0; p = unsafe.Add(p, 2) {
		n++
	}
	return syscall.UTF16ToString(unsafe.Slice(ptr, n))
}

// drives lists local, removable and network drive roots such as C:\.
// CD/DVD drives are skipped: an empty one would make the interface wait.
func drives() []string {
	mask, _, _ := procGetLogicalDrives.Call()
	var out []string
	for i := 0; i < 26; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		p, err := syscall.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		t, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(p)))
		if t == driveFixed || t == driveRemovable || t == driveRemote {
			out = append(out, root)
		}
	}
	return out
}
