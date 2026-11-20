package ui

import (
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The pixel kit (docs/REDESIGN.md §5): notched shapes and the components
// built from them. Corners are never rounded: each loses a 2px staircase.

// notchedFill fills r with its corners stepped in by 2px.
func notchedFill(dst *ebiten.Image, r rect, col color.RGBA) {
	x, y := float32(math.Round(r.x)), float32(math.Round(r.y))
	w, h := float32(math.Round(r.w)), float32(math.Round(r.h))
	if w < 5 || h < 5 {
		vector.FillRect(dst, x, y, w, h, col, false)
		return
	}
	vector.FillRect(dst, x+2, y, w-4, 1, col, false)
	vector.FillRect(dst, x+1, y+1, w-2, 1, col, false)
	vector.FillRect(dst, x, y+2, w, h-4, col, false)
	vector.FillRect(dst, x+1, y+h-2, w-2, 1, col, false)
	vector.FillRect(dst, x+2, y+h-1, w-4, 1, col, false)
}

// notchedStroke draws a 1px outline (or thicker) following the notch.
func notchedStroke(dst *ebiten.Image, r rect, t float32, col color.RGBA) {
	x, y := float32(math.Round(r.x)), float32(math.Round(r.y))
	w, h := float32(math.Round(r.w)), float32(math.Round(r.h))
	vector.FillRect(dst, x+2, y, w-4, t, col, false)     // top
	vector.FillRect(dst, x+2, y+h-t, w-4, t, col, false) // bottom
	vector.FillRect(dst, x, y+2, t, h-4, col, false)     // left
	vector.FillRect(dst, x+w-t, y+2, t, h-4, col, false) // right
	// The staircase: one pixel at each inner corner step.
	for _, p := range [][2]float32{{x + 1, y + 1}, {x + w - 2, y + 1}, {x + 1, y + h - 2}, {x + w - 2, y + h - 2}} {
		vector.FillRect(dst, p[0], p[1], 1, 1, col, false)
	}
}

// Button kinds (§5).
type btnKind int

const (
	kitSecondary btnKind = iota
	kitPrimary
	kitDestructive
)

type btnState struct {
	hover, press, disabled, focus bool
}

const (
	btnH        = 36.0
	btnHPrimary = 44.0
	btnPadX     = 16.0
	btnIconGap  = 8.0
)

// kitButtonW is the width a button needs for its label (and icon).
func kitButtonW(label string, kind btnKind, icon bool) float64 {
	w := 2*btnPadX + textW(label)
	if kind == kitPrimary {
		w = 2*btnPadX + textWScaled(label, dispS)
	}
	if icon {
		w += iconSize + btnIconGap
	}
	return w
}

// drawKitButton draws a button. icon < 0 means none.
func drawKitButton(dst *ebiten.Image, r rect, label string, icon iconID, kind btnKind, st btnState) {
	if st.press && !st.disabled {
		r.y++ // a pixel press: one pixel down
	}
	ink := colDim
	switch {
	case st.disabled:
		ink = colDisabled
	case kind == kitPrimary:
		ink = colBG
	case kind == kitDestructive:
		ink = colMiss
	case st.hover || st.focus:
		ink = colNote
	}
	switch {
	case kind == kitPrimary && !st.disabled:
		fill := tokBrand
		if st.hover {
			fill = lerpCol(tokBrand, tokText, 0.15)
		}
		notchedFill(dst, r, fill)
	case st.hover && !st.disabled:
		notchedFill(dst, r, colHover)
	default:
		notchedStroke(dst, r, 1, colPanelEdge)
	}
	if st.focus && !st.disabled && kind != kitPrimary {
		notchedStroke(dst, r, 1, tokBrand)
	}
	tx := r.x + btnPadX
	if icon >= 0 {
		drawPixelIcon(dst, icon, tx, r.y+(r.h-iconSize)/2, ink)
		tx += iconSize + btnIconGap
	}
	if kind == kitPrimary {
		drawTextScaled(dst, label, tx, r.y+(r.h-sizeDisplayS)/2-1, dispS, ink)
	} else {
		drawText(dst, label, tx, r.y+(r.h-uiTextH)/2, ink)
	}
}

// Pill variants (§5).
type pillKind int

const (
	pillBrand   pillKind = iota // filled brand: LIVE
	pillOutline                 // outline line: PAUSED / PLAYING
	pillWarn                    // outline close: NO SIGNAL
	pillYou                     // outline brand: YOU
	pillLive                    // filled brand with the pulsing dot: LIVE
)

const pillH = 24.0

func pillW(label string) float64 { return textWScaled(label, dispS) + 2*sp2 }

// drawPill draws an uppercase display-font pill and returns its width.
func drawPill(dst *ebiten.Image, x, y float64, label string, kind pillKind) float64 {
	return drawPillA(dst, x, y, label, kind, 1, true)
}

// liveDotW is the LIVE pill's dot and its gap.
const liveDotW = 4.0 + sp1

// pillKindW is a pill's width; the LIVE pill also holds its dot.
func pillKindW(label string, kind pillKind) float64 {
	if kind == pillLive {
		return pillW(label) + liveDotW
	}
	return pillW(label)
}

// drawPillA is drawPill at an opacity (for a crossfade); dot is the LIVE
// pill's pulse, lit or dimmed.
func drawPillA(dst *ebiten.Image, x, y float64, label string, kind pillKind, alpha float64, dot bool) float64 {
	w := pillKindW(label, kind)
	r := rect{x, y, w, pillH}
	a := func(c color.RGBA) color.RGBA { return withAlpha(c, alpha) }
	ink := colNote
	tx := x + sp2
	switch kind {
	case pillBrand, pillLive:
		notchedFill(dst, r, a(tokBrand))
		ink = colBG
		if kind == pillLive {
			dc := colBG
			if !dot {
				dc = lerpCol(colBG, tokBrand, 0.6)
			}
			vector.FillRect(dst, float32(math.Round(tx)), float32(math.Round(y+(pillH-4)/2)), 4, 4, a(dc), false)
			tx += liveDotW
		}
	case pillOutline:
		notchedStroke(dst, r, 1, a(colPanelEdge))
		ink = colDim
	case pillWarn:
		notchedStroke(dst, r, 1, a(colClose))
		ink = colClose
	case pillYou:
		notchedStroke(dst, r, 1, a(tokBrand))
		ink = tokBrand
	}
	drawTextScaled(dst, label, tx, y+(pillH-sizeDisplayS)/2-1, dispS, a(ink))
	return w
}

// drawKeycap draws a key name in a notched box and returns its width.
func drawKeycap(dst *ebiten.Image, x, y float64, key string) float64 {
	w := math.Max(pillH, textWScaled(key, dispS)+2*sp2)
	notchedStroke(dst, rect{x, y, w, pillH}, 1, colPanelEdge)
	drawTextScaled(dst, key, x+(w-textWScaled(key, dispS))/2, y+(pillH-sizeDisplayS)/2-1, dispS, colDim)
	return w
}

const (
	meterBlockW = 8.0
	meterBlockH = 12.0
	meterGap    = 2.0
)

// drawSegMeter draws n level blocks, lit up to level (0..1): hit, then
// close near the top, the top block miss.
func drawSegMeter(dst *ebiten.Image, x, y float64, n int, level float64) float64 {
	lit := int(math.Round(level * float64(n)))
	for i := 0; i < n; i++ {
		col := colBlockOff
		if i < lit {
			col = segBlockCol(i, n)
		}
		bx := x + float64(i)*(meterBlockW+meterGap)
		vector.FillRect(dst, float32(bx), float32(y), meterBlockW, meterBlockH, col, false)
	}
	return float64(n)*(meterBlockW+meterGap) - meterGap
}

// drawKitTooltip draws a tooltip: label and keycaps on a notched surface.
func drawKitTooltip(dst *ebiten.Image, x, y float64, label string, keys ...string) rect {
	w := sp3 + textWSmall(label) + sp3
	for _, k := range keys {
		w += math.Max(pillH, textWScaled(k, dispS)+2*sp2) + sp1
	}
	r := rect{x, y, w, pillH + 2*sp1}
	notchedFill(dst, r, colTipBG)
	notchedStroke(dst, r, 1, colPanelEdge)
	drawTextSmall(dst, label, x+sp3, y+(r.h-uiTextH)/2+1, colNote)
	kx := x + sp3 + textWSmall(label) + sp2
	for _, k := range keys {
		kx += drawKeycap(dst, kx, y+sp1, k) + sp1
	}
	return r
}

const listRowH = 48.0

// drawListRow draws a list row: icon, title, secondary line.
func drawListRow(dst *ebiten.Image, r rect, icon iconID, title, sub string, hover, selected bool) {
	if hover || selected {
		vector.FillRect(dst, float32(r.x), float32(r.y), float32(r.w), float32(r.h), colHover, false)
	}
	if selected {
		vector.FillRect(dst, float32(r.x), float32(r.y), 3, float32(r.h), tokBrand, false)
	}
	x := r.x + sp4
	if icon >= 0 {
		drawPixelIcon(dst, icon, x, r.y+(r.h-iconSize)/2, colDim)
		x += iconSize + sp3
	}
	titleCol := colDim
	if hover || selected {
		titleCol = colNote
	}
	drawText(dst, title, x, r.y+sp1+1, titleCol)
	drawTextSmall(dst, sub, x, r.y+r.h/2+2, colHint)
}

// drawSectionHeader draws an uppercase display-font header, with an
// optional count at the right edge.
func drawSectionHeader(dst *ebiten.Image, x, y, w float64, title string, count int) {
	drawTextScaled(dst, spaced(title), x, y, dispS, colBarline)
	if count > 0 {
		c := fmt.Sprintf("%d", count)
		drawTextScaled(dst, c, x+w-textWScaled(c, dispS), y, dispS, colBarline)
	}
}

// spaced adds the section headers' 2px letter spacing as hair spaces.
func spaced(s string) string {
	out := make([]rune, 0, 2*len(s))
	for i, r := range s {
		if i > 0 {
			out = append(out, ' ')
		}
		out = append(out, r)
	}
	return string(out)
}

const bannerH = 40.0

// drawBanner draws a full-width warning strip; more > 0 adds "N more".
// It returns the dismiss and "more" hit areas.
func drawBanner(dst *ebiten.Image, r rect, msg string, more int, ptr pointer) (dismiss, moreR rect) {
	vector.FillRect(dst, float32(r.x), float32(r.y), float32(r.w), float32(r.h), colPanel, false)
	drawPixelIcon(dst, icWarning, r.x+sp4, r.y+(r.h-iconSize)/2, colClose)
	tx := r.x + sp4 + iconSize + sp3
	dismiss = rect{r.x + r.w - 40, r.y, 40, r.h}
	right := dismiss.x - sp4
	if more > 0 {
		m := fmt.Sprintf("%d more", more)
		moreR = rect{right - textW(m), r.y, textW(m), r.h}
		col := colDim
		if ptr.over(moreR) {
			col = colNote
		}
		drawText(dst, m, moreR.x, r.y+(r.h-uiTextH)/2, col)
		vector.FillRect(dst, float32(moreR.x), float32(r.y+(r.h+uiTextH)/2), float32(moreR.w), 1, col, false)
		right = moreR.x - sp4
	}
	drawText(dst, truncateW(msg, right-tx), tx, r.y+(r.h-uiTextH)/2, colNote)
	xc := colDim
	if ptr.over(dismiss) {
		xc = colNote
	}
	drawText(dst, "×", centreX("×", dismiss.x, dismiss.w), r.y+(r.h-uiTextH)/2, xc)
	return dismiss, moreR
}

// drawKitMenuItem is a menu item (§5): icon and label, no box. Inactive
// text2; hover text with an underline growing from the centre; active
// brand with a full underline. icon < 0 draws the label alone.
func drawKitMenuItem(dst *ebiten.Image, r rect, icon iconID, label string, on, disabled bool, av animValues) {
	ink := menuInk(on, disabled, av)
	w := textW(label)
	if icon >= 0 {
		w += iconSize + sp1
	}
	x := r.x + (r.w-w)/2
	if icon >= 0 {
		drawPixelIcon(dst, icon, x, r.y+(r.h-iconSize)/2-2, ink)
		x += iconSize + sp1
	}
	if label != "" {
		drawText(dst, label, x, r.y+(r.h-uiTextH)/2-2, ink)
	}
	if !disabled {
		drawUnderline(dst, r, w, on, av)
	}
}

func kitMenuItemW(icon iconID, label string) float64 {
	w := textW(label) + 2*sp2
	if icon >= 0 {
		w += iconSize + sp1
	}
	return w
}

const wordmarkTrack = 2.0 // the wordmark's letter spacing

// drawWordmark draws the HEADLINER wordmark (§4): the spotlight mark, then
// the name in the display font at 32px, 2px apart, in brand amber. It
// returns its width.
func drawWordmark(dst *ebiten.Image, x, y float64) float64 {
	drawPixelIconScaled(dst, icSpotlight, x, y+(sizeDisplayL-iconPx)/2, 1, tokBrand)
	tx := x + iconPx + sp2
	for _, r := range "HEADLINER" {
		s := string(r)
		drawTextScaled(dst, s, tx, y, dispL, tokBrand)
		tx += textWScaled(s, dispL) + wordmarkTrack
	}
	return tx - wordmarkTrack - x
}

// drawDialogCard draws a prompt's card (REDESIGN §8): a notched surface
// with a 1px line edge and the title in display 24. It returns where the
// body starts.
func drawDialogCard(dst *ebiten.Image, r rect, title string) float64 {
	notchedFill(dst, r, colPanel)
	notchedStroke(dst, r, 1, colPanelEdge)
	drawTextScaled(dst, title, math.Round(r.x+sp6), math.Round(r.y+sp4), dispM, colNote)
	return r.y + sp4 + sizeDisplayM + sp3
}

// drawField draws a text field: a raised notched box with its text and a
// caret.
func drawField(dst *ebiten.Image, r rect, s string, scale float64, caret bool) {
	notchedFill(dst, r, colHover)
	h := uiTextH
	w := textW(s)
	if scale > 0 {
		h, w = displaySize(scale), textWScaled(s, scale)
		drawTextScaled(dst, s, math.Round(r.x+sp3), math.Round(r.y+(r.h-h)/2-2), scale, colNote)
	} else {
		drawText(dst, s, r.x+sp3, r.y+(r.h-h)/2, colNote)
	}
	if caret {
		vector.FillRect(dst, float32(math.Round(r.x+sp3+w+2)), float32(r.y+(r.h-h)/2), 2, float32(h), colNote, false)
	}
}
