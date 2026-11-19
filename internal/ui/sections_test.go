package ui

import (
	"strings"
	"testing"

	"github.com/S95F/musicTutor/internal/score"
)

func appWithSections(t *testing.T) *App {
	t.Helper()
	a := newApp(t, 8) // 8 bars of 3840 ticks
	a.sc.Sections = []score.Section{{Tick: 0, Name: "Intro"}, {Tick: 2 * 3840, Name: "Verse"}, {Tick: 6 * 3840, Name: "Solo"}}
	return a
}

func TestBracketsJumpBetweenSections(t *testing.T) {
	a := appWithSections(t)
	a.jumpSection(1)
	if got := a.eng.PosTick(); got != 2*3840 {
		t.Fatalf("] from the start went to %d, want the Verse at %d", got, 2*3840)
	}
	a.jumpSection(1)
	if got := a.eng.PosTick(); got != 6*3840 {
		t.Fatalf("] from the Verse went to %d, want the Solo", got)
	}
	a.jumpSection(1)
	if got := a.eng.PosTick(); got != 6*3840 || a.bpmMessage() == "" {
		t.Errorf("] in the last section moved to %d without saying why", got)
	}

	a.seekTo(7 * 3840) // a bar into the Solo
	a.jumpSection(-1)
	if got := a.eng.PosTick(); got != 6*3840 {
		t.Fatalf("[ inside the Solo went to %d, want back to its start", got)
	}
	a.jumpSection(-1)
	if got := a.eng.PosTick(); got != 2*3840 {
		t.Fatalf("[ at the Solo's start went to %d, want the Verse before it", got)
	}
}

func TestCaptionNamesTheSection(t *testing.T) {
	a := appWithSections(t)
	a.seekTo(3 * 3840)
	if c := a.positionCaption(); !strings.Contains(c, "Verse") {
		t.Errorf("caption in the Verse reads %q", c)
	}
	plain := newApp(t, 4)
	plain.jumpSection(1)
	if plain.bpmMessage() == "" {
		t.Error("] on a piece without sections should say there are none")
	}
}
