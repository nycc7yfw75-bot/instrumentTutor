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

	if err := o.switchTrack(path, first, 1); err != nil {
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
	if o.carry != nil {
		t.Error("the carry outlived the re-open it was for")
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
	if err := o.switchTrack(path, latest, 0); err == nil {
		t.Fatal("switching on a deleted file reported success")
	}
	if got := o.trackFor[path]; got != 1 {
		t.Errorf("a failed switch changed the remembered track to %d, want 1 kept", got)
	}
	if o.carry != nil {
		t.Error("a failed switch left a carry waiting for the next open")
	}
}
