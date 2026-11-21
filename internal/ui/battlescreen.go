package ui

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/battle"
	"github.com/S95F/musicTutor/internal/practice"
	"github.com/S95F/musicTutor/internal/progress"
	"github.com/S95F/musicTutor/internal/scoring"
)

// The battle screen (BATTLES.md §6.3–6.4): the scoreboard and crowd meter
// on top, the practice view's tab below, turns in rotating order with a
// banner, a sound check before each player's first turn, result cards,
// and the results at the end.

type battlePhase int

const (
	bpVS battlePhase = iota
	bpBanner
	bpSoundCheck
	bpPlaying
	bpResult
	bpResults
	bpFailed
)

// BattleScreen runs a battle.
type BattleScreen struct {
	sh     *Shell
	st     *progress.Store
	deps   BattleDeps
	b      *battle.Battle
	ghosts map[int]GhostEntry
	rng    *rand.Rand

	app   *App
	err   string
	phase battlePhase
	t     float64 // seconds in the phase
	ptr   pointer
	anim  animator

	exchange, player int
	lastHuman        int
	checked          map[int]bool
	check            soundCheck
	turn             battle.Turn
	pending          []practice.NoteResult // a bot's results, shown as the playhead passes
	crowdBefore      float64
	crown            float64 // the crown's x, sliding to the leader
	leader           int
	origActive       string

	confirm      bool
	confirmFocus int // 0 Cancel (the default), 1 Abandon

	filed   bool
	bonuses []int
	result  battle.Result

	rematch func()
	newOne  func()
}

// NewBattleScreen starts a battle from its setup.
func NewBattleScreen(sh *Shell, st *progress.Store, deps BattleDeps, s battle.Setup, ghosts []GhostEntry) (*BattleScreen, error) {
	b, err := battle.New(s)
	if err != nil {
		return nil, err
	}
	bs := &BattleScreen{sh: sh, st: st, deps: deps, b: b, ghosts: map[int]GhostEntry{},
		rng: rand.New(rand.NewSource(time.Now().UnixNano())), checked: map[int]bool{}, lastHuman: -1, leader: -1}
	for _, g := range ghosts {
		bs.ghosts[g.player] = g
	}
	if st != nil {
		for _, p := range st.Profiles() {
			if p.Active {
				bs.origActive = p.ID
			}
		}
	}
	if deps.Open != nil {
		app, err := deps.Open(s.Material.Path, s.Material.Track)
		if err != nil {
			bs.err, bs.phase = "could not open the song: "+err.Error(), bpFailed
		} else {
			app.SetGigMode(scoring.ExcitementStart, 1) // the locks; tempo is set per turn
			bs.app = app
		}
	}
	return bs, nil
}

// SetRematch and SetNewBattle wire the results' buttons.
func (s *BattleScreen) SetRematch(fn func())   { s.rematch = fn }
func (s *BattleScreen) SetNewBattle(fn func()) { s.newOne = fn }

const (
	bsBandH      = 176.0
	vsSeconds    = 1.6
	botUpSeconds = 1.0
	resultSteps  = 4
	resultStep   = 0.35
	autoAdvance  = 3.0
)

func (s *BattleScreen) setPhase(p battlePhase) { s.phase, s.t = p, 0 }

func (s *BattleScreen) participant(i int) battle.Participant { return s.b.Setup.Participants[i] }

func (s *BattleScreen) withBand(fn func()) {
	stageBandH = bsBandH
	defer func() { stageBandH = 0 }()
	fn()
}

