package ui

import (
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/progress"
	"github.com/S95F/musicTutor/internal/scoring"
)

// The Profiles screen (BATTLES.md §3): every local player — create,
// rename, delete (asking first), and choose who is playing.

// ProfilesScreen lists the profiles.
type ProfilesScreen struct {
	sh   *Shell
	st   *progress.Store
	ptr  pointer
	anim animator

	sel   int
	entry *nameEntry // a name being typed (new or rename)

	confirmDelete bool
	confirmFocus  int // 0 Cancel (the default), 1 Delete profile
	err           string
	clock         float64
}

type nameEntry struct {
	id, name string // id "" = a new profile
}

// NewProfilesScreen opens the profiles.
func NewProfilesScreen(sh *Shell, st *progress.Store) *ProfilesScreen {
	return &ProfilesScreen{sh: sh, st: st}
}

func (p *ProfilesScreen) profiles() []progress.ProfileInfo {
	if p.st == nil {
		return nil
	}
	return p.st.Profiles()
}

type profilesLayout struct {
	rows                      []rect
	newBtn, renameBtn, delBtn rect
	useBtn                    rect
	card, cancel, ok, field   rect
}

func (p *ProfilesScreen) layout() profilesLayout {
	var l profilesLayout
	w := math.Min(screenW-2*pgMargin, 760)
	x := math.Round((screenW - w) / 2)
	y := float64(zHeaderH) + sp6
	for range p.profiles() {
		l.rows = append(l.rows, rect{x, y, w, 64})
		y += 64 + sp1
	}
	by := y + sp4
	nw := kitButtonW("New profile", kitSecondary, true)
	l.newBtn = rect{x, by, nw, btnH}
	uw := kitButtonW("Play as this", kitPrimary, false)
	l.useBtn = rect{x + w - uw, by - (btnHPrimary-btnH)/2, uw, btnHPrimary}
	rw := kitButtonW("Rename", kitSecondary, false)
	l.renameBtn = rect{l.useBtn.x - sp3 - rw, by, rw, btnH}
	dw := kitButtonW("Delete", kitDestructive, false)
	l.delBtn = rect{l.renameBtn.x - sp3 - dw, by, dw, btnH}
	l.card = rect{math.Round((screenW - 520) / 2), math.Round((screenH - 220) / 2), 520, 220}
	l.field = rect{l.card.x + sp6, l.card.y + sp4 + sizeDisplayM + sp4, l.card.w - 2*sp6, 40}
	okLabel := "Save"
	if p.confirmDelete {
		okLabel = "Delete profile"
	}
	ow := kitButtonW(okLabel, kitDestructive, false)
	cw := kitButtonW("Cancel", kitSecondary, false)
	cy := l.card.y + l.card.h - sp4 - btnH
	l.ok = rect{l.card.x + l.card.w - sp6 - ow, cy, ow, btnH}
	l.cancel = rect{l.ok.x - sp3 - cw, cy, cw, btnH}
	return l
}

func (p *ProfilesScreen) Update() error {
	p.ptr = readPointer()
	p.clock += uiFrameSeconds()
	ps := p.profiles()
	p.sel = max(0, min(len(ps)-1, p.sel))
	l := p.layout()
	switch {
	case p.entry != nil:
		p.updateEntry(l)
		return nil
	case p.confirmDelete:
		p.updateDelete(l, ps)
		return nil
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape), escButtonPressed(escBackChip, p.ptr):
		return errQuit
	case inpututil.IsKeyJustPressed(ebiten.KeyUp):
		p.sel = max(0, p.sel-1)
	case inpututil.IsKeyJustPressed(ebiten.KeyDown):
		p.sel = min(len(ps)-1, p.sel+1)
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter):
		p.use(ps)
	case inpututil.IsKeyJustPressed(ebiten.KeyN):
		p.entry, p.err = &nameEntry{}, ""
	case inpututil.IsKeyJustPressed(ebiten.KeyR) && len(ps) > 0:
		p.entry, p.err = &nameEntry{id: ps[p.sel].ID, name: ps[p.sel].Name}, ""
	case inpututil.IsKeyJustPressed(ebiten.KeyDelete) && len(ps) > 0:
		p.confirmDelete, p.confirmFocus = true, 0
	}
	if !p.ptr.pressed {
		return nil
	}
	for i, r := range l.rows {
		if p.ptr.over(r) {
			if p.sel == i {
				p.use(ps)
			}
			p.sel = i
		}
	}
	switch {
	case p.ptr.over(l.newBtn):
		p.entry, p.err = &nameEntry{}, ""
	case len(ps) > 0 && p.ptr.over(l.renameBtn):
		p.entry, p.err = &nameEntry{id: ps[p.sel].ID, name: ps[p.sel].Name}, ""
	case len(ps) > 0 && p.ptr.over(l.delBtn):
		p.confirmDelete, p.confirmFocus = true, 0
	case p.ptr.over(l.useBtn):
		p.use(ps)
	}
	return nil
}

