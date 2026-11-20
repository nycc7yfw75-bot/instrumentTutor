package ui

import (
	"fmt"
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/practice"
	"github.com/S95F/musicTutor/internal/score"
	"github.com/S95F/musicTutor/internal/scoring"
)

// The pass review (docs/SCORING.md §8, MOTION §B1.7): the last pass's
// numbers, a timing strip with a dot per note, and the tab with every
// ghost. It opens by itself when a full run ends, and on P. Reviewing is
// read-only: nothing here touches XP or stats.

// passReview is a finished pass, kept for the review.
type passReview struct {
	sum        scoring.Summary
	notes      []scoring.Judgement // judged, not excluded
	ghosts     []ghost
	xp         int
	scored     bool
	peak       scoring.Band
	from, to   int64
	label      string // "Bars 3–6 · 70% speed"
	verdictMap map[noteKey]practice.Verdict
}

type reviewState struct {
	ov      overlay
	pass    *passReview
	t       float64 // seconds since it opened
	sweep   bool    // dots sweep in (auto-opened at the end of a run)
	wanted  bool    // P pressed while playing: open once stopped
	view    float64 // the mini tab's left edge, ticks
	viewA   float64 // travel from
	viewB   float64 // travel to
	viewT   float64 // travel progress 0..1
	blink   noteKey
	blinkT  float64 // seconds since a dot click; blinks while < 4 steps
	blinkOn bool
}

const (
	reviewW         = 1080.0
	reviewH         = 620.0
	reviewPad       = 24.0
	reviewStripH    = 190.0
	reviewRangeMs   = 150.0
	reviewSweepSecs = 0.600
	reviewStagger   = 0.090
	reviewPieceIn   = 0.200
	reviewTravel    = 0.350
	reviewBlinkStep = 0.100 // steps2, twice
	reviewTabGap    = 22.0
	reviewDotHit    = 12.0 // half the 24px hit zone
)

func reviewCard() rect {
	return rect{(screenW - reviewW) / 2, (screenH - reviewH) / 2, reviewW, reviewH}
}

func reviewStrip() rect {
	c := reviewCard()
	return rect{c.x + reviewPad, c.y + 96, c.w - 2*reviewPad, reviewStripH}
}

func reviewTab() rect {
	s := reviewStrip()
	return rect{s.x, s.y + s.h + 28, s.w, 5*reviewTabGap + 40}
}

func reviewButtons() (again, closeB rect) {
	c := reviewCard()
	y := c.y + c.h - reviewPad - settingsBtnH
	cw := textW("Close") + 2*settingsBtnPad
	aw := textW("Loop this again") + 2*settingsBtnPad
	closeB = rect{c.x + c.w - reviewPad - cw, y, cw, settingsBtnH}
	again = rect{closeB.x - settingsBtnGap - aw, y, aw, settingsBtnH}
	return
}

// makeReview keeps a finished pass for review; a pass too short to judge
// fairly is not kept.
func (a *App) makeReview(sum scoring.Summary, js []scoring.Judgement, from, to int64) *passReview {
	if _, ok := sum.AccuracyPct(); !ok || to <= from {
		return nil
	}
	p := &passReview{sum: sum, from: from, to: to, verdictMap: map[noteKey]practice.Verdict{}}
	p.peak = scoring.Excitement{Value: a.game.sess.Excitement.Peak}.Band()
	tr := a.displayed()
	for _, j := range js {
		if j.Excluded {
			continue
		}
		p.notes = append(p.notes, j)
		p.verdictMap[noteKey{j.Event.Start, j.Event.String}] = j.Verdict
		bar := tr.Bars[a.barAt(j.Event.Start)]
		if gh, ok := ghostFor(j, a.sc.Tempos, a.eng.TempoScale(), bar.Len()); ok {
			p.ghosts = append(p.ghosts, gh)
		}
	}
	p.label = fmt.Sprintf("Bars %d–%d  ·  %.0f%% speed",
		a.sc.BarNumber(a.barAt(from)), a.sc.BarNumber(a.barAt(max(from, to-1))), a.eng.TempoScale()*100)
	return p
}

