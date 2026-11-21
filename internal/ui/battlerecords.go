package ui

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/battle"
	"github.com/S95F/musicTutor/internal/progress"
)

// The Records tab of the Battles menu (BATTLES.md §8.2): one profile's
// wins, losses and draws, its best battle, its record against each
// opponent (locked rivals as silhouettes) and its last battles.

const (
	recRowH   = 40.0
	recLogH   = 44.0
	recTitleY = 8.0
)

// titleTabs are the header's "New battle" and "Records" tabs.
func (b *BattleSetup) titleTabs() [2]rect {
	x := b.titleX
	w0, w1 := textWScaled("New battle", dispL), textWScaled("Records", dispL)
	return [2]rect{{x, recTitleY, w0, 40}, {x + w0 + sp6, recTitleY, w1, 40}}
}

// drawTitleTabs draws the header's two tabs, the open one bright and
// underlined.
func (b *BattleSetup) drawTitleTabs(dst *ebiten.Image) {
	for i, r := range b.titleTabs() {
		label := [2]string{"New battle", "Records"}[i]
		on := b.records == (i == 1)
		col := colDim
		if on || b.ptr.over(r) {
			col = colNote
		}
		drawTextScaled(dst, label, r.x, r.y+6, dispL, col)
		if on {
			vector.FillRect(dst, float32(r.x), float32(r.y+r.h), float32(r.w), menuUnderline, colAccent, false)
		}
	}
}

// recordsLayout places the profile picker, the opponents column and the
// log column.
type recordsLayout struct {
	left, right   rect // the profile ‹ ›
	summaryY      float64
	opp, log      rect
	maxOpp, maxLg int
}

func recordsLayoutFor() recordsLayout {
	var l recordsLayout
	w := math.Min(screenW-2*pgMargin, 1000)
	x := math.Round((screenW - w) / 2)
	y := float64(zHeaderH) + sp2
	l.left = rect{x + 180, y + 8, 28, 28}
	l.right = rect{x + 180 + 360, y + 8, 28, 28}
	l.summaryY = y + 52
	top := l.summaryY + 44
	h := screenH - top - pgMargin
	colW := (w - sp6) / 2
	l.opp = rect{x, top, colW, h}
	l.log = rect{x + colW + sp6, top, colW, h}
	l.maxOpp = max(1, int((h-uiTextH-sp2)/recRowH))
	l.maxLg = max(1, int((h-uiTextH-sp2)/recLogH))
	return l
}

// updateRecords handles the Records tab: ← → pick the profile, ↑ ↓ and
// the wheel scroll the log.
func (b *BattleSetup) updateRecords() {
	n := len(b.profiles())
	l := recordsLayoutFor()
	step := func(d int) {
		if n > 0 {
			b.recProfile = ((b.recProfile+d)%n + n) % n
			b.recScroll = 0
		}
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyLeft):
		step(-1)
	case inpututil.IsKeyJustPressed(ebiten.KeyRight):
		step(1)
	case inpututil.IsKeyJustPressed(ebiten.KeyUp):
		b.recScroll = max(0, b.recScroll-1)
	case inpututil.IsKeyJustPressed(ebiten.KeyDown):
		b.recScroll++
	}
	if s, rem := wheelSteps(b.ptr.wheel); s != 0 {
		b.recScroll = max(0, b.recScroll-s)
		b.ptr.wheel = rem
	}
	if b.ptr.pressed {
		switch {
		case b.ptr.over(l.left):
			step(-1)
		case b.ptr.over(l.right):
			step(1)
		}
	}
}

// recordOpponent is one row of the "Against" column.
type recordOpponent struct {
	name   string
	p      battle.Participant // for the portrait
	locked bool
	unlock int
	t      progress.BattleTotals
}

// recordOpponents lists every rival (locked ones too), then the session
// levels, other profiles and ghosts the profile has met.
func recordOpponents(rec progress.BattleRecord, ps []progress.ProfileInfo, level int) []recordOpponent {
	var out []recordOpponent
	for _, r := range battle.Rivals {
		o := recordOpponent{name: r.Name, p: battle.Participant{Kind: battle.KindRival, ID: r.Name},
			t: rec.PerOpponent["rival:"+r.Name]}
		if !r.Unlocked(level) {
			o.name, o.locked, o.unlock = "???", true, r.Unlock
		}
		out = append(out, o)
	}
	for _, l := range battle.SessionLevels {
		if t, ok := rec.PerOpponent["session:"+l.Name]; ok {
			out = append(out, recordOpponent{name: "Session Player (" + l.Name + ")", p: battle.Participant{Kind: battle.KindSession}, t: t})
		}
	}
	var others []recordOpponent
	for key, t := range rec.PerOpponent {
		id, ok := strings.CutPrefix(key, "profile:")
		if !ok {
			continue
		}
		name, tint := "A deleted profile", 0
		for _, p := range ps {
			if p.ID == id {
				name, tint = p.Name, p.Tint
			}
		}
		others = append(others, recordOpponent{name: name, p: battle.Participant{Kind: battle.KindHuman, Tint: tint}, t: t})
	}
	sort.Slice(others, func(i, j int) bool { return others[i].name < others[j].name })
	out = append(out, others...)
	if t, ok := rec.PerOpponent["ghost"]; ok {
		out = append(out, recordOpponent{name: "Ghosts", p: battle.Participant{Kind: battle.KindGhost}, t: t})
	}
	return out
}

func totalsText(t progress.BattleTotals) string {
	return fmt.Sprintf("%dW  %dL  %dD", t.Wins, t.Losses, t.Draws)
}

