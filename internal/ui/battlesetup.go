package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/battle"
	"github.com/S95F/musicTutor/internal/progress"
	"github.com/S95F/musicTutor/internal/score"
)

// The battle setup (BATTLES.md §6.2): the material, the players, the
// rules, and Start battle — disabled, with the reason, when it can't go.

// BattleDeps is what the battle screens need from the app.
type BattleDeps struct {
	Songs      func() []PieceInfo                         // "Your songs"
	Riffs      func() []PieceInfo                         // the battle-riff pack, easiest first
	LoadScore  func(path string) (*score.Score, error)    // a piece, for planning
	Calibrated func() bool                                // the rig's timing is calibrated
	Open       func(path string, track int) (*App, error) // the practice view the battle plays in
	Close      func()
	Settings   func() // to calibrate
}

type slotKind int

const (
	slotNone slotKind = iota
	slotProfile
	slotRival
	slotGhost
	slotSession
	slotKinds
)

var slotKindNames = [slotKinds]string{"Nobody", "Profile", "Rival", "Ghost", "Session Player"}

type battleSlot struct {
	kind   slotKind
	choice int     // which profile / rival / ghost profile / session level
	tempo  float64 // a profile's handicap tempo
}

// BattleSetup is the setup screen.
type BattleSetup struct {
	sh   *Shell
	st   *progress.Store
	deps BattleDeps
	ptr  pointer
	anim animator

	riffs   bool // the material tab: your songs or battle riffs
	song    int
	pieces  []PieceInfo
	sc      *score.Score
	scErr   string
	scPath  string
	track   int // index into playable tracks
	section int // -1: the whole song

	slots      [battle.MaxParticipants]battleSlot
	phraseBars int
	exchanges  int

	focus int
	start func(battle.Setup, []GhostEntry)

	records    bool // the Records tab is open
	recProfile int
	recScroll  int
	titleX     float64
}

// GhostEntry is a ghost participant's stored notes, per phrase.
type GhostEntry struct {
	player int
	notes  []battle.GhostNote
	index  map[[2]int64]int
}

// NewBattleSetup opens the setup with you against Basement Benny.
func NewBattleSetup(sh *Shell, st *progress.Store, deps BattleDeps) *BattleSetup {
	b := &BattleSetup{sh: sh, st: st, deps: deps, section: -1, phraseBars: battle.DefaultPhraseBars, exchanges: battle.DefaultExchanges}
	b.slots[0] = battleSlot{kind: slotProfile, tempo: 1}
	b.slots[1] = battleSlot{kind: slotRival}
	b.loadPieces()
	return b
}

// SetStart wires Start battle.
func (b *BattleSetup) SetStart(fn func(battle.Setup, []GhostEntry)) { b.start = fn }

func (b *BattleSetup) loadPieces() {
	b.pieces = nil
	if b.riffs && b.deps.Riffs != nil {
		b.pieces = b.deps.Riffs()
	} else if !b.riffs && b.deps.Songs != nil {
		b.pieces = b.deps.Songs()
	}
	b.song = max(0, min(len(b.pieces)-1, b.song))
	b.loadScore()
}

func (b *BattleSetup) loadScore() {
	b.sc, b.scErr, b.scPath = nil, "", ""
	if b.song >= len(b.pieces) || b.deps.LoadScore == nil {
		return
	}
	p := b.pieces[b.song].Path
	sc, err := b.deps.LoadScore(p)
	if err != nil {
		b.scErr = err.Error()
		return
	}
	b.sc, b.scPath = sc, p
	b.track = max(0, min(len(b.playable())-1, b.track))
	b.section = max(-1, min(len(sc.Sections)-1, b.section))
}

// playable are the tracks a player can battle on (not drums).
func (b *BattleSetup) playable() []int {
	if b.sc == nil {
		return nil
	}
	var out []int
	for i, tr := range b.sc.Tracks {
		if !tr.Drums {
			out = append(out, i)
		}
	}
	return out
}

