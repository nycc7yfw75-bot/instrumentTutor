package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/progress"
	"github.com/S95F/musicTutor/internal/scoring"
)

// The start screen's player card (owner's choice, docs/SCORING.md §10):
// top right, the player's name, level and title, an XP bar, their latest
// badges as pixel medals (unearned ones locked, with how to earn them in
// the tooltip), and the practice streak. Settings and Help sit in the
// card's corner. B opens every badge.

const (
	cardW      = 372.0
	cardPad    = sp3
	medalSize  = 36.0
	medalGap   = sp2
	medalCount = 7
	cardXPH    = 8.0
	cardH      = cardPad + sizeDisplayM + sp1 + uiTextH + sp2 + cardXPH + sp3 + medalSize + sp2 + 16 + cardPad
)

// badgeIcon is a badge's medal picture.
var badgeIcon = map[string]iconID{
	progress.BadgeSlowClean:   icBadgeTurtle,
	progress.BadgeUpToSpeed:   icBadgeUp,
	progress.BadgeMastered:    icBadgeStar,
	progress.BadgeNailedIt:    icBadgeTarget,
	progress.BadgeRepertoire:  icBadgeStack,
	progress.BadgeFiveMore:    icBadgeClock,
	progress.BadgeSpeedLimit:  icBadgeGauge,
	progress.BadgeClickFriend: icMetronome,
	progress.BadgeSevenDays:   icBadgeWeek,
	progress.BadgeMonth:       icBadgeMonth,
	progress.BadgeBackAtIt:    icBadgeReturn,
}

// store is the progress store, or nil in a build without one.
func (b *Browser) store() *progress.Store {
	if b.sh == nil {
		return nil
	}
	return b.sh.Services().Progress
}

// hasCard reports whether the start screen shows the player card.
func (b *Browser) hasCard() bool { return b.store() != nil }

// medal is one tile on the card: a badge, earned or not.
type medal struct {
	def    progress.BadgeDef
	earned bool
}

// cardMedals are the card's tiles: earned badges newest first, then the
// ones still to earn, in catalog order.
func (b *Browser) cardMedals() []medal {
	st := b.store()
	if st == nil {
		return nil
	}
	var out []medal
	have := map[string]bool{}
	for _, e := range st.Earned() {
		out = append(out, medal{e.BadgeDef, true})
		have[e.ID] = true
	}
	for _, d := range progress.Badges {
		if !have[d.ID] {
			out = append(out, medal{d, false})
		}
	}
	return out
}

// layoutCard places the card and what is on it.
func (b *Browser) layoutCard(l *browserLayout) {
	l.card = rect{screenW - pgMargin - cardW, pgMargin - sp4, cardW, cardH}
	c := l.card
	// Settings and Help, icon only, in the card's top-right corner.
	x := c.x + c.w - sp1
	l.menu = l.menu[:0]
	for i := len(browserMenu) - 1; i >= 0; i-- {
		x -= zItemH
		l.menu = append([]rect{{x, c.y + sp1, zItemH, zItemH}}, l.menu...)
	}
	my := c.y + cardPad + sizeDisplayM + sp1 + uiTextH + sp2 + cardXPH + sp3
	for i := 0; i < medalCount; i++ {
		l.medals = append(l.medals, rect{c.x + cardPad + float64(i)*(medalSize+medalGap), my, medalSize, medalSize})
	}
	if st := b.store(); st != nil && st.NeedsProfile() {
		w := kitButtonW("Add your name", kitSecondary, false)
		l.addName = rect{c.x + cardPad, c.y + cardPad + sizeDisplayM + sp2, w, btnH}
	}
}

// drawPlayerCard draws the card.
func (b *Browser) drawPlayerCard(dst *ebiten.Image, l browserLayout, dt float64) {
	st := b.store()
	if st == nil {
		return
	}
	c := l.card
	notchedFill(dst, c, colPanel)
	for i, r := range l.menu {
		m := browserMenu[i]
		off := i == 0 && b.settings == nil
		over := b.ptr.over(r) && !off
		av := b.anim.step("menu:"+m.label, over, b.ptr.down, dt)
		drawKitMenuItem(dst, r, m.icon, "", false, off, av)
		b.tip.offer("menu:"+m.label, m.label, m.key, r, over, dt)
	}

	x, y := c.x+cardPad, c.y+cardPad
	name, xp, ok := st.Active()
	if !ok {
		drawTextScaled(dst, "New player", x, y, dispM, colNote)
		over := b.ptr.over(l.addName)
		drawKitButton(dst, l.addName, "Add your name", -1, kitSecondary, btnState{hover: over, press: over && b.ptr.down})
		drawTextSmall(dst, "Your level, XP and badges are saved under it", x, l.addName.y+l.addName.h+sp2, colDim)
		return
	}
	nameW := l.menu[0].x - sp2 - x
	drawTextScaled(dst, truncateWScaled(name, nameW, dispM), x, y, dispM, colNote)
	y += sizeDisplayM + sp1

	lvl, into, toNext := scoring.LevelFor(xp.TotalXP)
	drawText(dst, fmt.Sprintf("LV %d · %s", lvl, scoring.Title(lvl)), x, y, colDim)
	nums := fmt.Sprintf("%d / %d XP", into, toNext)
	drawMonoSized(dst, nums, c.x+c.w-cardPad-textWMonoSized(nums, sizeUISmall), y+2, sizeUISmall, colBarline)
	y += uiTextH + sp2

	// The XP bar: segmented, like every meter here.
	const blocks = 24
	w := c.w - 2*cardPad
	bw := (w - (blocks-1)*2) / blocks
	lit := int(math.Floor(float64(into) / float64(max(1, toNext)) * blocks))
	for i := 0; i < blocks; i++ {
		col := colBlockOff
		if i < lit {
			col = colXPFill
		}
		vector.FillRect(dst, float32(x+float64(i)*(bw+2)), float32(y), float32(bw), cardXPH, col, false)
	}

	medals := b.cardMedals()
	for i, r := range l.medals {
		if i >= len(medals) {
			break
		}
		m := medals[i]
		drawMedal(dst, r, m)
		tip := m.def.Name + " — " + m.def.How
		if !m.earned {
			tip = "Not yet: " + m.def.Name + " — " + m.def.How
		}
		b.tip.offer("medal:"+m.def.ID, tip, "B", r, b.ptr.over(r), dt)
	}
	earned := len(st.Earned())
	more := fmt.Sprintf("%d / %d", earned, len(progress.Badges))
	mx := l.medals[len(l.medals)-1].x + medalSize + sp2
	drawMonoSized(dst, more, mx, l.medals[0].y+(medalSize-16)/2, sizeUISmall, colBarline)

	y = l.medals[0].y + medalSize + sp2
	drawTextSmall(dst, streakLine(st.Streak()), x, y, colDim)
}

