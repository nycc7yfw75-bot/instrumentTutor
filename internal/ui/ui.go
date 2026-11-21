package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/S95F/musicTutor/internal/engine"
	"github.com/S95F/musicTutor/internal/score"
)

// The window: at least the design size, and it fills whatever the player
// makes it (REDESIGN §6). resize keeps the size-derived values in step.
const (
	designW = 1280
	designH = 720
)

var screenW, screenH float64 = designW, designH

// resize applies a new window size.
func resize(w, h int) {
	screenW, screenH = float64(max(w, designW)), float64(max(h, designH))
	edGridW = screenW - 2*uiPadX - edLabelW
	settingsWrap = screenW - settingsValueX - uiPadX
	settingsFooterW = screenW - 2*settingsLeft
}

const (
	playheadX = 0.3

	defaultPxQ = 96
)

const (
	engineMinTempoScale = 0.25
	engineMaxTempoScale = 2.0
)

const bpmEntryMaxDigits = 3

const bpmMsgFrames = 300

const defaultCountInBeats = 4

type App struct {
	eng   *engine.Engine
	sc    *score.Score
	track int

	// REDESIGN §6: the file (for the title fallback), and how long the input
	// has been flat while playing (for NO SIGNAL).
	piecePath string
	flatFor   float64
	chrome    chromeMotion // the header and HUD's small motions (MOTION §A)
	gig       *gigMode     // non-nil while this view plays a gig (CAREER.md §3)

	zoom float64

	metronome bool
	ramp      bool

	bpmEntry    bool
	bpmDigits   string
	bpmMsg      string
	bpmMsgUntil int64

	userMuted []bool
	solo      int

	countInBeats  int
	countInOn     bool
	engineCountIn int
	countInStale  bool
	countInApply  func(beats int) bool

	helpOpen bool

	quitAll func()

	settings func()

	reload func()

	// switchTrack re-opens the piece showing and scoring another track;
	// nil when nothing can re-open it (the bare `play` command).
	switchTrack func(track int)

	anim animator

	outLatency   func() time.Duration
	latency      float64
	latencyArmed bool

	settingsTouched bool

	ptr      pointer
	drag     dragTarget
	wheelAcc float64

	loopDrawTick       int64
	loopDrawX          float64
	loopDrawing        bool
	loopDrawOnTimeline bool
	loopDrawPxPerTick  float64

	ph playhead

	liveUI
	marks map[noteKey]tabMark // technique marks, built on first draw

	tip tips // hover tooltips carrying each menu item's key

	game gameUI // scoring, excitement, XP: the game layer
	mixerUI
	warnListUI
	inputHintUI
}

func New(eng *engine.Engine, sc *score.Score, track int) *App {
	eng.SetWaitTrack(track)
	a := &App{eng: eng, sc: sc, track: track, zoom: 1, countInBeats: defaultCountInBeats}
	a.mixerDrag = -1
	return a
}

func (a *App) Run() error {
	ebiten.SetWindowSize(designW, designH)
	ebiten.SetWindowSizeLimits(designW, designH, -1, -1)
	ebiten.SetWindowTitle("HEADLINER — " + a.songTitle())
	err := ebiten.RunGame(a)
	if err == errQuit {
		return nil
	}
	return err
}

var errQuit = fmt.Errorf("quit")

func (a *App) pxPerTick() float64 {
	return a.zoom * defaultPxQ / float64(score.PPQ)
}

func (a *App) Track() int { return a.track }

func (a *App) displayed() *score.Track { return a.sc.Tracks[a.track] }

func (a *App) barAt(tick int64) int {
	bars := a.displayed().Bars
	for i, b := range bars {
		if tick < b.Start+b.Len() {
			return i
		}
	}
	return len(bars) - 1
}

func (a *App) shiftHeld() bool {
	return ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)
}

func (a *App) SetQuitAll(fn func()) { a.quitAll = fn }

