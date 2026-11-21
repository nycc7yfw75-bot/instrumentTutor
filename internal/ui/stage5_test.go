package ui

import "testing"

// Stage 5 (REDESIGN §8) layout checks.

func TestMixerColumnsFitTheWindow(t *testing.T) {
	a := newAppTracks(t, 15, 1)
	rows := a.mixerRows()
	card := a.mixerCard(len(rows))
	if card.x < pgMargin || card.x+card.w > screenW-pgMargin {
		t.Errorf("the mixer card %+v leaves the %.0f px window's margins", card, screenW)
	}
	for i := range rows {
		c := a.mixerRowRect(card, i)
		if c.x < card.x || c.x+c.w > card.x+card.w {
			t.Errorf("column %d at %+v leaves the card", i, c)
		}
	}
	if w := textWSmall("Metronome"); w > mixColW-4 {
		t.Errorf("\"Metronome\" (%.0f px) would be cut in a %.0f px column", w, mixColW)
	}
}

func TestTunerMeterLightsTheRightBlock(t *testing.T) {
	cases := []struct {
		cents float64
		block int
	}{{0, 10}, {-4, 9}, {4, 11}, {-17, 7}, {50, 20}, {-80, 0}}
	for _, c := range cases {
		if got := tunerBlock(c.cents); got != c.block {
			t.Errorf("%+.0f cents lights block %d, want %d", c.cents, got, c.block)
		}
	}
	if tunerBlockCol(10, true) != colHit || tunerBlockCol(10, false) != colClose || tunerBlockCol(2, false) != colMiss {
		t.Error("the tuner's block colours do not follow in tune / close / far")
	}
}

func TestHelpKeycapsSplitTheKeys(t *testing.T) {
	cases := map[string]int{"left / right": 2, "? or F1": 2, "shift+drag, edges": 2, "space": 1, "A / B,  L": 3}
	for keys, n := range cases {
		if got := helpKeycaps(keys); len(got) != n {
			t.Errorf("%q splits into %v, want %d keys", keys, got, n)
		}
	}
}

func TestHelpGroupsFollowTheRedesign(t *testing.T) {
	a := newApp(t, 1)
	var got []string
	for _, g := range helpSections(a.helpRows()) {
		got = append(got, g.Name)
	}
	want := []string{"playback", "navigation", "tracks", "loop", "tempo", "tools", "app"}
	if len(got) != len(want) {
		t.Fatalf("practice help groups %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("group %d is %q, want %q", i, got[i], want[i])
		}
	}
}

func TestHelpTablesFitAtEverySize(t *testing.T) {
	atEachSize(t, func(t *testing.T) {
		a := newApp(t, 4)
		e := newTestEditor()
		for _, rows := range [][]helpBinding{a.helpRows(), e.editorBindings()} {
			f := helpLayout(rows, practiceHelpFootnote)
			if card := helpCard(); f.bottom > card.y+card.h {
				t.Errorf("a help table ends at %.0f, past its card at %.0f", f.bottom, card.y+card.h)
			}
		}
	})
}

func TestPeakHoldHoldsThenFallsInSteps(t *testing.T) {
	var p peakHold
	p.update(10, 0.016)
	p.update(2, 0.5) // held: under 600ms
	if p.block != 10 {
		t.Fatalf("the peak fell to %d inside its 600ms hold", p.block)
	}
	p.update(2, 0.2) // 700ms in: 100ms of falling = 2 blocks
	if p.block != 8 {
		t.Errorf("after 100ms of falling the peak is at %d, want 8 (one block per 50ms)", p.block)
	}
	p.update(12, 0.016)
	if p.block != 12 {
		t.Errorf("a new high left the peak at %d", p.block)
	}
	for i := 0; i < 100; i++ {
		p.update(3, 0.05)
	}
	if p.block != 3 {
		t.Errorf("the peak fell to %d, below the live level 3", p.block)
	}
}

func TestCrossfadeSwapsLabelsOver200ms(t *testing.T) {
	var c crossfade
	c.set("LIVE")
	if k := c.step(0); k != 1 {
		t.Errorf("the first label faded in (%.2f); it should simply be there", k)
	}
	c.set("NO SIGNAL")
	if c.prev != "LIVE" || c.cur != "NO SIGNAL" {
		t.Fatalf("crossfade holds %q -> %q", c.prev, c.cur)
	}
	if k := c.step(0.1); k < 0.45 || k > 0.55 {
		t.Errorf("halfway through the crossfade k = %.2f", k)
	}
	if k := c.step(0.2); k != 1 {
		t.Errorf("after 300ms the new label is at %.2f, want fully in", k)
	}
	defer SetReduceMotion(false)
	SetReduceMotion(true)
	c.set("LIVE")
	if k := c.step(motionDt()); k != 1 {
		t.Errorf("with reduced motion the swap takes time (%.2f)", k)
	}
}

func TestLiveDotPulsesUnlessReduced(t *testing.T) {
	defer SetReduceMotion(false)
	var m chromeMotion
	m.clock = 0.2
	on := m.liveDotOn()
	m.clock = 0.7
	if on == m.liveDotOn() {
		t.Error("the LIVE dot does not change across a second")
	}
	SetReduceMotion(true)
	if !m.liveDotOn() {
		t.Error("with reduced motion the LIVE dot should hold steady and lit")
	}
}

func TestHelpKeyTextNeverRunsIntoItsDescription(t *testing.T) {
	e := newTestEditor()
	for _, b := range e.editorBindings() {
		if helpKeysFit(b.Keys) {
			continue
		}
		for _, l := range helpKeyLines(b.Keys) {
			if w := textW(l); w > helpKeysW {
				t.Errorf("key text %q measures %.0f px, past the %.0f px key column", l, w, helpKeysW)
			}
		}
	}
}

func TestHyphenatedFileNamesKeepEveryWord(t *testing.T) {
	cases := map[string]string{
		`C:\x\little-wing.musicxml`:                        "Little Wing",
		`C:\x\Dire Straits-Sultans Of Swing-10-29-2026.gp`: "Sultans of Swing",
		`C:\x\First steps on guitar.gtab`:                  "First Steps on Guitar",
	}
	for path, want := range cases {
		if got := cleanPieceTitle("", path); got != want {
			t.Errorf("%s is titled %q, want %q", path, got, want)
		}
	}
}