// drawRecords draws the Records tab.
func (b *BattleSetup) drawRecords(dst *ebiten.Image) {
	l := recordsLayoutFor()
	ps := b.profiles()
	if len(ps) == 0 {
		drawText(dst, "No profiles yet: make one in Settings → Profiles.", l.opp.x+sp3, l.summaryY, colDim)
		return
	}
	p := ps[min(b.recProfile, len(ps)-1)]
	rec, _ := b.st.BattleRecordOf(p.ID)

	drawText(dst, "Profile", l.left.x-180+sp3, l.left.y+(l.left.h-uiTextH)/2, colDim)
	b.drawArrows(dst, l.left, l.right, p.Name)
	drawPixelIcon(dst, icGuitar, l.left.x+l.left.w+sp2+textW(p.Name)+sp4, l.left.y+(l.left.h-iconSize)/2, profileTint(p.Tint))

	// The summary line.
	t := rec.Totals
	played := t.Wins + t.Losses + t.Draws
	sum := fmt.Sprintf("%d %s · %d %s · %d %s", t.Wins, plural(t.Wins, "win", "wins"), t.Losses,
		plural(t.Losses, "loss", "losses"), t.Draws, plural(t.Draws, "draw", "draws"))
	if played == 0 {
		sum = "No battles yet"
	}
	drawTextScaled(dst, sum, l.opp.x+sp3, l.summaryY, dispM, colNote)
	if rec.Best > 0 {
		best := fmt.Sprintf("Best battle  %d", rec.Best)
		drawText(dst, best, l.log.x+l.log.w-textW(best), l.summaryY+6, colXPFill)
	}

	// Against each opponent.
	drawText(dst, "Against", l.opp.x+sp3, l.opp.y, colDim)
	y := l.opp.y + uiTextH + sp2
	for i, o := range recordOpponents(rec, ps, p.Level) {
		if i >= l.maxOpp {
			break
		}
		r := rect{l.opp.x, y, l.opp.w, recRowH - 4}
		notchedFill(dst, r, colPanel)
		if o.locked {
			drawSilhouette(dst, o.p, r.x+sp3, r.y+9)
			drawText(dst, o.name, r.x+48, r.y+(r.h-uiTextH)/2, colDim)
			drawTextSmall(dst, fmt.Sprintf("unlocks at level %d", o.unlock), r.x+48+textW("???")+sp3, r.y+(r.h-uiTextH)/2+3, colDim)
		} else {
			drawPortraitSmall(dst, o.p, r.x+sp3, r.y+9)
			drawText(dst, truncateW(o.name, r.w-48-140), r.x+48, r.y+(r.h-uiTextH)/2, colNote)
			s := totalsText(o.t)
			drawMonoSized(dst, s, r.x+r.w-sp3-textWMonoSized(s, sizeUI), r.y+(r.h-16)/2, sizeUI, colDim)
		}
		y += recRowH
	}

	// The last battles, newest first.
	drawText(dst, "Last battles", l.log.x+sp3, l.log.y, colDim)
	y = l.log.y + uiTextH + sp2
	if len(rec.Log) == 0 {
		drawText(dst, "Your battles will be listed here.", l.log.x+sp3, y+sp2, colDim)
		return
	}
	b.recScroll = min(b.recScroll, max(0, len(rec.Log)-l.maxLg))
	for k := 0; k < l.maxLg; k++ {
		i := len(rec.Log) - 1 - b.recScroll - k
		if i < 0 {
			break
		}
		e := rec.Log[i]
		r := rect{l.log.x, y, l.log.w, recLogH - 4}
		notchedFill(dst, r, colPanel)
		head := e.Date.Local().Format("Jan 2") + " · " + e.Material
		drawTextSmall(dst, truncateW(head, r.w-2*sp3), r.x+sp3, r.y+4, colDim)
		var parts []string
		for j, name := range e.Participants {
			if j < len(e.Totals) {
				parts = append(parts, fmt.Sprintf("%s %d", name, e.Totals[j]))
			}
		}
		verdict, col := "Draw", colDim
		switch {
		case e.Winner == p.Name:
			verdict, col = "Won", colHit
		case e.Winner != "":
			verdict, col = e.Winner+" won", colDim
		}
		drawText(dst, verdict, r.x+r.w-sp3-textW(verdict), r.y+r.h-uiTextH-4, col)
		drawText(dst, truncateW(strings.Join(parts, " · "), r.w-3*sp3-textW(verdict)), r.x+sp3, r.y+r.h-uiTextH-4, colNote)
		y += recLogH
	}
}

// drawPortraitSmall is a 32 px portrait for a records row.
func drawPortraitSmall(dst *ebiten.Image, p battle.Participant, x, y float64) {
	drawPortrait(dst, p, x, y, 1)
}

// silhouettes caches each rival's sprite drawn as a dark shape, for the
// locked rivals.
var silhouettes = map[string]*ebiten.Image{}

func drawSilhouette(dst *ebiten.Image, p battle.Participant, x, y float64) {
	img := silhouettes[p.ID]
	if img == nil {
		sprite := ebiten.NewImage(64, 64)
		drawPortrait(sprite, p, 0, 0, 1)
		img = ebiten.NewImage(64, 64)
		img.Fill(colDim) // one flat colour, cut to the sprite's shape
		mask := &ebiten.DrawImageOptions{}
		mask.Blend = ebiten.BlendDestinationIn
		img.DrawImage(sprite, mask)
		silhouettes[p.ID] = img
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleAlpha(0.6)
	dst.DrawImage(img, op)
}

// ShowRecordsForShot opens the Records tab (tools/uishot).
func (b *BattleSetup) ShowRecordsForShot() { b.records = true }
