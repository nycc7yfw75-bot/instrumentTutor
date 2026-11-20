package ui

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/score"
)

// The start screen (docs/REDESIGN.md §7): the wordmark, the
// getting-started card, the actions, and one SONGS list with filter tabs.

type browserAction struct {
	id      string
	icon    iconID
	label   string
	key     string
	primary bool
	enabled bool
	act     func(*Browser)
}

// browserLayout is where everything on the start screen sits.
type browserLayout struct {
	menu    []rect // Settings, Help (header, right)
	hint    rect   // the getting-started card (zero when hidden)
	hintBtn rect   // its dismiss ×
	steps   []rect
	actions []rect
	songsY  float64
	tabs    []rect // one per paneOrder()
	list    rect   // the rows
	rows    int
	empty   rect // the empty tab's button (zero when the tab has rows)

	statusY float64

	card    rect   // the player card (zero without a progress store)
	medals  []rect // its badge tiles
	addName rect   // its "Add your name" button, before there is a profile
}

const (
	brwCardHead = sizeDisplayS + sp3 // the card's title line
	brwStepH    = 56.0               // a title and two lines of detail
	brwLineH    = 18.0               // one status line
)

// Header items at the top right: Settings and Help.
var browserMenu = []struct {
	icon  iconID
	label string
	key   string
}{
	{icSettings, "Settings", "S"},
	{icHelp, "Help", "?"},
}

// nested reports a start screen opened over another screen: it shows Back.
func (b *Browser) nested() bool { return b.sh != nil && b.sh.Depth() > 1 }

// statusLines are the error and warning lines under the list.
func (b *Browser) statusLines() (lines []string, cols []bool) {
	if b.errMsg != "" {
		lines, cols = append(lines, b.errMsg), append(cols, true)
	}
	if head := b.warnsHeading(); head != "" {
		lines, cols = append(lines, head), append(cols, false)
	}
	for i, w := range b.warns {
		if i == brwStatusWarnings {
			lines, cols = append(lines, fmt.Sprintf("… and %d more warnings", len(b.warns)-brwStatusWarnings)), append(cols, false)
			break
		}
		lines, cols = append(lines, "warning: "+w), append(cols, false)
	}
	return lines, cols
}

func (b *Browser) layout() browserLayout {
	var l browserLayout
	y := float64(pgMargin)

	// Header: Settings and Help as menu items at the right, or in the
	// player card's corner when there is one.
	if b.hasCard() {
		b.layoutCard(&l)
	} else {
		x := screenW - pgMargin + menuPadX
		for i := len(browserMenu) - 1; i >= 0; i-- {
			m := browserMenu[i]
			w := kitMenuItemW(m.icon, m.label)
			x -= w
			l.menu = append([]rect{{x, y - sp1, w, zItemH}}, l.menu...)
		}
	}
	y += sizeDisplayL + sp1 + uiTextH + sp6 // wordmark, tagline, gap

	// Actions under the wordmark: Open song (primary), Write new, Edit.
	x := float64(pgMargin)
	for _, a := range b.actionList() {
		kind := kitSecondary
		if a.primary {
			kind = kitPrimary
		}
		w := kitButtonW(a.label, kind, true)
		l.actions = append(l.actions, rect{x, y, w, btnHPrimary})
		x += w + sp3
	}
	y += btnHPrimary + sp6
	if l.card.h > 0 {
		y = math.Max(y, l.card.y+l.card.h+sp6)
	}

	// The getting-started card.
	if b.hintOpen {
		steps := b.stepList()
		h := sp4 + brwCardHead + brwStepH + sp4
		l.hint = rect{pgMargin, y, screenW - 2*pgMargin, h}
		l.hintBtn = rect{l.hint.x + l.hint.w - zItemH, y, zItemH, zItemH}
		if n := len(steps); n > 0 {
			inner := l.hint.w - 2*sp4
			w := (inner - float64(n-1)*sp3) / float64(n)
			for i := range steps {
				l.steps = append(l.steps, rect{l.hint.x + sp4 + float64(i)*(w+sp3), y + sp4 + brwCardHead, w, brwStepH})
			}
		}
		y += h + sp6
	}

	// SONGS and its filter tabs on one line.
	l.songsY = y
	x = pgMargin + textWScaled(spaced("SONGS"), dispS) + sp6 - menuPadX
	for _, p := range b.paneOrder() {
		w := kitMenuItemW(-1, paneTitles[p])
		l.tabs = append(l.tabs, rect{x, y, w, zItemH})
		x += w
	}
	y += zItemH + sp2

	lines, _ := b.statusLines()
	l.statusY = screenH - pgMargin - float64(len(lines))*brwLineH
	bottom := l.statusY
	if len(lines) > 0 {
		bottom -= sp3
	}
	l.list = rect{pgMargin, y, screenW - 2*pgMargin, math.Max(listRowH, bottom-y)}
	l.rows = max(1, int(l.list.h/listRowH))

	if len(b.entries()) == 0 {
		if a, ok := b.emptyAction(); ok {
			w := kitButtonW(a.label, kitSecondary, true)
			mid := l.list.y + math.Min(l.list.h/2, 3*listRowH)
			l.empty = rect{l.list.x + (l.list.w-w)/2, mid + sp2, w, btnH}
		}
	}
	return l
}

