package ui

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/S95F/musicTutor/internal/progress"
)

// NamePrompt asks for the player's name: the first launch's question, and
// the Settings rename. Enter saves, Esc leaves it for later.
type NamePrompt struct {
	sh    *Shell
	store *progress.Store
	name  string
	err   string
	ptr   pointer
	anim  animator
	clock float64
}

const nameMaxRunes = 20

// NewNamePrompt opens the prompt, filled with the current name if there is one.
func NewNamePrompt(sh *Shell, store *progress.Store) *NamePrompt {
	p := &NamePrompt{sh: sh, store: store}
	if store != nil {
		if name, _, ok := store.Active(); ok {
			p.name = name
		}
	}
	return p
}

func (p *NamePrompt) save() error {
	if p.store == nil {
		return errQuit
	}
	name := strings.TrimSpace(p.name)
	if name == "" {
		p.err = "Type a name first, or press Esc to do this later"
		return nil
	}
	var err error
	if p.store.NeedsProfile() {
		_, err = p.store.CreateProfile(name)
	} else {
		err = p.store.Rename(name)
	}
	if err != nil {
		p.err = "Could not save: " + err.Error()
		return nil
	}
	return errQuit
}

func backspaceHeld() bool {
	d := inpututil.KeyPressDuration(ebiten.KeyBackspace)
	return d == 1 || (d > 30 && d%3 == 0)
}

func (p *NamePrompt) Update() error {
	p.ptr = readPointer()
	p.clock += uiFrameSeconds()
	for _, r := range ebiten.AppendInputChars(nil) {
		if r < 32 || utf8.RuneCountInString(p.name) >= nameMaxRunes {
			continue
		}
		p.name += string(r)
		p.err = ""
	}
	if backspaceHeld() && p.name != "" {
		_, n := utf8.DecodeLastRuneInString(p.name)
		p.name = p.name[:len(p.name)-n]
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape), escButtonPressed(escBackChip, p.ptr):
		return errQuit
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter), inpututil.IsKeyJustPressed(ebiten.KeyKPEnter):
		return p.save()
	case p.ptr.pressed && p.ptr.over(p.saveRect()):
		return p.save()
	}
	return nil
}

// nameCard is the prompt's card, centred on the screen (REDESIGN §8).
func nameCard() rect {
	return rect{math.Round((screenW - 480) / 2), math.Round(screenH/2 - 120), 480, 236}
}

func (p *NamePrompt) fieldRect() rect {
	c := nameCard()
	return rect{c.x + sp6, c.y + sp4 + sizeDisplayM + sp3 + uiTextH + sp3, c.w - 2*sp6, 40}
}

func (p *NamePrompt) saveRect() rect {
	c := nameCard()
	w := kitButtonW("Save", kitPrimary, false)
	return rect{c.x + c.w - sp6 - w, c.y + c.h - sp4 - btnHPrimary, w, btnHPrimary}
}

func (p *NamePrompt) Draw(dst *ebiten.Image) {
	dst.Fill(colBG)
	drawEscButton(dst, escBackChip, &p.anim, p.ptr)

	c := nameCard()
	y := drawDialogCard(dst, c, "Your name")
	drawText(dst, "What should we call you? Your progress is saved under it.", c.x+sp6, y, colDim)
	f := p.fieldRect()
	drawField(dst, f, p.name, 0, int(p.clock*2)%2 == 0) // the caret blinks once a second
	if p.err != "" {
		drawText(dst, p.err, f.x, f.y+f.h+sp2, colClose)
	}
	drawTextSmall(dst, "Enter saves · Esc asks again later", c.x+sp6, c.y+c.h-sp4-btnHPrimary/2-8, colBarline)
	s := p.saveRect()
	over := p.ptr.over(s)
	drawKitButton(dst, s, "Save", -1, kitPrimary, btnState{hover: over, press: over && p.ptr.down})
}
