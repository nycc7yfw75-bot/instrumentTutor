package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/career"
	"github.com/S95F/musicTutor/internal/progress"
)

// The Perform screen (CAREER.md §4.3): the tour as a pixel road, the tier
// in view opened into its gig cards, and the setlist picker.

// PerformScreen is the tour map.
type PerformScreen struct {
	sh   *Shell
	st   *progress.Store
	ptr  pointer
	anim animator
	tip  tips

	tier  int // the tier in view, 1..5
	gig   int // the selected card, 0..4
	slide struct {
		from, t float64 // the road's slide to a clicked tier (MOTION: move)
	}
	roadX float64
	clock float64

	picker   *setlistPicker
	helpOpen bool

	// play starts a gig with a setlist (Stage 4); nil until wired.
	play func(g career.Gig, crowd string, reveal bool, setlist []progress.MasteredSong)
}

// NewPerformScreen opens the tour at the player's current tier.
func NewPerformScreen(sh *Shell, st *progress.Store) *PerformScreen {
	p := &PerformScreen{sh: sh, st: st, tier: 1}
	if st != nil {
		c := st.Career()
		p.tier = c.CurrentTier(playerLevel(st), careerMastered(st)).Num
		if g, ok := nextGig(c, career.Tiers[p.tier-1]); ok {
			p.gig = g.Slot
		}
	}
	p.slide.t = durMove
	p.roadX = p.roadTarget()
	return p
}

// SetPlay wires the Play gig button.
func (p *PerformScreen) SetPlay(fn func(g career.Gig, crowd string, reveal bool, setlist []progress.MasteredSong)) {
	p.play = fn
}

func (p *PerformScreen) career() career.State {
	if p.st == nil {
		return career.State{}
	}
	return p.st.Career()
}

func (p *PerformScreen) unlocked(tier int) bool {
	t, ok := career.TierByNum(tier)
	if !ok {
		return false
	}
	c := p.career()
	return c.Unlocked(t, playerLevel(p.st), careerMastered(p.st))
}

type performLayout struct {
	road  rect
	stops []rect // one per tier, on the road
	cards []rect // the tier's gig cards
}

const performStopW = 150.0

func (p *PerformScreen) roadTarget() float64 {
	// The tier in view sits a third of the way in.
	return -(float64(p.tier-1) * (performStopW + sp8)) + screenW/3 - performStopW/2
}

func (p *PerformScreen) layout() performLayout {
	var l performLayout
	top := float64(zHeaderH) + sp6
	l.road = rect{0, top, screenW, 88}
	for i := range career.Tiers {
		x := p.roadX + float64(i)*(performStopW+sp8)
		l.stops = append(l.stops, rect{x, top + 16, performStopW, 56})
	}
	cy := top + l.road.h + sp8
	n := career.GigsPerTier + 1
	w := (screenW - 2*pgMargin - float64(n-1)*sp3) / float64(n)
	h := math.Min(240, screenH-pgMargin-cy)
	for i := 0; i < n; i++ {
		l.cards = append(l.cards, rect{pgMargin + float64(i)*(w+sp3), cy, w, h})
	}
	return l
}

func (p *PerformScreen) setTier(t int, animate bool) {
	t = max(1, min(len(career.Tiers), t))
	if t == p.tier {
		return
	}
	p.tier = t
	if animate {
		p.slide.from, p.slide.t = p.roadX, 0
	} else {
		p.roadX, p.slide.t = p.roadTarget(), durMove
	}
}

func (p *PerformScreen) Update() error {
	p.ptr = readPointer()
	p.clock += uiFrameSeconds()
	if p.helpOpen {
		if helpDismissed(p.ptr) {
			p.helpOpen = false
		}
		return nil
	}
	if p.picker != nil {
		p.picker.update(p)
		return nil
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape), escButtonPressed(escBackChip, p.ptr):
		return errQuit
	case helpKeyPressed():
		p.helpOpen = true
	case inpututil.IsKeyJustPressed(ebiten.KeyPageUp), inpututil.IsKeyJustPressed(ebiten.KeyUp):
		p.setTier(p.tier-1, false)
	case inpututil.IsKeyJustPressed(ebiten.KeyPageDown), inpututil.IsKeyJustPressed(ebiten.KeyDown):
		p.setTier(p.tier+1, false)
	case inpututil.IsKeyJustPressed(ebiten.KeyLeft):
		p.gig = max(0, p.gig-1)
	case inpututil.IsKeyJustPressed(ebiten.KeyRight):
		p.gig = min(career.GigsPerTier, p.gig+1)
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter), inpututil.IsKeyJustPressed(ebiten.KeyKPEnter):
		p.openPicker()
	}
	if !p.ptr.pressed {
		return nil
	}
	l := p.layout()
	for i, r := range l.stops {
		if p.ptr.over(r) {
			p.setTier(i+1, true)
			return nil
		}
	}
	for i, r := range l.cards {
		if p.ptr.over(r) {
			if p.gig == i {
				p.openPicker()
			}
			p.gig = i
			return nil
		}
	}
	return nil
}

