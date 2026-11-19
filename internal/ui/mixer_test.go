package ui

import (
	"math"
	"strings"
	"testing"

	"github.com/S95F/musicTutor/internal/engine"
)

func TestMixerClickSetsTheVolumeUnderThePointer(t *testing.T) {
	a := newAppTracks(t, 2, 1)
	a.toggleMixer()
	rows := a.mixerRows()
	card := a.mixerCard(len(rows))
	// Row 2 is track 1 (master, metronome, then tracks).
	bar := a.mixerBar(a.mixerRowRect(card, 2))
	a.mixerMouse(pointer{x: bar.x + bar.w/4, y: bar.y + 1, down: true, pressed: true}, rows)
	if got := a.eng.Mix().Tracks[0]; math.Abs(got-0.25) > 0.01 {
		t.Fatalf("clicking a quarter along track 1's slider set it to %.2f, want 0.25", got)
	}
	if a.mixerDrag != 2 {
		t.Fatalf("the click did not start a drag (drag row %d)", a.mixerDrag)
	}
	a.mixerMouse(pointer{x: bar.x + bar.w*0.75, y: bar.y + 1, down: true}, rows)
	if got := a.eng.Mix().Tracks[0]; math.Abs(got-0.75) > 0.01 {
		t.Fatalf("dragging to three quarters left the volume at %.2f", got)
	}
}

func TestMixerClickOutsideClosesIt(t *testing.T) {
	a := newAppTracks(t, 1, 1)
	a.toggleMixer()
	a.mixerMouse(pointer{x: 5, y: 5, down: true, pressed: true}, a.mixerRows())
	if a.mixer.open {
		t.Fatal("a click on the dimmed screen left the mixer open")
	}
}

func TestMixerReportsEveryChange(t *testing.T) {
	a := newAppTracks(t, 1, 1)
	var got float64 = -1
	a.onMix = func(m engine.Mix) { got = m.Master }
	a.mixSet(mixerRow{kind: mixMaster}, 0.4)
	if math.Abs(got-0.4) > 1e-9 {
		t.Fatalf("the change callback saw master %.2f, want 0.40", got)
	}
}

func TestPresenceReversesWithoutAJump(t *testing.T) {
	var p presence
	p.set(true)
	for i := 0; i < 6; i++ {
		p.step(1.0 / 60)
	}
	before := p.value
	p.set(false)
	p.step(0)
	if math.Abs(p.value-before) > 0.02 {
		t.Fatalf("closing mid-entrance jumped from %.3f to %.3f", before, p.value)
	}
	for i := 0; i < 60; i++ {
		p.step(1.0 / 60)
	}
	if p.visible() {
		t.Fatalf("a closed overlay is still visible at %.3f", p.value)
	}
}

func TestEnterCurveIsFastFirst(t *testing.T) {
	if v := curveEnter.at(0.25); v < 0.5 {
		t.Errorf("an ease-out entrance is only %.2f in at a quarter of its time", v)
	}
	if v := curveExit.at(0.25); v > 0.25 {
		t.Errorf("an ease-in exit is already %.2f out at a quarter of its time", v)
	}
}

func TestOpeningTheMixerCancelsADrag(t *testing.T) {
	a := newApp(t, 8)
	a.handleMouse(pressAt(a.layout().timeline), false)
	if a.drag != dragSeekTimeline {
		t.Fatal("the press should have started a scrub")
	}
	a.toggleMixer()
	if err := a.Update(); err != nil {
		t.Fatalf("Update with the mixer open: %v", err)
	}
	if a.drag != dragNone {
		t.Error("the mixer took the frame but the scrub survived it")
	}
}

func TestMixerOwnsUpToTracksThatDoNotFit(t *testing.T) {
	a := newAppTracks(t, 15, 1)
	rows := a.mixerRows()
	if len(rows) != mixMaxRows {
		t.Fatalf("the mixer has %d rows, want the cap of %d", len(rows), mixMaxRows)
	}
	if note := a.mixerHeadNote(rows); !strings.Contains(note, "+3 more tracks") {
		t.Errorf("15 tracks with room for 12 says %q; want it to name the 3 left out", note)
	}
	small := newAppTracks(t, 2, 1)
	if note := small.mixerHeadNote(small.mixerRows()); strings.Contains(note, "more") {
		t.Errorf("a mixer with every track shown still claims some are missing: %q", note)
	}
}

func TestTrackNameShowsThePartNotThePerformer(t *testing.T) {
	a := newAppTracks(t, 4, 1)
	a.sc.Tracks[0].Name = "Mark Knopfler | Fender Stratocaster | Lead Guitar"
	a.sc.Tracks[1].Name = "John Illsley | Fender Precision Bass (channel 14)"
	a.sc.Tracks[2].Name = "Rhythm"
	a.sc.Tracks[3].Name = "Odd | "
	for i, want := range []string{"Lead Guitar", "Fender Precision Bass (channel 14)", "Rhythm", "Odd | "} {
		if got := a.trackName(i); got != want {
			t.Errorf("track %d %q shows as %q, want %q", i+1, a.sc.Tracks[i].Name, got, want)
		}
	}
}