func (b *BattleSetup) trackIndex() int {
	if p := b.playable(); b.track < len(p) {
		return p[b.track]
	}
	return 0
}

// barRange is the section's bars (indexes; end exclusive).
func (b *BattleSetup) barRange() (int, int) {
	tr := b.sc.Tracks[b.trackIndex()]
	if b.section < 0 || b.section >= len(b.sc.Sections) {
		return 0, len(tr.Bars)
	}
	at := func(tick int64) int {
		for i, bar := range tr.Bars {
			if tick < bar.Start+bar.Len() {
				return i
			}
		}
		return len(tr.Bars)
	}
	from := at(b.sc.Sections[b.section].Tick)
	to := len(tr.Bars)
	if b.section+1 < len(b.sc.Sections) {
		to = at(b.sc.Sections[b.section+1].Tick)
	}
	return from, to
}

func (b *BattleSetup) profiles() []progress.ProfileInfo {
	if b.st == nil {
		return nil
	}
	return b.st.Profiles()
}

// participant builds slot i's participant; ok is false for an empty slot.
func (b *BattleSetup) participant(i int) (battle.Participant, bool) {
	s := b.slots[i]
	ps := b.profiles()
	switch s.kind {
	case slotProfile:
		if s.choice < len(ps) {
			p := ps[s.choice]
			return battle.Participant{Name: p.Name, Kind: battle.KindHuman, ID: p.ID, TempoScale: s.tempo, Tint: p.Tint}, true
		}
	case slotRival:
		r := battle.Rivals[min(s.choice, len(battle.Rivals)-1)]
		return battle.Participant{Name: r.Name, Kind: battle.KindRival, ID: r.Name}, true
	case slotGhost:
		if s.choice < len(ps) {
			p := ps[s.choice]
			return battle.Participant{Name: p.Name + "'s ghost", Kind: battle.KindGhost, ID: p.ID, Tint: p.Tint}, true
		}
	case slotSession:
		l := battle.SessionLevels[min(s.choice, len(battle.SessionLevels)-1)]
		return battle.Participant{Name: "Session Player (" + l.Name + ")", Kind: battle.KindSession, ID: l.Name}, true
	}
	return battle.Participant{}, false
}

// setup is the battle as chosen, with its plan.
func (b *BattleSetup) setup() (battle.Setup, battle.Plan, error) {
	var s battle.Setup
	if b.sc == nil {
		return s, battle.Plan{}, fmt.Errorf("pick a song to battle on")
	}
	from, to := b.barRange()
	title := ""
	if b.song < len(b.pieces) {
		title = cleanPieceTitle(b.pieces[b.song].Title, b.pieces[b.song].Path)
	}
	s.Material = battle.Material{Score: b.sc, Track: b.trackIndex(), Title: title, SongID: progress.SongID(b.sc),
		Path: b.scPath, StartBar: from, EndBar: to}
	s.PhraseBars, s.Exchanges = b.phraseBars, b.exchanges
	for i := range b.slots {
		if p, ok := b.participant(i); ok {
			s.Participants = append(s.Participants, p)
		}
	}
	plan, err := battle.NewPlan(s)
	return s, plan, err
}

// storedGhostFor finds a profile's stored pass covering every phrase.
func storedGhostFor(st *progress.Store, profileID string, s battle.Setup, plan battle.Plan) ([]battle.GhostNote, map[[2]int64]int, bool) {
	if st == nil {
		return nil, nil, false
	}
	m := s.Material
	index := map[[2]int64]int{}
	for k, v := range progress.NoteIndex(m.Score, m.Track) {
		index[[2]int64{k.Start, int64(k.String)}] = v
	}
	for _, pass := range st.StoredPasses(profileID, m.SongID, m.Track) {
		var notes []battle.GhostNote
		for _, n := range pass.Notes {
			notes = append(notes, battle.GhostNote{Index: n.Index, Verdict: n.Verdict, OffsetMs: n.OffsetMs, Timed: n.Timed})
		}
		ok := true
		for _, ph := range plan.Phrases {
			if _, _, covers := battle.GhostTurn(notes, index, m.Score, m.Track, ph); !covers {
				ok = false
				break
			}
		}
		if ok {
			return notes, index, true
		}
	}
	return nil, nil, false
}

