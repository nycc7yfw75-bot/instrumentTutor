package ui

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The update offer (ONLINE.md §A6): shown over the landing page only.
// "Headliner 1.3.0 is available", the notes, and Update now · Later ·
// Skip this version. Update now downloads, checks and unpacks with a
// progress bar, then Headliner restarts into the new version.

// UpdateOffer is one available update and what to do with it.
type UpdateOffer struct {
	Version string
	Notes   string
	// Install downloads, verifies, backs up and unpacks, reporting
	// progress; it runs off the UI thread.
	Install func(progress func(step string, frac float64)) error
	// Restart hands over to the new version and quits.
	Restart func()
	// Skip remembers not to offer this version again.
	Skip func()
}

type upPhase int

const (
	upOffer upPhase = iota
	upWorking
	upFailed
)

// UpdatePrompt is the offer dialog.
type UpdatePrompt struct {
	sh    *Shell
	offer UpdateOffer
	ptr   pointer
	focus int // 0 Update now, 1 Later, 2 Skip

	mu    sync.Mutex
	phase upPhase
	step  string
	frac  float64
	err   string
	done  bool
}

// NewUpdatePrompt makes the dialog.
func NewUpdatePrompt(sh *Shell, o UpdateOffer) *UpdatePrompt { return &UpdatePrompt{sh: sh, offer: o} }

type upLayout struct {
	card                rect
	notesY              float64
	update, later, skip rect
	barY                float64
	notes               []string
}

func (p *UpdatePrompt) layout() upLayout {
	var l upLayout
	w := math.Min(screenW-2*pgMargin, 620)
	l.notes = p.noteLines(w - 2*sp6)
	h := sp4 + sizeDisplayM + sp3 + float64(len(l.notes))*(uiTextH+4) + sp6 + btnHPrimary + sp6
	l.card = rect{math.Round((screenW - w) / 2), math.Round((screenH - h) / 2), w, h}
	l.notesY = l.card.y + sp4 + sizeDisplayM + sp3
	by := l.card.y + l.card.h - sp6 - btnHPrimary
	uw := kitButtonW("Update now", kitPrimary, false)
	l.update = rect{l.card.x + l.card.w - sp6 - uw, by, uw, btnHPrimary}
	lw := kitButtonW("Later", kitSecondary, false)
	l.later = rect{l.update.x - sp3 - lw, by + (btnHPrimary-btnH)/2, lw, btnH}
	sw := kitButtonW("Skip this version", kitSecondary, false)
	l.skip = rect{l.card.x + sp6, by + (btnHPrimary-btnH)/2, sw, btnH}
	l.barY = by + btnHPrimary/2 - 4
	return l
}

// noteLines are the release notes, wrapped, at most 8 lines.
func (p *UpdatePrompt) noteLines(w float64) []string {
	var out []string
	for _, para := range strings.Split(strings.ReplaceAll(p.offer.Notes, "\r", ""), "\n") {
		para = strings.TrimSpace(strings.TrimLeft(para, "#"))
		if para == "" {
			continue
		}
		out = append(out, wrapTextW(para, w)...)
	}
	if len(out) > 8 {
		out = append(out[:7], "…and more on the Releases page.")
	}
	if len(out) == 0 {
		out = []string{"Bug fixes and improvements."}
	}
	return out
}

func (p *UpdatePrompt) state() (upPhase, string, float64, string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.phase, p.step, p.frac, p.err, p.done
}

func (p *UpdatePrompt) start() {
	p.mu.Lock()
	p.phase, p.step, p.frac, p.err = upWorking, "Downloading", 0, ""
	p.mu.Unlock()
	go func() {
		err := p.offer.Install(func(step string, frac float64) {
			p.mu.Lock()
			p.step, p.frac = step, frac
			p.mu.Unlock()
		})
		p.mu.Lock()
		defer p.mu.Unlock()
		if err != nil {
			p.phase, p.err = upFailed, err.Error()
			return
		}
		p.done = true
	}()
}

func (p *UpdatePrompt) Update() error {
	p.ptr = readPointer()
	l := p.layout()
	phase, _, _, _, done := p.state()
	switch phase {
	case upWorking:
		if done && p.offer.Restart != nil {
			p.offer.Restart()
		}
		return nil // nothing to press while it works
	case upFailed:
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
			(p.ptr.pressed && p.ptr.over(l.later)) {
			return errQuit
		}
		return nil
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		return errQuit // Later
	case inpututil.IsKeyJustPressed(ebiten.KeyTab), inpututil.IsKeyJustPressed(ebiten.KeyRight):
		p.focus = (p.focus + 1) % 3
	case inpututil.IsKeyJustPressed(ebiten.KeyLeft):
		p.focus = (p.focus + 2) % 3
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter):
		return p.choose(p.focus)
	}
	if p.ptr.pressed {
		switch {
		case p.ptr.over(l.update):
			return p.choose(0)
		case p.ptr.over(l.later):
			return p.choose(1)
		case p.ptr.over(l.skip):
			return p.choose(2)
		}
	}
	return nil
}

func (p *UpdatePrompt) choose(i int) error {
	switch i {
	case 0:
		if p.offer.Install != nil {
			p.start()
		}
		return nil
	case 2:
		if p.offer.Skip != nil {
			p.offer.Skip()
		}
	}
	return errQuit
}

func (p *UpdatePrompt) Draw(dst *ebiten.Image) {
	vector.FillRect(dst, 0, 0, float32(screenW), float32(screenH), withAlpha(colBG, 0.8), false)
	l := p.layout()
	drawDialogCard(dst, l.card, fmt.Sprintf("Headliner %s is available", strings.TrimPrefix(p.offer.Version, "v")))
	phase, step, frac, errMsg, _ := p.state()
	y := l.notesY
	for _, line := range l.notes {
		drawText(dst, line, l.card.x+sp6, y, colDim)
		y += uiTextH + 4
	}
	switch phase {
	case upOffer:
		drawKitButton(dst, l.skip, "Skip this version", -1, kitSecondary, btnState{hover: p.ptr.over(l.skip), focus: p.focus == 2})
		drawKitButton(dst, l.later, "Later", -1, kitSecondary, btnState{hover: p.ptr.over(l.later), focus: p.focus == 1})
		drawKitButton(dst, l.update, "Update now", -1, kitPrimary, btnState{hover: p.ptr.over(l.update), focus: p.focus == 0})
	case upWorking:
		x, w := l.card.x+sp6, l.card.w-2*sp6
		label := step + "…"
		if step == "Downloading" {
			label = fmt.Sprintf("Downloading… %d%%", int(math.Round(frac*100)))
		}
		drawText(dst, label, x, l.barY-uiTextH-sp2, colNote)
		vector.FillRect(dst, float32(x), float32(l.barY), float32(w), 8, colHover, false)
		vector.FillRect(dst, float32(x), float32(l.barY), float32(math.Round(w*math.Max(0, math.Min(1, frac)))), 8, tokBrand, false)
	case upFailed:
		drawText(dst, truncateW("The update didn't install: "+errMsg, l.card.w-2*sp6), l.card.x+sp6, l.barY-uiTextH, colClose)
		drawText(dst, "Your saves are untouched. Try again later from Settings → About & updates.", l.card.x+sp6, l.barY+4, colDim)
		drawKitButton(dst, l.later, "Close", -1, kitSecondary, btnState{hover: p.ptr.over(l.later), focus: true})
	}
}
