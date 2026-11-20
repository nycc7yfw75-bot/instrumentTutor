package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/score"
)

const (
	ptTransportY = 58.0
	ptTracksY    = 100.0
	ptWarnY      = 160.0
	ptWarnH      = 60.0
	ptTimelineH  = 36.0

	ptTabHeadroom = 34.0

	ptTabFooting = 26.0

	ptBtnW = 36.0

	ptBtnH   = chipH
	ptBtnGap = 6.0

	ptGrab = 12.0 // a loop handle's hit zone is 24px wide (§6.6)

	ptTrackChipW = 150.0
)

type dragTarget int

const (
	dragNone dragTarget = iota
	dragSeekTab
	dragSeekTimeline
	dragLoopA
	dragLoopB

	dragLoopNew
)

type practiceChip struct {
	label string

	// icon is its pixel icon (§5).
	icon iconID

	// key is the shortcut, shown in the item's tooltip.
	key string

	// right puts the item in the right-aligned group.
	right bool

	on func(a *App) bool

	disabled func(a *App) bool

	whenOff func(a *App) string

	act func(a *App)
}

var practiceChips = []practiceChip{
	{label: "Loop", icon: icLoop, key: "A / B, L clears",
		on:  func(a *App) bool { _, _, on := a.eng.Loop(); return on },
		act: func(a *App) { a.toggleLoop() }},
	{label: "Click", icon: icMetronome, key: "M",
		on:  func(a *App) bool { return a.metronome },
		act: func(a *App) { a.toggleMetronome() }},
	{label: "Ramp", icon: icRamp, key: "R",
		on:  func(a *App) bool { return a.ramp },
		act: func(a *App) { a.toggleRamp() }},
	{label: "Count-in", icon: icCountIn, key: "C",
		on:  func(a *App) bool { return a.CountInBeats() > 0 },
		act: func(a *App) { a.toggleCountIn() }},
	{label: "Wait", icon: icWait, key: "W",
		on:       func(a *App) bool { return a.wait },
		disabled: func(a *App) bool { return !a.waitCtl },
		whenOff:  func(a *App) string { return "wait mode needs live input — " + a.liveRemedy() },
		act:      func(a *App) { a.toggleWait() }},
	{label: "Tuner", icon: icTuner, key: "T", right: true,
		on:  func(a *App) bool { return a.tunerView },
		act: func(a *App) { a.tunerView = !a.tunerView }},
	{label: "Mixer", icon: icMixer, key: "V", right: true,
		on:  func(a *App) bool { return a.mixer.open },
		act: func(a *App) { a.toggleMixer() }},
	{label: "Settings", icon: icSettings, key: "S", right: true,
		disabled: func(a *App) bool { return a.settings == nil },
		whenOff:  func(a *App) string { return "settings need the full app — start HEADLINER without a file" },
		act:      func(a *App) { a.openSettings() }},
	{label: "Help", icon: icHelp, key: "?", right: true,
		on:  func(a *App) bool { return a.helpOpen },
		act: func(a *App) { a.openHelp() }},
}

func (a *App) chipStates() []chipState {
	out := make([]chipState, len(practiceChips))
	for i, c := range practiceChips {
		out[i] = chipState{
			label:    c.label,
			key:      c.key,
			on:       c.on != nil && c.on(a),
			disabled: c.disabled != nil && c.disabled(a),
		}
	}
	return out
}

// backChip is the top-left button that does what esc does.
func (a *App) backChip() chipState {
	if a.quitAll == nil {
		return escQuitChip
	}
	return escBackChip
}

// titleX is where the piece title starts, right of the back button.
func (a *App) titleX() float64 {
	r := backButtonRect(a.backChip())
	return r.x + r.w + 14
}

type practiceLayout struct {
	transport [4]rect

	// tempo is the tempo control: slower, the readout (exact BPM), faster.
	tempo [3]rect

	// dividers are the x positions of the thin lines between menu groups.
	dividers []float64

	chips []rect

	tracks []rect

	tab rect

	timeline rect

	loopA, loopB rect

	tlLoopA, tlLoopB rect
}

func (a *App) tabBottom() float64 {
	return a.tabTop() + float64(a.laneLines()-1)*a.stringGap()
}

