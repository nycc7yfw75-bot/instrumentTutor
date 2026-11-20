package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/score"
	"github.com/S95F/musicTutor/internal/scoring"
)

// Timing ghosts (docs/SCORING.md §7, MOTION §B1.3): for a note played
// early or late, a faint copy of its fret number where it was actually
// played, joined to the written note. Perfect notes and misses get none.

var (
	colGhost     = colDim    // text2, drawn at ghostAlpha
	colGhostLine = colString // text3
)

const (
	ghostAlpha     = 0.4
	ghostFadeBars  = 2 // a ghost fades over the 2 bars after the playhead passes it
	ghostFadeSteps = 3 // steps3
	ghostDash      = 2.0
	maxGhosts      = 512
)

type ghost struct {
	start    int64 // the written note's tick
	str      int
	label    string
	offTicks int64 // where it was played, relative to start: - early, + late
	fadeLen  int64 // ticks of the ghostFadeBars bars it fades across
}

// ghostFor turns a judgement into a ghost, if it should have one. The
// offset is real time; the tab is drawn in score time, so it is scaled by
// the tempo the pass is played at.
func ghostFor(j scoring.Judgement, tempos score.TempoMap, tempoScale float64, barLen int64) (ghost, bool) {
	if j.Excluded || !j.Timed() || j.Tier == scoring.TierPerfect || j.Event.String < 1 {
		return ghost{}, false
	}
	t0 := tempos.TimeAt(j.Event.Start)
	off := tempos.TickAt(t0+j.OffsetMs/1000*tempoScale) - j.Event.Start
	if off == 0 {
		return ghost{}, false
	}
	label := fretLabel(score.Note{String: j.Event.String, Fret: j.Event.Fret})
	return ghost{start: j.Event.Start, str: j.Event.String, label: label, offTicks: off, fadeLen: ghostFadeBars * barLen}, true
}

// addGhosts records ghosts for newly judged notes.
func (a *App) addGhosts(js []scoring.Judgement) {
	g := &a.game
	tr := a.displayed()
	for _, j := range js {
		bar := tr.Bars[a.barAt(j.Event.Start)]
		gh, ok := ghostFor(j, a.sc.Tempos, a.eng.TempoScale(), bar.Len())
		if !ok {
			continue
		}
		if len(g.ghosts) >= maxGhosts {
			g.ghosts = g.ghosts[1:]
		}
		g.ghosts = append(g.ghosts, gh)
	}
}

// ghostAlphaAt is a ghost's opacity with the playhead at pos: full until the
// playhead passes it, then three held steps down across the fade bars.
func ghostAlphaAt(gh ghost, pos int64) float64 {
	passed := pos - max(gh.start, gh.start+gh.offTicks)
	if passed <= 0 {
		return ghostAlpha
	}
	if gh.fadeLen <= 0 || passed >= gh.fadeLen {
		return 0
	}
	if reduceMotion {
		return ghostAlpha // no fade: gone at the 2-bar mark
	}
	step := passed * ghostFadeSteps / gh.fadeLen // 0, 1, 2
	return ghostAlpha * float64(ghostFadeSteps-step) / ghostFadeSteps
}

// dashedRectNotched strokes a 1px dashed outline with its corners cut.
func dashedRectNotched(dst *ebiten.Image, x, y, w, h float32, col color.RGBA) {
	for dx := float32(1); dx < w-1; dx += 2 * ghostDash {
		l := min(ghostDash, w-1-dx)
		vector.FillRect(dst, x+dx, y, l, 1, col, false)
		vector.FillRect(dst, x+dx, y+h-1, l, 1, col, false)
	}
	for dy := float32(1); dy < h-1; dy += 2 * ghostDash {
		l := min(ghostDash, h-1-dy)
		vector.FillRect(dst, x, y+dy, 1, l, col, false)
		vector.FillRect(dst, x+w-1, y+dy, 1, l, col, false)
	}
}

// drawGhosts draws the live ghosts on the tab; it reads only what the tab
// renderer already has.
func (a *App) drawGhosts(screen *ebiten.Image, tickToX func(int64) float32, posTick int64) {
	g := &a.game
	if len(g.ghosts) == 0 || a.displayed().Wind != nil {
		return
	}
	keep := g.ghosts[:0]
	for _, gh := range g.ghosts {
		alpha := ghostAlphaAt(gh, posTick)
		if alpha <= 0 {
			continue // faded: dropped
		}
		keep = append(keep, gh)
		ny := float32(tabTop + (gh.str-1)*stringGap)
		drawGhost(screen, gh, alpha, tickToX(gh.start), tickToX(gh.start+gh.offTicks), ny)
	}
	g.ghosts = keep
}

// drawGhost draws one ghost at gx, joined to its written note at nx.
func drawGhost(dst *ebiten.Image, gh ghost, alpha float64, nx, gx, ny float32) {
	w := float32(textWMono(gh.label))
	// The joining line, from the written note's edge to the ghost's.
	lineCol := withAlpha(colGhostLine, alpha/ghostAlpha)
	if gh.offTicks > 0 {
		vector.FillRect(dst, nx+w+2, ny, max(0, gx-3-(nx+w+2)), 1, lineCol, false)
	} else {
		vector.FillRect(dst, gx+w+3, ny, max(0, nx-2-(gx+w+3)), 1, lineCol, false)
	}
	col := withAlpha(colGhost, alpha)
	dashedRectNotched(dst, gx-3, ny-10, w+6, 20, col)
	drawTextMono(dst, gh.label, float64(gx), float64(ny)-9, col)
	// Direction without colour: ‹ before an early ghost, › after a late one.
	if gh.offTicks < 0 {
		drawTextSmall(dst, "‹", float64(gx)-11, float64(ny)-8, col)
	} else {
		drawTextSmall(dst, "›", float64(gx+w)+5, float64(ny)-8, col)
	}
}
