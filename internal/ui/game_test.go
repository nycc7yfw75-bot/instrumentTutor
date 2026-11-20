package ui

import (
	"path/filepath"
	"testing"

	"github.com/S95F/musicTutor/internal/practice"
	"github.com/S95F/musicTutor/internal/progress"
)

func tempStore(t *testing.T, name string) *progress.Store {
	t.Helper()
	st, err := progress.OpenFile(filepath.Join(t.TempDir(), "progress.json"))
	if err != nil {
		t.Fatal(err)
	}
	if name != "" {
		if _, err := st.CreateProfile(name); err != nil {
			t.Fatal(err)
		}
	}
	return st
}

// playPass plays every note of the app's piece, hitting the first hits of them.
func playPass(a *App, hits int) {
	a.startPass(0)
	var rs []practice.NoteResult
	for i, ev := range a.sc.Events() {
		v := practice.VerdictMiss
		if i < hits {
			v = practice.VerdictHit
		}
		rs = append(rs, practice.NoteResult{Event: ev, Verdict: v, Matched: v != practice.VerdictMiss, Timed: v != practice.VerdictMiss})
	}
	a.gameResults(rs)
	a.game.passSecs = 20
}

func TestACalibratedPassEarnsXPAndIsSaved(t *testing.T) {
	a := newApp(t, 12)
	st := tempStore(t, "Player")
	a.SetProgress(st, progress.SongID(a.sc), "Riff", "riff.gtab", true)
	a.live = true
	playPass(a, 12)
	a.endPass(a.eng.PosTick())
	if _, xp, _ := st.Active(); xp.TotalXP <= 0 {
		t.Fatalf("a clean, calibrated pass earned %d XP", xp.TotalXP)
	}
	if len(a.game.toasts) != 1 {
		t.Errorf("want one XP toast waiting, got %v", a.game.toasts)
	}
	if pct, ok := a.game.lastSummary.AccuracyPct(); !ok || pct != 100 {
		t.Errorf("the pass shows %d%% (shown %v), want 100%%", pct, ok)
	}
}

func TestUncalibratedOrWaitModePlayIsUnscored(t *testing.T) {
	for _, tc := range []struct {
		name             string
		calibrated, wait bool
		want             string
	}{
		{"uncalibrated", false, false, "Unscored — calibrate to earn XP"},
		{"wait mode", true, true, "Unscored — wait mode is practice"},
	} {
		a := newApp(t, 12)
		st := tempStore(t, "Player")
		a.SetProgress(st, progress.SongID(a.sc), "Riff", "riff.gtab", tc.calibrated)
		a.live, a.wait = true, tc.wait
		if got := a.unscoredReason(); got != tc.want {
			t.Errorf("%s: HUD says %q, want %q", tc.name, got, tc.want)
		}
		playPass(a, 12)
		a.endPass(a.eng.PosTick())
		if _, xp, _ := st.Active(); xp.TotalXP != 0 {
			t.Errorf("%s: unscored play earned %d XP", tc.name, xp.TotalXP)
		}
	}
}

func TestLeavingThePieceFilesThePassInProgress(t *testing.T) {
	a := newApp(t, 12)
	st := tempStore(t, "Player")
	a.SetProgress(st, progress.SongID(a.sc), "Riff", "riff.gtab", true)
	a.live = true
	playPass(a, 12)
	a.Close()
	if _, xp, _ := st.Active(); xp.TotalXP <= 0 {
		t.Error("leaving mid-pass lost the pass")
	}
	a.Close() // a second close files nothing twice and does not panic
}

func TestNoNameYetMeansNothingIsSavedAndTheHUDAsks(t *testing.T) {
	a := newApp(t, 12)
	st := tempStore(t, "")
	a.SetProgress(st, progress.SongID(a.sc), "Riff", "riff.gtab", true)
	a.live = true
	if reason, _ := a.gameHUDReason(); reason != "Add your name to start saving your progress" {
		t.Errorf("HUD says %q", reason)
	}
	playPass(a, 12)
	a.endPass(a.eng.PosTick()) // must not panic or create a profile
	if !st.NeedsProfile() {
		t.Error("a pass created a profile without a name")
	}
}

func TestNamePromptSavesAndRenames(t *testing.T) {
	st := tempStore(t, "")
	p := NewNamePrompt(nil, st)
	p.name = "   "
	if err := p.save(); err != nil || p.err == "" {
		t.Errorf("an empty name: err %v, message %q; want it refused with a message", err, p.err)
	}
	p.name = "Aaditya"
	if err := p.save(); err != errQuit {
		t.Fatalf("saving a name returned %v, want the prompt to close", err)
	}
	p = NewNamePrompt(nil, st)
	if p.name != "Aaditya" {
		t.Errorf("rename prompt starts with %q, want the current name", p.name)
	}
	p.name = "Ady"
	p.save()
	if name, _, _ := st.Active(); name != "Ady" {
		t.Errorf("after rename the player is %q", name)
	}
}

func TestSettingsProfileRowFitsOnScreen(t *testing.T) {
	sh := NewShell(Services{Prefs: &settingsFakePrefs{}, Audio: newSettingsAudio(), Progress: tempStore(t, "Player")}, nil)
	s := NewSettings(sh)
	items := s.items()
	if items[0].text != "PROFILE" { // the backend note moved to the header
		t.Errorf("the Profile section should come first, got %q", items[0].text)
	}
	if s.rows[0] != srProfile {
		t.Errorf("first row is %v, want the profile row", s.rows[0])
	}
	last := items[len(items)-1]
	if bottom := pageBottom() - 2*settingsLineH; last.y+settingsRowH > bottom {
		t.Errorf("Settings runs to y=%.0f, into the config line at %.0f", last.y+settingsRowH, bottom)
	}
}