func (a *App) Update() error {
	a.frame++
	a.syncLive()
	a.gameStep()
	a.checkAudioStalled()

	a.stepPlayhead()
	a.ptr = readPointer()
	if a.gig != nil {
		// A gig is one take: the gig screen owns the keys. Only the mixer
		// (V) opens, for the crowd's volume.
		if a.mixer.open {
			a.updateMixer()
		} else if inpututil.IsKeyJustPressed(ebiten.KeyV) {
			a.toggleMixer()
		}
		return nil
	}

	if a.bpmEntry || a.helpOpen || a.mixer.open || a.warnList.open {

		a.drag = dragNone
	}
	if a.bpmEntry {
		a.updateBPMEntry()
		return nil
	}
	if a.helpOpen {
		a.updateHelp()
		return nil
	}
	if a.mixer.open {
		a.updateMixer()
		return nil
	}
	if a.warnList.open {
		a.updateWarnList()
		return nil
	}
	if a.gameModal() {
		return nil
	}
	if a.reviewModal() {
		return nil
	}
	if a.warningVisible() && inpututil.IsKeyJustPressed(ebiten.KeyD) {
		a.dismissWarning()
		return nil
	}
	if escButtonPressed(a.backChip(), a.ptr) {
		return errQuit
	}
	if a.gameClick(a.ptr) {
		return nil
	}
	if err := a.handleKeys(); err != nil {
		return err
	}
	a.handleMouse(a.ptr, a.shiftHeld())
	return nil
}

func (a *App) handleKeys() error {
	eng := a.eng
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):

		return errQuit
	case inpututil.IsKeyJustPressed(ebiten.KeyQ):
		return a.quitApp()
	case helpKeyPressed():
		a.openHelp()
	case inpututil.IsKeyJustPressed(ebiten.KeySpace):
		a.togglePlay()
	case inpututil.IsKeyJustPressed(ebiten.KeyHome):
		a.seekTo(0)
	case inpututil.IsKeyJustPressed(ebiten.KeyEnd):
		a.seekLastBar()
	case inpututil.IsKeyJustPressed(ebiten.KeyLeft):
		a.seekPrevBar()
	case inpututil.IsKeyJustPressed(ebiten.KeyRight):
		a.seekNextBar()
	case inpututil.IsKeyJustPressed(ebiten.KeyUp):
		eng.SetTempoScale(eng.TempoScale() + 0.05)
	case inpututil.IsKeyJustPressed(ebiten.KeyDown):
		eng.SetTempoScale(eng.TempoScale() - 0.05)
	case inpututil.IsKeyJustPressed(ebiten.KeyA):
		a.loopSetA()
	case inpututil.IsKeyJustPressed(ebiten.KeyB):

		if a.shiftHeld() {
			a.openBPMEntry()
		} else {
			a.loopSetB()
		}
	case inpututil.IsKeyJustPressed(ebiten.KeyC):
		a.toggleCountIn()
	case inpututil.IsKeyJustPressed(ebiten.KeyL):
		if _, _, on := eng.Loop(); on {
			eng.ClearLoop()
		}
	case inpututil.IsKeyJustPressed(ebiten.KeyM):
		a.toggleMetronome()
	case inpututil.IsKeyJustPressed(ebiten.KeyR):
		a.toggleRamp()
	case inpututil.IsKeyJustPressed(ebiten.KeyS):
		a.openSettings()
	case inpututil.IsKeyJustPressed(ebiten.KeyF5):
		a.reloadPiece()
	case inpututil.IsKeyJustPressed(ebiten.KeyTab):
		if a.shiftHeld() {
			a.cycleTrack(-1)
		} else {
			a.cycleTrack(1)
		}
	case inpututil.IsKeyJustPressed(ebiten.KeyP):
		if a.game.sess != nil {
			a.toggleReview()
		}
	case inpututil.IsKeyJustPressed(ebiten.KeyT):
		a.tunerView = !a.tunerView
	case inpututil.IsKeyJustPressed(ebiten.KeyV):
		a.toggleMixer()
	case inpututil.IsKeyJustPressed(ebiten.KeyI):
		a.toggleWarnList()
	case inpututil.IsKeyJustPressed(ebiten.KeyBracketLeft):
		a.jumpSection(-1)
	case inpututil.IsKeyJustPressed(ebiten.KeyBracketRight):
		a.jumpSection(1)
	case inpututil.IsKeyJustPressed(ebiten.KeyW):
		if a.waitCtl {
			a.toggleWait()
		}
	case inpututil.IsKeyJustPressed(ebiten.KeyEqual), inpututil.IsKeyJustPressed(ebiten.KeyKPAdd):
		a.zoomIn()
	case inpututil.IsKeyJustPressed(ebiten.KeyMinus), inpututil.IsKeyJustPressed(ebiten.KeyKPSubtract):
		a.zoomOut()
	}
	for i := 0; i < len(a.sc.Tracks) && i < 9; i++ {
		if inpututil.IsKeyJustPressed(ebiten.Key1 + ebiten.Key(i)) {
			if a.shiftHeld() {
				a.toggleSolo(i)
			} else {
				a.toggleMute(i)
			}
		}
	}
	return nil
}

