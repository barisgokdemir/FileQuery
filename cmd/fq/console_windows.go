//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var procGetConsoleProcessList = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleProcessList")

// startedFromExplorer reports whether fq was launched by double-clicking it.
// In that case Windows creates a new console that only this process uses;
// when run from cmd or PowerShell the shell shares the console too.
func startedFromExplorer() bool {
	var pids [4]uint32
	n, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	return n == 1
}