// openPicker opens the setlist picker for the selected gig, if its tier is
// booked.
func (p *PerformScreen) openPicker() {
	if p.st == nil || !p.unlocked(p.tier) {
		return
	}
	c := p.career()
	g := c.Route()[p.tier-1][p.gig]
	p.picker = newSetlistPicker(p.st, g)
}

func (p *PerformScreen) performBindings() []helpBinding {
	return []helpBinding{
		{Group: "tour", Keys: "up / down", Desc: "The previous / next stop on the tour (or click a stop on the road)"},
		{Group: "tour", Keys: "left / right", Desc: "Choose a gig"},
		{Group: "tour", Keys: "enter", Desc: "Pick the setlist for the chosen gig (or click its card again)"},
		{Group: "setlist", Keys: "up / down", Desc: "Move through your mastered songs"},
		{Group: "setlist", Keys: "space", Desc: "Add the song to the setlist, or take it out"},
		{Group: "setlist", Keys: "enter", Desc: "Play the gig once the setlist is full"},
		{Group: "app", Keys: "? or F1", Desc: "This key-binding list"},
		{Group: "app", Keys: "esc", Desc: "Back to the landing page (from the setlist: back to the tour)"},
	}
}

func (p *PerformScreen) Draw(dst *ebiten.Image) {
	dst.Fill(colBG)
	p.anim.tick()
	dt := motionDt()
	// The road slides to a clicked stop; a key moves it at once.
	target := p.roadTarget()
	if p.slide.t < durMove {
		p.slide.t += dt
		k := curveMove.at(math.Min(1, p.slide.t/durMove))
		p.roadX = p.slide.from + (target-p.slide.from)*k
	} else {
		p.roadX = target
	}
	l := p.layout()
	p.drawHeader(dst)
	p.drawRoad(dst, l, dt)
	p.drawCards(dst, l, dt)
	p.tip.draw(dst)
	if p.picker != nil {
		p.picker.draw(dst, p)
	}
	if p.helpOpen {
		drawHelpOverlay(dst, "PERFORM KEYS", p.performBindings(), "")
	}
}

func (p *PerformScreen) drawHeader(dst *ebiten.Image) {
	x := drawEscButton(dst, escBackChip, &p.anim, p.ptr)
	c := p.career()
	t := career.Tiers[p.tier-1]
	city := c.Hometown
	if route := c.Route(); len(route) >= p.tier {
		city = route[p.tier-1][p.gig].City.Name
	}
	drawTextScaled(dst, city, x, 6, dispL, colNote)
	prog := fmt.Sprintf("%s  ·  %d / %d gigs at 3 stars", t.Name, c.TierProgress(t.Num), career.ClearGigs)
	if c.TierCleared(t.Num) {
		prog = t.Name + "  ·  cleared"
	}
	drawText(dst, prog, x, 42, colDim)
	fame := fmt.Sprintf("%d", c.Fame)
	fx := screenW - pgMargin - textWScaled(fame, dispM)
	drawTextScaled(dst, fame, fx, 14, dispM, colNote)
	drawPixelIconScaled(dst, icBadgeStar, fx-iconPx-sp1, 18, 1, tokBrand)
	drawText(dst, "Fame", fx-iconPx-sp2-textW("Fame"), 18, colDim)
}

