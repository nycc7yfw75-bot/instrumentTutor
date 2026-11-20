package ui

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The practice view's menu: plain text items with no boxes. An idle item is
// muted; hovering brightens it and grows an underline out from its centre;
// an item that is on stays in the accent colour with its full underline.

const (
	menuPadX      = 4.0  // hit area beyond the text on each side
	menuGap       = 28.0 // between items
	menuGroupGap  = 22.0 // either side of a group divider
	menuUnderline = 2.0
	menuIconW     = 30.0
)

func menuItemW(label string) float64 { return textW(label) + 2*menuPadX }

// underlineY is where an item's underline sits in its hit area.
func underlineY(r rect) float32 { return float32(r.y + r.h/2 + uiTextH/2 + 3) }

// drawUnderline draws the item's underline: full when on, growing from the
// centre with hover otherwise.
func drawUnderline(dst *ebiten.Image, r rect, width float64, on bool, av animValues) {
	frac, col := av.hover, colNote
	if on {
		frac, col = 1, colAccent
	}
	if frac <= 0.01 {
		return
	}
	w := width * frac
	x := r.x + (r.w-w)/2
	vector.FillRect(dst, float32(x), underlineY(r), float32(w), menuUnderline, col, false)
}

func menuInk(on, disabled bool, av animValues) color.RGBA {
	switch {
	case disabled:
		return colDisabled
	case on:
		return colAccent
	}
	return lerpCol(colDim, colNote, av.hover)
}

func drawMenuItem(dst *ebiten.Image, r rect, label string, on, disabled bool, av animValues) {
	ink := menuInk(on, disabled, av)
	drawText(dst, label, centreX(label, r.x, r.w), r.y+(r.h-uiTextH)/2, ink)
	if !disabled {
		drawUnderline(dst, r, textW(label), on, av)
	}
}

// drawMenuIcon is a transport control (play, seek) drawn as a bare icon
// with the same hover underline as the text items.
func drawMenuIcon(dst *ebiten.Image, r rect, k iconKind, av animValues) {
	drawIcon(dst, k, r.x+r.w/2, r.y+r.h/2-2, menuInk(false, false, av))
	drawUnderline(dst, r, 16, false, av)
}

func drawMenuDivider(dst *ebiten.Image, x float64) {
	vector.StrokeLine(dst, float32(x), float32(ptTransportY+9), float32(x), float32(ptTransportY+chipH-9), 1, colPanelEdge, false)
}

// tempoText is the tempo control's readout: the effective BPM and, when the
// practice speed is not 100%, the speed.
func (a *App) tempoText() string {
	scale := a.eng.TempoScale()
	if scale > 0.995 && scale < 1.005 {
		return fmt.Sprintf("%.0f BPM", a.eng.EffectiveBPM())
	}
	return fmt.Sprintf("%.0f BPM · %.0f%%", a.eng.EffectiveBPM(), scale*100)
}

// tempoTextW is the readout's fixed width, so the menu does not shift as the
// numbers change.
var tempoTextW = textW("888 BPM · 188%") + 2*menuPadX

func (a *App) slower() { a.eng.SetTempoScale(a.eng.TempoScale() - 0.05) }
func (a *App) faster() { a.eng.SetTempoScale(a.eng.TempoScale() + 0.05) }

// drawTempo draws the tempo control: ‹ slower, the readout (click for an
// exact BPM), › faster.
func (a *App) drawTempo(dst *ebiten.Image, l practiceLayout, p pointer, dt float64) {
	parts := [3]struct{ id, tip, key string }{
		{"tempo:down", "Slower by 5%", "down"},
		{"tempo:bpm", "Type an exact BPM", "shift+B"},
		{"tempo:up", "Faster by 5%", "up"},
	}
	for i, part := range parts {
		r := l.tempo[i]
		av := a.anim.step(part.id, p.over(r), p.down, dt)
		cy := r.y + r.h/2
		switch i {
		case 1:
			bpm := fmt.Sprintf("%.0f", a.eng.EffectiveBPM())
			pct := fmt.Sprintf("· %.0f%%", a.eng.TempoScale()*100)
			w := textWScaled(bpm, dispM) + sp2 + textW(pct)
			x := r.x + (r.w-w)/2
			drawTextScaled(dst, bpm, x, cy-sizeDisplayM/2-3, dispM, lerpCol(colNote, colNote, av.hover))
			drawText(dst, pct, x+textWScaled(bpm, dispM)+sp2, cy-uiTextH/2-2, colDim)
			drawUnderline(dst, r, w, false, av)
		default:
			glyph := "‹"
			if i == 2 {
				glyph = "›"
			}
			drawTextScaled(dst, glyph, r.x+(r.w-textWScaled(glyph, dispM))/2, cy-sizeDisplayM/2-3, dispM, menuInk(false, false, av))
		}
		a.tip.offer(part.id, part.tip, part.key, r, p.over(r), dt)
	}
}

// sentenceCase capitalises the first letter, for labels shown as text.
func sentenceCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// tempoReadoutW is the tempo readout's fixed width (BPM in the display
// font, then "· 188%"), so the toolbar does not shift as it changes.
func tempoReadoutW() float64 {
	return textWScaled("888", dispM) + sp2 + textW("· 188%") + 2*sp1
}
