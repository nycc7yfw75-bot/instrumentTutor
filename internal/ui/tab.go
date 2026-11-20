package ui

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/practice"
	"github.com/S95F/musicTutor/internal/score"
)

// The practice tab (docs/REDESIGN.md §6.4–6.5). Only text, line and the
// verdict colours are used here: brand amber never appears in the tab.

// tabLabel is how a note reads: its fret (a tied note too: its tie is an
// arc now, not brackets), x when dead.
func tabLabel(n score.Note) string {
	if n.Tech&score.TechDead != 0 {
		return "x"
	}
	return fmt.Sprintf("%d", n.Fret)
}

// stringName is an open string's note name, top string first; a top
// string that repeats the bottom's name is lower case (e B G D A E).
func stringNames(t score.Tuning) []string {
	out := make([]string, len(t))
	for i, k := range t {
		out[i] = keyNames[((k%12)+12)%12]
	}
	if n := len(out); n > 1 && out[0] == out[n-1] {
		out[0] = toLowerASCII(out[0])
	}
	return out
}

func toLowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

type noteLook int

const (
	lookUpcoming noteLook = iota
	lookNextBeat
	lookPast
	lookWaiting
	lookVerdict
)

func (a *App) drawTab(screen *ebiten.Image) {
	pos := a.posF()
	posTick := int64(math.Round(pos))
	ppt := a.pxPerTick()
	phX := float32(playheadXAt())
	tr := a.displayed()
	wind := tr.Wind != nil
	nLines := a.laneLines()
	top, gap := a.tabTop(), a.stringGap()
	bandTop, bandBottom := top, a.tabBottom()
	fs := a.fretSize()
	left, right := float32(tabLeft()), float32(tabRight())

	var ladder windLadder
	if wind {
		ladder = windLadderFor(tr)
	}
	tickToX := func(t int64) float32 { return phX + float32((float64(t)-pos)*ppt) }
	minTick := posTick - int64(float64(phX)/ppt) - score.PPQ
	maxTick := posTick + int64((screenW-float64(phX))/ppt) + score.PPQ

	// Strings, inside the margins; the low string 2px. Names at the left.
	if wind {
		a.drawWindGrid(screen, ladder, bandTop, bandBottom)
	} else {
		names := stringNames(tr.Tuning)
		for si := 0; si < nLines; si++ {
			y := float32(math.Round(top + float64(si)*gap))
			th := float32(1)
			if si == nLines-1 {
				th = 2
			}
			vector.FillRect(screen, left, y, right-left, th, colString, false)
			if si < len(names) {
				drawTextScaled(screen, names[si], pgMargin, float64(y)-sizeDisplayS/2-1, dispS, colBarline)
			}
		}
	}

	// Everything that scrolls is clipped to the staff's width.
	clip := screen.SubImage(image.Rect(int(left), int(a.tabZone().y), int(right)+1, int(a.tabZone().y+a.tabZone().h))).(*ebiten.Image)

	if la, lb, on := a.eng.Loop(); on {
		x0, x1 := tickToX(la), tickToX(lb)
		vector.FillRect(clip, x0, float32(bandTop-12), x1-x0, float32(bandBottom-bandTop)+24, colLoop, false)
		drawLoopBracket(clip, float64(x0), bandTop-12, bandBottom-bandTop+24, true)
		drawLoopBracket(clip, float64(x1), bandTop-12, bandBottom-bandTop+24, false)
	}

	waiting := a.waitingKeys()
	nextBeatEnd := posTick + a.beatLenAt(posTick)
	for bi, bar := range tr.Bars {
		barEnd := bar.Start + bar.Len()
		if barEnd < minTick || bar.Start > maxTick {
			continue
		}
		x := float32(math.Round(float64(tickToX(bar.Start))))
		vector.FillRect(clip, x, float32(bandTop), 1, float32(bandBottom-bandTop)+1, colBarline, false)
		drawMonoSized(clip, fmt.Sprintf("%d", a.sc.BarNumber(bi)), float64(x)+sp1, bandTop-zRulerH+2, sizeUISmall, colBarline)

		for _, beat := range bar.Beats {
			for _, n := range beat.Notes {
				nx := tickToX(beat.Start)
				var ny float32
				var label string
				if wind {
					ny = float32(ladder.y(tr.Wind.Written(tr.Pitch(n)), bandTop, bandBottom))
					label = windNoteLabel(tr, n)
				} else {
					ny = float32(math.Round(top + float64(n.String-1)*gap))
					label = tabLabel(n)
				}
				size := fs
				if n.Tech&score.TechGrace != 0 {
					size = math.Round(fs * 0.7) // grace notes read small, as on printed tab
				}
				look, verdict := lookUpcoming, practice.Verdict(0)
				key := noteKey{beat.Start, n.String}
				switch v, ok := a.verdictAt(beat.Start, n.String, posTick); {
				case ok:
					look, verdict = lookVerdict, v
				case waiting[key]:
					look = lookWaiting
				case beat.Start < posTick:
					look = lookPast
				case beat.Start <= nextBeatEnd:
					look = lookNextBeat
				}
				w := float32(textWFret(label, size))
				drawTabNote(clip, label, nx, ny, w, size, look, verdict, n.Inferred, a.pulseCol())
				if !wind {
					drawTechMark(clip, a.tabMarks()[key], nx, ny, w, float32(size), func(t int64) float32 { return tickToX(t) })
				}
				if float64(beat.Start) <= pos && pos < float64(beat.Start+beat.Dur) {
					vector.FillRect(clip, nx-2, ny+float32(size)/2+3, w+4, 2, colTabSounding, false)
				}
			}
		}
	}
	if n := len(tr.Bars); n > 0 {
		endTick := tr.Bars[n-1].Start + tr.Bars[n-1].Len()
		x := tickToX(endTick)
		vector.FillRect(clip, x-1, float32(bandTop), 2, float32(bandBottom-bandTop)+1, colBarline, false)
	}

	a.drawGhosts(clip, tickToX, posTick)

	// The playhead: a 2px line, a soft halo, a pixel marker on top (§6.4).
	ph := float32(math.Round(float64(phX)))
	y0, y1 := float32(bandTop-16), float32(bandBottom+16)
	// The halo: 6px either side, soft (15% at the line, fading out).
	vector.FillRect(screen, ph-7, y0, 14, y1-y0, withAlpha(colPlayhead, 0.05), false)
	vector.FillRect(screen, ph-4, y0, 8, y1-y0, withAlpha(colPlayhead, 0.10), false)
	vector.FillRect(screen, ph-1, y0, 2, y1-y0, colPlayhead, false)
	for i, w := range []float32{6, 4, 2} {
		vector.FillRect(screen, ph-w/2, y0-4+float32(i)-1, w, 1, colPlayhead, false)
	}
}

