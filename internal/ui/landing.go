package ui

import (
	"fmt"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/career"
	"github.com/S95F/musicTutor/internal/progress"
	"github.com/S95F/musicTutor/internal/scene"
	"github.com/S95F/musicTutor/internal/scoring"
)

// The landing page (CAREER.md §1): the first screen. The wordmark and the
// player at the top; the doors across the middle — Practice, Perform, and
// Battles once it exists.

// Landing is the first screen.
type Landing struct {
	sh   *Shell
	ptr  pointer
	anim animator
	tip  tips

	helpOpen, badgesOpen bool

	practice, perform, battles, settings func()

	venue *ebiten.Image // the Perform door's stage, at base resolution
	clock float64
}

// NewLanding makes the landing page.
func NewLanding(sh *Shell) *Landing { return &Landing{sh: sh} }

// SetPractice, SetPerform, SetBattles and SetSettings wire the doors.
func (l *Landing) SetPractice(fn func()) { l.practice = fn }
func (l *Landing) SetPerform(fn func())  { l.perform = fn }
func (l *Landing) SetBattles(fn func())  { l.battles = fn }
func (l *Landing) SetSettings(fn func()) { l.settings = fn }

func (l *Landing) store() *progress.Store {
	if l.sh == nil {
		return nil
	}
	return l.sh.Services().Progress
}

// door is one of the landing page's doors.
type door struct {
	id, title, line, key string
	act                  func()
	primary              bool
}

// doors are the doors to show, left to right: Perform in the middle when
// there are three. Battles is absent until it is built (no "coming soon").
func (l *Landing) doors() []door {
	practice := door{id: "practice", title: "Practice", line: "Learn and master songs", key: "P", act: l.practice}
	perform := door{id: "perform", title: "Perform", key: "G", act: l.perform, primary: true}
	perform.line, _ = l.performLines()
	if l.battles == nil {
		return []door{practice, perform}
	}
	battles := door{id: "battles", title: "Battles", line: "Trade bars with friends and rivals", key: "B", act: l.battles}
	return []door{practice, perform, battles}
}

// careerMastered is the active profile's mastered songs, as the career
// rules need them.
func careerMastered(st *progress.Store) []career.MasteredSong {
	if st == nil {
		return nil
	}
	var out []career.MasteredSong
	for _, m := range st.Mastered() {
		out = append(out, career.MasteredSong{Difficulty: difficultyNamed(m.Difficulty)})
	}
	return out
}

func difficultyNamed(s string) scoring.Difficulty {
	for d := scoring.Easy; d <= scoring.Expert; d++ {
		if d.String() == s {
			return d
		}
	}
	return scoring.Easy
}

// playerLevel is the active profile's level (1 without one).
func playerLevel(st *progress.Store) int {
	if st == nil {
		return 1
	}
	if _, xp, ok := st.Active(); ok {
		return xp.Level()
	}
	return 1
}

// nextGig is where the career stands: the first gig of the current tier
// still short of three stars (or the tier's first gig).
func nextGig(c career.State, tier career.Tier) (career.Gig, bool) {
	route := c.Route()
	if tier.Num < 1 || tier.Num > len(route) {
		return career.Gig{}, false
	}
	for _, g := range route[tier.Num-1] {
		if c.Record(g.ID).BestStars < career.ClearStars {
			return g, true
		}
	}
	return route[tier.Num-1][0], true
}

// performLines are the Perform door's two lines: where the career is, and
// the next thing to do.
func (l *Landing) performLines() (string, string) {
	st := l.store()
	if st == nil {
		return "Your career needs saved progress", ""
	}
	c := st.Career()
	if !c.Started() {
		return "Set up your career to start the tour", ""
	}
	mastered := careerMastered(st)
	level := playerLevel(st)
	tier := c.CurrentTier(level, mastered)
	where := "Your career: " + tier.Name
	if g, ok := nextGig(c, tier); ok {
		where = fmt.Sprintf("Your career: %s, %s", g.Venue, g.City.Name)
	}
	return where, c.NextRequirement(level, mastered)
}

