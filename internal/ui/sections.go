package ui

import (
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

// drawSections marks each section on the timeline: a divider where it
// starts and its name in the space up to the next one.
func (a *App) drawSections(dst *ebiten.Image, tl rect) {
	secs := a.sc.Sections
	cur := a.sc.SectionAt(a.posTick())
	for i, s := range secs {
		x0 := a.timelineX(tl, s.Tick)
		x1 := tl.x + tl.w
		if i+1 < len(secs) {
			x1 = a.timelineX(tl, secs[i+1].Tick)
		}
		if i > 0 || s.Tick > 0 {
			vector.StrokeLine(dst, float32(x0), float32(tl.y+1), float32(x0), float32(tl.y+tl.h-1), 1, colDim, false)
		}
		col := colBarline
		if i == cur {
			col = colHUD
		}
		if room := x1 - x0 - 8; room > 12 {
			drawTextSmall(dst, truncateW(s.Name, room), x0+4, tl.y+3, col)
		}
	}
}
