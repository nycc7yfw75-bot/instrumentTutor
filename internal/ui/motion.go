package ui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"math"
)

// Motion tokens (docs/MOTION.md §1), in seconds. Every animation uses these.
const (
	durPress      = 0.100
	durHover      = 0.150
	durTooltipIn  = 0.125
	durTooltipOut = 0.090
	durPopIn      = 0.200
	durPopOut     = 0.140
	durOverlayIn  = 0.260
	durOverlayOut = 0.180
	durToastIn    = 0.240
	durToastOut   = 0.170
	durMove       = 0.250
	durTravel     = 0.350
	durRoll       = 0.300
	durRollBig    = 0.400
	durMomentStep = 0.500
	durMomentMax  = 2.500
	durCrossfade  = 0.200 // replacement: old fades out while new fades in

	staggerCopy  = 0.040
	staggerPiece = 0.090

	// A pixel step holds 3 frames at 60 fps (steps2/steps3/steps4).
	stepHold = 0.050
)

const (
	overlayInSeconds  = durOverlayIn
	overlayOutSeconds = durOverlayOut
	overlayFromScale  = 0.97
	overlayScrimAlpha = 0.40
)

// stepIndex is the frame of a pixel-step animation (steps2..steps4) t
// seconds in: 0..n-1, or n once it has finished.
func stepIndex(t float64, n int) int {
	if t < 0 {
		return 0
	}
	return min(n, int(t/stepHold))
}

// reduceMotion skips every transition (MOTION §A4): end states appear at
// once. The playhead and meters are time, not motion, and are unaffected.
var reduceMotion bool

// SetReduceMotion turns reduced motion on or off for the whole app.
func SetReduceMotion(on bool) { reduceMotion = on }

// ReduceMotion reports whether reduced motion is on.
func ReduceMotion() bool { return reduceMotion }

// motionDt is the frame's step for a transition: a whole transition at
// once under reduced motion.
func motionDt() float64 {
	if reduceMotion {
		return 1e6
	}
	return uiFrameSeconds()
}

// spring is a critically damped spring (MOTION §1): no bounce, settling in
// about 500ms. Used for released drags.
type spring struct{ x, v float64 }

const springOmega = 13.3 // (1+ωt)e^(-ωt) = 1% at t = 500ms

func (s *spring) step(target, dt float64) float64 {
	if reduceMotion {
		s.x, s.v = target, 0
		return s.x
	}
	// Exact critically damped step: x(t) = target + (c1 + c2 t) e^{-ωt}.
	c1 := s.x - target
	c2 := s.v + springOmega*c1
	e := math.Exp(-springOmega * dt)
	s.x = target + (c1+c2*dt)*e
	s.v = (c2 - springOmega*(c1+c2*dt)) * e
	return s.x
}

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
	used     bool // set once: a presence never opened stays gone
	progress float64 // linear time through the current direction's curve
	value    float64 // eased, what gets drawn
}

func (p *presence) set(open bool) {
	if open == p.open {
		return
	}
	p.used = true
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
	if !p.used {
		p.value = 0 // never opened: nothing to fade out
		return
	}
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
	o.step(motionDt())
	if !o.visible() {
		return
	}
	v := o.value
	vector.DrawFilledRect(screen, 0, 0, float32(screenW), float32(screenH), withAlpha(colScrim, overlayScrimAlpha*v), false)
	if o.canvas == nil || o.canvas.Bounds().Dx() != int(screenW) || o.canvas.Bounds().Dy() != int(screenH) {
		o.canvas = ebiten.NewImage(int(screenW), int(screenH)) // follows the window
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