// blocked is why Start battle can't go, in plain words ("" = it can).
func (b *BattleSetup) blocked() (string, bool) {
	s, plan, err := b.setup()
	if err != nil {
		return sentenceCase(err.Error()), false
	}
	level := playerLevel(b.st)
	seen := map[string]bool{}
	human := false
	for _, p := range s.Participants {
		switch p.Kind {
		case battle.KindHuman:
			human = true
			if seen[p.ID] {
				return p.Name + " is in two slots: pick a different profile", false
			}
			seen[p.ID] = true
		case battle.KindRival:
			if r, _ := battle.RivalNamed(p.ID); !r.Unlocked(level) {
				return fmt.Sprintf("%s unlocks at level %d", r.Name, r.Unlock), false
			}
		case battle.KindGhost:
			if _, _, ok := storedGhostFor(b.st, p.ID, s, plan); !ok {
				return "Play this section once to create a ghost", false
			}
		}
	}
	if human && b.deps.Calibrated != nil && !b.deps.Calibrated() {
		return "Your timing isn't calibrated, so no turn could be scored", true
	}
	return "", false
}

// rows, top to bottom.
const (
	bsRowTab = iota
	bsRowSong
	bsRowTrack
	bsRowSection
	bsRowSlot0
	bsRowPhrase = bsRowSlot0 + battle.MaxParticipants
	bsRowExchanges
	bsRowCount
)

type bsLayout struct {
	rows        []rect
	left, right []rect // the ‹ › of each row
	left2       []rect // a slot's tempo ‹ ›
	right2      []rect
	start       rect
	calibrate   rect
	tabs        [2]rect
}

func (b *BattleSetup) layout() bsLayout {
	var l bsLayout
	w := math.Min(screenW-2*pgMargin, 1000)
	x := math.Round((screenW - w) / 2)
	y := float64(zHeaderH) + sp2
	rowH := math.Min(44, (screenH-zHeaderH-pgMargin-btnHPrimary-sp8-uiTextH*3)/bsRowCount)
	for i := 0; i < bsRowCount; i++ {
		r := rect{x, y + float64(i)*rowH, w, rowH}
		l.rows = append(l.rows, r)
		vx := x + 180
		l.left = append(l.left, rect{vx, r.y + (r.h-28)/2, 28, 28})
		l.right = append(l.right, rect{vx + 360, r.y + (r.h-28)/2, 28, 28})
		l.left2 = append(l.left2, rect{vx + 420, r.y + (r.h-28)/2, 28, 28})
		l.right2 = append(l.right2, rect{vx + 540, r.y + (r.h-28)/2, 28, 28})
	}
	tw := kitMenuItemW(-1, "Your songs")
	l.tabs[0] = rect{x + 180, l.rows[0].y, tw, rowH}
	l.tabs[1] = rect{l.tabs[0].x + tw, l.rows[0].y, kitMenuItemW(-1, "Battle riffs"), rowH}
	sw := kitButtonW("Start battle", kitPrimary, false)
	l.start = rect{x + w - sw, screenH - pgMargin - btnHPrimary, sw, btnHPrimary}
	cw := kitButtonW("Calibrate", kitSecondary, false)
	l.calibrate = rect{l.start.x - sp3 - cw, l.start.y + (btnHPrimary-btnH)/2, cw, btnH}
	return l
}

