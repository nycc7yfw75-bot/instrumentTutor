package ui

import (
	"fmt"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/score"
)

// Technique marks on the practice tab, drawn the way online tab sites do:
// an arc from a note to the one it hammers on or pulls off to, a slanted
// line for a slide, an arrow curving up from a bend with its amount at the
// tip (and back down for a release), PM and ~~ (vibrato) over the note.

type legato int

const (
	legatoNone  legato = iota
	legatoArc          // hammer-on or pull-off: an arc from the note before
	legatoSlide        // a slanted line from the note before
)

type tabMark struct {
	link      legato
	prevStart int64 // the note before on this string, for arcs and slides
	prevFret  int
	up        bool // a slide to a higher fret
	bend      score.Bend
	bent      bool
	above     string // small text over the note: PM, ~~
}

// tabMarks works the marks out once per track: arcs and slides need the
// note before on the same string.
func (a *App) tabMarks() map[noteKey]tabMark {
	if a.marks != nil {
		return a.marks
	}
	a.marks = map[noteKey]tabMark{}
	type prev struct {
		start int64
		fret  int
	}
	last := map[int]prev{}
	for _, bar := range a.displayed().Bars {
		for _, beat := range bar.Beats {
			for _, n := range beat.Notes {
				p, ok := last[n.String]
				if m := markFor(n, p.start, p.fret, ok); m != (tabMark{}) {
					a.marks[noteKey{beat.Start, n.String}] = m
				}
				last[n.String] = prev{beat.Start, n.Fret}
			}
		}
	}
	return a.marks
}

func markFor(n score.Note, prevStart int64, prevFret int, hasPrev bool) tabMark {
	var m tabMark
	if hasPrev {
		switch {
		case n.Tech&(score.TechHammer|score.TechPull) != 0:
			m.link = legatoArc
		case n.Tech&score.TechSlide != 0:
			m.link = legatoSlide
		}
		if m.link != legatoNone {
			m.prevStart, m.prevFret, m.up = prevStart, prevFret, n.Fret >= prevFret
		}
	}
	if n.Tech&score.TechBend != 0 {
		m.bent, m.bend = true, n.Bend
	}
	var above []string
	if n.Tech&score.TechPalmMute != 0 {
		above = append(above, "PM")
	}
	if n.Tech&score.TechVibrato != 0 {
		above = append(above, "~~")
	}
	m.above = strings.Join(above, " ")
	return m
}

// fretLabel is how a note reads on the tab: its fret, in brackets when it
// is tied over from the note before, x when dead.
func fretLabel(n score.Note) string {
	switch {
	case n.Tech&score.TechDead != 0:
		return "x"
	case n.Tied:
		return fmt.Sprintf("(%d)", n.Fret)
	}
	return fmt.Sprintf("%d", n.Fret)
}

// bendText names a bend's amount: "full", "1/2", "1 1/2".
func bendText(b score.Bend) string {
	if b.Peak == 0 {
		return ""
	}
	return quarterToneText(b.Peak)
}

func quarterToneText(q uint8) string {
	whole, rest := q/4, q%4
	frac := map[uint8]string{1: "1/4", 2: "1/2", 3: "3/4"}[rest]
	switch {
	case whole == 0:
		return frac
	case whole == 1 && rest == 0:
		return "full"
	case rest == 0:
		return fmt.Sprintf("%d", whole)
	}
	return fmt.Sprintf("%d %s", whole, frac)
}

func strokeCurve(dst *ebiten.Image, p *vector.Path) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(colTechMark)
	vector.StrokePath(dst, p, &vector.StrokeOptions{Width: 1.4, LineCap: vector.LineCapRound}, op)
}

func arrowHead(dst *ebiten.Image, x, y float32, up bool) {
	d := float32(4)
	if !up {
		d = -d
	}
	var p vector.Path
	p.MoveTo(x, y)
	p.LineTo(x-3.5, y+d+1)
	p.LineTo(x+3.5, y+d+1)
	p.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(colTechMark)
	vector.FillPath(dst, &p, &vector.FillOptions{}, op)
}

// drawTechMark draws one note's technique marks. nx, ny are where its
// label sits (left edge, string line) and w the label's width; xOf maps a
// tick to the screen.
func drawTechMark(dst *ebiten.Image, m tabMark, nx, ny, w float32, xOf func(int64) float32) {
	if m.link != legatoNone {
		px := xOf(m.prevStart)
		pw := float32(textWMono(fmt.Sprintf("%d", m.prevFret)))
		x0, x1 := px+pw+3, nx-3
		if x1 > x0 {
			var p vector.Path
			switch m.link {
			case legatoArc:
				mid := (px + pw/2 + nx + w/2) / 2
				p.MoveTo(px+pw/2, ny-10)
				p.QuadTo(mid, ny-21, nx+w/2, ny-10)
			case legatoSlide:
				// Rising to a higher fret, falling to a lower one.
				dy := float32(5)
				if !m.up {
					dy = -dy
				}
				p.MoveTo(x0, ny+dy)
				p.LineTo(x1, ny-dy)
			}
			strokeCurve(dst, &p)
		}
	}
	if m.bent {
		// An arrow curving up from the note; a release curves back down.
		sx, sy := nx+w+3, ny-2
		tx, ty := sx+9, ny-22
		var p vector.Path
		p.MoveTo(sx, sy)
		p.QuadTo(tx, sy, tx, ty+2)
		strokeCurve(dst, &p)
		arrowHead(dst, tx, ty, true)
		if s := bendText(m.bend); s != "" {
			drawTextSmall(dst, s, float64(tx)-textWSmall(s)/2, float64(ty)-16, colHint)
		}
		if m.bend.To < m.bend.Peak {
			ex, ey := tx+12, ny-8
			var r vector.Path
			r.MoveTo(tx+1, ty)
			r.QuadTo(ex, ty, ex, ey-2)
			strokeCurve(dst, &r)
			arrowHead(dst, ex, ey, false)
		}
	}
	if m.above != "" {
		drawTextSmall(dst, m.above, float64(nx), float64(ny)-24, colHint)
	}
}
