package ui

import (
	"fmt"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The import-warnings list: every warning the importer raised for this
// piece, numbered and scrollable (key I, or a click on the banner).

const (
	wlCardW  = 940.0
	wlCardH  = 560.0
	wlHeadH  = 52.0
	wlFootH  = 40.0
	wlPadX   = 22.0
	wlNumW   = 40.0
	wlLineH  = 20.0
	wlWheelN = 3 // lines per wheel notch
)

type warnListUI struct {
	importWarns []string
	warnList    overlay
	warnScroll  int // first visible line
}

// SetImportWarnings hands the practice view every warning the importer
// raised, so the banner can offer the full list.
func (a *App) SetImportWarnings(w []string) { a.importWarns = w }

type wlLine struct {
	num  string // "12." on a warning's first line, else ""
	text string
}

func (a *App) warnListCard() rect {
	return rect{(screenW - wlCardW) / 2, (screenH - wlCardH) / 2, wlCardW, wlCardH}
}

func (a *App) warnListLines() []wlLine {
	var out []wlLine
	w := wlCardW - 2*wlPadX - wlNumW
	for i, warn := range a.warnItems() {
		for j, l := range wrapTextW(warn, w) {
			line := wlLine{text: l}
			if j == 0 {
				line.num = strconv.Itoa(i+1) + "."
			}
			out = append(out, line)
		}
	}
	return out
}

// warnListRows is how many lines fit, counting each gap as part of a line.
func (a *App) warnListRows() int {
	avail := wlCardH - wlHeadH - wlFootH
	return int(avail / wlLineH)
}

func (a *App) toggleWarnList() {
	if len(a.warnItems()) == 0 {
		a.setBPMMessage("this piece imported without warnings")
		return
	}
	a.warnList.set(!a.warnList.open)
}

func (a *App) scrollWarnList(d int) {
	maxTop := len(a.warnListLines()) - a.warnListRows()
	a.warnScroll += d
	if a.warnScroll > maxTop {
		a.warnScroll = maxTop
	}
	if a.warnScroll < 0 {
		a.warnScroll = 0
	}
}

func (a *App) updateWarnList() {
	page := a.warnListRows() - 1
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyI), inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		a.toggleWarnList()
		return
	case inpututil.IsKeyJustPressed(ebiten.KeyUp):
		a.scrollWarnList(-1)
	case inpututil.IsKeyJustPressed(ebiten.KeyDown):
		a.scrollWarnList(1)
	case inpututil.IsKeyJustPressed(ebiten.KeyPageUp):
		a.scrollWarnList(-page)
	case inpututil.IsKeyJustPressed(ebiten.KeyPageDown), inpututil.IsKeyJustPressed(ebiten.KeySpace):
		a.scrollWarnList(page)
	case inpututil.IsKeyJustPressed(ebiten.KeyHome):
		a.warnScroll = 0
	case inpututil.IsKeyJustPressed(ebiten.KeyEnd):
		a.scrollWarnList(len(a.warnListLines()))
	}
	a.warnListMouse(a.ptr)
}

func (a *App) warnListMouse(p pointer) {
	if p.wheel != 0 {
		a.scrollWarnList(-int(p.wheel) * wlWheelN)
	}
	if p.pressed && !p.over(a.warnListCard()) {
		a.toggleWarnList() // a click on the dimmed screen closes the list
	}
}

func (a *App) drawWarnList(screen *ebiten.Image) {
	card := a.warnListCard()
	a.warnList.draw(screen, card, func(c *ebiten.Image) { a.paintWarnList(c, card) })
}

func (a *App) paintWarnList(c *ebiten.Image, card rect) {
	drawPanel(c, card, colPanel, colPanelEdge)
	n := len(a.warnItems())
	drawSectionHeader(c, card.x+wlPadX, card.y+16, 0, fmt.Sprintf("WARNINGS (%d)", n), 0)
	drawTextRight(c, "I or esc closes", card.x+card.w-wlPadX, card.y+16, colDim)

	lines := a.warnListLines()
	rows := a.warnListRows()
	y := card.y + wlHeadH
	for i := a.warnScroll; i < len(lines) && i < a.warnScroll+rows; i++ {
		l := lines[i]
		if l.num != "" {
			drawTextRight(c, l.num, card.x+wlPadX+wlNumW-10, y, colDim)
		}
		drawText(c, l.text, card.x+wlPadX+wlNumW, y, colHUD)
		y += wlLineH
	}

	foot := "these are what the importer changed or skipped; the piece still plays"
	if len(lines) > rows {
		last := a.warnScroll + rows
		if last > len(lines) {
			last = len(lines)
		}
		foot = fmt.Sprintf("lines %d-%d of %d   wheel, arrows or page up/down to scroll", a.warnScroll+1, last, len(lines))
	}
	drawText(c, foot, centreX(foot, card.x, card.w), card.y+card.h-wlFootH+12, colDim)
}

// warnItems is every warning: the banner's conditions, then the importer's.
func (a *App) warnItems() []string {
	return append(a.bannerItems(), a.importWarns...)
}
