package ui

import (
	"fmt"
	"testing"
)

func appWithWarnings(t *testing.T, n int) *App {
	t.Helper()
	a := newApp(t, 4)
	var w []string
	for i := 0; i < n; i++ {
		w = append(w, fmt.Sprintf("warning %d: bar %d has a note the importer could not place, so it was dropped from the tab and will not be scored", i+1, i+3))
	}
	a.SetImportWarnings(w)
	return a
}

func TestEveryImportWarningIsListedAndNumbered(t *testing.T) {
	a := appWithWarnings(t, 32)
	nums := 0
	for _, l := range a.warnListLines() {
		if l.num != "" {
			nums++
		}
		if textW(l.text) > wlCardW-2*wlPadX-wlNumW {
			t.Errorf("line runs past the card: %q", l.text)
		}
	}
	if nums != 32 {
		t.Fatalf("the list numbers %d warnings, want all 32", nums)
	}
}

func TestWarnListScrollStaysInRange(t *testing.T) {
	a := appWithWarnings(t, 32)
	total, rows := len(a.warnListLines()), a.warnListRows()
	if total <= rows {
		t.Fatalf("fixture too short to scroll: %d lines, %d rows", total, rows)
	}
	a.scrollWarnList(-5)
	if a.warnScroll != 0 {
		t.Errorf("scrolling up from the top left it at %d", a.warnScroll)
	}
	a.scrollWarnList(10_000)
	if a.warnScroll != total-rows {
		t.Errorf("scrolling past the end stopped at %d, want the last page at %d", a.warnScroll, total-rows)
	}
}

func TestBannerClickOpensTheListWhenThereAreWarnings(t *testing.T) {
	a := appWithWarnings(t, 3)
	a.SetLiveWarning("imported with 3 warnings")
	a.syncLive()
	if more := len(a.warnItems()) - 1; more != 3 {
		t.Errorf("the banner offers %d more, want the 3 import warnings", more)
	}
	a.handleMouse(pressAt(bannerRect()), false)
	if !a.warnList.open {
		t.Fatal("a click on the banner did not open the warnings list")
	}

	plain := newApp(t, 4)
	plain.SetLiveWarning("timing is not calibrated")
	plain.syncLive()
	if more := len(plain.warnItems()) - 1; more != 0 {
		t.Errorf("a banner with nothing else to list offers %d more", more)
	}
	plain.handleMouse(pressAt(bannerRect()), false)
	if plain.warnList.open || plain.warningVisible() {
		t.Error("with no import warnings a banner click should just dismiss it")
	}
}

func TestWarnListKeyExplainsWhenThereIsNothingToList(t *testing.T) {
	a := newApp(t, 4)
	a.toggleWarnList()
	if a.warnList.open || a.bpmMessage() == "" {
		t.Error("I on a clean import should say so, not open an empty list")
	}
}
