package ui

import (
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type helpBinding struct {
	Group string

	Keys string

	Hint string

	Desc string

	Off bool

	Explained bool
}

type helpSection struct {
	Name string
	Rows []helpBinding
}

func helpSections(rows []helpBinding) []helpSection {
	var out []helpSection
	for _, b := range rows {
		if n := len(out); n > 0 && out[n-1].Name == b.Group {
			out[n-1].Rows = append(out[n-1].Rows, b)
			continue
		}
		out = append(out, helpSection{Name: b.Group, Rows: []helpBinding{b}})
	}
	return out
}

func hintLineOf(rows []helpBinding) string {
	parts := make([]string, 0, len(rows))
	for _, b := range rows {
		if b.Hint == "" || b.Off {
			continue
		}
		parts = append(parts, b.Hint)
	}
	return strings.Join(parts, "   ")
}

func helpKeyPressed() bool {
	return inpututil.IsKeyJustPressed(ebiten.KeyF1) || inpututil.IsKeyJustPressed(ebiten.KeySlash)
}

func helpDismissed(p pointer) bool {
	return helpKeyPressed() || inpututil.IsKeyJustPressed(ebiten.KeyEscape) || p.pressed
}

// The help overlay (REDESIGN §8): a notched surface card, the groups in
// two columns, each row the keys as keycaps and the description in UI 14,
// wrapped rather than cut. Generated from each screen's binding table.

const (
	helpPad     = sp6
	helpColGap  = sp6
	helpKeysW   = 150.0
	helpTopY    = helpPad + sizeDisplayM + sp4 // under the title
	helpRowGap  = 4.0
	helpHeadH   = sizeDisplayS + sp2
	helpGroupH  = sp2
	helpLineH   = uiTextH
	helpKeyH    = 20.0
	helpFootGap = sp3
)

func helpCard() rect {
	return rect{sp6, sp4, screenW - 2*sp6, screenH - 2*sp4}
}

func helpColW() float64 { return (helpCard().w - 2*helpPad - helpColGap) / 2 }

func helpDescW() float64 { return helpColW() - helpKeysW - sp3 }

const (
	editorHelpFootnote   = "rest the cursor on any toolbar symbol and it names itself, and gives its key"
	browserHelpFootnote  = "tab and the arrow keys move between the song lists; the wheel scrolls the list under the cursor"
	practiceHelpFootnote = "tracks:  click a track to mute it    right-click to solo    YOU marks your track    tab switches it"
)

// helpKeycaps splits a key label into its keys: "left / right" is two
// caps, "? or F1" two, "shift+drag, edges" two.
func helpKeycaps(keys string) []string {
	var out []string
	for _, a := range strings.Split(keys, " / ") {
		for _, b := range strings.Split(a, " or ") {
			for _, c := range strings.Split(b, ",") {
				if c = strings.TrimSpace(c); c != "" {
					out = append(out, c)
				}
			}
		}
	}
	return out
}

func helpKeycapW(k string) float64 { return math.Max(helpKeyH, textWScaled(k, dispS)+2*sp2) }

// helpKeysFit reports whether a row's keycaps fit the key column; if not
// the keys are written as text.
func helpKeysFit(keys string) bool {
	w := 0.0
	for _, k := range helpKeycaps(keys) {
		w += helpKeycapW(k) + sp1
	}
	return w-sp1 <= helpKeysW
}

// helpPlaced is one row or group header at its place on the card.
type helpPlaced struct {
	head  string
	b     helpBinding
	x, y  float64
	lines []string
}

type helpFlow struct {
	items    []helpPlaced
	footY    float64
	dismissY float64
	bottom   float64
}

func helpRowLines(b helpBinding) []string { return wrapTextW(overlayDesc(b), helpDescW()) }

func helpRowH(b helpBinding) float64 {
	keys := helpKeyH
	if !helpKeysFit(b.Keys) {
		keys = float64(len(helpKeyLines(b.Keys))) * helpLineH
	}
	return math.Max(keys, float64(len(helpRowLines(b)))*helpLineH) + helpRowGap
}

// helpKeyLines wraps a key label too long for keycaps within the key column.
func helpKeyLines(keys string) []string {
	return wrapTextW(strings.ReplaceAll(keys, " · ", " "), helpKeysW)
}

// helpLayout places the groups in two columns, splitting near the middle
// of the total height; a group never splits.
func helpLayout(rows []helpBinding, footnote string) helpFlow {
	card := helpCard()
	secs := helpSections(rows)
	heights := make([]float64, len(secs))
	total := 0.0
	for i, g := range secs {
		heights[i] = helpHeadH + helpGroupH
		for _, b := range g.Rows {
			heights[i] += helpRowH(b)
		}
		total += heights[i]
	}
	var f helpFlow
	top := card.y + helpTopY
	x := card.x + helpPad
	y, run, colBottom := top, 0.0, top
	for i, g := range secs {
		if run > 0 && run+heights[i]/2 > total/2 && x == card.x+helpPad {
			x, y = x+helpColW()+helpColGap, top
		}
		f.items = append(f.items, helpPlaced{head: g.Name, x: x, y: y})
		y += helpHeadH
		for _, b := range g.Rows {
			f.items = append(f.items, helpPlaced{b: b, x: x, y: y, lines: helpRowLines(b)})
			y += helpRowH(b)
		}
		y += helpGroupH
		run += heights[i]
		colBottom = math.Max(colBottom, y)
	}
	y = colBottom
	f.footY = -1
	if footnote != "" {
		f.footY = y + helpFootGap
		y += helpFootGap + uiTextH
	}
	f.dismissY = math.Max(card.y+card.h-helpPad-uiTextH, y+sp2)
	f.bottom = f.dismissY + uiTextH
	return f
}

func overlayDesc(b helpBinding) string {
	if b.Off && !b.Explained {
		return b.Desc + "  (not available now)"
	}
	return b.Desc
}

func drawHelpOverlay(dst *ebiten.Image, title string, rows []helpBinding, footnote string) {
	vector.FillRect(dst, 0, 0, float32(screenW), float32(screenH), colHelpDim, false)
	card := helpCard()
	notchedFill(dst, card, colPanel)
	notchedStroke(dst, card, 1, colPanelEdge)
	drawTextScaled(dst, titleCase(strings.ToLower(title)), card.x+helpPad, card.y+helpPad, dispM, colNote)

	f := helpLayout(rows, footnote)
	for _, it := range f.items {
		if it.head != "" {
			drawSectionHeader(dst, it.x, it.y, 0, strings.ToUpper(it.head), 0)
			continue
		}
		b := it.b
		col, kcol := colNote, colDim
		if b.Off {
			col, kcol = colDisabled, colDisabled
		}
		if helpKeysFit(b.Keys) {
			kx := it.x
			for _, k := range helpKeycaps(b.Keys) {
				w := helpKeycapW(k)
				notchedStroke(dst, rect{kx, it.y, w, helpKeyH}, 1, colPanelEdge)
				drawTextScaled(dst, k, kx+(w-textWScaled(k, dispS))/2, it.y+(helpKeyH-sizeDisplayS)/2-1, dispS, kcol)
				kx += w + sp1
			}
		} else {
			for j, l := range helpKeyLines(b.Keys) {
				drawText(dst, l, it.x, it.y+1+float64(j)*helpLineH, kcol)
			}
		}
		for j, l := range it.lines {
			drawText(dst, l, it.x+helpKeysW+sp3, it.y+1+float64(j)*helpLineH, col)
		}
	}
	if footnote != "" {
		drawTextSmall(dst, footnote, card.x+helpPad, f.footY, colDim)
	}
	drawTextSmall(dst, "Esc, F1, ? or a click closes this", card.x+helpPad, f.dismissY, colBarline)
}