func (p *ProfilesScreen) use(ps []progress.ProfileInfo) {
	if p.sel < len(ps) {
		if err := p.st.SetActive(ps[p.sel].ID); err != nil {
			p.err = err.Error()
		}
	}
}

func (p *ProfilesScreen) updateEntry(l profilesLayout) {
	e := p.entry
	for _, r := range ebiten.AppendInputChars(nil) {
		if r >= 32 && utf8.RuneCountInString(e.name) < nameMaxRunes {
			e.name += string(r)
		}
	}
	if backspaceHeld() && e.name != "" {
		_, n := utf8.DecodeLastRuneInString(e.name)
		e.name = e.name[:len(e.name)-n]
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape), p.ptr.pressed && p.ptr.over(l.cancel):
		p.entry = nil
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter), p.ptr.pressed && p.ptr.over(l.ok):
		p.saveEntry()
	}
}

// saveEntry adds or renames a profile from the name typed.
func (p *ProfilesScreen) saveEntry() {
	e := p.entry
	var err error
	if e.id == "" {
		_, err = p.st.AddProfile(e.name)
	} else {
		err = p.st.RenameProfile(e.id, e.name)
	}
	if err != nil {
		p.err = err.Error()
		return
	}
	p.entry, p.err = nil, ""
}

func (p *ProfilesScreen) updateDelete(l profilesLayout, ps []progress.ProfileInfo) {
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		p.confirmDelete = false
	case inpututil.IsKeyJustPressed(ebiten.KeyTab), inpututil.IsKeyJustPressed(ebiten.KeyLeft), inpututil.IsKeyJustPressed(ebiten.KeyRight):
		p.confirmFocus = 1 - p.confirmFocus
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter):
		if p.confirmFocus == 1 {
			p.deleteSelected(ps)
		}
		p.confirmDelete = false
	case p.ptr.pressed && p.ptr.over(l.cancel):
		p.confirmDelete = false
	case p.ptr.pressed && p.ptr.over(l.ok):
		p.deleteSelected(ps)
		p.confirmDelete = false
	}
}

func (p *ProfilesScreen) deleteSelected(ps []progress.ProfileInfo) {
	if p.sel < len(ps) {
		if err := p.st.DeleteProfile(ps[p.sel].ID); err != nil {
			p.err = err.Error()
		}
	}
}

