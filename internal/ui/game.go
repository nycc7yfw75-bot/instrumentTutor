package ui

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/S95F/musicTutor/internal/practice"
	"github.com/S95F/musicTutor/internal/progress"
	"github.com/S95F/musicTutor/internal/scoring"
)

// The game layer on the practice view (docs/SCORING.md §6, §10): it scores
// the live results, tracks passes and files them under the player's
// profile. Nothing here touches the audio path: it reads what the view
// already reads each frame, and every call into it is guarded so the game
// layer can never break a practice session.

// gameUI is the practice view's game state.
type gameUI struct {
	store      *progress.Store
	songID     string
	songTitle  string
	songPath   string
	calibrated bool // the device pair's calibration is confident enough
	sampleRate int
	noteIndex  map[progress.NoteKey]int
	difficulty scoring.Difficulty

	sess *scoring.Session

	// The pass being played.
	passOn      bool
	passStart   int64 // tick the pass began
	passLoops   int   // the engine's loop pass count at its start
	passSecs    float64
	passDrop    int64 // dropped samples at its start
	passWaited  bool  // wait mode was on at some point
	passJudges  []scoring.Judgement
	lastFrames  int64
	wasPlaying  bool
	lastSummary scoring.Summary // the last finished pass, for the HUD and review

	// Rewards waiting for the player to stop: the XP toast, then the
	// level-up card (MOTION §B1.5, §B1.6).
	askName func() // opens the name prompt

	toasts  []int
	levelUp *levelCard
	toast   *xpToast

	ghosts []ghost // timing ghosts on the tab (§7)
	review reviewState

	excite exciteView
	xpView xpBarView
}

// SetProgress turns the game layer on for this piece: results are scored
// and passes filed under the store's active profile. calibrated is whether
// the device pair's timing is calibrated with enough confidence.
func (a *App) SetProgress(store *progress.Store, songID, title, path string, calibrated bool) {
	a.game.store = store
	a.game.songID, a.game.songTitle, a.game.songPath = songID, title, path
	a.game.calibrated = calibrated
	a.game.sampleRate = a.eng.SampleRate()
	a.game.noteIndex = progress.NoteIndex(a.sc, a.track)
	a.game.difficulty = scoring.DifficultyOf(a.sc, a.track)
	a.game.sess = scoring.NewSession(a.sc, a.track, a.game.sampleRate, a.credible())
	if _, xp, ok := store.Active(); ok {
		a.game.xpView.snap(xp.TotalXP)
	}
}

// credible is the credibility gate right now: listening, calibrated, wait
// mode off, and no audio dropped during the pass.
func (a *App) credible() bool {
	g := &a.game
	return a.live && g.calibrated && !a.wait && !g.passWaited && a.dropped == g.passDrop
}

// unscoredReason explains, in plain words, why play is not earning XP.
func (a *App) unscoredReason() string {
	switch {
	case !a.live:
		return "Not listening — choose your input in Settings to score"
	case a.wait:
		return "Unscored — wait mode is practice"
	case !a.game.calibrated:
		return "Unscored — calibrate to earn XP"
	case a.dropped != a.game.passDrop:
		return "Unscored — audio dropped this pass"
	}
	return ""
}

// gameResults scores new results from the practice scorer.
func (a *App) gameResults(rs []practice.NoteResult) {
	g := &a.game
	if g.sess == nil {
		return
	}
	for _, r := range rs {
		before := g.sess.Excitement.Value
		js := g.sess.Add(r)
		g.passJudges = append(g.passJudges, js...)
		a.addGhosts(js)
		g.excite.note(before, g.sess.Excitement.Value)
	}
}