// drawRoad draws the tour: a dashed pixel road with a stop per tier.
func (p *PerformScreen) drawRoad(dst *ebiten.Image, l performLayout, dt float64) {
	ry := l.road.y + l.road.h/2
	vector.FillRect(dst, 0, float32(ry), float32(screenW), 4, colPanelEdge, false)
	for x := math.Mod(p.roadX, 24) - 24; x < screenW; x += 24 {
		vector.FillRect(dst, float32(math.Round(x)), float32(ry+1), 10, 2, colBarline, false)
	}
	c := p.career()
	for i, r := range l.stops {
		t := career.Tiers[i]
		on := t.Num == p.tier
		open := p.unlocked(t.Num)
		fill := colPanel
		switch {
		case on:
			fill = colHover
		case p.ptr.over(r):
			fill = lerpCol(colPanel, colHover, 0.6)
		}
		notchedFill(dst, r, fill)
		if on {
			notchedStroke(dst, r, 1, tokBrand)
		}
		col := colNote
		if !open {
			col = colBarline
		}
		name := t.Name
		drawText(dst, name, math.Round(r.x+(r.w-textW(name))/2), r.y+sp2, col)
		status := fmt.Sprintf("%d / %d", c.TierProgress(t.Num), career.ClearGigs)
		switch {
		case c.TierCleared(t.Num):
			status = "cleared"
		case !open:
			status = "locked"
		}
		drawTextSmall(dst, status, math.Round(r.x+(r.w-textWSmall(status))/2), r.y+sp2+uiTextH+2, colDim)
		p.tip.offer("stop:"+name, name, "", r, p.ptr.over(r), dt)
	}
}

// drawCards draws the tier's gig cards, or the tier's requirements when it
// is not booked yet.
func (p *PerformScreen) drawCards(dst *ebiten.Image, l performLayout, dt float64) {
	c := p.career()
	route := c.Route()
	if len(route) < p.tier {
		return
	}
	t := career.Tiers[p.tier-1]
	open := p.unlocked(t.Num)
	for i, g := range route[p.tier-1] {
		r := l.cards[i]
		sel := i == p.gig
		over := p.ptr.over(r)
		fill := colPanel
		if sel || over {
			fill = colHover
		}
		notchedFill(dst, r, fill)
		if sel {
			vector.FillRect(dst, float32(r.x), float32(r.y), float32(r.w), 3, tokBrand, false)
		}
		rec := c.Record(g.ID)
		x, y, w := r.x+sp3, r.y+sp3+2, r.w-2*sp3
		kind := fmt.Sprintf("Gig %d", g.Slot+1)
		if g.Recording {
			kind = "Recording session"
		}
		drawSectionHeader(dst, x, y, 0, strings.ToUpper(kind), 0)
		y += sizeDisplayS + sp2
		for j, line := range wrapTextW(g.Venue, w) {
			if j == 2 {
				break
			}
			drawText(dst, line, x, y, colNote)
			y += uiTextH
		}
		drawTextSmall(dst, g.City.Name, x, y+2, colDim)
		y += uiTextH + sp2
		if g.Recording {
			drawTextSmall(dst, "No crowd · judged on accuracy", x, y, colDim)
			drawTextSmall(dst, "and tightness", x, y+15, colDim)
		} else {
			songs := fmt.Sprintf("%d %s", t.Songs(), plural(t.Songs(), "song", "songs"))
			if t.Encore {
				songs = fmt.Sprintf("%d songs + encore", t.SetSize)
			}
			drawTextSmall(dst, songs, x, y, colDim)
		}
		// Best stars at the bottom, and the tough-crowd flames.
		sy := r.y + r.h - sp3 - 16
		for s := 0; s < 5; s++ {
			anim := "unlit"
			if s < rec.BestStars {
				anim = "lit"
			}
			drawSpriteOn(dst, "fx", "star", anim, 0, x+float64(s)*18+8, sy+8, 1)
		}
		if rec.Crowd == career.CrowdTough {
			// Flames: a 3-frame loop, phase-offset per card.
			f := int(p.clock*8+float64(i)) % 3
			if reduceMotion {
				f = 0
			}
			drawSpriteOn(dst, "fx", "flames", "burn", f, r.x+r.w-sp3-8, sy+16, 1)
			drawTextSmall(dst, "TOUGH CROWD", r.x+r.w-sp3-16-sp1-textWSmall("TOUGH CROWD"), sy+1, colMiss)
		}
		if !open {
			vector.FillRect(dst, float32(r.x), float32(r.y), float32(r.w), float32(r.h), withAlpha(colBG, 0.55), false)
		}
		tip := g.Card()
		if !open {
			tip = "Locked: " + t.Name
		}
		p.tip.offer("card:"+g.ID, tip, "Enter", r, over, dt)
	}
	// What a locked tier needs, with ticks for the ones met.
	if !open {
		y := l.cards[0].y + l.cards[0].h + sp6
		drawSectionHeader(dst, pgMargin, y, 0, "TO BOOK "+strings.ToUpper(t.Name), 0)
		y += sizeDisplayS + sp2
		for _, req := range c.Requirements(t, playerLevel(p.st), careerMastered(p.st)) {
			mark, col := "–", colDim
			if req.Met {
				mark, col = "✓", colHit
			}
			drawText(dst, mark, pgMargin, y, col)
			drawText(dst, req.Text, pgMargin+sp6, y, colNote)
			y += uiTextH + sp1
		}
	} else {
		hint := "Click a gig twice, or press Enter, to pick its setlist"
		drawTextSmall(dst, hint, pgMargin, l.cards[0].y+l.cards[0].h+sp4, colBarline)
	}
}