func (s *BattleScreen) Update() error {
	s.ptr = readPointer()
	dt := uiFrameSeconds()
	s.t += dt
	if s.app != nil {
		s.withBand(func() { _ = s.app.Update() })
	}
	if s.confirm {
		return s.updateConfirm()
	}
	space := inpututil.IsKeyJustPressed(ebiten.KeySpace)
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) && s.phase != bpResults && s.phase != bpFailed {
		s.confirm, s.confirmFocus = true, 0
		return nil
	}
	switch s.phase {
	case bpFailed:
		if space || inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			return s.leave()
		}
	case bpVS:
		if s.t >= vsSeconds || space || reduceMotion {
			s.nextTurn()
		}
	case bpBanner:
		p := s.participant(s.player)
		if p.Kind.IsBot() {
			if s.t >= botUpSeconds || reduceMotion {
				s.startTurn()
			}
		} else if space || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			if !s.checked[s.player] {
				s.check.retry()
				s.setPhase(bpSoundCheck)
			} else {
				s.startTurn()
			}
		}
	case bpSoundCheck:
		if s.app != nil {
			s.check.update(s.app.levelDB, dt)
		}
		if s.check.done {
			if s.check.passed && (s.t > 0.6 || space) {
				s.checked[s.player] = true
				s.startTurn()
			} else if !s.check.passed && space {
				s.check.retry()
			}
		}
	case bpPlaying:
		s.updatePlaying()
	case bpResult:
		last := resultStep * (resultSteps - 1)
		if reduceMotion {
			s.t = math.Max(s.t, last)
		}
		if space {
			if s.t < last {
				s.t = last
			} else {
				s.nextTurn()
			}
		} else if s.t >= last+autoAdvance {
			s.nextTurn()
		}
	case bpResults:
		return s.updateResults()
	}
	return nil
}

// nextTurn moves on, or ends the battle.
func (s *BattleScreen) nextTurn() {
	k, p, ok := s.b.Next()
	if !ok {
		s.finish()
		return
	}
	s.exchange, s.player = k, p
	s.setPhase(bpBanner)
}

// startTurn plays the phrase: the human's, at their tempo with the live
// input judged; or the bot's, the phrase as written with its results.
func (s *BattleScreen) startTurn() {
	a := s.app
	if a == nil {
		return
	}
	p := s.participant(s.player)
	ph := s.b.Plan.Phrases[s.exchange]
	a.ClearTurn()
	a.SetBotTurn(p.Kind.IsBot())
	s.pending = nil
	if p.Kind == battle.KindHuman && s.st != nil {
		s.st.SetActive(p.ID) // the turn's XP is the player's (§8.1)
		s.lastHuman = s.player
	}
	tempo := p.Tempo()
	a.eng.SetTempoScale(tempo)
	a.eng.SeekTick(ph.FromTick)
	if p.Kind.IsBot() {
		sum, js := s.botPlay(p, ph)
		s.turn = battle.NewTurn(s.player, s.exchange, sum, 1, true)
		s.turn.Judgements = js
		for _, j := range js {
			s.pending = append(s.pending, practice.NoteResult{Event: j.Event, Verdict: j.Verdict})
		}
		sort.Slice(s.pending, func(i, j int) bool { return s.pending[i].Event.Start < s.pending[j].Event.Start })
	}
	a.eng.Play()
	s.setPhase(bpPlaying)
}

// botPlay is a bot's turn at a phrase.
func (s *BattleScreen) botPlay(p battle.Participant, ph battle.Phrase) (scoring.Summary, []scoring.Judgement) {
	m := s.b.Setup.Material
	rate := 48000
	if s.app != nil && s.app.eng.SampleRate() > 0 {
		rate = s.app.eng.SampleRate()
	}
	switch p.Kind {
	case battle.KindGhost:
		g := s.ghosts[s.player]
		sum, js, _ := battle.GhostTurn(g.notes, g.index, m.Score, m.Track, ph)
		return sum, js
	case battle.KindSession:
		prof, _ := battle.SessionProfile(p.ID)
		return battle.BotTurn(prof, m.Score, m.Track, ph, s.rng, rate)
	}
	r, _ := battle.RivalNamed(p.ID)
	return battle.BotTurn(r.Profile, m.Score, m.Track, ph, s.rng, rate)
}

func (s *BattleScreen) updatePlaying() {
	a := s.app
	ph := s.b.Plan.Phrases[s.exchange]
	pos := a.posF()
	for len(s.pending) > 0 && float64(s.pending[0].Event.Start) <= pos {
		a.ShowBotResults(s.pending[:1])
		s.pending = s.pending[1:]
	}
	counting, _ := a.eng.CountingIn()
	if counting || (a.eng.Playing() && pos < float64(ph.ToTick)) {
		return
	}
	// The phrase is over: stop, let the pass close, and score the turn.
	a.eng.Pause()
	s.withBand(func() { _ = a.Update() })
	p := s.participant(s.player)
	if p.Kind == battle.KindHuman {
		s.turn = battle.NewTurn(s.player, s.exchange, a.game.lastSummary, p.Tempo(), a.credible())
		s.turn.Judgements = append([]scoring.Judgement(nil), a.game.passJudges...)
	}
	a.ShowBotResults(s.pending)
	s.pending = nil
	a.SetBotTurn(false)
	if s.b.Players() == 2 {
		s.crowdBefore = s.b.Crowd2()
	}
	s.b.Record(s.turn)
	s.setPhase(bpResult)
}