func (b *BattleSetup) Update() error {
	b.ptr = readPointer()
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape), escButtonPressed(escBackChip, b.ptr):
		return errQuit
	case inpututil.IsKeyJustPressed(ebiten.KeyR):
		b.records = !b.records
		return nil
	}
	if b.ptr.pressed {
		for i, r := range b.titleTabs() {
			if b.ptr.over(r) {
				b.records = i == 1
				return nil
			}
		}
	}
	if b.records {
		b.updateRecords()
		return nil
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyUp):
		b.focus = max(0, b.focus-1)
	case inpututil.IsKeyJustPressed(ebiten.KeyDown), inpututil.IsKeyJustPressed(ebiten.KeyTab):
		b.focus = min(bsRowCount-1, b.focus+1)
	case inpututil.IsKeyJustPressed(ebiten.KeyLeft):
		b.adjust(b.focus, -1, ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight))
	case inpututil.IsKeyJustPressed(ebiten.KeyRight):
		b.adjust(b.focus, 1, ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight))
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter):
		b.tryStart()
	}
	if !b.ptr.pressed {
		return nil
	}
	l := b.layout()
	for i := range l.rows {
		switch {
		case b.ptr.over(l.left[i]):
			b.focus = i
			b.adjust(i, -1, false)
		case b.ptr.over(l.right[i]):
			b.focus = i
			b.adjust(i, 1, false)
		case b.ptr.over(l.left2[i]):
			b.focus = i
			b.adjust(i, -1, true)
		case b.ptr.over(l.right2[i]):
			b.focus = i
			b.adjust(i, 1, true)
		case b.ptr.over(l.rows[i]):
			b.focus = i
		}
	}
	for i, r := range l.tabs {
		if b.ptr.over(r) && b.riffs != (i == 1) {
			b.riffs, b.song = i == 1, 0
			b.loadPieces()
		}
	}
	switch {
	case b.ptr.over(l.start):
		b.tryStart()
	case b.ptr.over(l.calibrate) && b.deps.Settings != nil:
		b.deps.Settings()
	}
	return nil
}

func (b *BattleSetup) tryStart() {
	if why, _ := b.blocked(); why != "" || b.start == nil {
		return
	}
	s, plan, _ := b.setup()
	var ghosts []GhostEntry
	for i, p := range s.Participants {
		if p.Kind == battle.KindGhost {
			notes, index, _ := storedGhostFor(b.st, p.ID, s, plan)
			ghosts = append(ghosts, GhostEntry{player: i, notes: notes, index: index})
		}
	}
	b.start(s, ghosts)
}

// adjust steps a row's value; second is a slot's tempo (or shift).
func (b *BattleSetup) adjust(row, d int, second bool) {
	wrap := func(v, n int) int {
		if n <= 0 {
			return 0
		}
		return ((v % n) + n) % n
	}
	switch {
	case row == bsRowTab:
		b.riffs, b.song = !b.riffs, 0
		b.loadPieces()
	case row == bsRowSong:
		b.song = wrap(b.song+d, len(b.pieces))
		b.section = -1
		b.loadScore()
	case row == bsRowTrack:
		b.track = wrap(b.track+d, len(b.playable()))
	case row == bsRowSection && b.sc != nil:
		b.section = wrap(b.section+1+d, len(b.sc.Sections)+1) - 1
	case row >= bsRowSlot0 && row < bsRowSlot0+battle.MaxParticipants:
		i := row - bsRowSlot0
		s := &b.slots[i]
		if second {
			if s.kind == slotProfile {
				s.tempo = math.Round((s.tempo+float64(d)*battle.HumanTempoStep)*100) / 100
				s.tempo = math.Max(battle.HumanTempoMin, math.Min(battle.HumanTempoMax, s.tempo))
			} else {
				s.choice = wrap(s.choice+d, b.choices(s.kind))
			}
			return
		}
		// The first two slots always play; the rest can be empty.
		first := slotNone
		if i < battle.MinParticipants {
			first = slotProfile
		}
		k := int(s.kind) + d
		if k >= int(slotKinds) {
			k = int(first)
		}
		if k < int(first) {
			k = int(slotKinds) - 1
		}
		*s = battleSlot{kind: slotKind(k), tempo: 1}
	case row == bsRowPhrase:
		b.phraseBars = max(battle.MinPhraseBars, min(battle.MaxPhraseBars, b.phraseBars+d))
	case row == bsRowExchanges:
		b.exchanges = max(battle.MinExchanges, min(battle.MaxExchanges, b.exchanges+d))
	}
}

