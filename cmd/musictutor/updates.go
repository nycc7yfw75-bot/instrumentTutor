package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/ncruces/zenity"

	"github.com/S95F/musicTutor/internal/appconfig"
	"github.com/S95F/musicTutor/internal/progress"
	"github.com/S95F/musicTutor/internal/ui"
	"github.com/S95F/musicTutor/internal/update"
)

// Updates and save safety (ONLINE.md §A6–A7), wired into the shell: a
// check on launch (release builds only), the offer on the landing page,
// and Settings → About & updates.

// updateState is the updater's last word for the About screen.
type updateState struct {
	mu     sync.Mutex
	status string
}

func (u *updateState) set(s string) { u.mu.Lock(); u.status = s; u.mu.Unlock() }
func (u *updateState) get() string  { u.mu.Lock(); defer u.mu.Unlock(); return u.status }

func (p *shellPrefs) UpdateChannel() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cfg.UpdateChannel == update.ChannelBeta {
		return update.ChannelBeta
	}
	return update.ChannelStable
}

func (p *shellPrefs) SetUpdateChannel(ch string) {
	p.mu.Lock()
	p.cfg.UpdateChannel = ch
	p.mu.Unlock()
	_ = p.Save()
}

func (p *shellPrefs) SkipVersion() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cfg.SkipVersion
}

func (p *shellPrefs) SetSkipVersion(v string) {
	p.mu.Lock()
	p.cfg.SkipVersion = v
	p.mu.Unlock()
	_ = p.Save()
}

// updatesEnabled: only an installed release build updates itself.
func updatesEnabled() bool { return !isDevBuild() && repo != "" }

// newUpdater is the updater for this install, or false on a dev build.
func (o *shellOpener) newUpdater() (update.Updater, bool) {
	if !updatesEnabled() {
		return update.Updater{}, false
	}
	cur, err := update.ParseVersion(version)
	if err != nil {
		return update.Updater{}, false
	}
	key, err := update.ParsePublicKey(update.ReleasePublicKey)
	if err != nil {
		return update.Updater{}, false
	}
	exe, err := os.Executable()
	if err != nil {
		return update.Updater{}, false
	}
	cache := ""
	if dir, err := appconfig.Dir(); err == nil {
		cache = filepath.Join(dir, "cache")
	}
	return update.Updater{
		Client:     update.Client{Repo: repo, CacheDir: cache},
		Current:    cur,
		Channel:    o.prefs.UpdateChannel(),
		Key:        key,
		InstallDir: filepath.Dir(exe),
	}, true
}

// wireUpdates sets up the landing page's update side, the notices, and
// the launch check.
func (o *shellOpener) wireUpdates(landing *ui.Landing, store *progress.Store) {
	landing.OnLanded(update.MarkLanded)
	switch {
	case os.Getenv(update.RolledBackEnv) != "":
		landing.SetNotice("The update wouldn't start, so Headliner went back to the version you had. Your saves are safe.")
	case store != nil && store.Quarantined() != "":
		landing.SetNotice("Your progress file was damaged and set aside. Settings → About & updates → Restore a backup brings back the last good one.")
	}
	o.landing = landing
	if updatesEnabled() {
		go func() {
			time.Sleep(3 * time.Second) // let the window settle first
			o.checkForUpdate(false)
		}()
	} else {
		o.updates.set("Updates are switched off in dev builds.")
	}
}

// checkForUpdate asks GitHub; when there is something newer (and not
// skipped, unless asked by hand) the landing page offers it.
func (o *shellOpener) checkForUpdate(byHand bool) {
	u, ok := o.newUpdater()
	if !ok {
		o.updates.set("Updates are switched off in dev builds.")
		return
	}
	o.updates.set("Checking…")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r, found, err := u.Check(ctx)
	switch {
	case errors.Is(err, update.ErrOffline):
		o.updates.set("Offline: Headliner will check next time.")
		return
	case err != nil:
		o.updates.set("Could not check: " + err.Error())
		return
	case !found:
		o.updates.set("Up to date (" + version + ").")
		return
	}
	if !byHand && r.Tag == o.prefs.SkipVersion() {
		o.updates.set(r.Tag + " is available (you chose to skip it).")
		return
	}
	o.updates.set(r.Tag + " is available: go back to the main screen to install it.")
	if o.landing == nil {
		return
	}
	o.landing.OfferUpdate(ui.UpdateOffer{
		Version: r.Tag,
		Notes:   r.Body,
		Skip:    func() { o.prefs.SetSkipVersion(r.Tag) },
		Install: func(progress func(string, float64)) error {
			return u.Prepare(context.Background(), r, o.backupSaves("update"), func(step string, done, total int64) {
				frac := 0.0
				if total > 0 {
					frac = float64(done) / float64(total)
				}
				progress(step, frac)
			})
		},
		Restart: func() {
			okFile := filepath.Join(os.TempDir(), "headliner-update-ok.txt")
			if err := update.Handoff(u.InstallDir, okFile); err != nil {
				o.updates.set("Could not restart into the update: " + err.Error())
				return
			}
			o.shell.Quit()
		},
	})
}