func (b *Browser) rowsPerPane() int { return b.layout().rows }

func (b *Browser) actionList() []browserAction {
	_, canEdit := b.editableSelection()
	return []browserAction{
		{id: "open", icon: icOpen, label: "Open song", key: "O", primary: true, enabled: b.openDialog != nil,
			act: func(b *Browser) { b.launchOpenDialog("") }},
		{id: "new", icon: icNew, label: "Write new", key: "N", enabled: b.newPiece != nil,
			act: (*Browser).startNewPiece},
		{id: "edit", icon: icEdit, label: "Edit", key: "E", enabled: canEdit,
			act: (*Browser).editSelected},
	}
}

// emptyAction is the button an empty tab offers.
func (b *Browser) emptyAction() (browserAction, bool) {
	want := "open"
	if b.focus == paneCreated || b.focus == paneLibrary {
		want = "new"
	}
	for _, a := range b.actionList() {
		if a.id == want && a.enabled {
			a.primary = false
			return a, true
		}
	}
	return browserAction{}, false
}

// rowAt is the list row under (x, y), if any.
func (b *Browser) rowAt(l browserLayout, x, y float64) (int, bool) {
	if !l.list.contains(x, y) {
		return 0, false
	}
	vis := int((y - l.list.y) / listRowH)
	if vis >= l.rows {
		return 0, false
	}
	row := b.top + vis
	if row < 0 || row >= b.listLen() {
		return 0, false
	}
	return row, true
}

func (b *Browser) handleMouse(pt pointer) {
	l := b.layout()
	b.hoverIdx = -1
	if l.list.contains(pt.x, pt.y) {
		if row, ok := b.rowAt(l, pt.x, pt.y); ok {
			b.hoverIdx = row
			if pt.pressed {
				b.click(row)
				return
			}
		}
		if pt.wheel != 0 {
			b.wheelAcc += pt.wheel
			var steps int
			steps, b.wheelAcc = wheelSteps(b.wheelAcc)
			if steps != 0 {
				b.scrollBy(-steps * 3)
			}
		}
	}
	if !pt.pressed {
		return
	}
	order := b.paneOrder()
	for i, r := range l.tabs {
		if pt.over(r) {
			if order[i] != b.focus {
				b.slideFrom, b.slideT = b.underNow, 0
				b.focusPane(order[i])
			}
			return
		}
	}
	if b.hintOpen && pt.over(l.hintBtn) {
		b.toggleHint()
		return
	}
	for i, r := range l.steps {
		if pt.over(r) {
			b.activateStep(i)
			return
		}
	}
	for i, a := range b.actionList() {
		if i < len(l.actions) && pt.over(l.actions[i]) {
			if a.enabled && a.act != nil {
				a.act(b)
			}
			return
		}
	}
	if l.empty.w > 0 && pt.over(l.empty) {
		if a, ok := b.emptyAction(); ok {
			a.act(b)
		}
		return
	}
	if l.addName.w > 0 && pt.over(l.addName) {
		b.sh.Show(NewNamePrompt(b.sh, b.store()))
		return
	}
	for _, r := range l.medals {
		if pt.over(r) {
			b.badgesOpen = true
			return
		}
	}
	for i, r := range l.menu {
		if !pt.over(r) {
			continue
		}
		if i == 0 && b.settings != nil {
			b.settings()
		}
		if i == 1 {
			b.helpOpen = true
		}
		return
	}
}