func (b *BattleSetup) choices(k slotKind) int {
	switch k {
	case slotProfile, slotGhost:
		return len(b.profiles())
	case slotRival:
		return len(battle.Rivals)
	case slotSession:
		return len(battle.SessionLevels)
	}
	return 0
}

// slotText is a slot's value and its second value (tempo or choice).
func (b *BattleSetup) slotText(i int) (string, string) {
	s := b.slots[i]
	p, ok := b.participant(i)
	if !ok {
		return slotKindNames[s.kind], ""
	}
	switch s.kind {
	case slotProfile:
		return "Profile: " + p.Name, fmt.Sprintf("%d%%|counts as ×%.2f", int(math.Round(s.tempo*100)), battle.HandicapFactor(s.tempo))
	case slotRival:
		r := battle.Rivals[min(s.choice, len(battle.Rivals)-1)]
		if !r.Unlocked(playerLevel(b.st)) {
			return "Rival: ???", fmt.Sprintf("%d of %d|Unlocks at level %d", s.choice+1, len(battle.Rivals), r.Unlock)
		}
		return "Rival: " + r.Name, fmt.Sprintf("%d of %d|%s", s.choice+1, len(battle.Rivals), r.Bio)
	case slotGhost:
		return "Ghost of " + strings.TrimSuffix(p.Name, "'s ghost"), "whose|their best pass"
	case slotSession:
		return "Session Player", battle.SessionLevels[min(s.choice, len(battle.SessionLevels)-1)].Name + "|how good"
	}
	return "", ""
}

func (b *BattleSetup) Draw(dst *ebiten.Image) {
	dst.Fill(colBG)
	b.anim.tick()
	l := b.layout()
	b.titleX = drawEscButton(dst, escBackChip, &b.anim, b.ptr)
	b.drawTitleTabs(dst)
	if b.records {
		b.drawRecords(dst)
		return
	}

	labels := []string{"Material", "Song", "Track", "Section"}
	for i := 0; i < battle.MaxParticipants; i++ {
		labels = append(labels, fmt.Sprintf("Player %d", i+1))
	}
	labels = append(labels, "Phrase bars", "Exchanges")
	for i, r := range l.rows {
		if i == b.focus {
			vector.FillRect(dst, float32(r.x), float32(r.y), float32(r.w), float32(r.h), colHover, false)
			vector.FillRect(dst, float32(r.x), float32(r.y), 3, float32(r.h), tokBrand, false)
		}
		drawText(dst, labels[i], r.x+sp3, r.y+(r.h-uiTextH)/2, colDim)
		value, second := b.rowText(i)
		if i == bsRowTab {
			for t, tr := range l.tabs {
				label := [2]string{"Your songs", "Battle riffs"}[t]
				on := b.riffs == (t == 1)
				drawText(dst, label, centreX(label, tr.x, tr.w), tr.y+(tr.h-uiTextH)/2-2, menuInk(on, false, animValues{}))
				if on {
					vector.FillRect(dst, float32(tr.x+(tr.w-textW(label))/2), underlineY(tr), float32(textW(label)), menuUnderline, colAccent, false)
				}
			}
			continue
		}
		b.drawArrows(dst, l.left[i], l.right[i], value)
		if second != "" {
			if i >= bsRowSlot0 && i < bsRowPhrase && b.slots[i-bsRowSlot0].kind != slotNone {
				val, note, _ := strings.Cut(second, "|")
				b.drawArrows(dst, l.left2[i], l.right2[i], val)
				drawTextSmall(dst, note, l.right2[i].x+l.right2[i].w+sp3, l.right2[i].y+6, colDim)
			}
		}
	}
	// The plan's warnings and why Start is off.
	why, needsCal := b.blocked()
	y := l.rows[len(l.rows)-1].y + l.rows[len(l.rows)-1].h + sp2
	if _, plan, err := b.setup(); err == nil {
		for _, w := range plan.Warnings {
			drawTextSmall(dst, w, l.rows[0].x+sp3, y, colClose)
			y += 16
		}
	}
	if why != "" {
		drawText(dst, why, l.rows[0].x+sp3, l.start.y+(l.start.h-uiTextH)/2, colClose)
	}
	if needsCal {
		drawKitButton(dst, l.calibrate, "Calibrate", -1, kitSecondary, btnState{hover: b.ptr.over(l.calibrate)})
	}
	drawKitButton(dst, l.start, "Start battle", -1, kitPrimary, btnState{hover: b.ptr.over(l.start), disabled: why != ""})
}