// drawSpriteOn draws a scene sprite straight onto the screen at a whole
// scale, its anchor at (x, y).
func drawSpriteOn(dst *ebiten.Image, group, name, anim string, frame int, x, y float64, scale int) {
	img, s, ok := sceneSheet(group, name)
	if !ok {
		return
	}
	r := s.Frame(anim, frame)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(s.Anchor[0]), -float64(s.Anchor[1]))
	op.GeoM.Scale(float64(scale), float64(scale))
	op.GeoM.Translate(math.Round(x), math.Round(y))
	dst.DrawImage(img.SubImage(r).(*ebiten.Image), op)
}

// ---- The setlist picker ----

type setlistPicker struct {
	gig    career.Gig
	songs  []progress.MasteredSong
	chosen []int // indexes into songs, in play order
	sel    int
	need   int
	best   map[int]float64
}

func newSetlistPicker(st *progress.Store, g career.Gig) *setlistPicker {
	t := career.Tiers[g.Tier-1]
	sp := &setlistPicker{gig: g, songs: st.Mastered(), need: t.Songs(), best: map[int]float64{}}
	if g.Recording {
		sp.need = t.SetSize
	}
	for i, s := range sp.songs {
		if acc, ok := st.BestAccuracy(s.SongID, s.Track); ok {
			sp.best[i] = acc
		}
	}
	// The last setlist played here, where its songs are still mastered.
	c := st.Career()
	for _, id := range c.Record(g.ID).LastSetlist {
		for i, s := range sp.songs {
			if songKey(s) == id && len(sp.chosen) < sp.need {
				sp.chosen = append(sp.chosen, i)
			}
		}
	}
	return sp
}

// songKey names a mastered song+track in a remembered setlist.
func songKey(s progress.MasteredSong) string { return fmt.Sprintf("%s#%d", s.SongID, s.Track) }

func (sp *setlistPicker) toggle(i int) {
	for j, c := range sp.chosen {
		if c == i {
			sp.chosen = append(sp.chosen[:j], sp.chosen[j+1:]...)
			return
		}
	}
	if len(sp.chosen) < sp.need {
		sp.chosen = append(sp.chosen, i)
	}
}

func (sp *setlistPicker) ready() bool { return len(sp.chosen) == sp.need }

type pickerLayout struct {
	card, play rect
	rows       []rect
}

func (sp *setlistPicker) layout() pickerLayout {
	var l pickerLayout
	w, h := math.Min(screenW-2*sp8, 760), math.Min(screenH-2*sp6, 520)
	l.card = rect{math.Round((screenW - w) / 2), math.Round((screenH - h) / 2), w, h}
	top := l.card.y + sp4 + sizeDisplayM + sp3 + uiTextH + sp4
	for i := range sp.songs {
		y := top + float64(i)*listRowH
		if y+listRowH > l.card.y+l.card.h-btnHPrimary-sp6 {
			break
		}
		l.rows = append(l.rows, rect{l.card.x + sp4, y, l.card.w - 2*sp4, listRowH})
	}
	pw := kitButtonW("Play gig", kitPrimary, false)
	l.play = rect{l.card.x + l.card.w - sp6 - pw, l.card.y + l.card.h - sp4 - btnHPrimary, pw, btnHPrimary}
	return l
}

