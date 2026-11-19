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
	mixCardW   = 600.0
	mixHeadH   = 52.0
	mixRowH    = 36.0
	mixFootH   = 40.0
	mixLabelW  = 210.0
	mixValueW  = 56.0
	mixPadX    = 22.0
	mixBarH    = 6.0
	mixKnobR   = 8.0
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

func (a *App) mixerCard(rows int) rect {
	h := mixHeadH + float64(rows)*mixRowH + mixFootH
	return rect{(screenW - mixCardW) / 2, (screenH - h) / 2, mixCardW, h}
}

func (a *App) mixerRowRect(card rect, i int) rect {
	return rect{card.x, card.y + mixHeadH + float64(i)*mixRowH, card.w, mixRowH}
}

func (a *App) mixerBar(row rect) rect {
	x := row.x + mixPadX + mixLabelW
	return rect{x, row.y + (row.h-mixBarH)/2, row.x + row.w - mixPadX - mixValueW - x, mixBarH}
}

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
		a.mixSet(rows[a.mixerDrag], (p.x-bar.x)/bar.w)
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
		grab := rect{bar.x - mixKnobR, rr.y, bar.w + 2*mixKnobR, rr.h}
		if p.over(grab) {
			a.mixerDrag = i
			a.mixSet(rows[i], (p.x-bar.x)/bar.w)
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
	drawPanel(c, card, colPanel, colPanelEdge)
	drawText(c, "MIXER", card.x+mixPadX, card.y+16, colInferred)
	drawTextRight(c, a.mixerHeadNote(rows), card.x+card.w-mixPadX, card.y+16, colDim)

	m := a.eng.Mix()
	for i, r := range rows {
		rr := a.mixerRowRect(card, i)
		val := mixGet(m, r)
		if i == a.mixerSel {
			vector.DrawFilledRect(c, float32(rr.x+1), float32(rr.y), float32(rr.w-2), float32(rr.h), colFocus, false)
		}
		labelCol := colHUD
		if r.kind == mixTrack && r.track == a.track {
			labelCol = colInferred // your track, as on the track chips
		}
		drawText(c, truncateW(r.label, mixLabelW-12), rr.x+mixPadX, rr.y+(rr.h-uiTextH)/2, labelCol)

		bar := a.mixerBar(rr)
		fillRounded(c, bar, colBG)
		filled := bar
		filled.w = bar.w * val
		if filled.w > 0 {
			fillRounded(c, filled, colOnEdge)
		}
		kx, ky := float32(bar.x+bar.w*val), float32(bar.y+bar.h/2)
		knob := colNote
		if a.ptr.over(rect{bar.x - mixKnobR, rr.y, bar.w + 2*mixKnobR, rr.h}) || a.mixerDrag == i {
			knob = colSounding
		}
		vector.DrawFilledCircle(c, kx, ky, mixKnobR, knob, true)
		drawTextRight(c, fmt.Sprintf("%d%%", int(math.Round(val*100))), rr.x+rr.w-mixPadX, rr.y+(rr.h-uiTextH)/2, colHUD)
	}
	foot := "up/down choose   left/right 5%   drag a slider   space play/pause"
	drawText(c, foot, centreX(foot, card.x, card.w), card.y+card.h-mixFootH+12, colDim)
}
