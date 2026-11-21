package ui

import (
	"math"
	"testing"
)

func TestSpringSettlesWithoutOvershoot(t *testing.T) {
	s := spring{x: 0}
	peak := 0.0
	for i := 0; i < 60; i++ { // half a second at 120 fps
		peak = math.Max(peak, s.step(1, 1.0/120))
	}
	if peak > 1+1e-9 {
		t.Errorf("the spring overshot to %.4f; it must not bounce", peak)
	}
	if s.x < 0.98 {
		t.Errorf("after 500ms the spring is at %.3f, want settled (≥ 0.98)", s.x)
	}
}

func TestPixelStepsHoldThreeFrames(t *testing.T) {
	for _, c := range []struct {
		t    float64
		n    int
		want int
	}{{0, 3, 0}, {0.049, 3, 0}, {0.051, 3, 1}, {0.149, 3, 2}, {0.2, 3, 3}, {-1, 2, 0}} {
		if got := stepIndex(c.t, c.n); got != c.want {
			t.Errorf("stepIndex(%.3f, %d) = %d, want %d", c.t, c.n, got, c.want)
		}
	}
}

func TestReducedMotionShowsEndStatesAtOnce(t *testing.T) {
	SetReduceMotion(true)
	defer SetReduceMotion(false)
	var p presence
	p.set(true)
	p.step(motionDt())
	if p.value != 1 {
		t.Errorf("an overlay opened to %.2f in one frame; reduced motion wants it whole", p.value)
	}
	var x xpBarView
	x.aim(scoringXPHalfLevel(), false)
	x.step()
	if x.shown != x.to {
		t.Errorf("the XP bar is at %.2f of %.2f after one frame", x.shown, x.to)
	}
	gh := ghost{start: 0, offTicks: 10, fadeLen: 300}
	if a := ghostAlphaAt(gh, 200); a != ghostAlpha {
		t.Errorf("a ghost faded to %.2f under reduced motion; it should stay, then go at the 2-bar mark", a)
	}
	if a := ghostAlphaAt(gh, 400); a != 0 {
		t.Errorf("a ghost past its 2 bars is still at %.2f", a)
	}
}

type settingsMotionPrefs struct {
	settingsTrimPrefs
	reduce bool
}

func (p *settingsMotionPrefs) ReduceMotion() bool      { return p.reduce }
func (p *settingsMotionPrefs) SetReduceMotion(on bool) { p.reduce = on }

func TestReduceMotionRowSavesAndEverythingStillFits(t *testing.T) {
	defer SetReduceMotion(false)
	prefs := &settingsMotionPrefs{}
	sh := NewShell(Services{Prefs: prefs, Audio: settingsTapAudio{settingsFakeAudio: newSettingsAudio()}, Progress: tempStore(t, "P")}, settingsStubScreen{})
	s := NewSettings(sh)
	s.focusRow(srMotion)
	s.activate()
	if !prefs.reduce || !ReduceMotion() {
		t.Fatal("Enter on the reduce-motion row did not turn it on and save it")
	}
	items := s.items()
	last := items[len(items)-1]
	if last.y+settingsRowH > screenH-68 {
		t.Errorf("with every row showing, Settings runs to y=%.0f, into the footer at %.0f", last.y+settingsRowH, screenH-68)
	}
}

// scoringXPHalfLevel is a total halfway into level 1.
func scoringXPHalfLevel() int { return 50 }

// A closed overlay that was never opened must not flash on its first
// frames (it used to "fade out" from fully shown).
func TestANeverOpenedOverlayStaysHidden(t *testing.T) {
	var p presence
	p.step(0.016)
	if p.visible() {
		t.Errorf("a never-opened overlay shows at %.2f", p.value)
	}
	p.set(true)
	p.step(1)
	p.set(false)
	p.step(0.01)
	if !p.visible() {
		t.Error("a closing overlay should still be fading out")
	}
}
