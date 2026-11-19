package main

import (
	"os"
	"testing"

	"github.com/S95F/musicTutor/internal/appconfig"
)

// TestMain points every test at a throwaway config directory, so a test that
// opens and closes a piece (which saves its setup) can never write to the
// real config.json.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "musictutor-test-config-*")
	if err != nil {
		panic(err)
	}
	os.Setenv(appconfig.EnvConfigDir, dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