type landingLayout struct {
	settings, help rect
	badges         rect   // the medal row and its count: opens every badge
	medals         []rect // the medal tiles
	doors          []rect
}

func (l *Landing) layout() landingLayout {
	var ll landingLayout
	y := float64(pgMargin)
	ll.help = rect{screenW - pgMargin - zItemH + sp2, y - sp1, zItemH, zItemH}
	ll.settings = rect{ll.help.x - zItemH, y - sp1, zItemH, zItemH}
	// Under the strip: the medals, newest first, then "n / 11" (the
	// owner's badges sit with their name, at the top).
	my := y + zItemH + sp2
	count := l.badgesLabel()
	cw := textWMonoSized(count, sizeUISmall)
	right := screenW - pgMargin - cw - sp2
	for i := landingMedals - 1; i >= 0; i-- {
		ll.medals = append([]rect{{right - float64(landingMedals-i)*(landingMedal+sp1), my, landingMedal, landingMedal}}, ll.medals...)
	}
	if l.store() == nil {
		ll.medals = nil
	}
	if len(ll.medals) > 0 {
		ll.badges = rect{ll.medals[0].x, my, screenW - pgMargin - ll.medals[0].x, landingMedal}
	}

	top := y + zItemH + sp2 + landingMedal + sp6
	bottom := screenH - pgMargin
	ds := l.doors()
	gap := float64(sp6)
	avail := screenW - 2*pgMargin - gap*float64(len(ds)-1)
	weights := map[string]float64{"practice": 1, "perform": 1.6, "battles": 1}
	total := 0.0
	for _, d := range ds {
		total += weights[d.id]
	}
	x := float64(pgMargin)
	for _, d := range ds {
		w := math.Round(avail * weights[d.id] / total)
		ll.doors = append(ll.doors, rect{x, top, w, bottom - top})
		x += w + gap
	}
	return ll
}

const (
	landingMedals = 7
	landingMedal  = 28.0
)

func (l *Landing) badgesLabel() string {
	st := l.store()
	if st == nil {
		return ""
	}
	return fmt.Sprintf("%d / %d", len(st.Earned()), len(progress.Badges))
}

// landingBindings are the landing page's keys (CAREER.md §1).
func (l *Landing) landingBindings() []helpBinding {
	return []helpBinding{
		{Group: "doors", Keys: "P", Hint: "P practice", Desc: "Practice: your songs, learning and mastering them"},
		{Group: "doors", Keys: "G", Hint: "G perform", Off: l.perform == nil, Desc: "Perform: your career and its gigs"},
		{Group: "doors", Keys: "B", Off: l.battles == nil, Desc: "Battles"},
		{Group: "player", Keys: "A", Off: l.store() == nil, Desc: "Every badge: the ones you have, and how to earn the rest"},
		{Group: "app", Keys: "S", Hint: "S settings", Off: l.settings == nil, Desc: "Settings: audio, calibration, your profile and career"},
		{Group: "app", Keys: "? or F1", Hint: "? help", Desc: "This key-binding list"},
		{Group: "app", Keys: "esc or Q", Desc: "Quit HEADLINER"},
	}
}

func (l *Landing) Update() error {
	l.ptr = readPointer()
	l.clock += uiFrameSeconds()
	if l.helpOpen {
		if helpDismissed(l.ptr) {
			l.helpOpen = false
		}
		return nil
	}
	if l.badgesOpen {
		if helpDismissed(l.ptr) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
			l.badgesOpen = false
		}
		return nil
	}
	call := func(fn func()) {
		if fn != nil {
			fn()
		}
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape), inpututil.IsKeyJustPressed(ebiten.KeyQ):
		return errQuit
	case helpKeyPressed():
		l.helpOpen = true
		return nil
	case inpututil.IsKeyJustPressed(ebiten.KeyP):
		call(l.practice)
		return nil
	case inpututil.IsKeyJustPressed(ebiten.KeyG):
		call(l.perform)
		return nil
	case inpututil.IsKeyJustPressed(ebiten.KeyB):
		call(l.battles)
		return nil
	case inpututil.IsKeyJustPressed(ebiten.KeyS):
		call(l.settings)
		return nil
	case inpututil.IsKeyJustPressed(ebiten.KeyA):
		l.badgesOpen = l.store() != nil
		return nil
	}
	l.handleMouse(l.ptr)
	return nil
}

