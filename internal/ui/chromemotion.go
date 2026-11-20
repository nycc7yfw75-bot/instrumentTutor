package ui

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The practice chrome's small motions (docs/MOTION.md §A, the rows for the
// LIVE pill, LIVE ↔ NO SIGNAL, the input meter and the section label).
// They run off the UI frame clock only and read nothing the view does not
// already read.

// crossfade replaces one label with another: the old fades out while the
// new fades in over durCrossfade (instant under reduced motion).
type crossfade struct {
	cur, prev string
	t         float64
	started   bool
}

// set names the label to show now.
func (c *crossfade) set(s string) {
	if !c.started {
		c.cur, c.t, c.started = s, durCrossfade, true
		return
	}
	if s != c.cur {
		c.prev, c.cur, c.t = c.cur, s, 0
	}
}

// step advances by dt and returns how far in the new label is (0..1).
func (c *crossfade) step(dt float64) float64 {
	c.t += dt
	return math.Min(1, c.t/durCrossfade)
}

// peakHold is the input meter's peak block: it holds 600ms, then falls a
// block every 50ms (stepped).
type peakHold struct {
	block      int
	held, fall float64
}

const (
	peakHoldFor  = 0.6
	peakFallStep = 0.05
)

// update takes the lit block count this frame.
func (p *peakHold) update(lit int, dt float64) {
	if lit >= p.block {
		p.block, p.held, p.fall = lit, 0, 0
		return
	}
	p.held += dt
	if p.held < peakHoldFor {
		return
	}
	p.fall += math.Min(dt, p.held-peakHoldFor) // only the time past the hold
	for p.fall >= peakFallStep-1e-9 && p.block > lit {
		p.block--
		p.fall -= peakFallStep
	}
}

type chromeMotion struct {
	clock   float64 // the LIVE dot's ambient pulse
	live    crossfade
	section crossfade
	peak    peakHold
}

// liveDotOn is the LIVE dot's 2-step pulse: full for half a second, dim
// for the other half; steady under reduced motion.
func (m *chromeMotion) liveDotOn() bool {
	return reduceMotion || math.Mod(m.clock, 1) < 0.5
}

// segBlockCol is a meter block's colour when lit (§5): hit, close near the
// top, miss for the top block.
func segBlockCol(i, n int) color.RGBA {
	switch {
	case i == n-1:
		return colMiss
	case i >= n*3/4:
		return colClose
	}
	return colHit
}

// drawPeakBlock lights the held peak block above the live level.
func drawPeakBlock(dst *ebiten.Image, x, y float64, n, peak, lit int) {
	if peak <= lit || peak < 1 || peak > n {
		return
	}
	i := peak - 1
	bx := x + float64(i)*(meterBlockW+meterGap)
	vector.FillRect(dst, float32(bx), float32(y), meterBlockW, meterBlockH, segBlockCol(i, n), false)
}