func (b *Browser) click(i int) {
	if i == b.sel {
		b.activate()
		return
	}
	b.setSel(i)
}

func (b *Browser) Draw(screen *ebiten.Image) {
	screen.Fill(colBG)
	b.anim.tick()
	l := b.layout()
	dt := motionDt()
	b.drawBrowserHeader(screen, l, dt)
	b.drawPlayerCard(screen, l, dt)
	b.drawHint(screen, l, dt)
	b.drawActions(screen, l, dt)
	b.drawSongs(screen, l, dt)
	b.drawStatus(screen, l)
	b.tip.draw(screen)

	if b.badgesOpen {
		b.drawBadges(screen)
	}
	if b.helpOpen {
		drawHelpOverlay(screen, "START SCREEN KEYS", b.browserBindings(), browserHelpFootnote)
	}
}

func (b *Browser) drawBrowserHeader(screen *ebiten.Image, l browserLayout, dt float64) {
	x := float64(pgMargin)
	if b.nested() {
		x = drawEscButton(screen, escBackChip, &b.anim, b.ptr)
	}
	drawWordmark(screen, x, pgMargin)
	drawTextSmall(screen, "Open mic to arena", x, pgMargin+sizeDisplayL+sp1, colDim)
	if b.hasCard() {
		return // Settings and Help are in the player card
	}

	for i, r := range l.menu {
		m := browserMenu[i]
		off := i == 0 && b.settings == nil
		av := b.anim.step("menu:"+m.label, b.ptr.over(r) && !off, b.ptr.down, dt)
		drawKitMenuItem(screen, r, m.icon, m.label, false, off, av)
		b.tip.offer("menu:"+m.label, m.label, m.key, r, b.ptr.over(r) && !off, dt)
	}
}

// drawHint draws the getting-started checklist as one card (§7): its
// steps side by side, a × to put it away (H brings it back).
func (b *Browser) drawHint(screen *ebiten.Image, l browserLayout, dt float64) {
	if !b.hintOpen {
		return
	}
	notchedFill(screen, l.hint, colPanel)
	drawSectionHeader(screen, l.hint.x+sp4, l.hint.y+sp4, 0, "GETTING STARTED", 0)

	overX := b.ptr.over(l.hintBtn)
	xc := colDim
	if overX {
		xc = colNote
	}
	drawText(screen, "×", centreX("×", l.hintBtn.x, l.hintBtn.w), l.hintBtn.y+(l.hintBtn.h-uiTextH)/2, xc)
	b.tip.offer("hint:hide", "Hide getting started", "H", l.hintBtn, overX, dt)

	for i, st := range b.stepList() {
		if i >= len(l.steps) {
			break
		}
		r := l.steps[i]
		hovered := st.act != nil && b.ptr.over(r)
		sv := b.anim.step("hint:step"+strconv.Itoa(i), hovered, b.ptr.down, dt)
		if sv.hover > 0.01 {
			notchedFill(screen, r, lerpCol(colPanel, colHover, sv.hover))
		}
		if hovered && b.ptr.down {
			r.y++
		}
		// The step's number in a notched box: filled hit when done.
		box := rect{r.x + sp2, r.y + (r.h-pillH)/2, pillH, pillH}
		num, numCol, titleCol := strconv.Itoa(i+1), colDim, colNote
		switch {
		case st.act == nil:
			numCol = colBarline
			notchedStroke(screen, box, 1, colPanelEdge)
		case st.done:
			notchedFill(screen, box, withAlpha(colHit, 0.25))
			numCol, titleCol = colHit, colDim
		default:
			notchedStroke(screen, box, 1, colPanelEdge)
		}
		drawTextScaled(screen, num, box.x+(box.w-textWScaled(num, dispS))/2, box.y+(box.h-sizeDisplayS)/2-1, dispS, numCol)
		tx := box.x + box.w + sp3
		tw := r.x + r.w - sp2 - tx
		drawText(screen, truncateW(st.title, tw), tx, r.y, titleCol)
		// The detail wraps onto a second line rather than losing words.
		for j, line := range wrapTextW(st.detail, tw) {
			if j == 2 {
				break
			}
			drawTextSmall(screen, line, tx, r.y+uiTextH+2+float64(j)*16, colDim)
		}
	}
}

