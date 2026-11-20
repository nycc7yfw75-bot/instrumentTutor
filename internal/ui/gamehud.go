package ui

import (
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/scoring"
)

// The scoring HUD on the practice view: the excitement bar, level and XP,
// the XP toast and the level-up card. Motion follows docs/MOTION.md §B1.
// Colours are named here, from the theme, so a restyle touches one place.
var (
	colExciteCold  = colString // text3
	colExciteWarm  = colDim    // text2
	colExciteHot   = colHit
	colExciteFire2 = colNote // the On-fire flicker's second tint
	colBlockOff    = colPanelEdge
	colXPFill      = colInferred
)

const (
	exciteBlocks   = 20
	exciteBlockW   = 8.0
	exciteBlockH   = 12.0
	exciteBlockGap = 2.0

	xpBlocks   = 12
	xpBlockW   = 10.0
	xpBlockH   = 6.0
	xpBlockGap = 2.0

	popSeconds       = 0.100        // a block lighting: steps2
	sweepPerBlock    = 0.008        // band change re-tint, left to right
	labelSwapSeconds = durCrossfade // band label crossfade
	flickerPeriod    = 0.600        // On fire
	streakSeconds    = 0.300        // "+5" rising
	moveSeconds      = durMove      // XP bar fill

	toastIn   = durToastIn
	toastHold = 1.200
	toastOut  = durToastOut
	rollSecs  = durRoll
)

// exciteView animates the excitement bar.
type exciteView struct {
	clock     float64 // UI seconds
	flicker   float64 // advances only while playing (frozen otherwise)
	lit       int
	popUntil  [exciteBlocks]float64
	band      scoring.Band
	prevBand  scoring.Band
	bandSince float64
	streaks   []float64 // start times of "+5" pops
}

func blocksLit(v float64) int {
	return min(exciteBlocks, int(v/(scoring.ExcitementMax/exciteBlocks)))
}

// note sees a judged note land; a jump of the streak bonus size starts a
// "+5" pop.
func (v *exciteView) note(before, after float64) {
	if after-before >= scoring.StreakBonus && !reduceMotion {
		v.streaks = append(v.streaks, v.clock)
	}
}

func (v *exciteView) frame(e *scoring.Excitement, playing bool) {
	dt := uiFrameSeconds()
	v.clock += dt
	if playing && !reduceMotion {
		v.flicker += dt
	}
	lit := blocksLit(e.Value)
	for i := v.lit; i < lit && !reduceMotion; i++ {
		v.popUntil[i] = v.clock + popSeconds // a rise pops; a drain is instant
	}
	v.lit = lit
	if b := e.Band(); b != v.band {
		v.prevBand, v.band, v.bandSince = v.band, b, v.clock
		if reduceMotion {
			v.bandSince = -1 // no sweep, no crossfade
		}
	}
	keep := v.streaks[:0]
	for _, s := range v.streaks {
		if v.clock-s < streakSeconds {
			keep = append(keep, s)
		}
	}
	v.streaks = keep
}

func bandColour(b scoring.Band) color.RGBA {
	return [...]color.RGBA{colExciteCold, colExciteWarm, colExciteHot, colExciteHot}[b]
}