func (p *ProfilesScreen) Draw(dst *ebiten.Image) {
	dst.Fill(colBG)
	p.anim.tick()
	l := p.layout()
	x := drawEscButton(dst, escBackChip, &p.anim, p.ptr)
	drawTextScaled(dst, "Profiles", x, 14, dispL, colNote)
	drawText(dst, "Everyone who plays on this laptop", x+textWScaled("Profiles", dispL)+sp4, 24, colDim)
	ps := p.profiles()
	for i, pi := range ps {
		drawProfileRow(dst, l.rows[i], pi, i == p.sel, p.ptr.over(l.rows[i]))
	}
	if len(ps) == 0 {
		drawText(dst, "No profiles yet. Add one to keep your progress.", l.newBtn.x, l.newBtn.y-uiTextH-sp4, colDim)
	}
	drawKitButton(dst, l.newBtn, "New profile", icNew, kitSecondary, btnState{hover: p.ptr.over(l.newBtn)})
	if len(ps) > 0 {
		drawKitButton(dst, l.delBtn, "Delete", -1, kitDestructive, btnState{hover: p.ptr.over(l.delBtn)})
		drawKitButton(dst, l.renameBtn, "Rename", -1, kitSecondary, btnState{hover: p.ptr.over(l.renameBtn)})
		drawKitButton(dst, l.useBtn, "Play as this", -1, kitPrimary, btnState{hover: p.ptr.over(l.useBtn), disabled: ps[p.sel].Active})
	}
	hint := "↑ ↓ choose · Enter play as · N new · R rename · Del delete · Esc back"
	drawTextSmall(dst, hint, l.newBtn.x, l.newBtn.y+btnHPrimary+sp3, colBarline)
	if p.err != "" {
		drawText(dst, p.err, l.newBtn.x, l.newBtn.y+btnHPrimary+sp3+uiTextH, colClose)
	}
	switch {
	case p.entry != nil:
		vector.FillRect(dst, 0, 0, float32(screenW), float32(screenH), colHelpDim, false)
		title := "New profile"
		if p.entry.id != "" {
			title = "Rename"
		}
		drawDialogCard(dst, l.card, title)
		drawField(dst, l.field, p.entry.name, 0, int(p.clock*2)%2 == 0)
		drawKitButton(dst, l.cancel, "Cancel", -1, kitSecondary, btnState{hover: p.ptr.over(l.cancel)})
		drawKitButton(dst, l.ok, "Save", -1, kitSecondary, btnState{hover: p.ptr.over(l.ok), focus: true})
	case p.confirmDelete && p.sel < len(ps):
		vector.FillRect(dst, 0, 0, float32(screenW), float32(screenH), colHelpDim, false)
		y := drawDialogCard(dst, l.card, "Delete "+ps[p.sel].Name+"?")
		msg := fmt.Sprintf("%s's level, XP, badges, career and battle record will be lost. This can't be undone.", ps[p.sel].Name)
		for i, line := range wrapTextW(msg, l.card.w-2*sp6) {
			drawText(dst, line, l.card.x+sp6, y+float64(i)*uiTextH, colDim)
		}
		drawKitButton(dst, l.cancel, "Cancel", -1, kitSecondary, btnState{hover: p.ptr.over(l.cancel), focus: p.confirmFocus == 0})
		drawKitButton(dst, l.ok, "Delete profile", -1, kitDestructive, btnState{hover: p.ptr.over(l.ok), focus: p.confirmFocus == 1})
	}
}

// drawProfileRow is a profile card (§3): a guitar in the profile's tint,
// the name, level and title, and the battle record.
func drawProfileRow(dst *ebiten.Image, r rect, pi progress.ProfileInfo, sel, over bool) {
	if sel || over {
		notchedFill(dst, r, colHover)
	} else {
		notchedFill(dst, r, colPanel)
	}
	if sel {
		vector.FillRect(dst, float32(r.x), float32(r.y), 3, float32(r.h), tokBrand, false)
	}
	drawPixelIcon(dst, icGuitar, r.x+sp4, r.y+(r.h-iconSize)/2, profileTint(pi.Tint))
	tx := r.x + sp4 + iconSize + sp3
	drawTextScaled(dst, pi.Name, tx, r.y+sp2+2, dispM, colNote)
	sub := fmt.Sprintf("LV %d · %s", pi.Level, scoring.Title(pi.Level))
	drawText(dst, sub, tx, r.y+r.h-uiTextH-sp2, colDim)
	rec := fmt.Sprintf("%dW  %dL  %dD", pi.Record.Wins, pi.Record.Losses, pi.Record.Draws)
	rx := r.x + r.w - sp4 - textWMonoSized(rec, sizeUI)
	drawMonoSized(dst, rec, rx, r.y+(r.h-16)/2, sizeUI, colDim)
	if pi.Active {
		drawPill(dst, rx-sp4-pillW("PLAYING"), r.y+(r.h-pillH)/2, "PLAYING", pillYou)
	}
}