// tabRect is the tab's clickable area: its whole zone.
func (a *App) tabRect() rect { return a.tabZone() }

// toolsLabelled: the Tools show their labels only on a wide window (§6.2).
func toolsLabelled() bool { return screenW >= 1600 }

// chipW is a toolbar item's width: icon and label, or the icon alone for a
// Tool on a narrow window.
func toolChipW(c practiceChip) float64 {
	if c.right && !toolsLabelled() {
		return zItemH
	}
	return kitMenuItemW(c.icon, c.label)
}

func (a *App) layout() practiceLayout {
	var l practiceLayout
	y := zToolbarY() + (zToolbarH-zItemH)/2

	// Transport (icons) | Loop Click Ramp Count-in Wait | ‹ tempo ›, and
	// Tuner Mixer Settings Help at the right (§6.2).
	x := pgMargin - (zItemH-iconSize)/2 // the first icon's ink on the margin
	for i := range l.transport {
		l.transport[i] = rect{x, y, zItemH, zItemH}
		x += zItemH
	}
	divide := func() {
		x += sp6
		l.dividers = append(l.dividers, x)
		x += sp6
	}
	divide()
	l.chips = make([]rect, len(practiceChips))
	for i, c := range practiceChips {
		if c.right {
			continue
		}
		w := toolChipW(c)
		l.chips[i] = rect{x, y, w, zItemH}
		x += w + sp1
	}
	divide()
	l.tempo[0] = rect{x, y, zItemH, zItemH}
	l.tempo[1] = rect{x + zItemH, y, tempoReadoutW(), zItemH}
	l.tempo[2] = rect{x + zItemH + tempoReadoutW(), y, zItemH, zItemH}

	rx := screenW - pgMargin + (zItemH-iconSize)/2
	for i := len(practiceChips) - 1; i >= 0; i-- {
		c := practiceChips[i]
		if !c.right {
			continue
		}
		w := toolChipW(c)
		rx -= w
		l.chips[i] = rect{rx, y, w, zItemH}
		rx -= sp1
	}

	// Track tags: yours first, then the rest, while they fit (§6.3).
	l.tracks = make([]rect, len(a.sc.Tracks))
	names := a.shortTrackNames()
	tx := pgMargin + textWScaled(spaced("TRACKS"), dispS) + sp6
	ty := zTrackBarY() + (zTrackBarH-zTrackTagH)/2
	for _, i := range a.trackOrder() {
		w := a.trackTagW(i, names[i])
		if tx+w > a.trackStripRight() {
			break
		}
		l.tracks[i] = rect{tx, ty, w, zTrackTagH}
		tx += w + sp6
	}

	l.tab = a.tabRect()
	l.timeline = rect{pgMargin, timelineZoneY() + sp2, screenW - 2*pgMargin, zTlTrackH}

	if la, lb, on := a.eng.Loop(); on {

		if !a.tunerView {
			l.loopA = a.loopHandleOnTab(la)
			l.loopB = a.loopHandleOnTab(lb)
		}
		l.tlLoopA = a.loopHandleOnTimeline(l.timeline, la)
		l.tlLoopB = a.loopHandleOnTimeline(l.timeline, lb)
	}
	return l
}

// trackStripRight is where the track tags stop: before the HUD.
func (a *App) trackStripRight() float64 {
	if a.live {
		return screenW - pgMargin - hudW
	}
	return screenW - pgMargin - textW("Not listening") - sp6
}

func (a *App) loopHandleOnTab(tick int64) rect {
	x := a.xAtTick(tick)
	if x < tabLeft()-ptGrab || x > tabRight()+ptGrab {
		return rect{}
	}
	t := a.tabRect()
	return rect{x - ptGrab, t.y, 2 * ptGrab, t.h}
}

func (a *App) loopHandleOnTimeline(tl rect, tick int64) rect {
	x := a.timelineX(tl, tick)
	return rect{x - ptGrab, tl.y - 4, 2 * ptGrab, tl.h + 8}
}

func (a *App) xAtTick(tick int64) float64 {
	return playheadXAt() + (float64(tick)-a.posF())*a.pxPerTick()
}

