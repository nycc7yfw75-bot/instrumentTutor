package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/S95F/musicTutor/internal/engine"
)

// The volume mixer: one overlay (key V) with a slider for the master,
// the metronome, the backing track and every track.

const (
	mixHeadH   = 52.0
	mixFootH   = 40.0
	mixPadX    = 22.0
	mixStep    = 0.05
	mixMaxRows = 14
)

type mixerKind int

const (
	mixMaster mixerKind = iota
	mixClick
	mixBacking
	mixTrack
)

type mixerRow struct {
	kind  mixerKind
	track int
	label string
}

type mixerUI struct {
	mixer     overlay
	mixerSel  int
	mixerDrag int // row being dragged, or -1
}

func (a *App) mixerRows() []mixerRow {
	rows := []mixerRow{{kind: mixMaster, label: "Master"}, {kind: mixClick, label: "Metronome"}}
	if a.eng.HasBacking() {
		rows = append(rows, mixerRow{kind: mixBacking, label: "Backing track"})
	}
	for i := range a.sc.Tracks {
		if len(rows) >= mixMaxRows {
			break
		}
		rows = append(rows, mixerRow{kind: mixTrack, track: i, label: fmt.Sprintf("%d %s", i+1, a.trackName(i))})
	}
	return rows
}

// mixerHeadNote says how to close the mixer, and owns up to any tracks that
// did not fit.
func (a *App) mixerHeadNote(rows []mixerRow) string {
	shown := 0
	for _, r := range rows {
		if r.kind == mixTrack {
			shown++
		}
	}
	if hidden := len(a.sc.Tracks) - shown; hidden > 0 {
		return fmt.Sprintf("+%d more tracks not shown   V or esc closes", hidden)
	}
	return "V or esc closes"
}

// trackName is the short name a track chip and the mixer show. Guitar Pro
// names read "Performer | Instrument | Part"; only the part tells tracks
// apart in a narrow chip, so the last segment is kept.
func (a *App) trackName(i int) string {
	name := a.sc.Tracks[i].Name
	if j := strings.LastIndex(name, "|"); j >= 0 {
		if part := strings.TrimSpace(name[j+1:]); part != "" {
			name = part
		}
	}
	if name == "" {
		if a.sc.Tracks[i].Drums {
			return "drums"
		}
		if w := a.sc.Tracks[i].Wind; w != nil {
			name = w.Name
		} else {
			name = "guitar"
		}
	}
	return name
}

func mixGet(m engine.Mix, r mixerRow) float64 {
	switch r.kind {
	case mixMaster:
		return m.Master
	case mixClick:
		return m.Click
	case mixBacking:
		return m.Backing
	}
	if r.track < len(m.Tracks) {
		return m.Tracks[r.track]
	}
	return 0
}

func (a *App) mixSet(r mixerRow, v float64) {
	m := a.eng.Mix()
	v = math.Max(0, math.Min(1, v))
	switch r.kind {
	case mixMaster:
		m.Master = v
	case mixClick:
		m.Click = v
	case mixBacking:
		m.Backing = v
	default:
		if r.track >= len(m.Tracks) {
			return
		}
		m.Tracks[r.track] = v
	}
	a.eng.SetMix(m)
}

func (a *App) toggleMixer() {
	a.mixer.set(!a.mixer.open)
	a.mixerDrag = -1
}

// The mixer's columns (REDESIGN §8): one per channel, each an instrument
// icon, a vertical segmented fader, its value in mono, and the track's
// mute / solo state. The keys are as before: up / down choose a channel,
// left / right step it 5%.
const (
	mixColW     = 78.0
	mixColGap   = 6.0
	mixIconH    = iconSize + sp2
	mixFaderH   = mixBlocks*(mixBlockH+mixBlockGap) - mixBlockGap
	mixBlocks   = 20
	mixBlockH   = 6.0
	mixBlockW   = 28.0
	mixBlockGap = 2.0
	mixValueH   = uiTextH + sp2
	mixLabelH   = 2*16 + sp2
	mixStateH   = iconPx + sp2
	mixColH     = mixIconH + mixFaderH + sp2 + mixValueH + mixStateH + mixLabelH
)

func (a *App) mixerCard(cols int) rect {
	w := 2*mixPadX + float64(cols)*mixColW + float64(max(0, cols-1))*mixColGap
	w = math.Max(w, textWSmall(mixFoot)+2*mixPadX)
	h := mixHeadH + mixColH + mixFootH
	return rect{math.Round((screenW - w) / 2), math.Round((screenH - h) / 2), w, h}
}

// mixerRowRect is channel i's column.
func (a *App) mixerRowRect(card rect, i int) rect {
	n := float64(len(a.mixerRows()))
	used := n*mixColW + (n-1)*mixColGap
	x0 := card.x + (card.w-used)/2
	return rect{x0 + float64(i)*(mixColW+mixColGap), card.y + mixHeadH, mixColW, mixColH}
}

// mixerBar is a column's fader: its value rises from the bottom.
func (a *App) mixerBar(col rect) rect {
	return rect{col.x + (col.w-mixBlockW)/2, col.y + mixIconH, mixBlockW, mixFaderH}
}

// mixValueAt is the volume a pointer at y means on a fader.
func mixValueAt(bar rect, y float64) float64 { return (bar.y + bar.h - y) / bar.h }

