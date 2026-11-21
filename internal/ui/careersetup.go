package ui

import (
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/career"
	"github.com/S95F/musicTutor/internal/progress"
)

// The career setup (CAREER.md §5.1): on first launch, the player's name,
// hometown and guitarist; later, from Settings, the hometown and guitarist
// again (a changed hometown asks first: it renames the route's cities).

// CareerSetup is the setup screen.
type CareerSetup struct {
	sh    *Shell
	st    *progress.Store
	ptr   pointer
	anim  animator
	clock float64

	name    string
	city    int // index into career.Cities
	char    int
	focus   int // 0 name, 1 hometown, 2 guitarist
	editing bool
	err     string

	origCity     string
	confirm      bool
	confirmFocus int // 0 Cancel (the default), 1 Change hometown

	preview *ebiten.Image
}

const (
	setupName = iota
	setupCity
	setupChar
	setupFocusCount
)

// NewCareerSetup opens the setup for the active profile (or a new one).
func NewCareerSetup(sh *Shell, st *progress.Store) *CareerSetup {
	c := &CareerSetup{sh: sh, st: st, name: "Player 1"}
	if st != nil {
		if n, _, ok := st.Active(); ok {
			c.name = n
		}
		cs := st.Career()
		c.editing = cs.Started()
		c.char = max(0, min(len(career.Characters)-1, cs.Character))
		for i, city := range career.Cities {
			if city.Name == cs.Hometown {
				c.city = i
			}
		}
		c.origCity = cs.Hometown
	}
	return c
}

type setupLayout struct {
	field  rect
	cities []rect
	chars  []rect
	start  rect
	// The confirm dialog.
	card, cancel, ok rect
}

func (c *CareerSetup) layout() setupLayout {
	var l setupLayout
	w := math.Min(screenW-2*pgMargin, 1100)
	x0 := math.Round((screenW - w) / 2)
	top := float64(pgMargin) + zItemH + sp6 + sizeDisplayL + sp6
	leftW := math.Round(w * 0.38)
	l.field = rect{x0, top + sizeDisplayS + sp2, leftW, 40}
	cy := l.field.y + l.field.h + sp6 + sizeDisplayS + sp2
	cw := (leftW - 3*sp2) / 4
	for i := range career.Characters {
		l.chars = append(l.chars, rect{x0 + float64(i)*(cw+sp2), cy, cw, 120})
	}
	rx := x0 + leftW + sp8
	rw := x0 + w - rx
	ry := top + sizeDisplayS + sp2
	const labelW, rowH = 120.0, 36.0
	region, row, col := "", -1, 0
	for _, city := range career.Cities {
		if city.Region != region {
			region, row, col = city.Region, row+1, 0
		}
		chipW := (rw - labelW - 3*sp1) / 4
		l.cities = append(l.cities, rect{rx + labelW + float64(col)*(chipW+sp1), ry + float64(row)*rowH, chipW, rowH - sp1})
		col++
	}
	label := "Start my career"
	if c.editing {
		label = "Save"
	}
	bw := kitButtonW(label, kitPrimary, false)
	l.start = rect{x0 + w - bw, screenH - pgMargin - btnHPrimary, bw, btnHPrimary}
	l.card = rect{math.Round((screenW - 520) / 2), math.Round((screenH - 200) / 2), 520, 200}
	cancelW := kitButtonW("Cancel", kitSecondary, false)
	okW := kitButtonW("Change hometown", kitSecondary, false)
	by := l.card.y + l.card.h - sp4 - btnH
	l.ok = rect{l.card.x + l.card.w - sp6 - okW, by, okW, btnH}
	l.cancel = rect{l.ok.x - sp3 - cancelW, by, cancelW, btnH}
	return l
}

func (c *CareerSetup) Update() error {
	c.ptr = readPointer()
	c.clock += uiFrameSeconds()
	if c.confirm {
		return c.updateConfirm()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || escButtonPressed(escBackChip, c.ptr) {
		return errQuit // later: nothing is saved
	}
	shift := ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyTab):
		d := 1
		if shift {
			d = setupFocusCount - 1
		}
		c.focus = (c.focus + d) % setupFocusCount
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter), inpututil.IsKeyJustPressed(ebiten.KeyKPEnter):
		return c.trySave()
	}
	switch c.focus {
	case setupName:
		c.typeName()
	case setupCity:
		c.city = stepIndex2(c.city, len(career.Cities), arrowStep(4))
	case setupChar:
		c.char = stepIndex2(c.char, len(career.Characters), arrowStep(0))
	}
	return c.handleMouse(c.layout())
}

