package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/S95F/musicTutor/internal/appconfig"
	"github.com/S95F/musicTutor/internal/ui"
)

// What a release build is stamped with (ONLINE.md §A4), through
//
//	-ldflags "-X main.version=v1.2.0 -X main.channel=stable -X main.repo=<owner>/<name>"
//
// A plain `go build` leaves version empty: a dev build, with its own save
// folder (§A3) and no updater.
var (
	version = ""
	channel = ""
	repo    = ""
)

func isDevBuild() bool { return version == "" }

// displayVersion is the version as the About screen shows it.
func displayVersion() string {
	if isDevBuild() {
		return "dev"
	}
	return version
}

// prepareFolders picks the save folder for this kind of build and, the
// first time, fills it with a checked copy of the earlier one.
func prepareFolders() {
	appconfig.SetDevBuild(isDevBuild())
	ui.Build = ui.BuildInfo{Version: displayVersion(), Channel: channel, Repo: repo, Dev: isDevBuild()}
	if from, err := appconfig.Prepare(); err != nil {
		fmt.Fprintln(os.Stderr, "headliner: could not copy your files to the new folder; using the old one:", err)
	} else if from != "" {
		fmt.Fprintln(os.Stderr, "headliner: copied your files from", from)
	}
}

// isInfoCommand is a command that touches no files.
func isInfoCommand(arg string) bool {
	switch arg {
	case "version", "-version", "--version", "help", "-h", "--help":
		return true
	}
	return false
}

// logUpdate appends the update helper's outcome to a log in the temp
// folder (the helper has no window to show it).
func logUpdate(msg string) {
	f, err := os.OpenFile(filepath.Join(os.TempDir(), "headliner-update.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s  %s\n", time.Now().Format(time.RFC3339), msg)
}
