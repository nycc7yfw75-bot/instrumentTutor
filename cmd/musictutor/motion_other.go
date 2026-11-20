//go:build !windows

package main

// systemReducesMotion: no system setting to follow off Windows.
func systemReducesMotion() bool { return false }