func (a *App) openReview(sweep bool) {
	r := &a.game.review
	if r.pass == nil {
		return
	}
	r.ov.set(true)
	r.t, r.sweep = 0, sweep
	r.view = float64(r.pass.from)
	r.viewT = 1
	r.blinkT = math.Inf(1)
}

func (a *App) closeReview() { a.game.review.ov.set(false) }

// toggleReview is the P key: a pass in progress is paused (which files it)
// and the review opens once the transport stops.
func (a *App) toggleReview() {
	r := &a.game.review
	if r.ov.open {
		a.closeReview()
		return
	}
	if a.eng.Playing() {
		r.wanted = true
		a.togglePlay()
		return
	}
	a.openReview(false)
}

// reviewModal handles input while the review is up; it spends the frame's
// input so nothing reaches the practice view behind it.
func (a *App) reviewModal() bool {
	r := &a.game.review
	if !r.ov.open {
		return false
	}
	again, closeB := reviewButtons()
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape), inpututil.IsKeyJustPressed(ebiten.KeyP):
		a.closeReview()
	case a.ptr.pressed && a.ptr.over(closeB):
		a.closeReview()
	case a.ptr.pressed && a.ptr.over(again):
		a.eng.SetLoop(r.pass.from, r.pass.to)
		a.seekTo(r.pass.from)
		a.closeReview()
	case inpututil.IsKeyJustPressed(ebiten.KeyLeft):
		a.scrollReview(-1)
	case inpututil.IsKeyJustPressed(ebiten.KeyRight):
		a.scrollReview(+1)
	case a.ptr.pressed:
		if i, ok := a.reviewDotAt(a.ptr); ok {
			n := r.pass.notes[i]
			a.travelReview(float64(n.Event.Start) - reviewTab().w/2/a.pxPerTick())
			r.blink, r.blinkT = noteKey{n.Event.Start, n.Event.String}, 0
		}
	}
	return true
}

// scrollReview moves the mini tab by one bar.
func (a *App) scrollReview(d int) {
	r := &a.game.review
	bars := a.displayed().Bars
	bi := a.barAt(int64(r.viewB))
	bi = max(0, min(len(bars)-1, bi+d))
	a.travelReview(float64(bars[bi].Start))
}

func (a *App) travelReview(to float64) {
	r := &a.game.review
	lo := float64(r.pass.from)
	hi := math.Max(lo, float64(r.pass.to)-reviewTab().w/a.pxPerTick())
	r.viewA, r.viewB, r.viewT = r.view, math.Max(lo, math.Min(hi, to)), 0
}

// dotXY places a judged note on the timing strip.
func (p *passReview) dotXY(j scoring.Judgement, s rect) (float64, float64) {
	x := s.x + float64(j.Event.Start-p.from)/float64(p.to-p.from)*s.w
	off := 0.0
	if j.Verdict != practice.VerdictMiss {
		off = math.Max(-reviewRangeMs, math.Min(reviewRangeMs, j.OffsetMs))
	}
	return x, s.y + s.h/2 - off/reviewRangeMs*(s.h/2-6)
}

// reviewDotAt finds the dot nearest the pointer within its hit zone.
func (a *App) reviewDotAt(p pointer) (int, bool) {
	r := &a.game.review
	if r.pass == nil {
		return 0, false
	}
	s := reviewStrip()
	best, bestD := -1, reviewDotHit*reviewDotHit
	for i, j := range r.pass.notes {
		x, y := r.pass.dotXY(j, s)
		if d := (x-p.x)*(x-p.x) + (y-p.y)*(y-p.y); d <= bestD {
			best, bestD = i, d
		}
	}
	return best, best >= 0
}

func (a *App) drawReview(screen *ebiten.Image) {
	r := &a.game.review
	if r.pass == nil || !r.ov.visible() {
		return
	}
	dt := uiFrameSeconds()
	r.t += dt
	r.blinkT += dt
	if r.viewT < 1 {
		r.viewT = math.Min(1, r.viewT+dt/reviewTravel)
		r.view = r.viewA + (r.viewB-r.viewA)*curveMove.at(r.viewT)
	}
	card := reviewCard()
	r.ov.draw(screen, card, func(c *ebiten.Image) { a.paintReview(c, card) })
}