// finish ends the battle: the results, then the records (§8).
func (s *BattleScreen) finish() {
	s.result = s.b.Result()
	s.bonuses = s.result.Bonuses()
	s.fileBattle()
	if s.st != nil && s.origActive != "" {
		s.st.SetActive(s.origActive)
	}
	s.setPhase(bpResults)
}

// leave closes the song and goes back.
func (s *BattleScreen) leave() error {
	if s.app != nil {
		s.app.eng.Pause()
		s.app.SetBotTurn(true) // an abandoned turn is not filed as a pass
		s.withBand(func() { _ = s.app.Update() })
	}
	if s.deps.Close != nil {
		s.deps.Close()
	}
	if s.st != nil && s.origActive != "" {
		s.st.SetActive(s.origActive)
	}
	return errQuit
}

func (s *BattleScreen) updateConfirm() error {
	c := s.cardRect(520, 200)
	cancel, abandon := s.confirmButtons(c)
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
	case s.ptr.pressed && s.ptr.over(cancel):
		s.confirm = false
	case s.ptr.pressed && s.ptr.over(abandon):
		return s.leave()
	}
	return nil
}

func (s *BattleScreen) cardRect(w, h float64) rect {
	return rect{math.Round((screenW - w) / 2), math.Round((screenH - h) / 2), w, h}
}

func (s *BattleScreen) confirmButtons(c rect) (cancel, abandon rect) {
	aw := kitButtonW("Abandon battle", kitDestructive, false)
	cw := kitButtonW("Cancel", kitSecondary, false)
	y := c.y + c.h - sp4 - btnH
	abandon = rect{c.x + c.w - sp6 - aw, y, aw, btnH}
	cancel = rect{abandon.x - sp3 - cw, y, cw, btnH}
	return cancel, abandon
}

// ---- Drawing ----

func (s *BattleScreen) Draw(dst *ebiten.Image) {
	dst.Fill(colBG)
	s.anim.tick()
	if s.phase == bpResults {
		s.drawResults(dst)
		return
	}
	if s.app != nil {
		s.withBand(func() { s.app.Draw(dst) })
	}
	band := rect{0, zHeaderH, screenW, bsBandH}
	vector.FillRect(dst, 0, float32(band.y), float32(screenW), float32(band.h), colBG, false)
	s.drawScoreboard(dst, band)

	switch s.phase {
	case bpFailed:
		c := s.cardRect(560, 180)
		y := drawDialogCard(dst, c, "The battle couldn't start")
		drawText(dst, truncateW(s.err, c.w-2*sp6), c.x+sp6, y, colClose)
		drawTextSmall(dst, "Esc goes back", c.x+sp6, y+uiTextH*2, colBarline)
	case bpVS:
		s.drawVS(dst)
	case bpBanner:
		s.drawBanner(dst)
	case bpSoundCheck:
		s.check.draw(dst, rect{0, band.y + band.h, screenW, screenH - band.y - band.h}, s.participant(s.player).Name)
	case bpResult:
		s.drawResultCard(dst)
	}
	if s.confirm {
		vector.FillRect(dst, 0, 0, float32(screenW), float32(screenH), colHelpDim, false)
		c := s.cardRect(520, 200)
		y := drawDialogCard(dst, c, "Abandon battle?")
		drawText(dst, "No record will be saved.", c.x+sp6, y, colDim)
		cancel, abandon := s.confirmButtons(c)
		drawKitButton(dst, cancel, "Cancel", -1, kitSecondary, btnState{hover: s.ptr.over(cancel), focus: s.confirmFocus == 0})
		drawKitButton(dst, abandon, "Abandon battle", -1, kitDestructive, btnState{hover: s.ptr.over(abandon), focus: s.confirmFocus == 1})
	}
}

