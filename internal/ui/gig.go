package ui

import (
	"fmt"
	"math"

	"github.com/S95F/musicTutor/internal/scene"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/career"
	"github.com/S95F/musicTutor/internal/progress"
	"github.com/S95F/musicTutor/internal/score"
	"github.com/S95F/musicTutor/internal/scoring"
)

// Performance mode (CAREER.md §3, §6): a gig is a setlist played in one
// take at full speed, each song through a practice view in gig mode, under
// the stage scene. Between songs a short break; at the end the rating.

// GigLoader opens one song of a set for a gig: the practice view on its
// mastered track, at full speed, with a one-bar count-in.
type GigLoader func(song progress.MasteredSong, recording bool) (*App, error)

// GigCrowd is the crowd's sound: SetLevel follows the mood (0..1), Cheer
// and Boo play once. nil plays no crowd.
type GigCrowd interface {
	SetLevel(level float64)
	Cheer()
	Boo()
}

type gigPhase int

const (
	gigIntro gigPhase = iota
	gigPlaying
	gigBetween
	gigEnd
	gigBooed
	gigFailed      // a song would not open
	gigTierCleared // the tier-cleared moment, after the end card
)

// GigScreen plays a gig.
type GigScreen struct {
	sh    *Shell
	st    *progress.Store
	gig   career.Gig
	crowd string
	set   []progress.MasteredSong

	load  GigLoader
	close func()
	sound func(app *App) GigCrowd

	app      *App
	audio    GigCrowd
	idx      int
	phase    gigPhase
	phaseT   float64
	songT    float64 // seconds of this song played (after the count-in)
	started  bool    // this song's playback has begun
	reveal   bool    // a first-time tough crowd, revealed at the intro
	credible bool    // every song so far was scored

	total      scoring.Summary
	exciteSum  float64 // excitement × seconds, for the time-weighted average
	exciteTime float64

	result *gigResult
	stage  *stage
	ptr    pointer
	anim   animator

	confirm      bool // "Leave the stage?"
	confirmFocus int  // 0 Cancel (the default), 1 Leave
	loaded       int  // the song idx the open view plays, or -1
	cheered      bool // the intro's cheer has played
	err          string
}

// gigResult is the end card's numbers.
type gigResult struct {
	stars    int
	accuracy float64
	reward   career.Reward
	scored   bool
	booed    bool
	xpPlayed int // XP from the notes, already credited pass by pass
}

// NewGigScreen sets a gig up; it starts at the intro.
func NewGigScreen(sh *Shell, st *progress.Store, g career.Gig, crowd string, reveal bool,
	set []progress.MasteredSong, load GigLoader, close func(), sound func(*App) GigCrowd) *GigScreen {
	preset := 0
	if st != nil {
		preset = st.Career().Character
	}
	return &GigScreen{sh: sh, st: st, gig: g, crowd: crowd, reveal: reveal, set: set,
		load: load, close: close, sound: sound, stage: newStage(g, crowd, preset), credible: true, loaded: -1}
}

const (
	gigIntroSeconds = 2.0
	gigRevealHold   = 1.0
)

// loadSong opens song idx (without starting it); false if it would not
// open, the gig then stopping with the reason.
func (s *GigScreen) loadSong() bool {
	if s.app != nil && s.loaded == s.idx {
		return true
	}
	if s.close != nil && s.app != nil {
		s.close()
	}
	s.app, s.started, s.songT = nil, false, 0
	app, err := s.load(s.set[s.idx], s.gig.Recording)
	if err != nil {
		s.err = fmt.Sprintf("could not open %s: %v", songName(s.set[s.idx]), err)
		s.setPhase(gigFailed)
		return false
	}
	app.SetGigMode(career.ExcitementStart(s.crowd), career.DrainMult(s.crowd))
	s.app, s.loaded = app, s.idx
	if s.sound != nil && !s.gig.Recording {
		s.audio = s.sound(app)
	}
	return true
}

