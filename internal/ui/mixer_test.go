package ui

import (
	"math"
	"testing"
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
	a.onMix = func(m mixArg) { got = m.Master }
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