// draw draws the bar at x, y and returns its right edge.
func (v *exciteView) draw(dst *ebiten.Image, x, y float64) float64 {
	for i := 0; i < exciteBlocks; i++ {
		bx := x + float64(i)*(exciteBlockW+exciteBlockGap)
		col := colBlockOff
		if i < v.lit {
			col = bandColour(v.band)
			if v.clock-v.bandSince < float64(i)*sweepPerBlock {
				col = bandColour(v.prevBand) // the re-tint has not reached this block yet
			}
			if v.band == scoring.OnFire && i >= v.lit-3 && !reduceMotion {
				phase := v.flicker/flickerPeriod*2*math.Pi + float64(i)*0.9
				if math.Sin(phase) > 0 {
					col = colExciteFire2
				}
			}
		}
		top, h := y, exciteBlockH
		if v.clock < v.popUntil[i] {
			top, h = y-1, exciteBlockH+1 // the pop: one pixel taller
		}
		vector.FillRect(dst, float32(bx), float32(top), exciteBlockW, float32(h), col, false)
	}
	right := x + exciteBlocks*(exciteBlockW+exciteBlockGap)
	for _, s := range v.streaks {
		u := (v.clock - s) / streakSeconds
		rise := math.Floor(u * 8) // whole pixels
		drawTextSmall(dst, "+5", right-18, y-14-rise, withAlpha(colExciteHot, 1-u))
	}
	// The band name: the new one fades in while the old fades out.
	lx := right + 8
	t := math.Min(1, (v.clock-v.bandSince)/labelSwapSeconds)
	if t < 1 {
		drawText(dst, v.prevBand.String(), lx, y-3, withAlpha(colHUD, 1-t))
	}
	drawText(dst, v.band.String(), lx, y-3, withAlpha(colHUD, t))
	return lx + textW("On fire")
}

// xpBarView animates the XP bar toward the player's progress in the level.
type xpBarView struct {
	from, to, shown float64
	t               float64
}

func xpFraction(total int) float64 {
	_, into, next := scoring.LevelFor(total)
	return float64(into) / float64(next)
}

func (x *xpBarView) snap(total int) {
	x.from, x.to, x.shown, x.t = 0, xpFraction(total), xpFraction(total), 1
}

// aim starts the fill toward total, from where it is now; a level-up fills
// the bar first (the reset happens behind the level card).
func (x *xpBarView) aim(total int, leveledUp bool) {
	x.from, x.t = x.shown, 0
	x.to = xpFraction(total)
	if leveledUp {
		x.to = 1
	}
}

func (x *xpBarView) step() {
	if x.t < 1 {
		x.t = math.Min(1, x.t+motionDt()/moveSeconds)
	}
	x.shown = x.from + (x.to-x.from)*curveMove.at(x.t)
}

func (x *xpBarView) draw(dst *ebiten.Image, right, y float64) float64 {
	w := xpBlocks*(xpBlockW+xpBlockGap) - xpBlockGap
	left := right - w
	filled := x.shown * xpBlocks
	for i := 0; i < xpBlocks; i++ {
		bx := left + float64(i)*(xpBlockW+xpBlockGap)
		vector.FillRect(dst, float32(bx), float32(y), xpBlockW, xpBlockH, colBlockOff, false)
		if f := math.Min(1, filled-float64(i)); f > 0 {
			vector.FillRect(dst, float32(bx), float32(y), float32(xpBlockW*f), xpBlockH, colXPFill, false)
		}
	}
	return left
}

// xpToast is the quiet "+42 XP" after a pass (MOTION §B1.5).
type xpToast struct {
	xp int
	t  float64
}

func (t *xpToast) done() bool { return t.t >= toastIn+toastHold+toastOut }

func (t *xpToast) draw(dst *ebiten.Image, right, y float64) {
	t.t += uiFrameSeconds()
	if reduceMotion && t.t < toastIn+rollSecs {
		t.t = toastIn + rollSecs // shown whole at once
	}
	if reduceMotion && t.t > toastIn+toastHold {
		t.t = toastIn + toastHold + toastOut // gone at once
		return
	}
	alpha, lift := 1.0, 0.0
	switch {
	case t.t < toastIn:
		u := curveEnter.at(t.t / toastIn)
		alpha, lift = u, 8*(1-u)
	case t.t > toastIn+toastHold:
		u := curveExit.at((t.t - toastIn - toastHold) / toastOut)
		alpha, lift = 1-u, -4*u
	}
	shown := int(math.Round(float64(t.xp) * curveEnter.at(math.Min(1, t.t/rollSecs))))
	s := fmt.Sprintf("+%d XP", shown)
	drawTextRight(dst, s, right, y+lift, withAlpha(colXPFill, alpha))
}