func (b *Browser) drawActions(screen *ebiten.Image, l browserLayout, dt float64) {
	for i, a := range b.actionList() {
		if i >= len(l.actions) {
			break
		}
		r := l.actions[i]
		over := b.ptr.over(r) && a.enabled
		kind := kitSecondary
		if a.primary {
			kind = kitPrimary
		}
		drawKitButton(screen, r, a.label, a.icon, kind, btnState{hover: over, press: over && b.ptr.down, disabled: !a.enabled})
		b.tip.offer("action:"+a.id, a.label, a.key, r, over, dt)
	}
}

// drawSongs draws SONGS, its filter tabs and the active tab's rows.
func (b *Browser) drawSongs(screen *ebiten.Image, l browserLayout, dt float64) {
	drawSectionHeader(screen, pgMargin, l.songsY+(zItemH-sizeDisplayS)/2-2, 0, "SONGS", 0)

	order := b.paneOrder()
	var target rect
	for i, r := range l.tabs {
		p := order[i]
		on := p == b.focus
		over := b.ptr.over(r)
		av := b.anim.step("tab:"+paneTitles[p], over && !on, b.ptr.down, dt)
		label := paneTitles[p]
		drawText(screen, label, centreX(label, r.x, r.w), r.y+(r.h-uiTextH)/2-2, menuInk(on, false, av))
		if !on {
			drawUnderline(screen, r, textW(label), false, av)
		} else {
			w := textW(label)
			target = rect{r.x + (r.w-w)/2, float64(underlineY(r)), w, menuUnderline}
		}
		b.tip.offer("tab:"+label, label, "Tab", r, over, dt)
	}
	// The active underline: it slides to a clicked tab, jumps on a key.
	u := target
	if b.slideT < durMove && b.slideFrom.w > 0 {
		b.slideT += dt
		k := curveMove.at(math.Min(1, b.slideT/durMove))
		u.x = b.slideFrom.x + (target.x-b.slideFrom.x)*k
		u.w = b.slideFrom.w + (target.w-b.slideFrom.w)*k
	}
	b.underNow = u
	vector.FillRect(screen, float32(u.x), float32(u.y), float32(u.w), menuUnderline, colAccent, false)

	// Right of the tabs: the library's state while it reads, or where the
	// list is scrolled to.
	es := b.entries()
	right := screenW - pgMargin
	ty := l.songsY + (zItemH-uiTextH)/2 - 2
	if note, isErr := b.libraryNote(); (b.focus == paneLibrary || b.focus == paneAll) && (isErr || b.libScanning) && note != "" {
		col := colHint
		if isErr {
			col = colMiss
		}
		last := l.tabs[len(l.tabs)-1]
		note = truncateW(note, right-(last.x+last.w+sp6))
		drawText(screen, note, right-textW(note), ty, col)
	} else if len(es) > l.rows {
		s := fmt.Sprintf("%d–%d of %d", b.top+1, min(b.top+l.rows, len(es)), len(es))
		drawMonoSized(screen, s, right-textWMonoSized(s, sizeUISmall), ty+2, sizeUISmall, colHint)
	}

	if len(es) == 0 {
		msg := paneEmpty[b.focus]
		my := l.list.y + math.Min(l.list.h/2, 3*listRowH) - uiTextH - sp2
		drawText(screen, msg, centreX(msg, l.list.x, l.list.w), my, colHint)
		if a, ok := b.emptyAction(); ok && l.empty.w > 0 {
			over := b.ptr.over(l.empty)
			drawKitButton(screen, l.empty, a.label, a.icon, kitSecondary, btnState{hover: over, press: over && b.ptr.down})
		}
		return
	}

	for row := 0; row < l.rows; row++ {
		i := b.top + row
		if i >= len(es) {
			break
		}
		e := es[i]
		r := rect{l.list.x, l.list.y + float64(row)*listRowH, l.list.w, listRowH}
		b.drawSongRow(screen, r, e, i == b.hoverIdx, i == b.sel)
		b.tip.offer("row:"+e.path, filepath.Dir(e.path), "", rect{r.x + sp4, r.y, r.w / 3, r.h}, i == b.hoverIdx, dt)
	}
}