func (l *Landing) handleMouse(p pointer) {
	if !p.pressed {
		return
	}
	ll := l.layout()
	switch {
	case p.over(ll.settings):
		if l.settings != nil {
			l.settings()
		}
		return
	case p.over(ll.help):
		l.helpOpen = true
		return
	case ll.badges.w > 0 && p.over(ll.badges):
		l.badgesOpen = true
		return
	}
	for i, d := range l.doors() {
		if p.over(ll.doors[i]) && d.act != nil {
			d.act()
			return
		}
	}
}

func (l *Landing) Draw(dst *ebiten.Image) {
	dst.Fill(colBG)
	l.anim.tick()
	dt := motionDt()
	ll := l.layout()
	drawWordmark(dst, pgMargin, pgMargin)
	drawTextSmall(dst, "Open mic to arena", pgMargin, pgMargin+sizeDisplayL+sp1, colDim)
	l.drawStrip(dst, ll, dt)
	for i, d := range l.doors() {
		l.drawDoor(dst, ll.doors[i], d, dt)
	}
	l.tip.draw(dst)
	if l.badgesOpen {
		drawBadgesOverlay(dst, l.store())
	}
	if l.helpOpen {
		drawHelpOverlay(dst, "LANDING KEYS", l.landingBindings(), "")
	}
}

// drawStrip draws the player at the top right: name, level and title, the
// XP bar, Fame, then Settings and Help; the streak and badges under it.
func (l *Landing) drawStrip(dst *ebiten.Image, ll landingLayout, dt float64) {
	for _, it := range []struct {
		r          rect
		icon       iconID
		label, key string
		off        bool
	}{{ll.settings, icSettings, "Settings", "S", l.settings == nil}, {ll.help, icHelp, "Help", "?", false}} {
		over := l.ptr.over(it.r) && !it.off
		drawKitMenuItem(dst, it.r, it.icon, "", false, it.off, l.anim.step("strip:"+it.label, over, l.ptr.down, dt))
		l.tip.offer("strip:"+it.label, it.label, it.key, it.r, over, dt)
	}
	st := l.store()
	if st == nil {
		return
	}
	name, xp, ok := st.Active()
	if !ok {
		return
	}
	y := float64(pgMargin)
	x := ll.settings.x - sp4

	// Fame: a pixel star and the number.
	fame := fmt.Sprintf("%d", st.Career().Fame)
	x -= textWScaled(fame, dispM)
	drawTextScaled(dst, fame, x, y, dispM, colNote)
	x -= iconPx + sp1
	drawPixelIconScaled(dst, icBadgeStar, x, y+4, 1, tokBrand)
	x -= sp6

	// The XP bar: 12 blocks to the next level.
	lvl, into, toNext := scoring.LevelFor(xp.TotalXP)
	const blocks = 12
	barW := blocks*(meterBlockW+meterGap) - meterGap
	x -= barW
	lit := int(math.Floor(float64(into) / float64(max(1, toNext)) * blocks))
	for i := 0; i < blocks; i++ {
		col := colBlockOff
		if i < lit {
			col = colXPFill
		}
		vector.FillRect(dst, float32(x+float64(i)*(meterBlockW+meterGap)), float32(y+8), meterBlockW, meterBlockH, col, false)
	}
	l.tip.offer("strip:xp", fmt.Sprintf("%d / %d XP to level %d", into, toNext, lvl+1), "", rect{x, y, barW, zItemH - 8}, l.ptr.over(rect{x, y, barW, zItemH - 8}), dt)
	x -= sp3

	lv := fmt.Sprintf("LV %d · %s", lvl, scoring.Title(lvl))
	x -= textW(lv)
	drawText(dst, lv, x, y+6, colDim)
	x -= sp4
	nm := truncateWScaled(name, 280, dispM)
	x -= textWScaled(nm, dispM)
	drawTextScaled(dst, nm, x, y, dispM, colNote)

	// Under it: the streak, then the medals and how many are earned.
	medals := storeMedals(st)
	for i, r := range ll.medals {
		if i >= len(medals) {
			break
		}
		m := medals[i]
		drawMedal(dst, r, m)
		tip := m.def.Name + " — " + m.def.How
		if !m.earned {
			tip = "Not yet: " + tip
		}
		l.tip.offer("medal:"+m.def.ID, tip, "A", r, l.ptr.over(r), dt)
	}
	if len(ll.medals) > 0 {
		last := ll.medals[len(ll.medals)-1]
		count := l.badgesLabel()
		cc := colBarline
		if l.ptr.over(ll.badges) {
			cc = colNote
		}
		drawMonoSized(dst, count, last.x+last.w+sp2, last.y+(last.h-16)/2+1, sizeUISmall, cc)
		s := streakLine(st.Streak())
		drawTextSmall(dst, s, ll.medals[0].x-sp4-textWSmall(s), last.y+(last.h-uiTextH)/2+1, colDim)
	}
}