// startSong opens song idx and starts it.
func (s *GigScreen) startSong() {
	if !s.loadSong() {
		return
	}
	s.stage.lightLevel, s.stage.walkX, s.stage.crowdPose = 1, 0, ""
	s.app.eng.Play()
	s.setPhase(gigPlaying)
	s.stage.setMood(moodPlaying)
}

func songName(m progress.MasteredSong) string {
	if m.Title != "" {
		return m.Title
	}
	return cleanPieceTitle("", m.Path)
}

func (s *GigScreen) setPhase(p gigPhase) { s.phase, s.phaseT = p, 0 }

func (s *GigScreen) Update() error {
	s.ptr = readPointer()
	dt := uiFrameSeconds()
	s.phaseT += dt
	if s.app != nil {
		s.withBand(func() { _ = s.app.Update() })
	}
	if s.confirm {
		return s.updateConfirm()
	}
	skip := inpututil.IsKeyJustPressed(ebiten.KeySpace) || (s.ptr.pressed && s.phase != gigEnd && s.phase != gigBooed)
	switch s.phase {
	case gigIntro:
		if s.app == nil && !s.loadSong() {
			return nil
		}
		s.choreoIntro(dt)
		hold := gigIntroSeconds
		if s.reveal && s.crowd == career.CrowdTough {
			hold += gigRevealHold
		}
		if s.phaseT >= hold || skip || reduceMotion {
			s.startSong()
		}
	case gigTierCleared:
		if s.phaseT >= tierClearedSeconds || skip || inpututil.IsKeyJustPressed(ebiten.KeyEnter) || reduceMotion {
			return s.leave()
		}
	case gigPlaying:
		s.updatePlaying(dt)
	case gigBetween:
		if s.phaseT >= career.BetweenSongsSeconds || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			s.idx++
			s.startSong()
		}
	case gigEnd, gigBooed, gigFailed:
		return s.updateEnd()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || (s.app != nil && escButtonPressed(s.app.backChip(), s.ptr)) {
		if s.phase == gigPlaying || s.phase == gigBetween || s.phase == gigIntro {
			s.confirm, s.confirmFocus = true, 0
		}
	}
	return nil
}

// withBand lays the practice view out with the stage band in the
// toolbar's place, for one call.
func (s *GigScreen) withBand(fn func()) {
	stageBandH = s.bandH()
	defer func() { stageBandH = 0 }()
	fn()
}

// bandH is the scene band: about 42% of the window, shrunk first so the tab
// keeps at least 36 px between strings (§6.1).
func (s *GigScreen) bandH() float64 {
	want := math.Round(screenH * 0.42)
	minTab := 36*5 + 48 + zRulerH + sp4
	room := screenH - zHeaderH - zTrackBarH - (pgMargin + zTimelineH + sp2) - minTab
	return math.Max(80, math.Min(want, room))
}

func (s *GigScreen) updatePlaying(dt float64) {
	a := s.app
	if a == nil {
		return
	}
	playing := a.eng.Playing()
	counting, _ := a.eng.CountingIn()
	exc := 0.0
	streak := 0
	if a.game.sess != nil {
		exc = a.game.sess.Excitement.Value
		streak = a.game.sess.Excitement.Streak()
	}
	if playing && !counting {
		s.started = true
		s.songT += dt
		s.exciteSum += exc * dt
		s.exciteTime += dt
	}
	s.stage.step(dt, exc, a.posF()/float64(score.PPQ), streak)
	if s.audio != nil {
		s.audio.SetLevel([...]float64{0.35, 0.55, 0.8, 1}[scoring.Excitement{Value: exc}.Band()])
	}
	// Booed off: a tough crowd at 0 after the song's grace period.
	if playing && career.BooedOff(s.crowd, s.songT, exc) {
		a.eng.Pause() // the pass ends; the notes played keep their XP
		s.withBand(func() { _ = a.Update() })
		s.collect()
		s.finish(true)
		return
	}
	// The song's end: playback stops at the last bar.
	if s.started && !playing && a.posF() >= float64(a.sc.End())-float64(score.PPQ) {
		s.collect()
		s.stage.setMood(moodSongEnd)
		if s.audio != nil {
			s.audio.Cheer()
		}
		if s.idx+1 < len(s.set) {
			s.setPhase(gigBetween)
			return
		}
		s.finish(false)
	}
}