// gameStep runs once a frame: it advances excitement by playing time and
// notices passes ending.
func (a *App) gameStep() {
	g := &a.game
	if g.sess == nil {
		return
	}
	defer func() {
		// The game layer must never take a practice session down with it.
		if r := recover(); r != nil {
			g.sess = nil
		}
	}()
	frames := a.eng.TotalFrames()
	dFrames := frames - g.lastFrames
	g.lastFrames = frames
	playing := a.eng.Playing()

	transport := scoring.Paused
	switch counting, _ := a.eng.CountingIn(); {
	case !playing:
	case counting:
		transport = scoring.CountingIn
	case a.eng.Waiting():
		transport = scoring.Waiting
	default:
		transport = scoring.Playing
	}
	if a.wait {
		g.passWaited = true
	}
	g.sess.SetCredible(a.credible())

	switch {
	case playing && !g.wasPlaying:
		// Playback starts: a fresh pass, and excitement back to 50.
		g.sess.Excitement = scoring.NewExcitement()
		a.startPass(a.eng.PosTick())
	case !playing && g.wasPlaying:
		// A pause or the song's end closes the pass.
		a.endPass(a.eng.PosTick())
	case playing && a.eng.PassCount() != g.passLoops:
		// The loop came round: one pass done, the next begins at its start.
		la, lb, _ := a.eng.Loop()
		a.endPass(lb - 1)
		a.startPass(la)
	}
	g.wasPlaying = playing
	if g.review.wanted && !playing {
		g.review.wanted = false
		a.openReview(false)
	}

	if dFrames > 0 && g.sampleRate > 0 {
		secs := float64(dFrames) / float64(g.sampleRate)
		g.sess.Tick(secs, transport)
		if transport == scoring.Playing {
			g.passSecs += secs
		}
	}
	g.excite.frame(&g.sess.Excitement, transport == scoring.Playing)
}

func (a *App) startPass(tick int64) {
	g := &a.game
	g.passOn, g.passStart = true, tick
	g.passLoops = a.eng.PassCount()
	g.passSecs, g.passDrop = 0, a.dropped
	g.passWaited = a.wait
	g.passJudges = g.passJudges[:0]
	g.ghosts = g.ghosts[:0] // a new pass starts with a clean tab
	g.sess.EndPass()        // results before the start belong to no pass
}

// endPass files the pass that just finished under the player's profile.
func (a *App) endPass(tick int64) {
	g := &a.game
	if !g.passOn {
		return
	}
	g.passOn = false
	credible := a.credible()
	flushed := g.sess.Flush()
	g.passJudges = append(g.passJudges, flushed...)
	a.addGhosts(flushed)
	sum := g.sess.EndPass()
	g.lastSummary = sum
	passFrom, passTo := min(g.passStart, tick), max(g.passStart, tick)
	if rv := a.makeReview(sum, g.passJudges, passFrom, passTo); rv != nil {
		g.review.pass = rv
		if tick >= a.sc.End() && !g.review.ov.open {
			defer a.openReview(true) // a full run ended: show how it went
		}
	}
	if g.store == nil || g.store.NeedsProfile() {
		return
	}
	from, to := a.barAt(passFrom), a.barAt(passTo)
	rec, err := g.store.RecordPass(progress.Pass{
		SongID: g.songID, Title: g.songTitle, Path: g.songPath,
		Track:   a.track,
		FromBar: a.sc.BarNumber(max(0, from)), ToBar: a.sc.BarNumber(max(0, to)),
		TempoScale: a.eng.TempoScale(), Difficulty: g.difficulty,
		Summary: sum, Judgements: append([]scoring.Judgement(nil), g.passJudges...),
		NoteIndex: g.noteIndex, Credible: credible, Seconds: g.passSecs,
	})
	if err != nil {
		a.setBPMMessage("could not save your progress: " + err.Error())
		return
	}
	if _, xp, ok := g.store.Active(); ok {
		g.xpView.aim(xp.TotalXP, rec.LeveledUp)
	}
	if rv := g.review.pass; rv != nil && rv.from == passFrom && rv.to == passTo {
		rv.xp, rv.scored = rec.XP, credible
	}
	if rec.XP > 0 {
		g.toasts = append(g.toasts, rec.XP)
	}
	if rec.LeveledUp {
		_, xp, _ := g.store.Active()
		lvl := xp.Level()
		g.levelUp = newLevelCard(lvl, scoring.Title(lvl), scoring.Title(lvl-1) != scoring.Title(lvl))
	}
}

