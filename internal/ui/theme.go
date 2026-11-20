package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	uiPadX    = 24.0
	uiHeaderY = 18.0

	uiHeaderH  = 64.0
	uiTitleScl = dispL
	uiBodyTop  = uiHeaderH + 8.0
	uiLineH    = 18.0
	uiRowH     = 26.0
	uiSectionH = 26.0
)

func centreX(s string, x, w float64) float64 { return x + (w-textW(s))/2 }

func centreXScaled(s string, x, w, scale float64) float64 {
	return x + (w-textWScaled(s, scale))/2
}

func drawTextRight(dst *ebiten.Image, s string, x, y float64, col color.RGBA) {
	drawText(dst, s, x-textW(s), y, col)
}

func drawHeader(dst *ebiten.Image, title, status string, statusCol color.RGBA) {
	drawHeaderAt(dst, uiPadX, title, status, statusCol)
}

// The esc button every screen shows at the top left of its header.
var (
	escBackChip = chipState{label: "Back", key: "esc"}
	escQuitChip = chipState{label: "Quit", key: "esc"}
)

// escIcon is the esc button's icon: the back arrow, or none for Quit.
func escIcon(c chipState) iconID {
	if c.label == escBackChip.label {
		return icBack
	}
	return -1
}

// drawEscButton draws a screen's esc button (pixel back arrow and "Back",
// or "Quit") and returns where the title starts.
func drawEscButton(dst *ebiten.Image, c chipState, an *animator, p pointer) float64 {
	r := backButtonRect(c)
	drawKitMenuItem(dst, r, escIcon(c), c.label, false, false, an.step("chip:esc", p.over(r), p.down, motionDt()))
	vector.StrokeLine(dst, float32(r.x+r.w+12), float32(r.y+8), float32(r.x+r.w+12), float32(r.y+r.h-8), 1, colPanelEdge, false)
	return r.x + r.w + 26
}

// escButtonPressed reports a click on a screen's esc button.
func escButtonPressed(c chipState, p pointer) bool {
	return p.pressed && p.over(backButtonRect(c))
}

// backButtonRect places a screen's esc button at the top left of the header.
func backButtonRect(c chipState) rect {
	// A text link like the menu items: its ink starts at the left margin.
	return rect{uiPadX - menuPadX, uiHeaderY - 6, kitMenuItemW(escIcon(c), c.label), zItemH}
}

// drawHeaderAt is drawHeader with the title starting at x, leaving room
// on the left for a button: the title in the display font, a status in
// text2 at the right. No rule under it: space separates (REDESIGN §3).
func drawHeaderAt(dst *ebiten.Image, x float64, title, status string, statusCol color.RGBA) {
	drawTextScaled(dst, title, x, uiHeaderY, uiTitleScl, colNote)
	if status != "" {
		drawTextRight(dst, status, screenW-uiPadX, uiHeaderY+8, statusCol)
	}
}

// drawSection is a section header (§5): display 16, uppercase, text3,
// letter-spaced, no rule.
func drawSection(dst *ebiten.Image, y *float64, title string) {
	drawSectionHeader(dst, uiPadX, *y, 0, title, 0)
	*y += uiSectionH
}

// Panels have notched corners, never rounded ones (REDESIGN §3).
func fillRounded(dst *ebiten.Image, r rect, col color.RGBA) { notchedFill(dst, r, col) }

func strokeRounded(dst *ebiten.Image, r rect, col color.RGBA) { notchedStroke(dst, r, 1, col) }

func drawPanel(dst *ebiten.Image, r rect, fill, edge color.RGBA) {
	fillRounded(dst, r, fill)
	strokeRounded(dst, r, edge)
}

type chipState struct {
	label string

	key      string
	on       bool
	disabled bool
}

const (
	chipH    = 36.0
	chipPadX = 11.0
	chipGap  = 8.0
)

