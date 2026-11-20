package ui

import (
	"fmt"
	"testing"
)

// The practice screen fills the window (REDESIGN §6); every check runs at
// the four sizes the redesign is reviewed at.
var reviewSizes = [][2]int{{1280, 720}, {1366, 768}, {1600, 900}, {1920, 1080}}

func atEachSize(t *testing.T, f func(t *testing.T)) {
	t.Helper()
	defer resize(designW, designH)
	for _, s := range reviewSizes {
		resize(s[0], s[1])
		t.Run(fmt.Sprintf("%dx%d", s[0], s[1]), f)
	}
}

type band struct {
	name   string
	top    float64
	bottom float64
}

// practiceBands are the screen's zones top to bottom, as drawn.
func practiceBands(a *App) []band {
	l := a.layout()
	staffTop := a.tabTop() - zRulerH
	staffBottom := a.tabBottom() + 16
	out := []band{
		{"header", 0, zHeaderH},
		{"toolbar", l.transport[0].y, l.transport[0].y + l.transport[0].h},
		{"track bar", zTrackBarY(), zTrackBarY() + zTrackBarH},
	}
	if a.warningVisible() {
		b := bannerRect()
		out = append(out, band{"banner", b.y, b.y + b.h})
	}
	return append(out,
		band{"staff", staffTop, staffBottom},
		band{"timeline", l.timeline.y - 4, l.timeline.y + l.timeline.h + 4},
		band{"timeline line", gameHUDY(), gameHUDY() + uiTextH},
		band{"bottom margin", screenH - pgMargin, screenH},
	)
}

func checkBands(t *testing.T, a *App, what string) {
	t.Helper()
	bands := practiceBands(a)
	for i := 1; i < len(bands); i++ {
		prev, cur := bands[i-1], bands[i]
		if cur.top < prev.bottom {
			t.Errorf("%s: %q (ends %.0f) overlaps %q (starts %.0f)", what, prev.name, prev.bottom, cur.name, cur.top)
		}
	}
	if last := bands[len(bands)-1]; last.bottom > screenH {
		t.Errorf("%s: %q ends at %.0f, past the %.0f px window", what, last.name, last.bottom, screenH)
	}
}

func TestPracticeBandsDoNotOverlap(t *testing.T) {
	atEachSize(t, func(t *testing.T) {
		a := newApp(t, 8)
		checkBands(t, a, "plain")
		a.SetLiveWarning("the saved capture device is not connected; timing is not calibrated")
		checkBands(t, a, "with the banner")
	})
}

func TestTabStaysInsideItsZoneForAnyNeck(t *testing.T) {
	atEachSize(t, func(t *testing.T) {
		for _, strings := range []int{4, 6, 7, 8} {
			a := newAppStrings(t, strings)
			checkBands(t, a, fmt.Sprintf("%d strings", strings))
			z := a.tabZone()
			if a.tabTop()-zRulerH < z.y || a.tabBottom() > z.y+z.h {
				t.Errorf("%d strings: the staff %.0f..%.0f leaves its zone %.0f..%.0f",
					strings, a.tabTop()-zRulerH, a.tabBottom(), z.y, z.y+z.h)
			}
		}
	})
}

func TestStringGapScalesWithTheWindow(t *testing.T) {
	defer resize(designW, designH)
	a := newApp(t, 4)
	resize(1280, 720)
	small := a.stringGap()
	resize(1920, 1080)
	big := a.stringGap()
	if small < 36 || big > 64 || big < small {
		t.Errorf("string gap %.0f at 720p and %.0f at 1080p; want 36..64, growing with the window", small, big)
	}
	if fs := a.fretSize(); fs < 16 || fs > 28 {
		t.Errorf("fret numbers at %.0fpx, want 16..28", fs)
	}
}

func TestToolbarFitsTheWindow(t *testing.T) {
	atEachSize(t, func(t *testing.T) {
		a := newApp(t, 4)
		l := a.layout()
		items := append(append([]rect{}, l.transport[:]...), l.chips...)
		for _, r := range l.tempo {
			items = append(items, r)
		}
		for i, r := range items {
			if r.x+r.w > screenW || r.x < 0 {
				t.Errorf("toolbar item %d spans %.0f..%.0f, off the %.0f px window", i, r.x, r.x+r.w, screenW)
			}
			if r.y != l.transport[0].y || r.h < 40 || r.w < 40 {
				t.Errorf("toolbar item %d is %+v; every item shares the row with a ≥ 40×40 target", i, r)
			}
		}
		// The left group (transport, practice, tempo) must not run into
		// the Tools at the right.
		lastLeft := l.tempo[2]
		for i, c := range practiceChips {
			if c.right && l.chips[i].x < lastLeft.x+lastLeft.w {
				t.Errorf("the %s tool at %.0f overlaps the tempo control ending at %.0f", c.label, l.chips[i].x, lastLeft.x+lastLeft.w)
			}
		}
	})
}

func TestBPMEntryBoxSitsOverTheTab(t *testing.T) {
	a := newApp(t, 4)
	r := bpmEntryRect()
	tab := a.layout().tab
	if r.x < 0 || r.x+r.w > screenW || r.y < 0 || r.y+r.h > screenH {
		t.Errorf("the tempo box %+v is not fully inside the window", r)
	}
	if r.y < tab.y || r.y > tab.y+tab.h {
		t.Errorf("the tempo box starts at y %.0f, outside the tab band %.0f..%.0f", r.y, tab.y, tab.y+tab.h)
	}
}

func TestTimelineSpansThePageMargins(t *testing.T) {
	atEachSize(t, func(t *testing.T) {
		tl := newApp(t, 4).layout().timeline
		if tl.x != pgMargin || tl.x+tl.w != screenW-pgMargin {
			t.Errorf("timeline spans %.0f..%.0f, want %.0f..%.0f", tl.x, tl.x+tl.w, pgMargin, screenW-pgMargin)
		}
	})
}

func newAppStrings(t *testing.T, n int) *App {
	t.Helper()
	a := newApp(t, 4)
	tuning := make([]int, n)
	for i := range tuning {
		tuning[i] = 40 + 5*i
	}
	tr := a.displayed()
	tr.Tuning = tuning

	for _, bar := range tr.Bars {
		for _, beat := range bar.Beats {
			for i := range beat.Notes {
				if beat.Notes[i].String > n {
					beat.Notes[i].String = n
				}
			}
		}
	}
	return a
}
