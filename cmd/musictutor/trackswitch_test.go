package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/S95F/musicTutor/internal/appconfig"
	"github.com/S95F/musicTutor/internal/audio"
	"github.com/S95F/musicTutor/internal/engine"
	"github.com/S95F/musicTutor/internal/score"
	"github.com/S95F/musicTutor/internal/ui"
)

func TestTrackSwitchCarriesThePracticeStateAndSurvivesAFailure(t *testing.T) {
	t.Setenv(appconfig.EnvConfigDir, t.TempDir())
	useStubBackend(t, &stubBackend{
		capture:  []audio.DeviceInfo{{ID: "cap-1", Name: "Stub In", Default: true}},
		playback: []audio.DeviceInfo{{ID: "pb-1", Name: "Stub Out", Default: true}},
	})
	// Every live open wires its engine through setEventTap; keep the latest.
	var latest *engine.Engine
	prevTap := setEventTap
	setEventTap = func(eng *engine.Engine, fn func(ev score.NoteEvent, outFrame int64)) {
		if fn != nil {
			latest = eng
		}
		prevTap(eng, fn)
	}
	t.Cleanup(func() { setEventTap = prevTap })

	path := filepath.Join(t.TempDir(), "two.gtab")
	src := `\tempo 120
\time 4/4
0.6.1 | 0.6.1 | 0.6.1 | 0.6.1 |
\track Lead
0.1.1 | 0.1.1 | 0.1.1 | 0.1.1 |
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	prefs := &shellPrefs{}
	prefs.SetDevices("cap-1", "pb-1")
	o := &shellOpener{prefs: prefs}
	o.shell, o.browser = ui.NewBrowserShell(ui.Services{Opener: o, Prefs: prefs, Library: pieceLibrary{}})
	t.Cleanup(o.CloseCurrent)

	scr, _, err := o.Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if got := scr.(*ui.App).Track(); got != 0 {
		t.Fatalf("the piece opened on track %d, want the first", got)
	}
	first := latest
	first.SetTempoScale(0.7)
	first.SetLoop(1920, 5760)
	first.SeekTick(3840)

	if err := o.switchTrack(path, 1); err != nil {
		t.Fatalf("switchTrack: %v", err)
	}
	second := latest
	if second == first {
		t.Fatal("the switch did not open a new engine")
	}
	if got := second.TempoScale(); got != 0.7 {
		t.Errorf("tempo after the switch = %.2f, want the 0.70 practised at", got)
	}
	if got := second.PosTick(); got != 3840 {
		t.Errorf("playhead after the switch = %d, want 3840", got)
	}
	if lo, hi, on := second.Loop(); !on || lo != 1920 || hi != 5760 {
		t.Errorf("loop after the switch = %d..%d on=%v, want 1920..5760", lo, hi, on)
	}
	again, _, err := o.Open(path)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	if got := again.(*ui.App).Track(); got != 1 {
		t.Errorf("re-opening after the switch shows track %d, want the chosen 1", got)
	}

	// A failed switch keeps the old choice and leaves nothing behind.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := o.switchTrack(path, 0); err == nil {
		t.Fatal("switching on a deleted file reported success")
	}
	if s, _ := o.prefs.piece(path); s.Track != 1 {
		t.Errorf("a failed switch changed the remembered track to %d, want 1 kept", s.Track)
	}
}

func TestLeavingAPieceRemembersItsSetup(t *testing.T) {
	t.Setenv(appconfig.EnvConfigDir, t.TempDir())
	useStubBackend(t, &stubBackend{
		capture:  []audio.DeviceInfo{{ID: "cap-1", Name: "Stub In", Default: true}},
		playback: []audio.DeviceInfo{{ID: "pb-1", Name: "Stub Out", Default: true}},
	})
	var latest *engine.Engine
	prevTap := setEventTap
	setEventTap = func(eng *engine.Engine, fn func(ev score.NoteEvent, outFrame int64)) {
		if fn != nil {
			latest = eng
		}
		prevTap(eng, fn)
	}
	t.Cleanup(func() { setEventTap = prevTap })

	path := filepath.Join(t.TempDir(), "two.gtab")
	src := `\tempo 120
\time 4/4
0.6.1 | 0.6.1 | 0.6.1 | 0.6.1 |
\track Lead
0.1.1 | 0.1.1 | 0.1.1 | 0.1.1 |
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	open := func() (*ui.App, *engine.Engine) {
		prefs := &shellPrefs{}
		if c, err := appconfig.Load(); err == nil {
			prefs.cfg = c // a fresh run of the app reads the saved file
		}
		prefs.SetDevices("cap-1", "pb-1")
		o := &shellOpener{prefs: prefs}
		o.shell, o.browser = ui.NewBrowserShell(ui.Services{Opener: o, Prefs: prefs, Library: pieceLibrary{}})
		scr, _, err := o.Open(path)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		t.Cleanup(o.closeAudio)
		return scr.(*ui.App), latest
	}

	prefs := &shellPrefs{}
	prefs.SetDevices("cap-1", "pb-1")
	o := &shellOpener{prefs: prefs}
	o.shell, o.browser = ui.NewBrowserShell(ui.Services{Opener: o, Prefs: prefs, Library: pieceLibrary{}})
	scr, _, err := o.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	app, eng := scr.(*ui.App), latest
	eng.SetTempoScale(0.6)
	eng.SetLoop(1920, 5760)
	eng.SeekTick(3840)
	app.SetMuteState([]bool{true, false}, 0)
	m := eng.Mix()
	m.Click, m.Tracks[0] = 0, 0.3
	eng.SetMix(m)
	o.CloseCurrent() // the player leaves the piece

	app2, eng2 := open() // the app is started again
	if eng2 == eng {
		t.Fatal("re-opening did not build a new engine")
	}
	if got := eng2.TempoScale(); got != 0.6 {
		t.Errorf("tempo came back at %.2f, want 0.60", got)
	}
	if lo, hi, on := eng2.Loop(); !on || lo != 1920 || hi != 5760 {
		t.Errorf("loop came back as %d..%d on=%v, want 1920..5760", lo, hi, on)
	}
	if got := eng2.PosTick(); got != 3840 {
		t.Errorf("playhead came back at %d, want 3840", got)
	}
	if muted, _ := app2.MuteState(); !muted[0] || muted[1] {
		t.Errorf("mutes came back as %v, want track 1 muted", muted)
	}
	if got := eng2.Mix(); got.Click != 0 || got.Tracks[0] != 0.3 {
		t.Errorf("mix came back as %+v, want a silent click and track 1 at 0.30", got)
	}
}