// songTitle is a row's title, cleaned as on the practice screen (§6.1):
// the library's title metadata when it has the file, else the file name.
func (b *Browser) songTitle(e browserEntry) string {
	title := ""
	if li, ok := b.libInfo[pathKey(e.path)]; ok && !li.problem && li.name != filepath.Base(e.path) {
		title = li.name
	}
	return cleanPieceTitle(title, e.path)
}

// songSub is a row's second line, the same on every tab: what the library
// knows about the file, else its kind. Never its folder (§7).
func (b *Browser) songSub(e browserEntry) (string, bool) {
	switch {
	case e.missing:
		return "missing — press Del to forget", true
	case e.problem:
		return e.sub, true
	}
	if li, ok := b.libInfo[pathKey(e.path)]; ok {
		if li.problem {
			return li.sub, true
		}
		if li.sub != "" {
			return li.sub, false
		}
	}
	return pieceKind(e.path), false
}

// pieceKind names a file's format for a row with no library summary.
func pieceKind(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".gtab":
		return "HEADLINER tab"
	case ".gp":
		return "Guitar Pro"
	case ".mid", ".midi", ".smf":
		return "MIDI"
	case ".musicxml", ".mxl", ".xml":
		return "MusicXML"
	}
	return ""
}

// songIcon is a row's instrument: the horn when the library's summary
// names a wind instrument, else the guitar.
func songIcon(sub string) iconID {
	for _, n := range score.WindNames() {
		if strings.Contains(sub, n) {
			return icHorn
		}
	}
	return icGuitar
}

// drawSongRow is a list row (§5): no box; hover raised; selected adds the
// brand bar at the left.
func (b *Browser) drawSongRow(dst *ebiten.Image, r rect, e browserEntry, hover, selected bool) {
	if hover || selected {
		vector.FillRect(dst, float32(r.x), float32(r.y), float32(r.w), float32(r.h), colHover, false)
	}
	if selected {
		vector.FillRect(dst, float32(r.x), float32(r.y), 3, float32(r.h), tokBrand, false)
	}
	iconCol := colDim
	if selected {
		iconCol = colNote
	}
	x := r.x + sp4
	sub, warn := b.songSub(e)
	drawPixelIcon(dst, songIcon(sub), x, r.y+(r.h-iconSize)/2, iconCol)
	x += iconSize + sp3
	w := r.x + r.w - sp4 - x

	title, titleCol := b.songTitle(e), colNote
	if e.missing {
		titleCol = colMissing
	}
	face := faceOf(srcBody, sizeUILarge)
	drawFace(dst, fitToWidth(title, w, face, false), face, x, r.y+sp1, titleCol)
	subCol := colDim
	if warn {
		subCol = colClose
		if e.missing {
			subCol = colMissing
		}
	}
	drawTextSmall(dst, ellipsizeWSmall(sub, w), x, r.y+r.h/2+3, subCol)
}

func (b *Browser) drawStatus(screen *ebiten.Image, l browserLayout) {
	width := screenW - 2*pgMargin
	lines, isErr := b.statusLines()
	for i, s := range lines {
		col := colClose
		if isErr[i] {
			col = colMiss
		}
		drawText(screen, truncateW(s, width), pgMargin, l.statusY+float64(i)*brwLineH, col)
	}
}

func (b *Browser) warnsHeading() string {
	if len(b.warns) == 0 || b.warnsFrom == "" {
		return ""
	}
	if b.warnsFailed {
		return "warnings from " + b.warnsFrom + ":"
	}
	return b.warnsFrom + " opened with warnings:"
}