// drawPortrait draws a participant's picture: a profile's guitar in its
// tint, a rival's pixel portrait, a ghost's faded guitar, the Session
// Player's amp.
func drawPortrait(dst *ebiten.Image, p battle.Participant, x, y float64, scale int) {
	switch p.Kind {
	case battle.KindHuman:
		drawPixelIconScaled(dst, icGuitar, x, y, scale, profileTint(p.Tint))
	case battle.KindGhost:
		drawPixelIconScaled(dst, icGuitar, x, y, scale, withAlpha(profileTint(p.Tint), 0.5))
	case battle.KindSession:
		drawPixelIconScaled(dst, icMixer, x, y, scale, colDim)
	default:
		i := 0
		for k, r := range battle.Rivals {
			if r.Name == p.ID {
				i = k
			}
		}
		drawSpriteOn(dst, "crowd", "member"+itoa(i%6), "idle", 0, x+float64(8*scale), y+float64(16*scale), scale)
	}
}

// drawScoreboard draws a column per player (portrait, name, BOT, total,
// exchange dots) and the crowd meter under them.
func (s *BattleScreen) drawScoreboard(dst *ebiten.Image, band rect) {
	n := s.b.Players()
	colW := math.Min(260, (screenW-2*pgMargin)/float64(n))
	x0 := math.Round((screenW - colW*float64(n)) / 2)
	totals := s.b.Totals()
	leader := s.b.Leader()
	// The crown slides to a new leader (MOTION §B2.6).
	if leader >= 0 {
		target := x0 + float64(leader)*colW + colW/2
		if s.leader < 0 || reduceMotion {
			s.crown = target
		} else {
			s.crown += (target - s.crown) * math.Min(1, uiFrameSeconds()/durTravel*3)
		}
	}
	s.leader = leader
	for i := 0; i < n; i++ {
		p := s.participant(i)
		c := rect{x0 + float64(i)*colW + sp1, band.y + sp2, colW - sp2, 108}
		fill := colPanel
		if (s.phase == bpPlaying || s.phase == bpBanner || s.phase == bpSoundCheck) && i == s.player {
			fill = colHover
		}
		notchedFill(dst, c, fill)
		if (s.phase == bpPlaying || s.phase == bpBanner) && i == s.player {
			vector.FillRect(dst, float32(c.x), float32(c.y+c.h-3), float32(c.w), 3, tokBrand, false)
		}
		drawPortrait(dst, p, c.x+sp3, c.y+sp3, 2)
		name := truncateW(p.Name, c.w-iconSize-3*sp3)
		drawText(dst, name, c.x+sp3+iconSize+sp3, c.y+sp3, colNote)
		if p.Kind.IsBot() {
			drawPill(dst, c.x+sp3+iconSize+sp3, c.y+sp3+uiTextH+2, "BOT", pillOutline)
		} else if p.Tempo() < 1 {
			drawTextSmall(dst, fmt.Sprintf("%d%% speed", int(math.Round(p.Tempo()*100))), c.x+sp3+iconSize+sp3, c.y+sp3+uiTextH+2, colDim)
		}
		tot := fmt.Sprintf("%d", totals[i])
		drawTextScaled(dst, tot, c.x+sp3, c.y+c.h-sp3-sizeDisplayM-10, dispM, colNote)
		for k := 0; k < s.b.Exchanges(); k++ {
			dx := c.x + c.w - sp3 - float64(s.b.Exchanges()-k)*12
			dot := rect{dx, c.y + c.h - sp3 - 14, 8, 8}
			if _, done := s.b.Turn(k, i); done {
				vector.FillRect(dst, float32(dot.x), float32(dot.y), 8, 8, colNote, false)
			} else {
				vector.StrokeRect(dst, float32(dot.x), float32(dot.y), 8, 8, 1, colPanelEdge, false)
			}
		}
	}
	if leader >= 0 && !s.result.Unscored {
		drawPixelIconScaled(dst, icBadgeStar, math.Round(s.crown-iconPx/2), band.y-6, 1, tokBrand) // the leader's crown
	}
	// The crowd meter.
	my := band.y + sp2 + 108 + sp3
	if n == 2 {
		s.drawTugOfWar(dst, x0, my, colW*2)
	} else {
		s.drawSplitBar(dst, x0, my, colW*float64(n))
	}
}

