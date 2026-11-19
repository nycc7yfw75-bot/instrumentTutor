package ui

import (
	"image/color"
	"time"
)

// Motion tokens. Hover is a colour change only (about 130ms to arrive); a press
// shrinks small controls to animPressScale and only shades wide surfaces,
// where a scale would read as a jump.
const (
	animHoverRate = 22.0
	animPressRate = 34.0

	animForgetFrames = 120

	animPressScale = 0.97
	animWideW      = 200.0
)

type animValues struct {
	hover float64
	press float64
}

type animState struct {
	animValues
	down bool
	seen uint64
}

type animator struct {
	states map[string]*animState
	frame  uint64
}

func (a *animator) tick() {
	measureFrame()
	a.frame++
	for id, st := range a.states {
		if a.frame-st.seen > animForgetFrames {
			delete(a.states, id)
		}
	}
}

func (a *animator) step(id string, hovered, down bool, dt float64) animValues {
	if a.states == nil {
		a.states = map[string]*animState{}
	}
	st, ok := a.states[id]
	if !ok {
		st = &animState{}
		a.states[id] = st

		if hovered {
			st.hover = 1
		}
	}
	st.seen = a.frame

	pressed := hovered && down
	st.down = pressed

	st.hover = ease(st.hover, boolTo(hovered), animHoverRate, dt)
	st.press = ease(st.press, boolTo(pressed), animPressRate, dt)
	return st.animValues
}

func ease(v, target, rate, dt float64) float64 {
	k := rate * dt
	if k > 1 {
		k = 1
	}
	if k < 0 {
		return v
	}
	return v + (target-v)*k
}

func boolTo(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

var (
	uiFrameDT   = 1.0 / playheadFallbackTPS
	uiFrameLast time.Time
)

func uiFrameSeconds() float64 { return uiFrameDT }

func measureFrame() {
	now := time.Now()
	if uiFrameLast.IsZero() {
		uiFrameLast = now
		return
	}
	dt := now.Sub(uiFrameLast).Seconds()
	uiFrameLast = now

	if dt < 1.0/480 {
		dt = 1.0 / 480
	}
	if dt > 1.0/20 {
		dt = 1.0 / 20
	}
	uiFrameDT = dt
}

func lerpCol(a, b color.RGBA, t float64) color.RGBA {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	mix := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.RGBA{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), mix(a.A, b.A)}
}

func withAlpha(c color.RGBA, alpha float64) color.RGBA {
	if alpha <= 0 {
		return color.RGBA{}
	}
	if alpha > 1 {
		alpha = 1
	}
	scale := func(v uint8) uint8 { return uint8(float64(v) * alpha) }
	return color.RGBA{scale(c.R), scale(c.G), scale(c.B), scale(uint8(255 * alpha))}
}

func (av animValues) animate(r rect) rect {
	if r.w >= animWideW || av.press <= 0 {
		return r
	}
	s := 1 - (1-animPressScale)*av.press
	w, h := r.w*s, r.h*s
	return rect{r.x + (r.w-w)/2, r.y + (r.h-h)/2, w, h}
}

// pressShade darkens a wide surface while it is held, its press feedback.
func pressShade(c color.RGBA, av animValues) color.RGBA {
	return lerpCol(c, colBG, 0.6*av.press)
}