// arrowStep reads the arrow keys: left/right step 1, up/down step rows
// (0 means up/down do nothing).
func arrowStep(rows int) int {
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyLeft):
		return -1
	case inpututil.IsKeyJustPressed(ebiten.KeyRight):
		return 1
	case rows > 0 && inpututil.IsKeyJustPressed(ebiten.KeyUp):
		return -rows
	case rows > 0 && inpututil.IsKeyJustPressed(ebiten.KeyDown):
		return rows
	}
	return 0
}

func stepIndex2(i, n, d int) int { return max(0, min(n-1, i+d)) }

func (c *CareerSetup) typeName() {
	for _, r := range ebiten.AppendInputChars(nil) {
		if r < 32 || utf8.RuneCountInString(c.name) >= nameMaxRunes {
			continue
		}
		c.name += string(r)
		c.err = ""
	}
	if backspaceHeld() && c.name != "" {
		_, n := utf8.DecodeLastRuneInString(c.name)
		c.name = c.name[:len(c.name)-n]
	}
}

func (c *CareerSetup) handleMouse(l setupLayout) error {
	if !c.ptr.pressed {
		return nil
	}
	switch {
	case c.ptr.over(l.field):
		c.focus = setupName
	case c.ptr.over(l.start):
		return c.trySave()
	}
	for i, r := range l.cities {
		if c.ptr.over(r) {
			c.city, c.focus = i, setupCity
		}
	}
	for i, r := range l.chars {
		if c.ptr.over(r) {
			c.char, c.focus = i, setupChar
		}
	}
	return nil
}

// trySave saves, or asks first when an existing career changes hometown.
func (c *CareerSetup) trySave() error {
	if strings.TrimSpace(c.name) == "" {
		c.err, c.focus = "Your name can't be empty", setupName
		return nil
	}
	if c.editing && c.origCity != "" && career.Cities[c.city].Name != c.origCity {
		c.confirm, c.confirmFocus = true, 0
		return nil
	}
	return c.save()
}

func (c *CareerSetup) updateConfirm() error {
	l := c.layout()
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		c.confirm = false
	case inpututil.IsKeyJustPressed(ebiten.KeyTab), inpututil.IsKeyJustPressed(ebiten.KeyLeft), inpututil.IsKeyJustPressed(ebiten.KeyRight):
		c.confirmFocus = 1 - c.confirmFocus
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter), inpututil.IsKeyJustPressed(ebiten.KeyKPEnter):
		if c.confirmFocus == 0 {
			c.confirm = false
			return nil
		}
		return c.save()
	case c.ptr.pressed && c.ptr.over(l.cancel):
		c.confirm = false
	case c.ptr.pressed && c.ptr.over(l.ok):
		return c.save()
	}
	return nil
}

// save writes the name and the career, then closes.
func (c *CareerSetup) save() error {
	if c.st == nil {
		return errQuit
	}
	if c.st.NeedsProfile() {
		if _, err := c.st.CreateProfile(c.name); err != nil {
			c.err = err.Error()
			return nil
		}
	} else if n, _, _ := c.st.Active(); n != strings.TrimSpace(c.name) {
		if err := c.st.Rename(c.name); err != nil {
			c.err = err.Error()
			return nil
		}
	}
	city, char := career.Cities[c.city].Name, c.char
	err := c.st.UpdateCareer(func(s *career.State) {
		if !s.Started() {
			s.Setup(city, char, time.Now().UnixNano())
			return
		}
		s.Hometown, s.Character = city, char // the route keeps its seed
	})
	if err != nil {
		c.err = err.Error()
		return nil
	}
	return errQuit
}

