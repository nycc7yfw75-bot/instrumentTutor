package ui

import (
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/score"
)

const (
	ptTransportY = 58.0
	ptTracksY    = 100.0
	ptWarnY      = 160.0
	ptWarnH      = 56.0
	ptTimelineH  = 36.0

	ptTabHeadroom = 34.0

	ptTabFooting = 26.0

	ptBtnW = 36.0

	ptBtnH   = chipH
	ptBtnGap = 6.0

	ptGrab = 6.0

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
	{label: "Loop", key: "A / B, L clears",
		on:  func(a *App) bool { _, _, on := a.eng.Loop(); return on },
		act: func(a *App) { a.toggleLoop() }},
	{label: "Click", key: "M",
		on:  func(a *App) bool { return a.metronome },
		act: func(a *App) { a.toggleMetronome() }},
	{label: "Ramp", key: "R",
		on:  func(a *App) bool { return a.ramp },
		act: func(a *App) { a.toggleRamp() }},
	{label: "Count-in", key: "C",
		on:  func(a *App) bool { return a.CountInBeats() > 0 },
		act: func(a *App) { a.toggleCountIn() }},
	{label: "Wait", key: "W",
		on:       func(a *App) bool { return a.wait },
		disabled: func(a *App) bool { return !a.waitCtl },
		whenOff:  func(a *App) string { return "wait mode needs live input — " + a.liveRemedy() },
		act:      func(a *App) { a.toggleWait() }},
	{label: "Tuner", key: "T", right: true,
		on:  func(a *App) bool { return a.tunerView },
		act: func(a *App) { a.tunerView = !a.tunerView }},
	{label: "Mixer", key: "V", right: true,
		on:  func(a *App) bool { return a.mixer.open },
		act: func(a *App) { a.toggleMixer() }},
	{label: "Settings", key: "S", right: true,
		disabled: func(a *App) bool { return a.settings == nil },
		whenOff:  func(a *App) string { return "settings need the full app — start musicTutor without a file" },
		act:      func(a *App) { a.openSettings() }},
	{label: "Help", key: "?", right: true,
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
	return float64(tabTop + (a.laneLines()-1)*stringGap)
}

func (a *App) tabRect() rect {
	top := tabTop - ptTabHeadroom
	return rect{0, top, screenW, a.tabBottom() + ptTabFooting - top}
}

func (a *App) stateChipY() float64 { return a.tabBottom() + 34 }

const (
	ptCaptionY  = screenH - 158.0
	ptTimelineY = screenH - 134.0
	ptLegendY   = screenH - 86.0
	ptMsgY      = screenH - 64.0
)

func (a *App) captionY() float64 { return ptCaptionY }

func (a *App) timelineY() float64 { return ptTimelineY }

func (a *App) legendY() float64 { return ptLegendY }

func (a *App) msgY() float64 { return ptMsgY }

func (a *App) layout() practiceLayout {
	var l practiceLayout

	// One row: transport | Loop Click Ramp Count-in Wait | tempo, then
	// Tuner Mixer Settings Help pinned to the right.
	x := uiPadX - (menuIconW-16)/2 // align the first icon's ink with the margin
	for i := range l.transport {
		l.transport[i] = rect{x, ptTransportY, menuIconW, chipH}
		x += menuIconW + 4
	}
	divide := func() {
		x += menuGroupGap - menuGap/2
		l.dividers = append(l.dividers, x)
		x += menuGroupGap
	}
	divide()
	l.chips = make([]rect, len(practiceChips))
	for i, c := range practiceChips {
		if c.right {
			continue
		}
		w := menuItemW(c.label)
		l.chips[i] = rect{x, ptTransportY, w, chipH}
		x += w + menuGap
	}
	x -= menuGap / 2
	divide()
	arrowW := textW("‹") + 2*menuPadX + 8
	l.tempo[0] = rect{x, ptTransportY, arrowW, chipH}
	l.tempo[1] = rect{x + arrowW, ptTransportY, tempoTextW, chipH}
	l.tempo[2] = rect{x + arrowW + tempoTextW, ptTransportY, arrowW, chipH}

	rx := screenW - uiPadX
	for i := len(practiceChips) - 1; i >= 0; i-- {
		c := practiceChips[i]
		if !c.right {
			continue
		}
		w := menuItemW(c.label)
		rx -= w
		l.chips[i] = rect{rx, ptTransportY, w, chipH}
		rx -= menuGap
	}

	l.tracks = make([]rect, len(a.sc.Tracks))
	tx := uiPadX
	for i := range a.sc.Tracks {

		if tx+ptTrackChipW > a.trackStripRight() {
			break
		}
		l.tracks[i] = rect{tx, ptTracksY, ptTrackChipW, chipH}
		tx += ptTrackChipW + chipGap
	}

	l.tab = a.tabRect()
	l.timeline = rect{uiPadX, a.timelineY(), screenW - 2*uiPadX, ptTimelineH}

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

func (a *App) trackStripRight() float64 {
	if a.live {
		return screenW - uiPadX - 260
	}
	return screenW - uiPadX
}

func (a *App) loopHandleOnTab(tick int64) rect {
	x := a.xAtTick(tick)
	if x < -ptGrab || x > screenW+ptGrab {
		return rect{}
	}
	t := a.tabRect()
	return rect{x - ptGrab, t.y, 2 * ptGrab, t.h}
}

func (a *App) loopHandleOnTimeline(tl rect, tick int64) rect {
	x := a.timelineX(tl, tick)
	return rect{x - ptGrab, tl.y, 2 * ptGrab, tl.h}
}

func (a *App) xAtTick(tick int64) float64 {
	return screenW*playheadX + (float64(tick)-a.posF())*a.pxPerTick()
}

func (a *App) tickAtX(x float64) int64 {
	return int64(math.Round(a.posF() + (x-screenW*playheadX)/a.pxPerTick()))
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
		spots = append(spots, hotspot{r: warnRect(), on: func() {
			if len(a.importWarns) > 0 {
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
	icons := [4]iconKind{iconToStart, iconPrevBar, iconPlay, iconNextBar}
	if a.eng.Playing() {
		icons[2] = iconPause
	}
	dt := uiFrameSeconds()
	names := [4]string{"transport:start", "transport:prev", "transport:play", "transport:next"}
	tipText := [4]string{"Back to the start", "Previous bar", "Play / pause", "Next bar"}
	tipKey := [4]string{"home", "left", "space", "right"}
	for i, r := range l.transport {
		drawMenuIcon(dst, r, icons[i], a.anim.step(names[i], p.over(r), p.down, dt))
		a.tip.offer(names[i], tipText[i], tipKey[i], r, p.over(r), dt)
	}
	for _, x := range l.dividers {
		drawMenuDivider(dst, x)
	}
	for i, c := range a.chipStates() {
		r := l.chips[i]
		drawMenuItem(dst, r, c.label, c.on, c.disabled, a.anim.step("chip:"+c.label, p.over(r) && !c.disabled, p.down, dt))
		a.tip.offer("chip:"+c.label, c.label, c.key, r, p.over(r), dt)
	}
	a.drawTempo(dst, l, p, dt)
}

func (a *App) drawTrackStrip(dst *ebiten.Image, l practiceLayout, p pointer) {
	for i, r := range l.tracks {
		if r.empty() {
			continue
		}
		// No boxes: two lines of text. Your track has a 2px accent edge and
		// accent name; a muted one recedes; a solo reads in green.
		av := a.anim.step(fmt.Sprintf("track:%d", i), p.over(r), p.down, uiFrameSeconds())
		muted := a.mutedAudibly(i)
		name, sub := lerpCol(colHUD, colNote, av.hover), colDim
		switch {
		case i == a.track:
			name = colAccent
		case muted:
			name, sub = lerpCol(colString, colDim, av.hover), colBarline
		}
		if a.solo == i+1 {
			sub = colOnEdge
		}
		if i == a.track {
			vector.FillRect(dst, float32(r.x), float32(r.y+3), 2, float32(r.h-6), colAccent, false)
		}
		label := truncateW(fmt.Sprintf("%d  %s", i+1, a.trackName(i)), r.w-18)
		drawText(dst, label, r.x+10, r.y+2, name)
		state := sentenceCase(a.trackStateText(i))
		if i == a.track {
			state = "You · " + state
		}
		drawTextSmall(dst, state, r.x+10, r.y+20, sub)
	}
	if n := a.hiddenTracks(l); n > 0 {
		drawText(dst, a.hiddenTracksHint(len(l.tracks)-n, n), uiPadX, ptTracksY+chipH+4, colHint)
	}
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
	hover := p.over(tl)
	edge := colPanelEdge
	if hover {
		edge = colDim
	}
	drawPanel(dst, tl, colPanel, edge)

	a.drawSections(dst, tl)

	bars := a.displayed().Bars
	step := 1
	if len(bars) > 0 && tl.w/float64(len(bars)) < 3 {
		step = 4
	}
	for i := 0; i < len(bars); i += step {
		x := float32(a.timelineX(tl, bars[i].Start))
		vector.StrokeLine(dst, x, float32(tl.y+tl.h-8), x, float32(tl.y+tl.h-2), 1, colBarline, false)
	}

	if la, lb, on := a.eng.Loop(); on {
		x0, x1 := a.timelineX(tl, la), a.timelineX(tl, lb)
		vector.DrawFilledRect(dst, float32(x0), float32(tl.y+1), float32(x1-x0), float32(tl.h-2), colLoop, false)
		for _, x := range []float64{x0, x1} {
			vector.StrokeLine(dst, float32(x), float32(tl.y), float32(x), float32(tl.y+tl.h), 2, colLoopEdge, false)
		}
	}

	x := float32(a.timelineXf(tl, a.posF()))
	vector.StrokeLine(dst, x, float32(tl.y-4), x, float32(tl.y+tl.h+4), 2, colPlayhead, false)

	if hover && a.drag == dragNone {
		vector.StrokeLine(dst, float32(p.x), float32(tl.y+tl.h), float32(p.x), float32(tl.y+tl.h+4), 1, colDim, false)
	}
	drawTextMono(dst, a.positionCaption(), uiPadX, a.captionY(), colHUD)
	hint, hintCol := a.timelineHint(), colBarline
	if hover {
		if i := a.sc.SectionAt(a.timelineTick(tl, p.x)); i >= 0 {
			// Names too long for their stretch of timeline read in full here.
			hint, hintCol = a.sc.Sections[i].Name+"   ·   Click or drag to move   ·   [ ] jumps to a section start", colHUD
		}
	}
	drawTextSmall(dst, hint, screenW-uiPadX-textWSmall(hint), a.captionY()+2, hintCol)
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
		return fmt.Sprintf("Bar %d of %d   %s     %s / %s", bar, lastBar, sec, clockText(at), clockText(total))
	}
	return fmt.Sprintf("Bar %d of %d     %s / %s", bar, lastBar, clockText(at), clockText(total))
}

func clockText(sec float64) string {
	if sec < 0 || sec != sec {
		sec = 0
	}
	s := int(sec + 0.5)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func (a *App) drawStateChip(dst *ebiten.Image, label string, col color.RGBA, y float64) {
	x := screenW*playheadX - textWScaled(label, 1.5)/2
	drawTextScaled(dst, label, x, y, 1.5, col)
}