func chipW(c chipState) float64 {
	w := textW(c.label)
	if kw := textWSmall(c.key); kw > w {
		w = kw
	}
	return w + 2*chipPadX
}

func drawChip(dst *ebiten.Image, r rect, c chipState, av animValues) {
	fill, edge, text := colPanel, colPanelEdge, colHUD
	switch {
	case c.disabled:
		drawPanel(dst, r, colBG, colBarline)
		drawText(dst, c.label, centreX(c.label, r.x, r.w), r.y+3, colBarline)
		if c.key != "" {
			drawTextSmall(dst, c.key, r.x+(r.w-textWSmall(c.key))/2, r.y+20, colBarline)
		}
		return
	case c.on:
		fill, edge, text = colOn, colOnEdge, colNote
		fill = lerpCol(fill, colOnHover, av.hover)
	default:
		fill = lerpCol(fill, colHover, av.hover)
		edge = lerpCol(edge, colDim, av.hover)
	}
	r = av.animate(r)
	drawPanel(dst, r, fill, edge)
	drawText(dst, c.label, centreX(c.label, r.x, r.w), r.y+3, text)
	if c.key != "" {

		kc := colDim
		if c.on {
			kc = colHUD
		}
		drawTextSmall(dst, c.key, r.x+(r.w-textWSmall(c.key))/2, r.y+20,
			lerpCol(kc, colNote, av.hover))
	}
}

const (
	iconBtnSize = 32.0
	iconBtnGap  = 4.0
	iconGrpGap  = 22.0
	iconCapH    = 14.0
)

func drawIconGlyphButton(dst *ebiten.Image, r rect, id glyphID, on, disabled bool, av animValues) {
	fill, edge, ink := colPanel, colPanelEdge, colHUD
	switch {
	case disabled:
		drawPanel(dst, r, colBG, colBarline)
		drawGlyph(dst, id, glyphInset(r), colDisabled)
		return
	case on:
		fill, edge, ink = colOn, colOnEdge, colNote
		fill = lerpCol(fill, colOnHover, av.hover)
	default:
		fill = lerpCol(fill, colHover, av.hover)
		edge = lerpCol(edge, colDim, av.hover)
		ink = lerpCol(ink, colNote, av.hover)
	}
	r = av.animate(r)
	drawPanel(dst, r, fill, edge)
	drawGlyph(dst, id, glyphInset(r), ink)
}

func glyphInset(r rect) rect {
	m := (r.h - glyphBox) / 2
	return rect{r.x + (r.w-glyphBox)/2, r.y + m, glyphBox, glyphBox}
}

func drawGroupCaption(dst *ebiten.Image, text string, x, y float64) {
	drawTextSmall(dst, text, x, y, colGroupCap)
}

type buttonStyle int

const (
	btnNormal buttonStyle = iota
	btnPrimary
	btnDisabled
)

