package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"os/exec"
	"unicode/utf16"

	"github.com/S95F/musicTutor/internal/appconfig"
	"github.com/S95F/musicTutor/internal/online"
	"github.com/S95F/musicTutor/internal/progress"
	"github.com/S95F/musicTutor/internal/ui"
)

// Accounts and sync (ONLINE.md Part B), wired into the shell.

const releasesURL = "https://github.com/nycc7yfw75-bot/instrumentTutor/releases/latest"

// accountSide is the online account for this run.
type accountSide struct {
	client *online.Client
	syncer *online.Syncer
	stop   context.CancelFunc
}

// startAccount makes the client and starts the background sync. With no
// project configured or no progress store it does nothing harmful: the
// Account screen then says accounts aren't set up.
func startAccount(store *progress.Store) *accountSide {
	dir, _ := appconfig.Dir()
	a := &accountSide{client: &online.Client{
		Project:     online.LoadProject(dir),
		Store:       online.SystemStore(),
		OpenBrowser: openURL,
	}}
	if store == nil {
		return a
	}
	a.syncer = online.NewSyncer(a.client, store)
	if a.client.Project.Configured() {
		ctx, cancel := context.WithCancel(context.Background())
		a.stop = cancel
		go a.syncer.Run(ctx)
	}
	return a
}

// finish stops the background sync and sends what's left (on exit).
func (a *accountSide) finish() {
	if a == nil || a.stop == nil {
		return
	}
	a.stop()
	if _, ok := a.client.SignedIn(); ok {
		a.syncer.Flush()
	}
}

func (a *accountSide) signedIn() bool {
	if a == nil || !a.client.Project.Configured() {
		return false
	}
	_, ok := a.client.SignedIn()
	return ok
}

// status is the Settings row's value.
func (a *accountSide) status() string {
	switch {
	case a == nil || !a.client.Project.Configured():
		return "not set up in this copy of Headliner"
	case !a.signedIn():
		return "not signed in  ·  sign in to sync and add friends"
	}
	acct, _ := a.client.SignedIn()
	st := ""
	if a.syncer != nil {
		st = a.syncer.Status()
	}
	if st == "" {
		return acct.Email
	}
	return acct.Email + "  ·  " + st
}

func (o *shellOpener) showAccount() {
	if o.shell == nil || o.account == nil {
		return
	}
	o.shell.Show(ui.NewAccountScreen(o.shell, ui.AccountDeps{
		Client: o.account.client, Syncer: o.account.syncer, Store: o.store,
		Copy: copyText, Releases: releasesURL,
	}))
}

// openURL opens a web address in the default browser.
func openURL(u string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
}

// copyText puts text on the Windows clipboard (clip.exe takes UTF-16, so
// names and the dash survive).
func copyText(s string) error {
	var buf bytes.Buffer
	for _, u := range utf16.Encode([]rune(s)) {
		_ = binary.Write(&buf, binary.LittleEndian, u)
	}
	cmd := exec.Command("clip")
	cmd.Stdin = &buf
	return cmd.Run()
}