// drawTugOfWar is the two-player crowd meter: 24 blocks, the crowd marker
// leaning toward whoever is ahead.
func (s *BattleScreen) drawTugOfWar(dst *ebiten.Image, x, y, w float64) {
	const blocks = 24
	c := s.b.Crowd2()
	bw := (w - (blocks-1)*2) / blocks
	mark := int(math.Round((1 - (c+1)/2) * (blocks - 1))) // c = 1 leans to player 0 (left)
	for i := 0; i < blocks; i++ {
		col := colBlockOff
		switch {
		case i < blocks/2 && i >= mark:
			col = profileTintOf(s.participant(0))
		case i >= blocks/2 && i <= mark:
			col = profileTintOf(s.participant(1))
		}
		vector.FillRect(dst, float32(x+float64(i)*(bw+2)), float32(y), float32(bw), 10, col, false)
	}
	mx := x + float64(mark)*(bw+2) + bw/2
	drawSpriteOn(dst, "crowd", "member2", "armsup", 0, mx, y+34, 1)
}

func profileTintOf(p battle.Participant) color.RGBA {
	if p.Kind == battle.KindHuman || p.Kind == battle.KindGhost {
		return profileTint(p.Tint)
	}
	return colDim
}

// drawSplitBar is the 3–4 player crowd bar: each player's share.
func (s *BattleScreen) drawSplitBar(dst *ebiten.Image, x, y, w float64) {
	shares := s.b.CrowdShares()
	cx := x
	for i, sh := range shares {
		sw := w * sh
		col := profileTintOf(s.participant(i))
		if s.participant(i).Kind.IsBot() && s.participant(i).Kind != battle.KindGhost {
			col = lerpCol(colDim, colBG, float64(i%2)*0.3)
		}
		vector.FillRect(dst, float32(cx), float32(y), float32(math.Max(0, sw-2)), 10, col, false)
		cx += sw
	}
}

// drawVS is the battle's intro (MOTION §B2.2): the names slide in, VS.
func (s *BattleScreen) drawVS(dst *ebiten.Image) {
	t := s.t
	if reduceMotion {
		t = vsSeconds
	}
	k := curveEnter.at(math.Min(1, t/durTravel))
	n := s.b.Players()
	y := screenH * 0.55
	for i := 0; i < n; i++ {
		p := s.participant(i)
		dir := -1.0
		if i%2 == 1 {
			dir = 1
		}
		w := textWScaled(p.Name, dispL)
		x := screenW/2 + dir*(screenW*0.18) - w/2 + dir*(1-k)*screenW*0.5
		if n > 2 {
			x = screenW*(float64(i)+0.5)/float64(n) - w/2 + dir*(1-k)*200
		}
		drawTextScaled(dst, p.Name, math.Round(x), y+float64(i/2)*48, dispL, profileTintOf(p))
	}
	if t > durTravel {
		vs := "VS"
		drawTextScaled(dst, vs, math.Round((screenW-textWScaled(vs, dispX))/2), y-8, dispX, tokBrand)
	}
	m := s.b.Setup.Material
	sub := fmt.Sprintf("%s · %d exchanges of %d bars", m.Title, s.b.Exchanges(), s.b.Setup.PhraseBars)
	drawText(dst, sub, math.Round((screenW-textW(sub))/2), y+96, colDim)
}

// drawBanner is the turn banner: "EXCHANGE 2 / 4 — PLAYER: Aaditya".
func (s *BattleScreen) drawBanner(dst *ebiten.Image) {
	p := s.participant(s.player)
	c := s.cardRect(620, 170)
	notchedFill(dst, c, colPanel)
	notchedStroke(dst, c, 1, colPanelEdge)
	head := fmt.Sprintf("EXCHANGE %d / %d", s.exchange+1, s.b.Exchanges())
	drawSectionHeader(dst, c.x+sp6, c.y+sp4, 0, head, 0)
	who := "PLAYER: " + p.Name
	if p.Kind.IsBot() {
		who = p.Name + " is up…"
	}
	drawTextScaled(dst, who, c.x+sp6, c.y+sp4+sizeDisplayS+sp3, dispM, colNote)
	foot := ""
	switch {
	case p.Kind.IsBot():
		foot = "BOT"
	case s.lastHuman >= 0 && s.lastHuman != s.player:
		foot = "Pass the guitar to " + p.Name + " · Space when ready"
	default:
		foot = "Space when ready · one bar of count-in"
	}
	drawText(dst, foot, c.x+sp6, c.y+c.h-sp4-uiTextH, colDim)
}