// drawDoor draws a door: a notched panel with an illustration, its title
// in the display font and one line. Perform is the page's one filled
// brand element.
func (l *Landing) drawDoor(dst *ebiten.Image, r rect, d door, dt float64) {
	over := l.ptr.over(r) && d.act != nil
	av := l.anim.step("door:"+d.id, over, l.ptr.down, dt)
	if over && l.ptr.down {
		r.y++
	}
	ink, sub := colNote, colDim
	if d.primary {
		notchedFill(dst, r, lerpCol(tokBrand, tokText, 0.12*av.hover))
		ink, sub = colBG, withAlpha(colBG, 0.75)
	} else {
		notchedFill(dst, r, lerpCol(colPanel, colHover, av.hover))
	}
	l.tip.offer("door:"+d.id, d.title, d.key, r, over, dt)

	// The illustration fills the door above its words.
	textH := sizeDisplayL + sp2 + uiTextH*2 + sp6
	art := rect{r.x + sp4, r.y + sp4, r.w - 2*sp4, r.h - 2*sp4 - textH}
	switch d.id {
	case "perform":
		l.drawPerformArt(dst, art)
	case "practice":
		notchedFill(dst, art, colBG)
		scale := max(2, min(8, int(math.Min(art.w, art.h)/iconPx/2)))
		s := float64(scale * iconPx)
		drawPixelIconScaled(dst, icGuitar, math.Round(art.x+(art.w-s)/2), math.Round(art.y+(art.h-s)/2), scale, colDim)
	default:
		notchedFill(dst, art, colBG)
	}

	ty := r.y + r.h - sp4 - textH + sp4
	drawTextScaled(dst, d.title, math.Round(r.x+sp6), math.Round(ty), dispL, ink)
	ty += sizeDisplayL + sp2
	drawText(dst, truncateW(d.line, r.w-2*sp6), r.x+sp6, ty, sub)
	if d.id == "perform" {
		if _, next := l.performLines(); next != "" {
			drawTextSmall(dst, truncateW(next, r.w-2*sp6), r.x+sp6, ty+uiTextH+2, sub)
		}
	}
}

// drawPerformArt draws the current venue's stage inside the Perform door,
// at the largest whole-number scale that fits, centred on a dark inset.
func (l *Landing) drawPerformArt(dst *ebiten.Image, art rect) {
	notchedFill(dst, art, colBG)
	if l.venue == nil {
		l.venue = ebiten.NewImage(scene.BaseW, scene.BaseH)
	}
	tier, region, preset := "bedroom", "", 0
	if st := l.store(); st != nil {
		c := st.Career()
		if c.Started() {
			t := c.CurrentTier(playerLevel(st), careerMastered(st))
			tier = tierSlug(t.Num, false)
			if g, ok := nextGig(c, t); ok {
				region = g.City.Region
			}
			preset = c.Character
		}
	}
	l.venue.Clear()
	drawVenue(l.venue, tier, region, int(l.clock*2)%4)
	drawSprite(l.venue, "characters/"+presetSlug(preset), "guitarist", "idle", int(l.clock*2)%2, 140, 168, 1)

	drawSceneFilled(dst, l.venue, art)
}