func (a *App) quitApp() error {
	if a.quitAll != nil {
		a.quitAll()
		return nil
	}
	return errQuit
}

func (a *App) SetSettingsOpener(fn func()) { a.settings = fn }

func (a *App) SetReloader(fn func()) { a.reload = fn }

func (a *App) MarkSettingsChanged() { a.settingsTouched = a.reload != nil }

func (a *App) openSettings() {
	if a.settings != nil {
		a.settings()
	}
}

func (a *App) SetTrackSwitcher(fn func(track int)) { a.switchTrack = fn }

// cycleTrack moves the shown-and-scored track by dir (wrapping) and hands it
// to the switcher, which re-opens the piece so scoring follows the new track.
// playableTracks counts the tracks that can be practised: all but drum kits.
func (a *App) playableTracks() int {
	n := 0
	for _, tr := range a.sc.Tracks {
		if !tr.Drums {
			n++
		}
	}
	return n
}

func (a *App) cycleTrack(dir int) {
	n := len(a.sc.Tracks)
	switch {
	case a.playableTracks() < 2:
		a.setBPMMessage("this piece has only one track to practise")
	case a.switchTrack == nil:
		a.setBPMMessage("start HEADLINER without a file to switch tracks here")
	default:
		// Step past drum kits: they play along but cannot be practised.
		next := a.track
		for {
			next = ((next+dir)%n + n) % n
			if !a.sc.Tracks[next].Drums {
				break
			}
		}
		a.switchTrack(next)
	}
}

func (a *App) reloadPiece() {
	if a.reload == nil {
		return
	}
	a.reload()
}

func (a *App) reloadPrompt() string {
	if !a.settingsTouched && !a.countInStale {
		return ""
	}
	return "settings changed: press F5 to re-open this piece with them"
}

func (a *App) SetTunerView(on bool) { a.tunerView = on }

func (a *App) SetHelpOpen(on bool) { a.helpOpen = on }

// SetMixerOpen and SetWarnListOpen open an overlay as V and I do.
func (a *App) SetMixerOpen(on bool)    { a.mixer.set(on) }
func (a *App) SetWarnListOpen(on bool) { a.warnList.set(on) }

func (a *App) openHelp() { a.helpOpen = true }

func (a *App) closeHelp() { a.helpOpen = false }

func (a *App) updateHelp() {
	if helpDismissed(a.ptr) {
		a.closeHelp()
	}
}

func (a *App) loopSetA() {
	i := a.barAt(a.posTick())
	if i < 0 {
		return
	}
	b := a.displayed().Bars[i]
	_, end, on := a.eng.Loop()
	if !on || end <= b.Start {
		end = b.Start + b.Len()
	}
	a.eng.SetLoop(b.Start, end)
}