// drawResultCard chains the turn's result (MOTION §B2.5): accuracy, then
// tightness, then the turn score, then the crowd's swing and the total.
func (s *BattleScreen) drawResultCard(dst *ebiten.Image) {
	t := s.t
	if reduceMotion {
		t = 1e6
	}
	p := s.participant(s.turn.Player)
	c := s.cardRect(480, 230)
	y := drawDialogCard(dst, c, p.Name)
	lines := []string{}
	if !s.turn.Scored {
		lines = append(lines, "Unscored: this turn wasn't judged (calibrate your timing)")
	} else {
		lines = append(lines, fmt.Sprintf("Accuracy %d%%", int(math.Round(s.turn.Summary.Accuracy()*100))))
		tt := s.turn.Summary.Tightness()
		tl := "No timed notes"
		if tt.N > 0 {
			tl = tt.Label() + " · " + tt.SpreadLabel()
		}
		lines = append(lines, tl)
		lines = append(lines, fmt.Sprintf("+%d", s.turn.Score))
		tail := fmt.Sprintf("Total %d", s.b.Totals()[s.turn.Player])
		if s.b.Players() == 2 {
			swing := s.b.Crowd2() - s.crowdBefore
			if s.turn.Player == 1 {
				swing = -swing
			}
			if math.Abs(swing) > 0.005 {
				tail += fmt.Sprintf("  ·  the crowd %+.0f", swing*100)
			}
		}
		lines = append(lines, tail)
	}
	for i, l := range lines {
		if t < float64(i)*resultStep {
			break
		}
		if i == 2 {
			drawTextScaled(dst, l, c.x+sp6, y+float64(i)*(uiTextH+sp2), dispM, colNote)
			continue
		}
		drawText(dst, l, c.x+sp6, y+float64(i)*(uiTextH+sp2)+map[bool]float64{true: 10, false: 0}[i > 2], colDim)
	}
	drawTextSmall(dst, "Space continues", c.x+sp6, c.y+c.h-sp4-14, colBarline)
}

// ---- Results ----

type resultsLayout struct {
	rematch, newBattle, done rect
	cells                    [][]rect // [exchange][player]
}

func (s *BattleScreen) resultsLayout() resultsLayout {
	var l resultsLayout
	n := s.b.Players()
	colW := math.Min(220, (screenW-2*pgMargin-140)/float64(n))
	x0 := math.Round((screenW - 140 - colW*float64(n)) / 2)
	y0 := screenH*0.36 + sp6
	for k := 0; k < s.b.Exchanges(); k++ {
		var row []rect
		for i := 0; i < n; i++ {
			row = append(row, rect{x0 + 140 + float64(i)*colW, y0 + float64(k)*30, colW, 28})
		}
		l.cells = append(l.cells, row)
	}
	dw := kitButtonW("Done", kitPrimary, false)
	l.done = rect{screenW - pgMargin - dw, screenH - pgMargin - btnHPrimary, dw, btnHPrimary}
	nw := kitButtonW("New battle", kitSecondary, false)
	l.newBattle = rect{l.done.x - sp3 - nw, l.done.y + (btnHPrimary-btnH)/2, nw, btnH}
	rw := kitButtonW("Rematch", kitSecondary, false)
	l.rematch = rect{l.newBattle.x - sp3 - rw, l.newBattle.y, rw, btnH}
	return l
}

func (s *BattleScreen) updateResults() error {
	l := s.resultsLayout()
	ready := s.t >= s.resultsReadyAt() || reduceMotion
	if !ready {
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) || s.ptr.pressed {
			s.t = s.resultsReadyAt()
		}
		return nil
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape), inpututil.IsKeyJustPressed(ebiten.KeyEnter),
		s.ptr.pressed && s.ptr.over(l.done):
		return s.leave()
	case inpututil.IsKeyJustPressed(ebiten.KeyR), s.ptr.pressed && s.ptr.over(l.rematch):
		if s.rematch != nil {
			s.leave()
			s.rematch()
		}
	case inpututil.IsKeyJustPressed(ebiten.KeyN), s.ptr.pressed && s.ptr.over(l.newBattle):
		if s.newOne != nil {
			s.leave()
			s.newOne()
		}
	}
	return nil
}

// resultsReadyAt: rows, totals, the trophy, the XP, then the buttons
// (MOTION §B2.8).
func (s *BattleScreen) resultsReadyAt() float64 {
	return float64(s.b.Exchanges())*0.12 + 0.3 + 0.4 + durRoll + 0.2
}