func (a *App) paintReview(c *ebiten.Image, card rect) {
	r := &a.game.review
	p := r.pass
	drawPanel(c, card, colPanel, colPanelEdge)
	drawText(c, "Pass review", card.x+reviewPad, card.y+18, colInferred)
	drawTextRight(c, p.label, card.x+card.w-reviewPad, card.y+18, colDim)

	// Header stats arrive as pieces: accuracy, tightness, XP, peak.
	pieces := []string{"Accuracy —", "No timed notes", "Unscored", "Peak: " + p.peak.String()}
	if pct, ok := p.sum.AccuracyPct(); ok {
		pieces[0] = fmt.Sprintf("Accuracy %d%%", pct)
	}
	if t := p.sum.Tightness(); t.N > 0 {
		pieces[1] = t.Label() + " · " + t.SpreadLabel()
	}
	if p.scored {
		pieces[2] = fmt.Sprintf("+%d XP", p.xp)
	}
	x := card.x + reviewPad
	for i, s := range pieces {
		alpha := math.Max(0, math.Min(1, (r.t-float64(i)*reviewStagger)/reviewPieceIn))
		drawText(c, s, x, card.y+54, withAlpha(colNote, alpha))
		x += textW(s) + 40
	}

	a.paintTimingStrip(c, p)
	a.paintReviewTab(c, p)

	again, closeB := reviewButtons()
	for _, b := range []struct {
		r     rect
		label string
	}{{again, "Loop this again"}, {closeB, "Close"}} {
		fill, edge, tc := colPanel, colPanelEdge, colHUD
		if a.ptr.over(b.r) {
			fill, edge, tc = colHover, colDim, colNote
		}
		drawPanel(c, b.r, fill, edge)
		drawText(c, b.label, centreX(b.label, b.r.x, b.r.w), settingsBtnTextY(b.r), tc)
	}
	drawText(c, "← → scroll  ·  click a dot to find the note  ·  esc close", card.x+reviewPad, again.y+6, colHint)
}

func (a *App) paintTimingStrip(c *ebiten.Image, p *passReview) {
	r := &a.game.review
	s := reviewStrip()
	mid := s.y + s.h/2
	yAt := func(ms float64) float64 { return mid - ms/reviewRangeMs*(s.h/2-6) }
	// The ±70 and ±35 ms bands, and the beat itself.
	for _, b := range []struct {
		ms    float64
		alpha float64
	}{{scoring.GoodMs, 0.25}, {scoring.PerfectMs, 0.35}} {
		top, bot := yAt(b.ms), yAt(-b.ms)
		vector.FillRect(c, float32(s.x), float32(top), float32(s.w), float32(bot-top), withAlpha(colPanelEdge, b.alpha), false)
	}
	vector.FillRect(c, float32(s.x), float32(mid), float32(s.w), 1, colString, false)
	drawTextSmall(c, "late", s.x+4, s.y+2, colHint)
	drawTextSmall(c, "early", s.x+4, s.y+s.h-16, colHint)

	hover, hovering := a.reviewDotAt(a.ptr)
	for i, j := range p.notes {
		x, y := p.dotXY(j, s)
		if r.sweep && (x-s.x)/s.w > r.t/reviewSweepSecs {
			continue // the sweep replays the run left to right
		}
		col := verdictColor(j.Verdict)
		fx, fy := float32(math.Round(x)), float32(math.Round(y))
		switch j.Verdict {
		case practice.VerdictMiss:
			drawTextSmall(c, "×", x-3, mid-8, col)
		case practice.VerdictHit:
			vector.FillRect(c, fx-2, fy-2, 5, 5, col, false)
		default:
			vector.StrokeRect(c, fx-2, fy-2, 5, 5, 1, col, false)
		}
		if hovering && i == hover {
			vector.StrokeRect(c, fx-4, fy-4, 9, 9, 1, colNote, false)
		}
	}
	if hovering {
		a.paintDotTip(c, p.notes[hover], s)
	}
}