func (a *App) loopSetB() {
	i := a.barAt(a.posTick())
	if i < 0 {
		return
	}
	b := a.displayed().Bars[i]
	start, _, on := a.eng.Loop()
	if !on || start >= b.Start+b.Len() {
		start = b.Start
	}
	a.eng.SetLoop(start, b.Start+b.Len())
}

func (a *App) toggleMetronome() {
	a.metronome = !a.metronome
	a.eng.SetMetronome(a.metronome)
}

func (a *App) toggleRamp() {
	a.ramp = !a.ramp
	a.eng.SetRamp(engine.RampConfig{Enabled: a.ramp, Increment: 0.05, Target: 1.0})
	if !a.ramp {
		return
	}

	if _, _, on := a.eng.Loop(); !on {
		a.setBPMMessage("ramp raises the speed after each loop pass — set a loop first")
	} else if a.eng.TempoScale() >= 1.0 {
		a.setBPMMessage("already at full speed — slow down first and ramp brings you back up")
	}
}

func (a *App) SetInitialMetronome(on bool) { a.metronome = on }

func (a *App) openBPMEntry() {
	a.bpmEntry, a.bpmDigits = true, ""
	a.bpmMsg, a.bpmMsgUntil = "", 0
}

func (a *App) updateBPMEntry() {
	for d := 0; d <= 9; d++ {
		if inpututil.IsKeyJustPressed(ebiten.Key0+ebiten.Key(d)) ||
			inpututil.IsKeyJustPressed(ebiten.KeyKP0+ebiten.Key(d)) {
			a.bpmDigit(byte('0' + d))
		}
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyBackspace):
		a.bpmBackspace()
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter), inpututil.IsKeyJustPressed(ebiten.KeyKPEnter):
		a.commitBPMEntry()
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		a.cancelBPMEntry()
	case a.ptr.pressed && !a.ptr.over(bpmEntryRect()):

		a.cancelBPMEntry()
	}
}

func bpmEntryRect() rect {
	const w, h = 380, 168
	z := practiceTabZone()
	return rect{(screenW - w) / 2, z.y + sp6, w, h}
}

func (a *App) bpmDigit(d byte) {
	if !a.bpmEntry || d < '0' || d > '9' {
		return
	}
	if a.bpmDigits == "0" {
		a.bpmDigits = ""
	}
	if len(a.bpmDigits) >= bpmEntryMaxDigits {
		return
	}
	a.bpmDigits += string(d)
}

func (a *App) bpmBackspace() {
	if n := len(a.bpmDigits); n > 0 {
		a.bpmDigits = a.bpmDigits[:n-1]
	}
}

func (a *App) cancelBPMEntry() {
	a.bpmEntry, a.bpmDigits = false, ""
}

func (a *App) commitBPMEntry() {
	digits := a.bpmDigits
	a.bpmEntry, a.bpmDigits = false, ""
	if digits == "" {
		return
	}
	target, err := strconv.Atoi(digits)
	if err != nil || target <= 0 {
		a.setBPMMessage("enter a target BPM between 1 and 999")
		return
	}
	scale, actual, clamped := a.scaleForBPM(float64(target))
	a.eng.SetTempoScale(scale)
	if clamped {
		a.setBPMMessage(fmt.Sprintf("%d BPM is outside the x%.2f-x%.2f practice range: set to %.0f BPM (x%.2f)",
			target, engineMinTempoScale, engineMaxTempoScale, actual, scale))
		return
	}
	a.setBPMMessage(fmt.Sprintf("tempo set to %.0f BPM (x%.2f)", actual, scale))
}

func (a *App) baseBPM() float64 {
	us := a.sc.Tempos.At(a.posTick())
	if us <= 0 {
		us = 500000
	}
	return 60e6 / float64(us)
}