func (s *BattleScreen) drawResults(dst *ebiten.Image) {
	t := s.t
	if reduceMotion {
		t = 1e6
	}
	r := s.result
	title := "Results"
	switch {
	case r.Unscored:
		title = "No winner: a turn wasn't scored"
	case r.Winner >= 0:
		title = s.participant(r.Winner).Name + " wins!"
	default:
		title = "A draw!"
	}
	// The trophy drops in after the rows and totals.
	trophyAt := float64(s.b.Exchanges())*0.12 + 0.3
	if r.Winner >= 0 && t >= trophyAt {
		drop := math.Min(1, (t-trophyAt)/0.3)
		ty := screenH*0.08 - (1-curveEnter.at(drop))*40
		drawPixelIconScaled(dst, icBadgeStar, math.Round((screenW-64)/2), math.Round(ty), 4, tokBrand)
	}
	drawTextScaled(dst, title, math.Round((screenW-textWScaled(title, dispL))/2), screenH*0.08+76, dispL, colNote)
	if r.TieBreak != "" {
		drawText(dst, r.TieBreak, math.Round((screenW-textW(r.TieBreak))/2), screenH*0.08+76+sizeDisplayL+sp2, colDim)
	}
	l := s.resultsLayout()
	// The table: rows are exchanges, columns players; a cell is the turn
	// score and its accuracy.
	for i := 0; i < s.b.Players(); i++ {
		c := l.cells[0][i]
		drawText(dst, truncateW(s.participant(i).Name, c.w-sp2), c.x, c.y-uiTextH-sp2, colDim)
	}
	for k, row := range l.cells {
		if t < float64(k)*0.12 {
			break
		}
		drawText(dst, fmt.Sprintf("Exchange %d", k+1), row[0].x-140, row[0].y+4, colBarline)
		for i, c := range row {
			tn, ok := s.b.Turn(k, i)
			txt := "—"
			if ok && tn.Scored {
				txt = fmt.Sprintf("%d · %d%%", tn.Score, int(math.Round(tn.Summary.Accuracy()*100)))
			} else if ok {
				txt = "unscored"
			}
			drawMonoSized(dst, txt, c.x, c.y+4, sizeUI, colNote)
		}
	}
	totY := l.cells[len(l.cells)-1][0].y + 40
	if t >= float64(s.b.Exchanges())*0.12+0.15 {
		drawText(dst, "Total", l.cells[0][0].x-140, totY+4, colNote)
		for i, tot := range r.Totals {
			drawTextScaled(dst, fmt.Sprintf("%d", tot), l.cells[0][i].x, totY, dispM, colNote)
		}
	}
	// XP for each human player (§8.1).
	xpAt := trophyAt + 0.4
	if t >= xpAt {
		y := totY + sizeDisplayM + sp6
		for i := range r.Totals {
			p := s.participant(i)
			if p.Kind != battle.KindHuman || i >= len(s.bonuses) || s.bonuses[i] == 0 {
				continue
			}
			drawText(dst, fmt.Sprintf("+%d XP", rolled(s.bonuses[i], t, xpAt)), l.cells[0][i].x, y, colXPFill)
		}
	}
	if t >= s.resultsReadyAt() || reduceMotion {
		drawKitButton(dst, l.rematch, "Rematch", -1, kitSecondary, btnState{hover: s.ptr.over(l.rematch)})
		drawKitButton(dst, l.newBattle, "New battle", -1, kitSecondary, btnState{hover: s.ptr.over(l.newBattle)})
		drawKitButton(dst, l.done, "Done", -1, kitPrimary, btnState{hover: s.ptr.over(l.done)})
	}
}

// fileBattle saves the battle (§8): filled in by the records stage.
func (s *BattleScreen) fileBattle() {}

// ShotState puts the battle in a moment for the dev screenshot tool
// (tools/uishot): "vs", "banner", "result" or "results".
func (s *BattleScreen) ShotState(kind string) {
	play := func(n int) {
		for i := 0; i < n; i++ {
			k, p, ok := s.b.Next()
			if !ok {
				return
			}
			s.exchange, s.player = k, p
			sum, js := s.botPlay(s.participant(p), s.b.Plan.Phrases[k])
			s.turn = battle.NewTurn(p, k, sum, 1, true)
			s.turn.Judgements = js
			s.crowdBefore = s.b.Crowd2()
			s.b.Record(s.turn)
		}
	}
	switch kind {
	case "banner":
		play(1)
		s.nextTurn()
	case "result":
		play(3)
		s.setPhase(bpResult)
		s.t = 10
	case "results":
		play(100)
		s.finish()
		s.t = 10
	}
}