func (c *CareerSetup) Draw(dst *ebiten.Image) {
	dst.Fill(colBG)
	l := c.layout()
	dt := motionDt()
	drawEscButton(dst, escBackChip, &c.anim, c.ptr)
	title := "Start your career"
	sub := "Who you are, where you start, and who's on stage"
	if c.editing {
		title, sub = "Your career", "Change your hometown or your guitarist"
	}
	x0 := l.field.x
	ty := float64(pgMargin) + zItemH + sp6
	drawTextScaled(dst, title, x0, ty, dispL, colNote)
	drawText(dst, sub, x0+textWScaled(title, dispL)+sp4, ty+10, colDim)

	// Name.
	drawSectionHeader(dst, l.field.x, l.field.y-sizeDisplayS-sp2, 0, "NAME", 0)
	drawField(dst, l.field, c.name, 0, c.focus == setupName && int(c.clock*2)%2 == 0)
	if c.focus == setupName {
		notchedStroke(dst, l.field, 1, tokBrand)
	}
	if c.err != "" {
		drawText(dst, c.err, l.field.x, l.field.y+l.field.h+2, colClose)
	}

	// Guitarist.
	drawSectionHeader(dst, l.chars[0].x, l.chars[0].y-sizeDisplayS-sp2, 0, "GUITARIST", 0)
	if c.preview == nil {
		c.preview = ebiten.NewImage(32, 48)
	}
	for i, r := range l.chars {
		sel := i == c.char
		over := c.ptr.over(r)
		fill := colPanel
		if over || sel {
			fill = colHover
		}
		notchedFill(dst, r, fill)
		if sel {
			vector.FillRect(dst, float32(r.x), float32(r.y), float32(r.w), 3, tokBrand, false)
		}
		if sel && c.focus == setupChar {
			notchedStroke(dst, r, 1, tokBrand)
		}
		c.preview.Clear()
		anim, f := "idle", int(c.clock*2)%2
		if sel {
			anim, f = "strum", int(c.clock*6)%4
		}
		drawSprite(c.preview, "characters/"+presetSlug(i), "guitarist", anim, f, 16, 48, 1)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(2, 2)
		op.GeoM.Translate(math.Round(r.x+(r.w-64)/2), r.y+sp2)
		dst.DrawImage(c.preview, op)
		n := career.Characters[i].Name
		col := colDim
		if sel {
			col = colNote
		}
		drawText(dst, n, math.Round(centreX(n, r.x, r.w)), r.y+r.h-uiTextH-sp1, col)
	}
	look := career.Characters[c.char].Look
	drawTextSmall(dst, look, l.chars[0].x, l.chars[0].y+l.chars[0].h+sp2, colDim)

	// Hometown.
	cityTop := l.cities[0].y
	drawSectionHeader(dst, l.cities[0].x-120, cityTop-sizeDisplayS-sp2, 0, "HOMETOWN", 0)
	region := ""
	for i, city := range career.Cities {
		r := l.cities[i]
		if city.Region != region {
			region = city.Region
			drawTextSmall(dst, region, r.x-120, r.y+(r.h-uiTextH)/2+1, colBarline)
		}
		sel, over := i == c.city, c.ptr.over(l.cities[i])
		if sel || over {
			notchedFill(dst, r, colHover)
		}
		col := colDim
		if sel {
			col = colNote
			vector.FillRect(dst, float32(r.x), float32(r.y), 3, float32(r.h), tokBrand, false)
			if c.focus == setupCity {
				notchedStroke(dst, r, 1, tokBrand)
			}
		}
		drawText(dst, truncateW(city.Name, r.w-sp4), r.x+sp3, r.y+(r.h-uiTextH)/2, col)
	}

	// Start.
	label := "Start my career"
	if c.editing {
		label = "Save"
	}
	over := c.ptr.over(l.start)
	drawKitButton(dst, l.start, label, -1, kitPrimary, btnState{hover: over, press: over && c.ptr.down})
	hint := "Tab moves between name, guitarist and hometown · arrows choose · Enter " + strings.ToLower(label)
	drawTextSmall(dst, hint, x0, l.start.y+(l.start.h-uiTextH)/2+1, colBarline)
	_ = dt

	if c.confirm {
		vector.FillRect(dst, 0, 0, float32(screenW), float32(screenH), colHelpDim, false)
		y := drawDialogCard(dst, l.card, "Change hometown?")
		for i, line := range wrapTextW("The cities on your route will be renamed to start from "+career.Cities[c.city].Name+". Your gigs, stars and Fame all stay.", l.card.w-2*sp6) {
			drawText(dst, line, l.card.x+sp6, y+float64(i)*uiTextH, colDim)
		}
		drawKitButton(dst, l.cancel, "Cancel", -1, kitSecondary, btnState{hover: c.ptr.over(l.cancel), focus: c.confirmFocus == 0})
		drawKitButton(dst, l.ok, "Change hometown", -1, kitSecondary, btnState{hover: c.ptr.over(l.ok), focus: c.confirmFocus == 1})
	}
}
