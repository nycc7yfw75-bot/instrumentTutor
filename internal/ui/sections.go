package ui

import (
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Named sections (Intro, Verse, Solo) from the file: drawn on the timeline,
// named in the position caption, and jumped between with [ and ].

// sectionRestartTicks is how far into a section [ still counts as "at its
// start", so a second press goes to the section before.
const sectionRestartTicks = 960 // one beat at the score's 960 PPQ

func (a *App) currentSection() string {
	if i := a.sc.SectionAt(a.posTick()); i >= 0 {
		return a.sc.Sections[i].Name
	}
	return ""
}

// jumpSection moves to the next section (dir > 0), or back to the start of
// this one, or the one before if already at its start (dir < 0).
func (a *App) jumpSection(dir int) {
	secs := a.sc.Sections
	if len(secs) == 0 {
		a.setBPMMessage("this piece has no named sections")
		return
	}
	pos := a.posTick()
	if dir > 0 {
		for _, s := range secs {
			if s.Tick > pos {
				a.seekTo(s.Tick)
				return
			}
		}
		a.setBPMMessage("already in the last section")
		return
	}
	for i := len(secs) - 1; i >= 0; i-- {
		if secs[i].Tick < pos-sectionRestartTicks || (secs[i].Tick <= pos && i == 0) {
			a.seekTo(secs[i].Tick)
			return
		}
	}
	a.seekTo(0)
}

// drawSections draws each section as a solid segment of the timeline
// (§6.6): shaded by its kind, the current one in brand at 35%, with its
// name only where the whole name fits.
func (a *App) drawSections(dst *ebiten.Image, tl rect, p pointer) {
	secs := a.sc.Sections
	cur := a.sc.SectionAt(a.posTick())
	for i, s := range secs {
		x0 := math.Round(a.timelineX(tl, s.Tick))
		x1 := tl.x + tl.w
		if i+1 < len(secs) {
			x1 = math.Round(a.timelineX(tl, secs[i+1].Tick)) - 2 // a 2px gap between segments
		}
		if x1 <= x0 {
			continue
		}
		fill, ink := sectionTint(s.Name), colDim
		if i == cur {
			fill, ink = withAlpha(tokBrand, 0.35), tokBrand
		}
		seg := rect{x0, tl.y, x1 - x0, tl.h}
		if i == 0 || i == len(secs)-1 {
			notchedFill(dst, seg, fill)
		} else {
			vector.FillRect(dst, float32(seg.x), float32(seg.y), float32(seg.w), float32(seg.h), fill, false)
		}
		label := strings.ToUpper(s.Name)
		if w := textWScaled(label, dispS); w+2*sp1 <= seg.w {
			drawTextScaled(dst, label, seg.x+sp1, seg.y+(seg.h-sizeDisplayS)/2-1, dispS, ink)
		}
	}
}

// sectionTint tells section kinds apart by lightness (§6.6).
func sectionTint(name string) color.RGBA {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "solo"):
		return tokLine
	case strings.Contains(n, "chorus"):
		return lerpCol(tokRaised, tokLine, 0.66)
	case strings.Contains(n, "verse"):
		return lerpCol(tokRaised, tokLine, 0.33)
	case strings.Contains(n, "intro"), strings.Contains(n, "outro"):
		return tokRaised
	}
	return lerpCol(tokRaised, tokLine, 0.5)
}
