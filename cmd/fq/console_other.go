//go:build !windows

package main

// startedFromExplorer is only meaningful on Windows.
func startedFromExplorer() bool { return false }