// collect adds the finished song's scored notes to the set.
func (s *GigScreen) collect() {
	a := s.app
	if a == nil {
		return
	}
	s.total.Merge(a.game.lastSummary)
	if !a.credible() {
		s.credible = false
	}
}

// finish rates the gig and files it. Booed off is 0 stars and no Fame;
// an unscored set (uncalibrated) earns no rating at all.
func (s *GigScreen) finish(booed bool) {
	res := &gigResult{booed: booed, scored: s.credible && s.total.Judged >= scoring.MinJudgedNotes}
	res.accuracy = s.total.Accuracy()
	if res.scored && !booed {
		score := 0.0
		if s.gig.Recording {
			t := s.total.Tightness()
			score = career.RecordingScore(res.accuracy, t.MeanMs, t.SpreadMs)
		} else {
			avg := 0.0
			if s.exciteTime > 0 {
				avg = s.exciteSum / s.exciteTime
			}
			score = career.GigScore(res.accuracy, avg)
		}
		res.stars = career.Stars(score)
	}
	if s.st != nil && (res.scored || booed) {
		keys := make([]string, len(s.set))
		for i, m := range s.set {
			keys[i] = songKey(m)
		}
		s.st.UpdateCareer(func(c *career.State) {
			res.reward = c.FinishGig(s.gig, career.Result{Stars: res.stars, BooedOff: booed, Setlist: keys})
		})
		if res.reward.BonusXP > 0 {
			s.st.AddXP(res.reward.BonusXP)
		}
	}
	s.result = res
	if booed {
		s.stage.setMood(moodBooed)
		if s.audio != nil {
			s.audio.Boo()
		}
		s.setPhase(gigBooed)
		return
	}
	s.setPhase(gigEnd)
}

// gigEndLayout places the end card's buttons.
func (s *GigScreen) endButtons() (back, retry rect) {
	card := s.cardRect()
	bw := kitButtonW("Back to tour", kitPrimary, false)
	back = rect{card.x + card.w - sp6 - bw, card.y + card.h - sp4 - btnHPrimary, bw, btnHPrimary}
	rw := kitButtonW("Retry", kitSecondary, false)
	retry = rect{back.x - sp3 - rw, back.y + (btnHPrimary-btnH)/2, rw, btnH}
	return back, retry
}

func (s *GigScreen) cardRect() rect {
	w, h := 520.0, 260.0
	band := s.bandH()
	return rect{math.Round((screenW - w) / 2), math.Round(zHeaderH + math.Max(0, (band-h)/2)), w, h}
}

func (s *GigScreen) updateEnd() error {
	back, retry := s.endButtons()
	s.stage.step(uiFrameSeconds(), 0, 0, 0)
	if s.phase != gigFailed && s.phaseT < s.endReadyAt() && !reduceMotion {
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) || s.ptr.pressed {
			s.phaseT = s.endReadyAt() // skip to the end state
		}
		return nil
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape), inpututil.IsKeyJustPressed(ebiten.KeyEnter):
		return s.backToTour()
	case s.ptr.pressed && s.ptr.over(back):
		return s.backToTour()
	case s.phase == gigBooed && (inpututil.IsKeyJustPressed(ebiten.KeyR) || (s.ptr.pressed && s.ptr.over(retry))):
		s.retry()
	}
	return nil
}