func (a *App) tickAtX(x float64) int64 {
	return int64(math.Round(a.posF() + (x-playheadXAt())/a.pxPerTick()))
}

func (a *App) pieceEnd() int64 {
	bars := a.displayed().Bars
	if len(bars) == 0 {
		return 1
	}
	last := bars[len(bars)-1]
	if end := last.Start + last.Len(); end > 0 {
		return end
	}
	return 1
}

func (a *App) timelineX(tl rect, tick int64) float64 { return a.timelineXf(tl, float64(tick)) }

func (a *App) timelineXf(tl rect, tick float64) float64 {
	f := tick / float64(a.pieceEnd())
	if f < 0 {
		f = 0
	} else if f > 1 {
		f = 1
	}
	return tl.x + f*tl.w
}

func (a *App) timelineTick(tl rect, x float64) int64 {
	if tl.w <= 0 {
		return 0
	}
	return clampTick((x-tl.x)/tl.w*float64(a.pieceEnd()), a.pieceEnd())
}

func clampTick(t float64, end int64) int64 {
	if t < 0 {
		return 0
	}
	if int64(t) > end {
		return end
	}
	return int64(t)
}

func (a *App) snapToBeat(tick int64) int64 {
	best, found := int64(0), false
	consider := func(t int64) {
		if !found || abs64(t-tick) < abs64(best-tick) {
			best, found = t, true
		}
	}
	for _, bar := range a.displayed().Bars {
		consider(bar.Start)
		consider(bar.Start + bar.Len())
		for _, beat := range bar.Beats {
			consider(beat.Start)
		}
	}
	if !found {
		return tick
	}
	return best
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func (a *App) seekTo(tick int64) {
	if tick < 0 {
		tick = 0
	}
	if end := a.pieceEnd(); tick > end {
		tick = end
	}
	if tick != a.eng.PosTick() {
		a.eng.SeekTick(tick)
	}
}

func (a *App) handleMouse(p pointer, shift bool) {
	if a.drag != dragNone {

		if !p.down {
			a.drag = dragNone
			return
		}
		a.continueDrag(p, shift)
		return
	}
	l := a.layout()

	if p.wheel != 0 && !a.tunerView && p.over(l.tab) {
		a.wheelAcc += p.wheel
		var steps int
		steps, a.wheelAcc = wheelSteps(a.wheelAcc)
		for ; steps > 0; steps-- {
			a.zoomIn()
		}
		for ; steps < 0; steps++ {
			a.zoomOut()
		}
	}
	if !p.pressed && !p.right {
		return
	}
	if a.hitChrome(p, l) {
		return
	}

	if !p.pressed {
		return
	}

	switch {
	case p.over(l.loopA) || p.over(l.tlLoopA):
		a.drag = dragLoopA
	case p.over(l.loopB) || p.over(l.tlLoopB):
		a.drag = dragLoopB
	case shift && p.over(l.timeline):
		a.beginLoopDraw(a.timelineTick(l.timeline, p.x), p.x, true)
	case shift && p.over(l.tab) && !a.tunerView:
		a.beginLoopDraw(a.tickAtX(p.x), p.x, false)
	case p.over(l.timeline):
		a.drag = dragSeekTimeline
		a.continueDrag(p, shift)
	case p.over(l.tab) && !a.tunerView:
		a.drag = dragSeekTab
		a.continueDrag(p, shift)
	}
}

func (a *App) beginLoopDraw(tick int64, x float64, onTimeline bool) {
	a.drag = dragLoopNew
	a.loopDrawTick, a.loopDrawX, a.loopDrawing = tick, x, false
	a.loopDrawOnTimeline = onTimeline

	a.loopDrawPxPerTick = a.pxPerTick()
	a.seekTo(tick)
}

const loopDrawSlop = 4.0

func (a *App) continueLoopDraw(p pointer, l practiceLayout) {
	if !a.loopDrawing && math.Abs(p.x-a.loopDrawX) <= loopDrawSlop {
		return
	}
	a.loopDrawing = true

	var cur int64
	if a.loopDrawOnTimeline {
		cur = a.timelineTick(l.timeline, p.x)
	} else {
		cur = a.loopDrawTick + int64(math.Round((p.x-a.loopDrawX)/a.loopDrawPxPerTick))
	}
	lo, hi := a.loopDrawTick, cur
	if hi < lo {
		lo, hi = hi, lo
	}

	lo, hi = a.snapToBeat(lo), a.snapToBeat(hi)
	minLen := a.beatLenAt(lo)
	if hi < lo+minLen {
		hi = lo + minLen
	}
	if e := a.pieceEnd(); hi > e {
		hi = e
		if lo > hi-minLen {
			lo = hi - minLen
			if lo < 0 {
				lo = 0
			}
		}
	}
	a.eng.SetLoop(lo, hi)
}

func (a *App) hitChrome(p pointer, l practiceLayout) bool {
	spots := make([]hotspot, 0, len(l.transport)+len(l.chips)+len(l.tracks))
	spots = append(spots,
		hotspot{r: l.transport[0], on: func() { a.seekTo(0) }},
		hotspot{r: l.transport[1], on: a.seekPrevBar},
		hotspot{r: l.transport[2], on: a.togglePlay},
		hotspot{r: l.transport[3], on: a.seekNextBar},
	)
	states := a.chipStates()
	for i := range practiceChips {
		c := practiceChips[i]
		if states[i].disabled {

			whenOff := c.whenOff
			spots = append(spots, hotspot{r: l.chips[i], on: func() {
				if whenOff != nil {
					a.setBPMMessage(whenOff(a))
				}
			}})
			continue
		}
		spots = append(spots, hotspot{r: l.chips[i], on: func() { c.act(a) }})
	}
	spots = append(spots,
		hotspot{r: l.tempo[0], on: a.slower},
		hotspot{r: l.tempo[1], on: a.openBPMEntry},
		hotspot{r: l.tempo[2], on: a.faster},
	)
	if a.warningVisible() {
		// × dismisses; the rest of the strip (and its "N more") opens the
		// list of every warning, or dismisses when there is only the one.
		b := bannerRect()
		spots = append(spots,
			hotspot{r: rect{b.x + b.w - 40, b.y, 40, b.h}, on: a.dismissWarning},
			hotspot{r: b, on: func() {
				if len(a.warnItems()) > 1 {
					a.toggleWarnList()
				} else {
					a.dismissWarning()
				}
			}})
	}
	for i := range l.tracks {
		i := i
		spots = append(spots, hotspot{
			r:       l.tracks[i],
			on:      func() { a.toggleMute(i) },
			onRight: func() { a.toggleSolo(i) },
		})
	}
	return p.hit(spots)
}

func (a *App) continueDrag(p pointer, shift bool) {
	l := a.layout()
	switch a.drag {
	case dragSeekTab:
		a.seekTo(a.tickAtX(p.x))
	case dragSeekTimeline:
		a.seekTo(a.timelineTick(l.timeline, p.x))
	case dragLoopNew:
		a.continueLoopDraw(p, l)
	case dragLoopA, dragLoopB:

		tick := a.tickAtX(p.x)
		if p.over(l.timeline) {
			tick = a.timelineTick(l.timeline, p.x)
		}
		if !shift {
			tick = a.snapToBeat(tick)
		}
		a.moveLoopEdge(a.drag, tick)
	}

}

func (a *App) moveLoopEdge(which dragTarget, tick int64) {
	start, end, on := a.eng.Loop()
	if !on {
		return
	}

	minLen := a.minLoopLen()
	if which == dragLoopA {
		if tick > end-minLen {
			tick = end - minLen
		}
		if tick < 0 {
			tick = 0
		}
		start = tick
	} else {
		if tick < start+minLen {
			tick = start + minLen
		}
		if e := a.pieceEnd(); tick > e {
			tick = e
		}
		end = tick
	}
	a.eng.SetLoop(start, end)
}

func (a *App) minLoopLen() int64 {
	start, _, _ := a.eng.Loop()
	return a.beatLenAt(start)
}

func (a *App) beatLenAt(tick int64) int64 {
	if n := a.sc.Meters.At(tick).BeatLen(); n > 0 {
		return n
	}
	return score.PPQ
}

func (a *App) toggleLoop() {
	if _, _, on := a.eng.Loop(); on {
		a.eng.ClearLoop()
		return
	}
	a.loopSetA()
}

func (a *App) togglePlay() {
	if a.eng.Playing() {
		a.eng.Pause()
	} else {
		a.eng.Play()
	}
}

func (a *App) seekPrevBar() {
	bars := a.displayed().Bars
	pos := a.posTick()
	i := a.barAt(pos)
	if i < 0 || len(bars) == 0 {
		return
	}
	b := bars[i]
	if pos-b.Start < b.Len()/8 && i > 0 {
		a.eng.SeekTick(bars[i-1].Start)
		return
	}
	a.eng.SeekTick(b.Start)
}

func (a *App) seekNextBar() {
	bars := a.displayed().Bars
	pos := a.posTick()
	i := a.barAt(pos)
	if i < 0 || i+1 >= len(bars) {
		return
	}
	target := bars[i+1].Start
	if la, lb, on := a.eng.Loop(); on && target == lb && pos >= la {
		a.eng.SeekTick(la)
		return
	}
	a.eng.SeekTick(target)
}

func (a *App) seekLastBar() {
	bars := a.displayed().Bars
	if len(bars) == 0 {
		return
	}
	a.seekTo(bars[len(bars)-1].Start)
}

func (a *App) zoomIn() {
	if a.zoom < 4 {
		a.zoom *= 1.25
	}
}

func (a *App) zoomOut() {
	if a.zoom > 0.3 {
		a.zoom /= 1.25
	}
}

func (a *App) drawTransport(dst *ebiten.Image, l practiceLayout, p pointer) {
	icons := [4]iconID{icSkipStart, icPrevBar, icPlay, icNextBar}
	if a.eng.Playing() {
		icons[2] = icPause
	}
	dt := motionDt()
	names := [4]string{"transport:start", "transport:prev", "transport:play", "transport:next"}
	tipText := [4]string{"Back to the start", "Previous bar", "Play / pause", "Next bar"}
	tipKey := [4]string{"home", "left", "space", "right"}
	for i, r := range l.transport {
		drawKitMenuItem(dst, r, icons[i], "", false, false, a.anim.step(names[i], p.over(r), p.down, dt))
		a.tip.offer(names[i], tipText[i], tipKey[i], r, p.over(r), dt)
	}
	for _, x := range l.dividers {
		drawToolbarDivider(dst, x)
	}
	for i, c := range a.chipStates() {
		r := l.chips[i]
		label := c.label
		if practiceChips[i].right && !toolsLabelled() {
			label = "" // icon only; the tooltip names it
		}
		drawKitMenuItem(dst, r, practiceChips[i].icon, label, c.on, c.disabled, a.anim.step("chip:"+c.label, p.over(r) && !c.disabled, p.down, dt))
		a.tip.offer("chip:"+c.label, c.label, c.key, r, p.over(r), dt)
	}
	a.drawTempo(dst, l, p, dt)
}

func (a *App) drawTrackStrip(dst *ebiten.Image, l practiceLayout, p pointer) {
	a.drawTrackTags(dst, l, p)
}

func (a *App) hiddenTracksHint(shown, hidden int) string {
	first, total := shown+1, shown+hidden
	switch {
	case first > 9:
		return fmt.Sprintf("+%d more (no keys past track 9)", hidden)
	case total > 9:
		return fmt.Sprintf("+%d more (keys %d-9; tracks past 9 have no key)", hidden, first)
	default:
		return fmt.Sprintf("+%d more (keys %d-%d)", hidden, first, total)
	}
}

func (a *App) hiddenTracks(l practiceLayout) int {
	n := 0
	for _, r := range l.tracks {
		if r.empty() {
			n++
		}
	}
	return n
}

func (a *App) trackStateText(i int) string {
	switch {
	case a.solo == i+1:
		return "solo"
	case a.userMutedAt(i):
		return "muted"
	case a.solo > 0:
		return "muted by solo"
	}
	return "audible"
}

func (a *App) userMutedAt(i int) bool {
	a.ensureMuteState()
	return i >= 0 && i < len(a.userMuted) && a.userMuted[i]
}

func (a *App) drawTimeline(dst *ebiten.Image, l practiceLayout, p pointer) {
	tl := l.timeline
	notchedFill(dst, tl, colPanel)
	a.drawSections(dst, tl, p)

	if la, lb, on := a.eng.Loop(); on {
		x0, x1 := a.timelineX(tl, la), a.timelineX(tl, lb)
		vector.FillRect(dst, float32(x0), float32(tl.y), float32(x1-x0), float32(tl.h), colLoop, false)
		drawLoopBracket(dst, x0, tl.y-3, tl.h+6, true)
		drawLoopBracket(dst, x1, tl.y-3, tl.h+6, false)
	}
	x := float32(math.Round(a.timelineXf(tl, a.posF())))
	vector.FillRect(dst, x-1, float32(tl.y-4), 2, float32(tl.h+8), colPlayhead, false)
	if p.over(tl) && a.drag == dragNone {
		// Anchored to run to the window's foot, so the tip opens above.
		a.tip.offer("timeline", a.timelineTip(tl, p.x), "", rect{tl.x, tl.y, tl.w, screenH - tl.y}, true, motionDt())
	}

	// One line under it: the section, the bar, the time (§6.6).
	y := tl.y + tl.h + sp3
	x2 := pgMargin
	if sec := a.currentSection(); sec != "" {
		s := strings.ToUpper(sec)
		drawTextScaled(dst, s, x2, y-1, dispS, tokBrand)
		x2 += textWScaled(s, dispS) + sp4
	}
	drawMonoSized(dst, a.positionCaption(), x2, y, sizeMono, colDim)
}

// drawLoopBracket draws a loop end: a 2px upright with feet toward the loop.
func drawLoopBracket(dst *ebiten.Image, x, y, h float64, left bool) {
	x = math.Round(x)
	vector.FillRect(dst, float32(x-1), float32(y), 2, float32(h), colLoopEdge, false)
	foot := float32(4)
	fx := float32(x - 1)
	if !left {
		fx = float32(x+1) - foot
	}
	vector.FillRect(dst, fx, float32(y), foot, 2, colLoopEdge, false)
	vector.FillRect(dst, fx, float32(y+h-2), foot, 2, colLoopEdge, false)
}

// timelineTip names what is under the pointer: the section, its bars and
// length, or the bar and time; and how to use the timeline.
func (a *App) timelineTip(tl rect, x float64) string {
	tick := a.timelineTick(tl, x)
	if i := a.sc.SectionAt(tick); i >= 0 {
		s := a.sc.Sections[i]
		end := a.pieceEnd()
		if i+1 < len(a.sc.Sections) {
			end = a.sc.Sections[i+1].Tick
		}
		from, to := a.sc.BarNumber(a.barAt(s.Tick)), a.sc.BarNumber(a.barAt(max(s.Tick, end-1)))
		secs := (a.sc.Tempos.TimeAt(end) - a.sc.Tempos.TimeAt(s.Tick)) / max(a.eng.TempoScale(), 0.01)
		return fmt.Sprintf("%s · bars %d–%d · %s   (click to move, shift-drag to loop)", s.Name, from, to, clockText(secs))
	}
	return fmt.Sprintf("Bar %d   (click to move, shift-drag to loop)", a.sc.BarNumber(a.barAt(tick)))
}

func (a *App) timelineHint() string {
	if _, _, on := a.eng.Loop(); on {
		return "Click or drag to move   ·   Drag the loop edges to resize"
	}
	return "Click or drag to move   ·   Shift-drag to loop a section"
}

func (a *App) positionCaption() string {
	bars := a.displayed().Bars
	pos := a.posTick()
	bar := a.sc.BarNumber(a.barAt(pos))
	lastBar := a.sc.BarNumber(len(bars) - 1)
	scale := a.eng.TempoScale()
	if scale <= 0 {
		scale = 1
	}
	at := a.sc.Tempos.TimeAt(pos) / scale
	total := a.sc.Tempos.TimeAt(a.pieceEnd()) / scale
	if sec := a.currentSection(); sec != "" {
		_ = sec // the section is drawn on its own, in the display font
	}
	return fmt.Sprintf("Bar %d / %d   ·   %s / %s", bar, lastBar, clockText(at), clockText(total))
}

func clockText(sec float64) string {
	if sec < 0 || sec != sec {
		sec = 0
	}
	s := int(sec + 0.5)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