func (sp *setlistPicker) update(p *PerformScreen) {
	l := sp.layout()
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		p.picker = nil
		return
	case inpututil.IsKeyJustPressed(ebiten.KeyUp):
		sp.sel = max(0, sp.sel-1)
	case inpututil.IsKeyJustPressed(ebiten.KeyDown):
		sp.sel = min(len(sp.songs)-1, sp.sel+1)
	case inpututil.IsKeyJustPressed(ebiten.KeySpace):
		if len(sp.songs) > 0 {
			sp.toggle(sp.sel)
		}
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter), inpututil.IsKeyJustPressed(ebiten.KeyKPEnter):
		sp.start(p)
		return
	}
	if !p.ptr.pressed {
		return
	}
	if p.ptr.over(l.play) {
		sp.start(p)
		return
	}
	for i, r := range l.rows {
		if p.ptr.over(r) {
			sp.sel = i
			sp.toggle(i)
		}
	}
	if !p.ptr.over(l.card) {
		p.picker = nil
	}
}

// start plays the gig: the crowd is rolled (once, ever) here, then the
// setlist is remembered and the gig begins.
func (sp *setlistPicker) start(p *PerformScreen) {
	if !sp.ready() || p.play == nil || p.st == nil {
		return
	}
	var set []progress.MasteredSong
	var keys []string
	for _, i := range sp.chosen {
		set = append(set, sp.songs[i])
		keys = append(keys, songKey(sp.songs[i]))
	}
	var crowd string
	var reveal bool
	p.st.UpdateCareer(func(c *career.State) {
		if forceTough() {
			c.ForceTough(sp.gig)
		}
		crowd, reveal = c.StartGig(sp.gig, gigRand())
		c.Gigs[sp.gig.ID].LastSetlist = keys
	})
	p.picker = nil
	p.play(sp.gig, crowd, reveal, set)
}

func (sp *setlistPicker) draw(dst *ebiten.Image, p *PerformScreen) {
	vector.FillRect(dst, 0, 0, float32(screenW), float32(screenH), colHelpDim, false)
	l := sp.layout()
	y := drawDialogCard(dst, l.card, "Setlist")
	t := career.Tiers[sp.gig.Tier-1]
	head := fmt.Sprintf("%s  ·  pick %d of your mastered songs, in the order you'll play them", sp.gig.Card(), sp.need)
	drawText(dst, truncateW(head, l.card.w-2*sp6), l.card.x+sp6, y, colDim)
	_ = t
	if len(sp.songs) == 0 {
		msg := "No mastered songs yet. Master a song in Practice: one full run at 100% speed with 90% or more."
		for i, line := range wrapTextW(msg, l.card.w-2*sp6) {
			drawText(dst, line, l.card.x+sp6, y+uiTextH*2+float64(i)*uiTextH, colNote)
		}
	}
	for i, r := range l.rows {
		s := sp.songs[i]
		order := 0
		for j, c := range sp.chosen {
			if c == i {
				order = j + 1
			}
		}
		if i == sp.sel || p.ptr.over(r) {
			vector.FillRect(dst, float32(r.x), float32(r.y), float32(r.w), float32(r.h), colHover, false)
		}
		if i == sp.sel {
			vector.FillRect(dst, float32(r.x), float32(r.y), 3, float32(r.h), tokBrand, false)
		}
		box := rect{r.x + sp3, r.y + (r.h-pillH)/2, pillH, pillH}
		if order > 0 {
			notchedFill(dst, box, tokBrand)
			n := fmt.Sprint(order)
			drawTextScaled(dst, n, box.x+(box.w-textWScaled(n, dispS))/2, box.y+(box.h-sizeDisplayS)/2-1, dispS, colBG)
		} else {
			notchedStroke(dst, box, 1, colPanelEdge)
		}
		title := s.Title
		if title == "" {
			title = cleanPieceTitle("", s.Path)
		}
		tx := box.x + box.w + sp3
		drawText(dst, truncateW(title, r.w-260), tx, r.y+sp1+1, colNote)
		sub := s.Difficulty
		if acc, ok := sp.best[i]; ok {
			if sub != "" {
				sub += " · "
			}
			sub += fmt.Sprintf("best %d%%", int(math.Round(acc*100)))
		}
		drawTextSmall(dst, sub, tx, r.y+r.h/2+3, colDim)
	}
	status := fmt.Sprintf("%d / %d chosen", len(sp.chosen), sp.need)
	drawText(dst, status, l.card.x+sp6, l.play.y+(l.play.h-uiTextH)/2, colDim)
	drawKitButton(dst, l.play, "Play gig", -1, kitPrimary, btnState{hover: p.ptr.over(l.play), disabled: !sp.ready()})
}