// updateMixer takes all input while the mixer is open.
func (a *App) updateMixer() {
	rows := a.mixerRows()
	if a.mixerSel >= len(rows) {
		a.mixerSel = len(rows) - 1
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyV), inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		a.toggleMixer()
		return
	case inpututil.IsKeyJustPressed(ebiten.KeySpace):
		a.togglePlay()
	case inpututil.IsKeyJustPressed(ebiten.KeyUp):
		a.mixerSel = (a.mixerSel - 1 + len(rows)) % len(rows)
	case inpututil.IsKeyJustPressed(ebiten.KeyDown):
		a.mixerSel = (a.mixerSel + 1) % len(rows)
	case inpututil.IsKeyJustPressed(ebiten.KeyLeft), inpututil.IsKeyJustPressed(ebiten.KeyRight):
		d := mixStep
		if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
			d = -d
		}
		r := rows[a.mixerSel]
		// Snap to the 5% grid, so stepping always lands on round numbers.
		v := math.Round((mixGet(a.eng.Mix(), r)+d)/mixStep) * mixStep
		a.mixSet(r, v)
	}
	a.mixerMouse(a.ptr, rows)
}

func (a *App) mixerMouse(p pointer, rows []mixerRow) {
	card := a.mixerCard(len(rows))
	if a.mixerDrag >= 0 {
		if !p.down || a.mixerDrag >= len(rows) {
			a.mixerDrag = -1
			return
		}
		// A drag is direct: the value follows the pointer 1:1.
		bar := a.mixerBar(a.mixerRowRect(card, a.mixerDrag))
		a.mixSet(rows[a.mixerDrag], mixValueAt(bar, p.y))
		return
	}
	if !p.pressed {
		return
	}
	if !p.over(card) {
		a.toggleMixer() // a click on the dimmed practice screen closes the mixer
		return
	}
	for i := range rows {
		rr := a.mixerRowRect(card, i)
		if !p.over(rr) {
			continue
		}
		a.mixerSel = i
		bar := a.mixerBar(rr)
		grab := rect{rr.x, bar.y - sp2, rr.w, bar.h + 2*sp2}
		if p.over(grab) {
			a.mixerDrag = i
			a.mixSet(rows[i], mixValueAt(bar, p.y))
		}
		return
	}
}

func (a *App) drawMixer(screen *ebiten.Image) {
	rows := a.mixerRows()
	card := a.mixerCard(len(rows))
	a.mixer.draw(screen, card, func(c *ebiten.Image) { a.paintMixer(c, rows, card) })
}

func (a *App) paintMixer(c *ebiten.Image, rows []mixerRow, card rect) {
	notchedFill(c, card, colPanel)
	notchedStroke(c, card, 1, colPanelEdge)
	drawSectionHeader(c, card.x+mixPadX, card.y+sp4+2, 0, "MIXER", 0)
	note := a.mixerHeadNote(rows)
	drawTextSmall(c, note, card.x+card.w-mixPadX-textWSmall(note), card.y+sp4+2, colDim)

	m := a.eng.Mix()
	short := a.shortTrackNames()
	for i, r := range rows {
		col := a.mixerRowRect(card, i)
		val := mixGet(m, r)
		sel := i == a.mixerSel
		if sel {
			notchedFill(c, col, colHover)
			vector.FillRect(c, float32(col.x+2), float32(col.y), float32(col.w-4), 3, tokBrand, false)
		}
		ink := colDim
		if sel || a.mixerDrag == i {
			ink = colNote
		}
		icon, line1, line2 := a.mixerChannel(r, short)
		drawPixelIcon(c, icon, math.Round(col.x+(col.w-iconSize)/2), col.y+sp1, ink)

		bar := a.mixerBar(col)
		lit := int(math.Round(val * mixBlocks))
		for b := 0; b < mixBlocks; b++ {
			bc := colBlockOff
			if b < lit {
				bc = ink
			}
			y := bar.y + bar.h - float64(b+1)*(mixBlockH+mixBlockGap) + mixBlockGap
			vector.FillRect(c, float32(bar.x), float32(y), mixBlockW, mixBlockH, bc, false)
		}

		y := bar.y + bar.h + sp2
		v := fmt.Sprintf("%d%%", int(math.Round(val*100)))
		drawMonoSized(c, v, math.Round(col.x+(col.w-textWMonoSized(v, sizeUI))/2), y, sizeUI, ink)
		y += mixValueH
		if r.kind == mixTrack {
			st, stCol := icSpeakerOn, colBarline
			switch {
			case a.solo == r.track+1:
				st, stCol = icSolo, colNote
			case a.mutedAudibly(r.track):
				st, stCol = icSpeakerMuted, colDim
			}
			drawPixelIconScaled(c, st, math.Round(col.x+(col.w-iconPx)/2), y, 1, stCol)
		}
		y += mixStateH
		labelCol := colDim
		if r.kind == mixTrack && r.track == a.track {
			labelCol = colNote // your track
		}
		for j, l := range []string{line1, line2} {
			l = truncateW(l, col.w-4)
			drawTextSmall(c, l, math.Round(col.x+(col.w-textWSmall(l))/2), y+float64(j)*16, labelCol)
		}
	}
	drawTextSmall(c, mixFoot, math.Round(card.x+(card.w-textWSmall(mixFoot))/2), card.y+card.h-mixFootH+12, colBarline)
}

// mixerChannel is a column's icon and its two label lines.
func (a *App) mixerChannel(r mixerRow, short []string) (iconID, string, string) {
	switch r.kind {
	case mixMaster:
		return icSpeakerOn, "Master", ""
	case mixClick:
		return icMetronome, "Metronome", ""
	case mixBacking:
		return icMixer, "Backing", "track"
	}
	_, icon := instrumentOf(a.sc.Tracks[r.track])
	name := short[r.track]
	line1, line2 := name, ""
	if j := strings.Index(name, " · "); j >= 0 {
		line1, line2 = name[:j], name[j+len(" · "):]
	}
	return icon, fmt.Sprintf("%d %s", r.track+1, line1), line2
}

const mixFoot = "up/down choose   left/right 5%   drag a fader   space play/pause"