// backupSaves returns a backup step for the save folder.
func (o *shellOpener) backupSaves(reason string) func() error {
	return func() error {
		dir, err := appconfig.Dir()
		if err != nil {
			return err
		}
		if o.store != nil {
			_ = o.prefs.Save() // what's on disk is what's backed up
		}
		_, err = appconfig.BackupSaves(dir, reason)
		return err
	}
}

// systemHooks are Settings → About & updates' actions.
func (o *shellOpener) systemHooks() ui.SystemHooks {
	h := ui.SystemHooks{
		Status: o.updates.get,
		Export: func() (string, error) {
			dir, err := appconfig.Dir()
			if err != nil {
				return "", err
			}
			path, err := zenity.SelectFileSave(zenity.Title("Export your progress"), zenity.ConfirmOverwrite(),
				zenity.Filename("Headliner progress "+time.Now().Format("2006-01-02")+".zip"),
				zenity.FileFilter{Name: "Zip files", Patterns: []string{"*.zip"}, CaseFold: true})
			if errors.Is(err, zenity.ErrCanceled) {
				return "", nil
			}
			if err != nil {
				return "", err
			}
			if filepath.Ext(path) == "" {
				path += ".zip"
			}
			return path, appconfig.ExportSaves(dir, path)
		},
		Import: func() error {
			dir, err := appconfig.Dir()
			if err != nil {
				return err
			}
			path, err := zenity.SelectFile(zenity.Title("Import progress"),
				zenity.FileFilter{Name: "Headliner exports", Patterns: []string{"*.zip"}, CaseFold: true})
			if errors.Is(err, zenity.ErrCanceled) {
				return nil
			}
			if err != nil {
				return err
			}
			if err := appconfig.ImportSaves(dir, path); err != nil {
				return err
			}
			o.closeForNewSaves()
			return nil
		},
		OpenFolder: func() {
			if dir, err := appconfig.Dir(); err == nil {
				_ = exec.Command("explorer", dir).Start()
			}
		},
		Backups: func() []ui.BackupEntry {
			dir, err := appconfig.Dir()
			if err != nil {
				return nil
			}
			var out []ui.BackupEntry
			for _, b := range appconfig.Backups(dir) {
				out = append(out, ui.BackupEntry{Name: b.Name, When: b.Time, Reason: b.Reason})
			}
			return out
		},
		Restore: func(name string) error {
			dir, err := appconfig.Dir()
			if err != nil {
				return err
			}
			if err := appconfig.RestoreBackup(dir, name); err != nil {
				return err
			}
			o.closeForNewSaves()
			return nil
		},
	}
	if updatesEnabled() {
		h.Channel = o.prefs.UpdateChannel
		h.SetChannel = o.prefs.SetUpdateChannel
		h.CheckNow = func() { go o.checkForUpdate(true) }
	}
	return h
}

// closeForNewSaves quits after an import or restore, so the saves in
// memory can't be written over the ones just put back.
func (o *shellOpener) closeForNewSaves() {
	o.restored = true
	if o.account != nil && o.account.stop != nil {
		o.account.stop() // never send the replaced saves' old copy
		o.account.stop = nil
	}
	o.prefs.saveMu.Lock()
	o.prefs.frozen = true
	o.prefs.saveMu.Unlock()
	if o.store != nil {
		o.store.Freeze()
	}
	fmt.Fprintln(os.Stderr, "headliner: saves replaced; closing so the next start loads them")
	if o.shell != nil {
		o.shell.Quit()
	}
}