// streakLine is the card's streak line; it leads with what never goes
// down (PROGRESS.md invariant 6).
func streakLine(s progress.Streak) string {
	parts := []string{"No practice days yet"}
	if s.TotalDays > 0 {
		parts[0] = fmt.Sprintf("%d %s practised", s.TotalDays, plural(s.TotalDays, "day", "days"))
	}
	if s.Days > 0 {
		parts = append(parts, fmt.Sprintf("%d-day streak", s.Days))
	}
	parts = append(parts, fmt.Sprintf("%d %s", s.Freezes, plural(s.Freezes, "freeze", "freezes")))
	if !s.TodayDone {
		left := int(math.Ceil((progress.StreakDaySeconds - s.TodaySecs) / 60))
		parts = append(parts, fmt.Sprintf("%d min to go today", left))
	}
	return strings.Join(parts, " · ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// drawMedal draws a badge tile: earned, an amber medal on a dark tile;
// not yet, a dim outline with a small lock.
func drawMedal(dst *ebiten.Image, r rect, m medal) {
	ic := badgeIcon[m.def.ID]
	if m.earned {
		notchedFill(dst, r, colHover)
		drawPixelIcon(dst, ic, math.Round(r.x+(r.w-iconSize)/2), math.Round(r.y+(r.h-iconSize)/2), tokBrand)
		return
	}
	notchedStroke(dst, r, 1, colPanelEdge)
	drawPixelIcon(dst, ic, math.Round(r.x+(r.w-iconSize)/2), math.Round(r.y+(r.h-iconSize)/2), colBlockOff)
	// The lock: a 6×5 body and a 4×3 shackle at the bottom-right corner.
	lx, ly := float32(r.x+r.w-10), float32(r.y+r.h-10)
	vector.FillRect(dst, lx, ly+3, 7, 5, colDim, false)
	vector.FillRect(dst, lx+1, ly, 1, 3, colDim, false)
	vector.FillRect(dst, lx+5, ly, 1, 3, colDim, false)
	vector.FillRect(dst, lx+1, ly, 5, 1, colDim, false)
}

// ---- Every badge (B) ----

func badgesCard() rect {
	w, h := math.Min(screenW-2*sp8, 960), 500.0
	return rect{math.Round((screenW - w) / 2), math.Round((screenH - h) / 2), w, h}
}

const badgeRowH = 64.0

// drawBadges draws the overlay listing every badge.
func (b *Browser) drawBadges(dst *ebiten.Image) {
	st := b.store()
	if st == nil {
		return
	}
	vector.FillRect(dst, 0, 0, float32(screenW), float32(screenH), colHelpDim, false)
	c := badgesCard()
	y := drawDialogCard(dst, c, "Badges")
	earnedAt := map[string]string{}
	for _, e := range st.Earned() {
		earnedAt[e.ID] = e.At.Local().Format("2 Jan 2006")
	}
	sub := fmt.Sprintf("%d of %d earned. A badge is never taken away.", len(earnedAt), len(progress.Badges))
	drawTextSmall(dst, sub, c.x+sp6, y-sp2, colDim)
	y += sp4
	colW := (c.w - 2*sp6 - sp6) / 2
	perCol := (len(progress.Badges) + 1) / 2
	for i, d := range progress.Badges {
		col, row := i/perCol, i%perCol
		x := c.x + sp6 + float64(col)*(colW+sp6)
		ry := y + float64(row)*badgeRowH
		at, earned := earnedAt[d.ID]
		drawMedal(dst, rect{x, ry, medalSize, medalSize}, medal{d, earned})
		tx := x + medalSize + sp3
		nameCol := colNote
		if !earned {
			nameCol = colDim
		}
		drawText(dst, d.Name, tx, ry-2, nameCol)
		how := d.How
		if earned {
			how = "Earned " + at + " · " + how
		}
		for j, line := range wrapTextW(how, colW-medalSize-sp3) {
			if j == 2 {
				break
			}
			drawTextSmall(dst, line, tx, ry+uiTextH+float64(j)*15, colBarline)
		}
	}
	drawTextSmall(dst, "B, Esc or a click closes this", c.x+sp6, c.y+c.h-sp4-14, colBarline)
}
