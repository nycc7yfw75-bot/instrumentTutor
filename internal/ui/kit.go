package ui

import (
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// KitScreen is the dev-only pixel kit sheet (docs/REDESIGN.md §2 stage 2):
// every icon and component in every state. It opens only with
// HEADLINER_KIT=1.
type KitScreen struct {
	ptr  pointer
	anim animator
}

// KitRequested reports whether HEADLINER_KIT=1 asks for the kit sheet.
func KitRequested() bool { return os.Getenv("HEADLINER_KIT") == "1" }

func NewKitScreen() *KitScreen { return &KitScreen{} }

func (k *KitScreen) Update() error {
	k.ptr = readPointer()
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || escButtonPressed(escBackChip, k.ptr) {
		return errQuit
	}
	return nil
}

func (k *KitScreen) Draw(dst *ebiten.Image) {
	dst.Fill(colBG)
	k.anim.tick()
	x := drawEscButton(dst, escBackChip, &k.anim, k.ptr)
	drawWordmark(dst, x+sp2, 16)

	// Icons: tinted text, with names beneath.
	y := 80.0
	drawSectionHeader(dst, sp8, y, screenW-2*sp8, "ICONS", int(iconCount))
	y += 28
	const cellW, cellH = 92.0, 58.0
	for i := iconID(0); i < iconCount; i++ {
		cx := sp8 + float64(int(i)%13)*cellW
		cy := y + float64(int(i)/13)*cellH
		drawPixelIcon(dst, i, cx+(cellW-iconSize)/2, cy, colNote)
		drawTextSmall(dst, iconNames[i], centreXSmall(iconNames[i], cx, cellW), cy+iconSize+4, colHint)
	}
	y += 3*cellH + 8

	// Buttons: primary, secondary, destructive in their states.
	drawSectionHeader(dst, sp8, y, screenW-2*sp8, "BUTTONS", 0)
	y += 28
	bx := sp8
	for _, b := range []struct {
		label string
		icon  iconID
		kind  btnKind
		st    btnState
	}{
		{"Open song", -1, kitPrimary, btnState{}},
		{"Hover", -1, kitPrimary, btnState{hover: true}},
		{"Pressed", -1, kitPrimary, btnState{hover: true, press: true}},
		{"Write new", icNew, kitSecondary, btnState{}},
		{"Hover", icEdit, kitSecondary, btnState{hover: true}},
		{"Focus", -1, kitSecondary, btnState{focus: true}},
		{"Disabled", -1, kitSecondary, btnState{disabled: true}},
		{"Delete profile", -1, kitDestructive, btnState{}},
	} {
		h := btnH
		if b.kind == kitPrimary {
			h = btnHPrimary
		}
		w := kitButtonW(b.label, b.kind, b.icon >= 0)
		r := rect{bx, y + (btnHPrimary-h)/2, w, h}
		st := b.st
		if k.ptr.over(r) {
			st.hover, st.press = true, k.ptr.down
		}
		drawKitButton(dst, r, b.label, b.icon, b.kind, st)
		bx += w + sp3
	}
	y += btnHPrimary + 20

	// Menu items: inactive, hover, active, disabled; pills; keycaps.
	drawSectionHeader(dst, sp8, y, 560, "MENU ITEMS", 0)
	drawSectionHeader(dst, 640, y, screenW-640-sp8, "PILLS AND KEYCAPS", 0)
	y += 28
	mx := sp8
	for _, m := range []struct {
		icon    iconID
		label   string
		on, dis bool
		hover   float64
	}{
		{icLoop, "Loop", false, false, 0},
		{icMetronome, "Click", false, false, 1},
		{icRamp, "Ramp", true, false, 0},
		{icWait, "Wait", false, true, 0},
	} {
		w := kitMenuItemW(m.icon, m.label)
		r := rect{mx, y, w, 40}
		av := animValues{hover: m.hover}
		if k.ptr.over(r) && !m.dis {
			av.hover = 1
		}
		drawKitMenuItem(dst, r, m.icon, m.label, m.on, m.dis, av)
		mx += w + sp4
	}
	px := 640.0
	px += drawPill(dst, px, y+8, "LIVE", pillBrand) + sp2
	px += drawPill(dst, px, y+8, "PAUSED", pillOutline) + sp2
	px += drawPill(dst, px, y+8, "NO SIGNAL", pillWarn) + sp2
	px += drawPill(dst, px, y+8, "YOU", pillYou) + sp4
	for _, key := range []string{"SPACE", "A", "SHIFT", "?"} {
		px += drawKeycap(dst, px, y+8, key) + sp1
	}
	y += 56

	// Meter, tooltip, section header, list rows, banner.
	drawSectionHeader(dst, sp8, y, 560, "METER AND TOOLTIP", 0)
	drawSectionHeader(dst, 640, y, screenW-640-sp8, "SONGS", 3)
	y += 28
	drawTextScaled(dst, "IN", sp8, y-2, dispS, colBarline)
	for i, lv := range []float64{0.25, 0.8, 1} {
		drawSegMeter(dst, sp8+28, y+float64(i)*20, 12, lv)
	}
	drawKitTooltip(dst, 200, y, "Loop a section", "A", "B")
	rows := []struct {
		icon       iconID
		title, sub string
		hover, sel bool
	}{
		{icGuitar, "Sultans of Swing", "4/4 · 148 BPM · 212 bars", false, false},
		{icBass, "First Steps on Guitar", "4/4 · 90 BPM · 4 bars", true, false},
		{icHorn, "First Steps on Soprano Sax", "4/4 · 90 BPM · 4 bars · soprano sax", false, true},
	}
	for i, r := range rows {
		rr := rect{640, y + float64(i)*listRowH, screenW - 640 - sp8, listRowH}
		drawListRow(dst, rr, r.icon, r.title, r.sub, r.hover || k.ptr.over(rr), r.sel)
	}
	y += 3*listRowH + 12
	drawBanner(dst, rect{sp8, y, screenW - 2*sp8, bannerH},
		"Imported with 3 warnings: a bend on bar 12 was simplified", 2, k.ptr)
}

func centreXSmall(s string, x, w float64) float64 { return x + (w-textWSmall(s))/2 }