// retry plays the gig again from the top: a new attempt, the same crowd.
func (s *GigScreen) retry() {
	if s.st != nil {
		s.st.UpdateCareer(func(c *career.State) { c.StartGig(s.gig, gigRand()) })
	}
	*s = *NewGigScreen(s.sh, s.st, s.gig, s.crowd, false, s.set, s.load, s.close, s.sound)
}

// leave closes the song and goes back to the tour.
func (s *GigScreen) leave() error {
	if s.app != nil {
		s.app.eng.Pause()
		s.withBand(func() { _ = s.app.Update() }) // the pass in play ends; its XP is kept
	}
	if s.close != nil {
		s.close()
	}
	s.app = nil
	return errQuit
}

func (s *GigScreen) updateConfirm() error {
	card := s.cardRect()
	cancelW := kitButtonW("Cancel", kitSecondary, false)
	leaveW := kitButtonW("Leave the stage", kitDestructive, false)
	by := card.y + card.h - sp4 - btnH
	leaveR := rect{card.x + card.w - sp6 - leaveW, by, leaveW, btnH}
	cancelR := rect{leaveR.x - sp3 - cancelW, by, cancelW, btnH}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		s.confirm = false
	case inpututil.IsKeyJustPressed(ebiten.KeyTab), inpututil.IsKeyJustPressed(ebiten.KeyLeft), inpututil.IsKeyJustPressed(ebiten.KeyRight):
		s.confirmFocus = 1 - s.confirmFocus
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter):
		if s.confirmFocus == 1 {
			return s.leave()
		}
		s.confirm = false
	case s.ptr.pressed && s.ptr.over(cancelR):
		s.confirm = false
	case s.ptr.pressed && s.ptr.over(leaveR):
		return s.leave()
	}
	return nil
}

// ---- Drawing ----

func (s *GigScreen) Draw(dst *ebiten.Image) {
	dst.Fill(colBG)
	s.anim.tick()
	band := rect{0, zHeaderH, screenW, s.bandH()}
	if s.app != nil {
		s.withBand(func() { s.app.Draw(dst) })
	} else {
		s.drawBareHeader(dst)
	}
	if s.phase != gigPlaying {
		s.stage.step(0, s.stage.excitement, s.stage.beats, 0)
	}
	drawSceneFilled(dst, s.stage.draw(), band)

	switch s.phase {
	case gigTierCleared:
		s.drawTierCleared(dst)
		return
	case gigIntro:
		s.drawIntro(dst, band)
	case gigBetween:
		next := songName(s.set[s.idx+1])
		// The title card fades in, holds, and fades out before the next song.
		a := math.Min(1, s.phaseT/durToastIn)
		if left := career.BetweenSongsSeconds - s.phaseT; left < durToastOut {
			a = math.Max(0, left/durToastOut)
		}
		if reduceMotion {
			a = 1
		}
		s.drawTitleCard(dst, band, fmt.Sprintf("Song %d of %d", s.idx+2, len(s.set)), next, "Space starts it now", a)
	case gigEnd, gigBooed, gigFailed:
		s.drawEndCard(dst)
	}
	if s.confirm {
		s.drawConfirm(dst)
	}
}

// drawBareHeader is the header while no song is open.
func (s *GigScreen) drawBareHeader(dst *ebiten.Image) {
	x := drawEscButton(dst, escBackChip, &s.anim, s.ptr)
	drawTextScaled(dst, s.gig.Venue, x, 6, dispL, colNote)
	drawText(dst, career.Tiers[s.gig.Tier-1].Name+" · "+s.gig.City.Name, x, 42, colDim)
}