// levelCard is the level-up moment (MOTION §B1.6), shown only when the
// player has stopped; Space or a click jumps to its exit.
type levelCard struct {
	level        int
	title        string
	titleChanged bool
	t            float64 // seconds
}

func newLevelCard(level int, title string, titleChanged bool) *levelCard {
	return &levelCard{level: level, title: title, titleChanged: titleChanged}
}

const (
	lvScrimAt   = 0.300
	lvCardAt    = 0.350
	lvCardFor   = 0.350
	lvPopAt     = 0.600
	lvSparkAt   = 0.700
	lvTitleAt   = 0.900
	lvExitAt    = 2.300
	lvExitFor   = 0.180
	lvStepFrame = 0.050 // one step of a pixel pop (3 frames at 60 fps)
)

func (c *levelCard) done() bool { return c.t >= lvExitAt+lvExitFor }

func (c *levelCard) skip() {
	if c.t < lvExitAt {
		c.t = lvExitAt
	}
}

func (c *levelCard) draw(dst *ebiten.Image) {
	c.t += uiFrameSeconds()
	if reduceMotion {
		if c.t < lvTitleAt+0.2 {
			c.t = lvTitleAt + 0.2 // the end state, at once
		}
		if c.t > lvExitAt {
			c.t = lvExitAt + lvExitFor
			return
		}
	}
	exit := 0.0
	if c.t > lvExitAt {
		exit = curveExit.at(math.Min(1, (c.t-lvExitAt)/lvExitFor))
	}
	scrim := math.Min(1, math.Max(0, (c.t-lvScrimAt)/0.2)) * (1 - exit)
	vector.FillRect(dst, 0, 0, screenW, screenH, withAlpha(colBG, 0.4*scrim), false)

	if c.t < lvCardAt {
		return
	}
	in := curveEnter.at(math.Min(1, (c.t-lvCardAt)/lvCardFor))
	alpha := in * (1 - exit)
	w, h := 420.0, 180.0
	s := overlayFromScale + (1-overlayFromScale)*in
	cw, ch := w*s, h*s
	card := rect{(screenW - cw) / 2, (screenH-ch)/2 + 16*(1-in) + 4*exit, cw, ch}
	drawPanel(dst, card, withAlpha(colPanel, alpha), withAlpha(colPanelEdge, alpha))

	cx, cy := card.x+card.w/2, card.y+card.h/2
	if c.t >= lvPopAt {
		// "LEVEL 8" pops in three held steps.
		scale := dispL
		if step := int((c.t - lvPopAt) / lvStepFrame); step < 2 {
			scale = []float64{dispX, dispX}[step]
		}
		txt := fmt.Sprintf("LEVEL %d", c.level)
		drawTextScaled(dst, txt, centreXScaled(txt, card.x, card.w, scale), cy-30, scale, withAlpha(colInferred, alpha))
	}
	if c.t >= lvSparkAt && c.t < lvSparkAt+4*lvStepFrame {
		step := float64(int((c.t-lvSparkAt)/lvStepFrame) + 1)
		for i := 0; i < 8; i++ {
			ang := float64(i) * math.Pi / 4
			px := math.Round(cx + math.Cos(ang)*(90+3*step))
			py := math.Round(cy - 14 + math.Sin(ang)*(40+3*step))
			vector.FillRect(dst, float32(px), float32(py), 3, 3, withAlpha(colSounding, alpha*(1-step/5)), false)
		}
	}
	if c.titleChanged && c.t >= lvTitleAt {
		ta := math.Min(1, (c.t-lvTitleAt)/0.2) * alpha
		drawText(dst, c.title, centreX(c.title, card.x, card.w), cy+22, withAlpha(colNote, ta))
	}
}