// drawTabNote draws one note: its knockout, verdict shape and number.
func drawTabNote(dst *ebiten.Image, label string, nx, ny, w float32, size float64, look noteLook, v practice.Verdict, inferred bool, pulse color.RGBA) {
	h := float32(size)
	bw := max(w, h*0.7) + 8
	box := rect{float64(nx + w/2 - bw/2), float64(ny - h/2 - 4), float64(bw), float64(h + 8)}
	vector.FillRect(dst, nx-3, ny-h/2, w+6, h, colNoteBG, false) // the knockout
	ink := colNote
	if inferred {
		ink = colInferredNote
	}
	switch look {
	case lookPast:
		ink = colDim
	case lookWaiting:
		ink = pulse
	case lookNextBeat:
		notchedStroke(dst, box, 1, colDim)
	case lookVerdict:
		switch v {
		case practice.VerdictHit:
			notchedFill(dst, box, withAlpha(colHit, 0.25))
			ink = colHit
		case practice.VerdictClose:
			notchedStroke(dst, box, 2, colClose)
			ink = colClose
		default:
			ink = colMiss
			// A 2px pixel × at the knockout's top-right corner.
			cx, cy := float32(math.Round(float64(nx+w+1))), float32(math.Round(float64(ny-h/2-4)))
			for i := float32(0); i < 3; i++ {
				vector.FillRect(dst, cx+i*2, cy+i*2, 2, 2, colMiss, false)
				vector.FillRect(dst, cx+4-i*2, cy+i*2, 2, 2, colMiss, false)
			}
		}
	}
	drawFretText(dst, label, float64(nx), float64(ny)-size*0.62, size, ink)
}