// drawTitleCard is a centred card over the scene band.
func (s *GigScreen) drawTitleCard(dst *ebiten.Image, band rect, kicker, title, foot string, alpha float64) {
	if alpha <= 0 {
		return
	}
	w := math.Max(360, textWScaled(title, dispL)+2*sp8)
	h := 128.0
	r := rect{math.Round((screenW - w) / 2), math.Round(band.y + (band.h-h)/2), w, h}
	notchedFill(dst, r, withAlpha(colPanel, 0.94*alpha))
	notchedStroke(dst, r, 1, withAlpha(colPanelEdge, alpha))
	k := spaced(kicker)
	drawTextScaled(dst, k, math.Round(r.x+(r.w-textWScaled(k, dispS))/2), r.y+sp4, dispS, withAlpha(colBarline, alpha))
	drawTextScaled(dst, title, math.Round(r.x+(r.w-textWScaled(title, dispL))/2), r.y+sp4+sizeDisplayS+sp3, dispL, withAlpha(colNote, alpha))
	if foot != "" {
		drawTextSmall(dst, foot, math.Round(r.x+(r.w-textWSmall(foot))/2), r.y+r.h-sp4-14, withAlpha(colBarline, alpha))
	}
}

// drawIntro is the venue's card, and the tough-crowd reveal on a first
// attempt (the full choreography is §9).
func (s *GigScreen) drawIntro(dst *ebiten.Image, band rect) {
	foot := fmt.Sprintf("%d %s · Space starts", len(s.set), plural(len(s.set), "song", "songs"))
	a := math.Min(1, s.phaseT/durOverlayIn)
	if reduceMotion {
		a = 1
	}
	s.drawTitleCard(dst, band, career.Tiers[s.gig.Tier-1].Name+" · "+s.gig.City.Name, s.gig.Venue, foot, a)
	if s.reveal && s.crowd == career.CrowdTough && (s.phaseT >= 1.9 || reduceMotion) {
		w := textWScaled("TOUGH CROWD", dispM)
		x := math.Round((screenW - w) / 2)
		y := band.y + band.h/2 + 64 + sp3
		drawTextScaled(dst, "TOUGH CROWD", x, y, dispM, colMiss)
		f := stepIndex(s.phaseT-1.9, 3) // the flames pop in three steps
		drawSpriteOn(dst, "fx", "flames", "burn", min(2, f), x-sp4, y+28, 2)
		drawSpriteOn(dst, "fx", "flames", "burn", min(2, f), x+w+sp4+16, y+28, 2)
	}
}

func (s *GigScreen) drawEndCard(dst *ebiten.Image) {
	t := s.phaseT
	if reduceMotion || s.phase == gigFailed {
		t = 1e6 // end states at once
	}
	cardAt := endCardAt
	if s.phase == gigBooed {
		cardAt = booCardAt
	}
	if t < cardAt {
		return // the bow, or the boos, come first
	}
	rollAt := endCardAt + 5*starStep
	vector.FillRect(dst, 0, 0, float32(screenW), float32(screenH), withAlpha(colBG, 0.45), false)
	card := s.cardRect()
	res := s.result
	title := "Set complete"
	switch {
	case s.phase == gigFailed:
		title = "The gig stopped"
	case res != nil && res.booed:
		title = "Booed off"
	case s.gig.Recording:
		title = "Session complete"
	}
	y := drawDialogCard(dst, card, title)
	x := card.x + sp6
	switch {
	case s.phase == gigFailed:
		for i, l := range wrapTextW(s.err, card.w-2*sp6) {
			drawText(dst, l, x, y+float64(i)*uiTextH, colClose)
		}
	case res == nil:
	case res.booed:
		drawText(dst, "0 stars. The XP for the notes you played is kept.", x, y, colDim)
		drawText(dst, "A tough crowd stays tough: try again any time.", x, y+uiTextH, colDim)
	case !res.scored:
		drawText(dst, "Unscored: calibrate your timing in Settings to earn", x, y, colDim)
		drawText(dst, "stars and Fame. The XP from your notes still counts.", x, y+uiTextH, colDim)
	default:
		// The stars, one at a time, 150 ms apart; each pops in three steps.
		for i := 0; i < 5; i++ {
			anim, scale := "unlit", 2
			litAt := endCardAt + float64(i)*starStep
			if i < res.stars && t >= litAt {
				anim = "lit"
				if stepIndex(t-litAt, 3) < 2 {
					scale = 3
				}
			}
			drawSpriteOn(dst, "fx", "star", anim, 0, x+16+float64(i)*40, y+16, scale)
		}
		line := fmt.Sprintf("Accuracy %d%%", int(math.Round(res.accuracy*100)))
		if res.reward.BonusXP > 0 {
			line += fmt.Sprintf("  ·  +%d XP", rolled(res.reward.BonusXP, t, rollAt))
		}
		if res.reward.Fame > 0 {
			line += fmt.Sprintf("  ·  +%d Fame", rolled(res.reward.Fame, t, rollAt))
		}
		drawText(dst, line, x, y+44, colNote)
		if res.reward.NewBest && t >= rollAt+durRoll {
			drawPill(dst, x, y+44+uiTextH+sp2, "NEW BEST", pillBrand)
		}
		if t := res.reward.TierCleared; t > 0 {
			drawText(dst, career.Tiers[t-1].Name+" cleared!", x+pillW("NEW BEST")+sp3, y+44+uiTextH+sp2+3, colHit)
		}
	}
	back, retry := s.endButtons()
	if t < s.endReadyAt() {
		return
	}
	drawKitButton(dst, back, "Back to tour", -1, kitPrimary, btnState{hover: s.ptr.over(back)})
	if s.phase == gigBooed {
		drawKitButton(dst, retry, "Retry", -1, kitSecondary, btnState{hover: s.ptr.over(retry)})
	}
}