func (a *App) paintDotTip(c *ebiten.Image, j scoring.Judgement, s rect) {
	tr := a.displayed()
	bi := a.barAt(j.Event.Start)
	bar := tr.Bars[bi]
	beat := float64(j.Event.Start-bar.Start)/float64(4*score.PPQ/int64(bar.Den)) + 1
	when := "missed"
	switch {
	case j.Verdict == practice.VerdictMiss:
	case !j.Timed():
		when = "not timed"
	case math.Abs(j.OffsetMs) < 0.5:
		when = "on the beat"
	case j.OffsetMs > 0:
		when = fmt.Sprintf("%.0f ms late", j.OffsetMs)
	default:
		when = fmt.Sprintf("%.0f ms early", -j.OffsetMs)
	}
	txt := fmt.Sprintf("Bar %d, beat %.4g  ·  string %d, fret %d  ·  %s", a.sc.BarNumber(bi), math.Floor(beat*4)/4, j.Event.String, j.Event.Fret, when)
	w := textW(txt) + 20
	x := math.Min(a.ptr.x+12, s.x+s.w-w)
	y := a.ptr.y - 34
	drawPanel(c, rect{x, y, w, uiTextH + 12}, colPanel, colDim)
	drawText(c, txt, x+10, y+6, colNote)
}

// paintReviewTab draws the pass's notes in its verdict colours, with every
// ghost shown and no fading.
func (a *App) paintReviewTab(c *ebiten.Image, p *passReview) {
	r := &a.game.review
	t := reviewTab()
	clip := c.SubImage(image.Rect(int(t.x), int(t.y), int(t.x+t.w), int(t.y+t.h))).(*ebiten.Image)
	ppt := a.pxPerTick()
	tickToX := func(tick int64) float32 { return float32(t.x + (float64(tick)-r.view)*ppt) }
	top := t.y + 20
	for si := 0; si < 6; si++ {
		y := float32(top + float64(si)*reviewTabGap)
		vector.FillRect(clip, float32(t.x), y, float32(t.w), 1, colString, false)
	}
	tr := a.displayed()
	left, right := int64(r.view), int64(r.view+t.w/ppt)
	blinking := r.blinkT < 4*reviewBlinkStep && int(r.blinkT/reviewBlinkStep)%2 == 0
	for bi, bar := range tr.Bars {
		if bar.Start+bar.Len() < left || bar.Start > right {
			continue
		}
		x := tickToX(bar.Start)
		vector.FillRect(clip, x, float32(top-8), 1, float32(5*reviewTabGap+16), colBarline, false)
		drawTextSmall(clip, fmt.Sprintf("%d", a.sc.BarNumber(bi)), float64(x)+4, top-22, colHint)
		for _, beat := range bar.Beats {
			if beat.Start < p.from || beat.Start >= p.to {
				continue
			}
			for _, n := range beat.Notes {
				if n.String < 1 || n.String > 6 {
					continue
				}
				nx := tickToX(beat.Start)
				ny := float32(top + float64(n.String-1)*reviewTabGap)
				label := fretLabel(n)
				w := float32(textWMono(label))
				col := colDim
				if v, ok := p.verdictMap[noteKey{beat.Start, n.String}]; ok {
					col = verdictColor(v)
				}
				vector.FillRect(clip, nx-2, ny-9, w+4, 18, colPanel, false)
				drawTextMono(clip, label, float64(nx), float64(ny)-9, col)
				if blinking && r.blink == (noteKey{beat.Start, n.String}) {
					vector.StrokeRect(clip, nx-4, ny-11, w+8, 22, 1, colNote, false)
				}
			}
		}
	}
	for _, gh := range p.ghosts {
		ny := float32(top + float64(gh.str-1)*reviewTabGap)
		drawGhost(clip, gh, ghostAlpha, tickToX(gh.start), tickToX(gh.start+gh.offTicks), ny)
	}
}