func (a *App) scaleForBPM(target float64) (scale, actual float64, clamped bool) {
	base := a.baseBPM()
	scale = target / base
	if scale < engineMinTempoScale {
		scale, clamped = engineMinTempoScale, true
	}
	if scale > engineMaxTempoScale {
		scale, clamped = engineMaxTempoScale, true
	}
	return scale, base * scale, clamped
}

func (a *App) setBPMMessage(msg string) {
	a.bpmMsg, a.bpmMsgUntil = msg, a.frame+bpmMsgFrames
}

func (a *App) bpmMessage() string {
	if a.bpmMsg == "" || a.frame >= a.bpmMsgUntil {
		return ""
	}
	return a.bpmMsg
}

func (a *App) ensureMuteState() {
	if len(a.userMuted) == len(a.sc.Tracks) {
		return
	}
	a.userMuted = make([]bool, len(a.sc.Tracks))
	for i := range a.userMuted {
		a.userMuted[i] = a.eng.TrackMuted(i)
	}
}

// MuteState returns the player's own mutes and the soloed track (1-based,
// 0 for none), for remembering a piece's setup.
func (a *App) MuteState() (muted []bool, solo int) {
	a.ensureMuteState()
	return append([]bool(nil), a.userMuted...), a.solo
}

// SetMuteState restores MuteState; a mismatched track count or an
// out-of-range solo is ignored.
func (a *App) SetMuteState(muted []bool, solo int) {
	if len(muted) != len(a.sc.Tracks) {
		return
	}
	a.userMuted = append([]bool(nil), muted...)
	if solo >= 0 && solo <= len(a.sc.Tracks) {
		a.solo = solo
	}
	a.applyMutes()
}

func (a *App) mutedAudibly(i int) bool {
	a.ensureMuteState()
	return a.userMuted[i] || (a.solo > 0 && a.solo != i+1)
}

func (a *App) applyMutes() {
	a.ensureMuteState()
	for i := range a.userMuted {
		a.eng.SetTrackMuted(i, a.mutedAudibly(i))
	}
}

func (a *App) toggleMute(i int) {
	a.ensureMuteState()
	if i < 0 || i >= len(a.userMuted) {
		return
	}
	a.userMuted[i] = !a.userMuted[i]
	a.applyMutes()
}

func (a *App) toggleSolo(i int) {
	a.ensureMuteState()
	if i < 0 || i >= len(a.userMuted) {
		return
	}
	if a.solo == i+1 {
		a.solo = 0
	} else {
		a.solo = i + 1
	}
	a.applyMutes()
}

func (a *App) SetCountIn(beats int) {
	if beats > 0 {
		a.countInBeats, a.countInOn = beats, true
		a.engineCountIn = beats
	} else {
		a.countInBeats, a.countInOn = defaultCountInBeats, false
		a.engineCountIn = 0
	}
	a.countInStale = false
}

func (a *App) SyncCountIn(beats int) {
	if beats > 0 {
		a.countInBeats, a.countInOn = beats, true
	} else {
		a.countInBeats, a.countInOn = defaultCountInBeats, false
	}
	a.countInStale = a.CountInBeats() != a.engineCountIn
}

func (a *App) CountInBeats() int {
	if !a.countInOn {
		return 0
	}
	if a.countInBeats <= 0 {
		return defaultCountInBeats
	}
	return a.countInBeats
}

func (a *App) SetCountInApplier(fn func(beats int) bool) { a.countInApply = fn }

func (a *App) toggleCountIn() {
	a.countInOn = !a.countInOn
	applied := a.countInApply != nil && a.countInApply(a.CountInBeats())
	if applied {
		a.engineCountIn = a.CountInBeats()
		a.countInStale = false
		return
	}
	a.countInStale = a.CountInBeats() != a.engineCountIn
	if a.countInStale && a.reload == nil {
		a.setBPMMessage("the count-in change applies when the piece is next opened")
	}
}