func (s *GigScreen) drawConfirm(dst *ebiten.Image) {
	vector.FillRect(dst, 0, 0, float32(screenW), float32(screenH), colHelpDim, false)
	card := s.cardRect()
	y := drawDialogCard(dst, card, "Leave the stage?")
	drawText(dst, "This gig won't count. XP for the notes you've played is kept.", card.x+sp6, y, colDim)
	cancelW := kitButtonW("Cancel", kitSecondary, false)
	leaveW := kitButtonW("Leave the stage", kitDestructive, false)
	by := card.y + card.h - sp4 - btnH
	leaveR := rect{card.x + card.w - sp6 - leaveW, by, leaveW, btnH}
	cancelR := rect{leaveR.x - sp3 - cancelW, by, cancelW, btnH}
	drawKitButton(dst, cancelR, "Cancel", -1, kitSecondary, btnState{hover: s.ptr.over(cancelR), focus: s.confirmFocus == 0})
	drawKitButton(dst, leaveR, "Leave the stage", -1, kitDestructive, btnState{hover: s.ptr.over(leaveR), focus: s.confirmFocus == 1})
}

// ShotState puts the gig in a moment for the dev screenshot tool
// (tools/uishot): "playing", "intro-tough", "booed" or "end".
func (s *GigScreen) ShotState(kind string) {
	switch kind {
	case "intro-tough":
		s.reveal, s.phaseT = true, 2.4
		s.choreoIntro(0)
	case "playing":
		s.startSong()
	case "booed":
		s.startSong()
		s.total = scoring.Summary{Judged: 20, SumScore: 12}
		s.finish(true)
		s.stage.moodT = 1
		s.phaseT = 10
	case "end":
		s.startSong()
		s.total = scoring.Summary{Judged: 40, SumScore: 36}
		s.exciteSum, s.exciteTime = 70*60, 60
		s.finish(false)
		s.stage.setMood(moodSongEnd)
		s.phaseT = 10
	}
}

// ---- Choreography (CAREER.md §9; Space skips each to its end) ----

const (
	tierClearedSeconds = 2.5
	endCardAt          = 0.6  // the bow, then the card
	booCardAt          = 1.2  // the boos, the dimming, the slump, then the card
	starStep           = 0.15 // between stars
)

