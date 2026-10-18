//go:build !windows

package engine

import "testing"

// On Unix a leading dot already makes a file hidden.
func hideOnWindows(*testing.T, string) {}
