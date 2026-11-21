package ui

import (
	"fmt"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/S95F/musicTutor/internal/pitch"
)

// The sound check (BATTLES.md §3): before a player's first turn, "Play
// your open strings" for three seconds. It passes when the input level
// rises above the detector's own gate on three separate plucks. It only
// looks at the level: calibration and detection are untouched.

const (
	soundCheckSeconds = 3.0
	soundCheckOnsets  = 3
	soundCheckReset   = 6.0 // dB under the gate the level must fall to before the next pluck counts
)

// soundCheckGate is the detector's gate: below it, it hears nothing.
var soundCheckGate = pitch.DefaultConfig(48000).NoiseFloorDB

type soundCheck struct {
	t       float64
	onsets  int
	above   bool
	done    bool
	passed  bool
	levelDB float64
}

// update takes this frame's input level.
func (c *soundCheck) update(levelDB, dt float64) {
	c.levelDB = levelDB
	if c.done {
		return
	}
	c.t += dt
	switch {
	case !c.above && levelDB > soundCheckGate:
		c.above = true
		c.onsets++
	case c.above && levelDB < soundCheckGate-soundCheckReset:
		c.above = false
	}
	if c.onsets >= soundCheckOnsets {
		c.done, c.passed = true, true
	} else if c.t >= soundCheckSeconds {
		c.done = true
	}
}

// retry starts the check again.
func (c *soundCheck) retry() { *c = soundCheck{} }

// draw is the check's card for a player, centred in area.
func (c *soundCheck) draw(dst *ebiten.Image, area rect, player string) {
	r := rect{math.Round(area.x + (area.w-520)/2), math.Round(area.y + (area.h-200)/2), 520, 200}
	y := drawDialogCard(dst, r, "Sound check: "+player)
	msg := "Play your open strings, one at a time."
	col := colDim
	switch {
	case c.done && c.passed:
		msg, col = "Heard you. Ready!", colHit
	case c.done:
		msg, col = "Can't hear you — turn your guitar volume up. Space tries again.", colClose
	}
	drawText(dst, msg, r.x+sp6, y, col)
	lv := (math.Max(-60, math.Min(0, c.levelDB)) + 60) / 60
	drawSegMeter(dst, r.x+sp6, y+uiTextH+sp4, 24, lv)
	count := fmt.Sprintf("%d / %d", min(c.onsets, soundCheckOnsets), soundCheckOnsets)
	drawMonoSized(dst, count, r.x+r.w-sp6-textWMonoSized(count, sizeUI), y+uiTextH+sp4-2, sizeUI, colNote)
	left := math.Max(0, soundCheckSeconds-c.t)
	if !c.done {
		drawTextSmall(dst, fmt.Sprintf("%.0f s", math.Ceil(left)), r.x+sp6, r.y+r.h-sp4-14, colBarline)
	}
}