type practiceBinding struct {
	Group string

	Keys string

	Hint string

	Desc string

	Enabled func(a *App) bool

	Reword func(a *App) (hint, desc string)

	Caveat func(a *App) string
}

func (b practiceBinding) enabled(a *App) bool { return b.Enabled == nil || b.Enabled(a) }

func liveInputCaveat(have func(a *App) bool) func(a *App) string {
	return func(a *App) string {
		if have(a) {
			return ""
		}
		return "needs live input — " + a.liveRemedy()
	}
}

var practiceBindings = []practiceBinding{
	{Group: "playback", Keys: "space", Hint: "space play/pause", Desc: "Start or pause playback"},

	{Group: "navigation", Keys: "left / right", Hint: "arrows seek", Desc: "Jump to the previous / next bar; inside a loop, right wraps to the loop start"},
	{Group: "navigation", Keys: "home / end", Desc: "Jump to the first / last bar"},
	{Group: "navigation", Keys: "[ / ]", Hint: "[ ] section", Desc: "Jump to the previous / next named section (Intro, Verse, Solo)",
		Enabled: func(a *App) bool { return len(a.sc.Sections) > 0 }},
	{Group: "navigation", Keys: "click / drag", Desc: "On the notation or the timeline: move the playhead"},

	{Group: "tracks", Keys: "1..9", Hint: "1-9 mute", Desc: "Mute / unmute a track (or click its chip)"},
	{Group: "tracks", Keys: "shift+1..9", Desc: "Solo a track; press again to release (or right-click it)"},
	{Group: "tracks", Keys: "tab / shift+tab", Hint: "tab track", Desc: "Show and score the next / previous track",
		Enabled: func(a *App) bool { return a.switchTrack != nil && a.playableTracks() > 1 }},

	{Group: "loop", Keys: "A / B,  L", Hint: "A/B loop", Desc: "Set the loop start / end at the current bar; L clears the loop"},
	{Group: "loop", Keys: "shift+drag, edges", Desc: "Shift+drag draws a new loop; drag an edge to move it (shift: tick-exact)"},

	{Group: "tempo", Keys: "up / down", Hint: "up/dn tempo", Desc: "Practice speed up / down by 5%"},
	{Group: "tempo", Keys: "shift+B", Hint: "shift+B bpm", Desc: "Type an exact target BPM"},
	{Group: "tempo", Keys: "R", Desc: "Ramp the speed up 5% after each loop pass"},
	{Group: "tempo", Keys: "C", Desc: "Count-in on / off for the next play"},

	{Group: "tools", Keys: "M", Hint: "M click", Desc: "Metronome click on / off"},
	{Group: "tools", Keys: "V", Hint: "V mixer", Desc: "Volume mixer: master, metronome, backing track and each track"},

	{Group: "tools", Keys: "T", Hint: "T tuner", Desc: "Tuner overlay",
		Caveat: liveInputCaveat(func(a *App) bool { return a.live })},
	{Group: "tools", Keys: "W", Hint: "W wait", Desc: "Wait at each note until you play it",
		Enabled: func(a *App) bool { return a.waitCtl },
		Caveat:  liveInputCaveat(func(a *App) bool { return a.waitCtl })},

	{Group: "tools", Keys: "P", Desc: "Review your last pass: the timing of every note, and where you rushed or dragged",
		Enabled: func(a *App) bool { return a.game.review.pass != nil }},
	{Group: "tools", Keys: "+ / -", Desc: "Zoom the notation in / out (or the wheel over it)"},

	{Group: "app", Keys: "S", Hint: "S settings", Desc: "Settings, without leaving the piece",
		Enabled: func(a *App) bool { return a.settings != nil },
		Caveat: func(a *App) string {
			if a.settings == nil {
				return "needs the full app — start HEADLINER without a file"
			}
			return ""
		}},
	{Group: "app", Keys: "F5", Desc: "Re-open this piece, picking up changed settings",
		Enabled: func(a *App) bool { return a.reload != nil }},
	{Group: "app", Keys: "? or F1", Hint: "? help", Desc: "This key-binding list"},
	{Group: "app", Keys: "D / I", Desc: "Dismiss the warning banner; I lists every import warning"},
	{Group: "app", Keys: "esc", Hint: "esc back", Desc: "Leave this piece and go back",
		Reword: func(a *App) (string, string) {
			if a.quitAll == nil {
				return "esc quit", "Leave the piece — with nothing behind it, this quits"
			}
			return "esc back", "Leave this piece and go back"
		}},
	{Group: "app", Keys: "Q", Hint: "Q quit", Desc: "Quit HEADLINER"},
}