// choreoIntro runs the gig's intro (≤ 2.5 s): the lights off, the card,
// the lights up in four steps, the guitarist walking on in whole pixels,
// the crowd's cheer, and a first tough crowd's reveal.
func (s *GigScreen) choreoIntro(dt float64) {
	t := s.phaseT
	st := s.stage
	switch {
	case t < 0.3:
		st.lightLevel = 0
	case t < 0.7:
		st.lightLevel = math.Floor((t-0.3)/0.4*4) / 4 // steps4
	default:
		st.lightLevel = 1
	}
	switch {
	case t < 0.7:
		st.walkX = -16
	case t < 1.4:
		st.walkX = -16 + 156*curveMove.at((t-0.7)/0.7) // travel to the mark
	default:
		st.walkX = 0
	}
	st.crowdPose = ""
	if t >= 1.4 && !s.cheered {
		s.cheered = true
		if s.audio != nil {
			s.audio.Cheer()
		}
	}
	if t >= 1.4 && t < 1.9 && !s.gig.Recording {
		st.crowdPose = "jump" // the cheer swell
	}
	if s.reveal && s.crowd == career.CrowdTough && t >= 1.9 {
		st.crowdPose = "crossed"
	}
	st.step(dt, career.ExcitementStart(s.crowd), 0, 0)
}

// endReadyAt is when the end card's buttons arrive.
func (s *GigScreen) endReadyAt() float64 {
	if s.phase == gigBooed {
		return booCardAt + 0.2
	}
	return endCardAt + 5*starStep + durRoll + 0.3
}

// rolled is a number counting up across durRoll from start (a roll).
func rolled(n int, t, start float64) int {
	if reduceMotion || t >= start+durRoll {
		return n
	}
	if t <= start {
		return 0
	}
	return int(math.Round(float64(n) * curveEnter.at((t-start)/durRoll)))
}

// drawTierCleared is the tier-cleared moment: the tier's name, the next
// tier's venue sliding in, where the player is booked next, Fame rolling.
func (s *GigScreen) drawTierCleared(dst *ebiten.Image) {
	dst.Fill(colBG)
	t := s.phaseT
	tier := s.result.reward.TierCleared
	name := career.Tiers[tier-1].Name + " cleared"
	drawTextScaled(dst, name, math.Round((screenW-textWScaled(name, dispX))/2), screenH*0.16, dispX, tokBrand)
	next, ok := career.TierByNum(tier + 1)
	if !ok || s.st == nil {
		return
	}
	c := s.st.Career()
	g := c.Route()[next.Num-1][0]
	img := ebiten.NewImage(scene.BaseW, scene.BaseH)
	drawVenue(img, tierSlug(next.Num, false), g.City.Region, 0)
	area := rect{0, screenH * 0.3, screenW, screenH * 0.42}
	slide := 1.0
	if !reduceMotion {
		slide = curveMove.at(math.Min(1, t/durTravel))
	}
	area.x += math.Round((1 - slide) * screenW)
	drawSceneFilled(dst, img, area)
	img.Deallocate()
	booked := fmt.Sprintf("Booked: %s, %s", next.Name, g.City.Name)
	drawText(dst, booked, math.Round((screenW-textW(booked))/2), area.y+area.h+sp6, colNote)
	fame := fmt.Sprintf("Fame %d", c.Fame-s.result.reward.Fame+rolled(s.result.reward.Fame, t, durTravel))
	drawTextScaled(dst, fame, math.Round((screenW-textWScaled(fame, dispM))/2), area.y+area.h+sp6+uiTextH+sp4, dispM, colNote)
}

// backToTour leaves the end card: through the tier-cleared moment when
// this gig cleared its tier.
func (s *GigScreen) backToTour() error {
	if s.phase == gigEnd && s.result != nil && s.result.reward.TierCleared > 0 {
		if s.app != nil {
			s.app.eng.Pause()
		}
		s.setPhase(gigTierCleared)
		return nil
	}
	return s.leave()
}
