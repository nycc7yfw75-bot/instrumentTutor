package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

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

func (p *NamePrompt) fieldRect() rect {
	return rect{(screenW - 420) / 2, screenH/2 - 10, 420, uiTextH + 20}
}

func (p *NamePrompt) saveRect() rect {
	f := p.fieldRect()
	label := "Save"
	w := textW(label) + 2*settingsBtnPad
	return rect{f.x + f.w - w, f.y + f.h + 16, w, settingsBtnH}
}

func (p *NamePrompt) Draw(dst *ebiten.Image) {
	dst.Fill(colBG)
	x := drawEscButton(dst, escBackChip, &p.anim, p.ptr)
	drawHeaderAt(dst, x, "Your name", "Your progress is saved under it", colDim)

	f := p.fieldRect()
	q := "What should we call you?"
	drawText(dst, q, f.x, f.y-34, colNote)
	drawPanel(dst, f, colPanel, colPanelEdge)
	tx, ty := f.x+10, f.y+10
	drawText(dst, p.name, tx, ty, colNote)
	if int(p.clock*2)%2 == 0 { // the caret blinks once a second
		cx := float32(tx + textW(p.name) + 1)
		vector.FillRect(dst, cx, float32(ty-1), 2, uiTextH+2, colSounding, false)
	}
	if p.err != "" {
		drawText(dst, p.err, f.x, f.y+f.h+20, colClose)
	}

	s := p.saveRect()
	fill, edge, tc := colPanel, colPanelEdge, colHUD
	if p.ptr.over(s) {
		fill, edge, tc = colHover, colDim, colNote
	}
	drawPanel(dst, s, fill, edge)
	drawText(dst, "Save", centreX("Save", s.x, s.w), settingsBtnTextY(s), tc)
	drawFooter(dst, "type your name   ·   enter save   ·   esc later")
}