func (a *App) helpRows() []helpBinding {
	out := make([]helpBinding, len(practiceBindings))
	for i, b := range practiceBindings {
		hint, desc := b.Hint, b.Desc
		if b.Reword != nil {
			hint, desc = b.Reword(a)
		}
		explained := false
		if b.Caveat != nil {
			if c := b.Caveat(a); c != "" {
				desc += " (" + c + ")"
				explained = true
			}
		}
		off := !b.enabled(a)
		out[i] = helpBinding{Group: b.Group, Keys: b.Keys, Hint: hint, Desc: desc,
			Off: off, Explained: off && explained}
	}
	return out
}

func (a *App) hintLine() string { return hintLineOf(a.helpRows()) }

func (a *App) Draw(screen *ebiten.Image) {
	screen.Fill(colBG)
	a.anim.tick()
	l := a.layout()
	if a.tunerView {
		a.drawTuner(screen)
	} else {
		a.drawTab(screen)
	}
	a.drawHUD(screen, l)
	if a.warningVisible() {
		a.drawPracticeBanner(screen)
	}
	if a.bpmEntry {
		a.drawBPMEntry(screen)
	}
	a.drawMixer(screen)
	a.drawWarnList(screen)
	if a.helpOpen {
		a.drawHelp(screen)
	}
}

func (a *App) noteCue(start, dur int64, str int, inferred bool, pos float64, posTick int64, waiting map[noteKey]bool) (col color.RGBA, sounding bool) {
	col = colNote
	if inferred {
		col = colInferredNote
	}
	if v, ok := a.verdictAt(start, str, posTick); ok {
		col = verdictColor(v)
	}
	if waiting[noteKey{start, str}] {
		col = a.pulseCol()
	}
	return col, float64(start) <= pos && pos < float64(start+dur)
}

func (a *App) drawHUD(screen *ebiten.Image, l practiceLayout) {
	a.trackSignal()
	a.drawPracticeHeader(screen)
	if a.gig == nil {
		a.drawTransport(screen, l, a.ptr) // a gig has no controls
	}
	a.drawTrackStrip(screen, l, a.ptr)
	a.drawPracticeHUD(screen)
	a.drawTimeline(screen, l, a.ptr)
	if a.game.sess != nil {
		a.drawGameHUD(screen)
	}
	a.drawPracticeMessages(screen)
	if a.helpOpen || a.mixer.visible() || a.warnList.visible() || a.bpmEntry {
		a.tip.hide()
	} else {
		a.tip.draw(screen)
	}
}

func (a *App) drawBPMEntry(screen *ebiten.Image) {
	r := bpmEntryRect()
	y := drawDialogCard(screen, r, "Target BPM")
	drawField(screen, rect{r.x + sp6, y, r.w - 2*sp6, 48}, a.bpmDigits, dispL, true)
	drawTextSmall(screen, "Enter applies · Esc or a click away cancels", r.x+sp6, y+48+sp3, colBarline)
}

func (a *App) drawHelp(screen *ebiten.Image) {
	drawHelpOverlay(screen, "PRACTICE KEYS", a.helpRows(), practiceHelpFootnote)
}

func (a *App) Layout(w, h int) (int, int) {
	resize(w, h)
	return int(screenW), int(screenH)
}