// gameHUDRects are the HUD row's clickable parts.
func (a *App) gameHUDReason() (string, rect) {
	if a.game.store != nil && a.game.store.NeedsProfile() {
		s := "Add your name to start saving your progress"
		return s, rect{uiPadX, a.legendY() - 4, textW(s), uiTextH + 8}
	}
	s := a.unscoredReason()
	return s, rect{uiPadX, a.legendY() - 4, textW(s), uiTextH + 8}
}

// drawGameHUD draws the HUD row under the timeline: excitement (or why play
// is unscored) on the left, the level and XP on the right; then any XP toast
// and level-up card once the player has stopped.
func (a *App) drawGameHUD(screen *ebiten.Image) {
	g := &a.game
	y := a.legendY()
	if reason, r := a.gameHUDReason(); reason != "" {
		col := colClose
		if a.ptr.over(r) {
			col = colNote
		}
		drawText(screen, reason, r.x, y, col)
	} else {
		g.excite.draw(screen, uiPadX, y+3)
	}
	if g.store != nil {
		if _, xp, ok := g.store.Active(); ok {
			g.xpView.step()
			left := g.xpView.draw(screen, screenW-uiPadX, y+7)
			lvl := xp.Level()
			label := fmt.Sprintf("LV %d · %s", lvl, scoring.Title(lvl))
			drawTextRight(screen, label, left-10, y, colHUD)
		}
	}

	// Rewards wait for the player to stop (MOTION §B1.5, §B1.6).
	if a.eng.Playing() {
		return
	}
	a.drawReview(screen)
	if g.toast == nil && len(g.toasts) > 0 {
		g.toast = &xpToast{xp: g.toasts[0]}
		g.toasts = g.toasts[1:]
	}
	if g.toast != nil {
		g.toast.draw(screen, screenW-uiPadX, y-24)
		if g.toast.done() {
			g.toast = nil
		}
	}
	if g.levelUp != nil {
		g.levelUp.draw(screen)
		if g.levelUp.done() {
			g.levelUp = nil
			if _, xp, ok := g.store.Active(); ok {
				g.xpView.snap(xp.TotalXP) // the reset, behind the card
			}
		}
	}
}

// gameModal reports whether the level-up card is up; Space or a click skips
// it to its exit, and the frame's input is spent.
func (a *App) gameModal() bool {
	c := a.game.levelUp
	if c == nil || a.eng.Playing() {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) || a.ptr.pressed {
		c.skip()
	}
	return true
}

// gameClick handles a click on the HUD row's message: calibrate, or name
// the profile.
func (a *App) gameClick(p pointer) bool {
	if a.game.sess == nil || !p.pressed {
		return false
	}
	reason, r := a.gameHUDReason()
	if reason == "" || !p.over(r) {
		return false
	}
	if a.game.store != nil && a.game.store.NeedsProfile() {
		if a.game.askName != nil {
			a.game.askName()
		}
		return true
	}
	if a.live && !a.wait {
		a.openSettings()
	}
	return true
}

// SetNameAsker lets the HUD's "add your name" open the name prompt.
func (a *App) SetNameAsker(fn func()) { a.game.askName = fn }

// Close files the pass in progress when the player leaves the piece.
func (a *App) Close() {
	g := &a.game
	if g.sess == nil || !g.passOn {
		return
	}
	defer func() { _ = recover() }() // leaving must never fail
	a.endPass(a.eng.PosTick())
}

// gameStatsLine is the stats line with the scored accuracy of the pass:
// "—" until enough notes are judged for a fair number.
func gameStatsLine(st practice.Stats, pass scoring.Summary) string {
	acc := "—"
	if pct, ok := pass.AccuracyPct(); ok {
		acc = fmt.Sprintf("%d%%", pct)
	}
	return fmt.Sprintf("Hit %d   Close %d   Miss %d   ·   %s", st.Hit, st.Close, st.Miss, acc)
}