func (b *BattleSetup) rowText(i int) (string, string) {
	switch {
	case i == bsRowSong:
		if len(b.pieces) == 0 {
			if b.riffs {
				return "No battle riffs yet", ""
			}
			return "No songs yet: open one in Practice", ""
		}
		p := b.pieces[b.song]
		return cleanPieceTitle(p.Title, p.Path), p.Difficulty
	case i == bsRowTrack:
		if b.sc == nil {
			return b.scErr, ""
		}
		if n := b.sc.Tracks[b.trackIndex()].Name; n != "" {
			return n, ""
		}
		return fmt.Sprintf("Track %d", b.trackIndex()+1), ""
	case i == bsRowSection:
		if b.sc == nil {
			return "", ""
		}
		from, to := b.barRange()
		name := "Whole song"
		if b.section >= 0 {
			name = b.sc.Sections[b.section].Name
		}
		return name, fmt.Sprintf("bars %d–%d", b.sc.BarNumber(from), b.sc.BarNumber(max(from, to-1)))
	case i >= bsRowSlot0 && i < bsRowPhrase:
		return b.slotText(i - bsRowSlot0)
	case i == bsRowPhrase:
		return fmt.Sprintf("%d", b.phraseBars), ""
	case i == bsRowExchanges:
		return fmt.Sprintf("%d", b.exchanges), ""
	}
	return "", ""
}

// drawArrows draws a row's ‹ value › with pixel chevrons.
func (b *BattleSetup) drawArrows(dst *ebiten.Image, left, right rect, value string) {
	for _, it := range []struct {
		r  rect
		ic iconID
	}{{left, icChevronLeft}, {right, icChevronRight}} {
		col := colDim
		if b.ptr.over(it.r) {
			col = colNote
			notchedFill(dst, it.r, colHover)
		}
		drawPixelIconScaled(dst, it.ic, it.r.x+(it.r.w-iconPx)/2, it.r.y+(it.r.h-iconPx)/2, 1, col)
	}
	if value != "" {
		v := truncateW(value, right.x-left.x-left.w-2*sp2)
		drawText(dst, v, left.x+left.w+sp2, left.y+(left.h-uiTextH)/2, colNote)
	}
}

// SetupForShot is the dev screenshot tool's battle: Basement Benny against
// Rusher Rex on the setup's song, two exchanges of two bars.
func (b *BattleSetup) SetupForShot() (battle.Setup, bool) {
	b.slots[0] = battleSlot{kind: slotRival}
	b.slots[1] = battleSlot{kind: slotRival, choice: 1}
	b.phraseBars, b.exchanges = 2, 2
	s, _, err := b.setup()
	return s, err == nil
}
