package ui

import (
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	tipDelay = 0.35

	tipGap = 8.0
	tipH   = pillH + 2*sp1
)

type tips struct {
	id, text, key string
	at            rect

	dwell float64

	seen bool
}

func (t *tips) offer(id, text, key string, at rect, hovered bool, dt float64) {
	if !hovered || text == "" {
		if t.id == id {

			t.id, t.dwell = "", 0
		}
		return
	}
	if t.id != id {
		t.id, t.dwell = id, 0
	}
	t.text, t.key, t.at = text, key, at
	t.dwell += dt
	t.seen = true
}

func (t *tips) hide() { t.id, t.dwell, t.seen = "", 0, false }

func (t *tips) visible() bool { return t.id != "" && t.dwell >= tipDelay && t.seen }

// tipKeyText is a shortcut as its keycap shows it.
func tipKeyText(k string) string { return strings.ToUpper(k) }

// placement is where the tooltip goes: under its control, or above when
// that would run off the page; never past the side margins.
func (t *tips) placement() rect {
	w := sp3 + textWSmall(t.text) + sp3
	if t.key != "" {
		w += math.Max(pillH, textWScaled(tipKeyText(t.key), dispS)+2*sp2) + sp1
	}
	x := t.at.x
	if x+w > screenW-uiPadX {
		x = screenW - uiPadX - w
	}
	if x < uiPadX {
		x = uiPadX
	}
	y := t.at.y + t.at.h + tipGap
	if y+tipH > screenH-uiPadX {
		y = t.at.y - tipGap - tipH
	}
	return rect{x, y, w, tipH}
}

// draw draws the tooltip (§5): a notched surface, the label, and the
// shortcut in a keycap.
func (t *tips) draw(dst *ebiten.Image) {
	show := t.visible()
	t.seen = false
	if !show {
		return
	}
	r := t.placement()
	if t.key == "" {
		drawKitTooltip(dst, r.x, r.y, t.text)
		return
	}
	drawKitTooltip(dst, r.x, r.y, t.text, tipKeyText(t.key))
}
