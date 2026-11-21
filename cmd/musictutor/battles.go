package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/S95F/musicTutor/internal/appconfig"
	"github.com/S95F/musicTutor/internal/battle"
	"github.com/S95F/musicTutor/internal/score"
	"github.com/S95F/musicTutor/internal/score/textfmt"
	"github.com/S95F/musicTutor/internal/scoring"
	"github.com/S95F/musicTutor/internal/ui"
)

// Battles (BATTLES.md): the landing page's Battles door opens the setup;
// Start battle opens the battle screen on the song, through the same
// practice view a gig uses.

// battleDeps is what the battle screens need from the app.
func (o *shellOpener) battleDeps() ui.BattleDeps {
	return ui.BattleDeps{
		Songs: o.battleSongs,
		Riffs: battleRiffs,
		LoadScore: func(path string) (*score.Score, error) {
			sc, _, err := load(path)
			return sc, err
		},
		Calibrated: o.rigCalibrated,
		Open: func(path string, track int) (*ui.App, error) {
			app, _, err := o.open(path, &gigOpen{track: track})
			return app, err
		},
		Close:    o.CloseCurrent,
		Settings: func() { o.showSettings(nil) },
	}
}

// showBattleSetup opens a new battle's setup.
func (o *shellOpener) showBattleSetup() {
	bs := ui.NewBattleSetup(o.shell, o.store, o.battleDeps())
	bs.SetStart(func(s battle.Setup, ghosts []ui.GhostEntry) { o.startBattle(s, ghosts) })
	o.shell.Show(bs)
}

// startBattle opens the battle screen over the setup.
func (o *shellOpener) startBattle(s battle.Setup, ghosts []ui.GhostEntry) {
	scr, err := ui.NewBattleScreen(o.shell, o.store, o.battleDeps(), s, ghosts)
	if err != nil {
		return
	}
	scr.SetRematch(func() { o.startBattle(s, ghosts) })
	scr.SetNewBattle(func() {}) // the setup is underneath: leaving returns to it
	o.shell.Show(scr)
}

// rigCalibrated reports the saved devices' timing calibrated with enough
// confidence to score (the shared rig, §3).
func (o *shellOpener) rigCalibrated() bool {
	in, out := o.prefs.Devices()
	if in == "" {
		return false
	}
	conf, ok := o.prefs.confidenceFor(in, out)
	_, hasOff := o.prefs.offsetFor(in, out)
	return ok && hasOff && conf >= scoring.CredibleConfidence
}

// battleSongs are "Your songs": the recent pieces and the library, each
// file once.
func (o *shellOpener) battleSongs() []ui.PieceInfo {
	seen := map[string]bool{}
	var out []ui.PieceInfo
	add := func(p ui.PieceInfo) {
		k := strings.ToLower(filepath.Clean(p.Path))
		if seen[k] || p.Problem != "" {
			return
		}
		seen[k] = true
		out = append(out, p)
	}
	for _, path := range o.prefs.Recents() {
		if _, err := os.Stat(path); err == nil {
			add(ui.PieceInfo{Path: path, Name: filepath.Base(path)})
		}
	}
	if lib, err := (pieceLibrary{}).Scan(); err == nil {
		for _, p := range lib {
			add(p)
		}
	}
	return out
}

// battleRiffs are the battle-riff pack in the pieces folder's battles/,
// easiest first (§7).
func battleRiffs() []ui.PieceInfo {
	dir, err := appconfig.PiecesDir()
	if err != nil {
		return nil
	}
	dir = filepath.Join(dir, "battles")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []ui.PieceInfo
	rank := map[string]int{}
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".gtab") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		sc, err := textfmt.ParseFile(path)
		if err != nil {
			continue
		}
		d := scoring.DifficultyOf(sc, firstPlayable(sc))
		out = append(out, ui.PieceInfo{Path: path, Name: e.Name(), Title: sc.Title, Summary: describePiece(sc), Difficulty: d.String()})
		rank[path] = int(d)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if rank[out[i].Path] != rank[out[j].Path] {
			return rank[out[i].Path] < rank[out[j].Path]
		}
		return out[i].Name < out[j].Name
	})
	return out
}