func drawButton(dst *ebiten.Image, r rect, id glyphID, label, key string, style buttonStyle, av animValues) {

	textX := func(r rect, s string, scale float64) float64 {
		w := textW(s)
		if id == glyphNone {
			return r.x + (r.w-w)/2
		}
		return r.x + (r.w-w-glyphBox-8)/2 + glyphBox + 8
	}

	avail := r.w - 22
	if id != glyphNone {
		avail -= glyphBox
	}
	if style == btnDisabled {
		drawPanel(dst, r, colBG, colBarline)
		l := truncateW(label, avail)
		x := textX(r, l, 1)
		// Same positions as the enabled button, so nothing shifts when it enables.
		glyphY, textY := r.y+5, r.y+6
		if key == "" {
			glyphY, textY = r.y+(r.h-glyphBox)/2, r.y+(r.h-uiTextH)/2+1
		}
		if id != glyphNone {
			drawGlyph(dst, id, rect{x - glyphBox - 8, glyphY, glyphBox, glyphBox}, colDisabled)
		}
		drawText(dst, l, x, textY, colBarline)
		if key != "" {
			drawTextSmall(dst, key, r.x+(r.w-textWSmall(key))/2, r.y+23, colBarline)
		}
		return
	}
	fill, edge, text := colPanel, colPanelEdge, colHUD
	if style == btnPrimary {
		fill, edge, text = colFocus, colInferred, colNote
	}
	fill = lerpCol(fill, lerpCol(fill, colNote, 0.16), av.hover)
	edge = lerpCol(edge, colNote, av.hover)
	r = av.animate(r)
	drawPanel(dst, r, fill, edge)

	label = truncateW(label, avail)
	x := textX(r, label, 1)
	if key == "" {
		if id != glyphNone {
			drawGlyph(dst, id, rect{x - glyphBox - 8, r.y + (r.h-glyphBox)/2, glyphBox, glyphBox}, text)
		}
		drawText(dst, label, x, r.y+(r.h-uiTextH)/2+1, text)
	} else {
		if id != glyphNone {
			drawGlyph(dst, id, rect{x - glyphBox - 8, r.y + 5, glyphBox, glyphBox}, text)
		}
		drawText(dst, label, x, r.y+6, text)
		drawTextSmall(dst, key, r.x+(r.w-textWSmall(key))/2, r.y+23, lerpCol(colDim, colNote, av.hover))
	}
}

type iconKind int

const (
	iconPlay iconKind = iota
	iconPause
	iconPrevBar
	iconNextBar
	iconToStart
)

func drawIconButton(dst *ebiten.Image, r rect, k iconKind, av animValues) {
	fill := lerpCol(colPanel, colHover, av.hover)
	edge := lerpCol(colPanelEdge, colDim, av.hover)
	r = av.animate(r)
	drawPanel(dst, r, fill, edge)
	cx, cy := r.x+r.w/2, r.y+r.h/2
	drawIcon(dst, k, cx, cy, colNote)
}

func drawIcon(dst *ebiten.Image, k iconKind, cx, cy float64, col color.RGBA) {
	const s = 6.0
	switch k {
	case iconPlay:
		fillTriangle(dst, cx-s*0.7, cy-s, cx-s*0.7, cy+s, cx+s*0.9, cy, col)
	case iconPause:
		vector.DrawFilledRect(dst, float32(cx-s+1), float32(cy-s), 3, float32(2*s), col, false)
		vector.DrawFilledRect(dst, float32(cx+2), float32(cy-s), 3, float32(2*s), col, false)
	case iconPrevBar:
		fillTriangle(dst, cx+s*0.9, cy-s, cx+s*0.9, cy+s, cx-s*0.3, cy, col)
		vector.DrawFilledRect(dst, float32(cx-s), float32(cy-s), 2, float32(2*s), col, false)
	case iconNextBar:
		fillTriangle(dst, cx-s*0.9, cy-s, cx-s*0.9, cy+s, cx+s*0.3, cy, col)
		vector.DrawFilledRect(dst, float32(cx+s-2), float32(cy-s), 2, float32(2*s), col, false)
	case iconToStart:
		vector.DrawFilledRect(dst, float32(cx-s), float32(cy-s), 2, float32(2*s), col, false)
		fillTriangle(dst, cx+s*0.6, cy-s, cx+s*0.6, cy+s, cx-s*0.6, cy, col)
	}
}

func fillTriangle(dst *ebiten.Image, x0, y0, x1, y1, x2, y2 float64, col color.RGBA) {
	var p vector.Path
	p.MoveTo(float32(x0), float32(y0))
	p.LineTo(float32(x1), float32(y1))
	p.LineTo(float32(x2), float32(y2))
	p.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(col)
	vector.FillPath(dst, &p, nil, op)
}

// pageBottom is where a screen's content ends: the bottom margin. There
// is no footer shortcut bar any more (REDESIGN §6.8); keys live in
// tooltips and the help overlay.
func pageBottom() float64 { return screenH - uiPadX }
