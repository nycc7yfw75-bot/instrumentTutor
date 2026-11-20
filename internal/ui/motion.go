package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Motion tokens for overlays that enter and leave (a modal-sized panel:
// 200–300ms in on an ease-out, 20–30% shorter out on an ease-in).
const (
	overlayInSeconds  = 0.24
	overlayOutSeconds = 0.18
	overlayFromScale  = 0.97
	overlayScrimAlpha = 0.45
)

// curve is a CSS-style cubic-bezier(x1, y1, x2, y2) timing function.
type curve struct{ x1, y1, x2, y2 float64 }

var (
	curveEnter = curve{0.32, 0.72, 0, 1}  // appearing
	curveExit  = curve{0.55, 0, 1, 0.45}  // leaving
	curveMove  = curve{0.77, 0, 0.175, 1} // moving or resizing in place
)

func bez(a, b, t float64) float64 {
	// One axis of a cubic bezier from (0,0) to (1,1) with control points a, b.
	u := 1 - t
	return 3*u*u*t*a + 3*u*t*t*b + t*t*t
}

// at maps linear progress x in [0,1] to eased progress.
func (c curve) at(x float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	lo, hi := 0.0, 1.0
	for i := 0; i < 30; i++ {
		mid := (lo + hi) / 2
		if bez(c.x1, c.x2, mid) < x {
			lo = mid
		} else {
			hi = mid
		}
	}
	return bez(c.y1, c.y2, (lo+hi)/2)
}

// inverse finds the linear progress at which the curve reaches v.
func (c curve) inverse(v float64) float64 {
	lo, hi := 0.0, 1.0
	for i := 0; i < 30; i++ {
		mid := (lo + hi) / 2
		if c.at(mid) < v {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2
}

// presence animates an overlay in and out. Its value is 0 (gone) to 1
// (fully shown); turning it around mid-way carries on from where it is.
type presence struct {
	open     bool
	progress float64 // linear time through the current direction's curve
	value    float64 // eased, what gets drawn
}

func (p *presence) set(open bool) {
	if open == p.open {
		return
	}
	p.open = open
	// Re-enter the other curve at the point that draws the same value, so a
	// reversal never jumps.
	if open {
		p.progress = curveEnter.inverse(p.value)
	} else {
		p.progress = curveExit.inverse(1 - p.value)
	}
}

func (p *presence) step(dt float64) {
	if p.open {
		p.progress += dt / overlayInSeconds
		if p.progress > 1 {
			p.progress = 1
		}
		p.value = curveEnter.at(p.progress)
		return
	}
	p.progress += dt / overlayOutSeconds
	if p.progress > 1 {
		p.progress = 1
	}
	p.value = 1 - curveExit.at(p.progress)
}

func (p *presence) visible() bool { return p.open || p.value > 0 }

// overlay is a pop-up card over a dimmed screen: it fades and grows in from
// overlayFromScale about the card's centre, and shrinks back out.
type overlay struct {
	presence
	canvas *ebiten.Image
}

// draw advances the animation and, while visible, dims the screen and draws
// the card that paint draws onto a full-screen canvas.
func (o *overlay) draw(screen *ebiten.Image, card rect, paint func(c *ebiten.Image)) {
	o.step(uiFrameSeconds())
	if !o.visible() {
		return
	}
	v := o.value
	vector.DrawFilledRect(screen, 0, 0, screenW, screenH, withAlpha(color.RGBA{0, 0, 0, 255}, overlayScrimAlpha*v), false)
	if o.canvas == nil {
		o.canvas = ebiten.NewImage(screenW, screenH)
	}
	o.canvas.Clear()
	paint(o.canvas)

	s := overlayFromScale + (1-overlayFromScale)*v
	cx, cy := card.x+card.w/2, card.y+card.h/2
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-cx, -cy)
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(cx, cy)
	op.ColorScale.ScaleAlpha(float32(v))
	op.Filter = ebiten.FilterLinear
	screen.DrawImage(o.canvas, op)
}
