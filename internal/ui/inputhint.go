package ui

import "image/color"

// The input-level hint next to the meter: "clipping" when the interface's
// converter runs out of room, "too quiet" when a weak signal is costing
// notes. A quiet signal reads as misses, which looks like bad detection.

const (
	inputClipHoldFrames = 180 // keep the clipping hint up 3 s after the last clip
	inputQuietWindow    = 240 // judge "too quiet" over 4 s of playback
	inputQuietDB        = -45 // loudest input in a window below this is too quiet (detector floor is -55)

	inputClipHint  = "input clipping: turn the guitar or the interface's input down"
	inputQuietHint = "input too quiet: turn the guitar or the interface's input up"
)

type inputHintUI struct {
	clipCount func() int64
	clipSeen  int64
	clipUntil int64

	quietActive bool
	quietStart  int64
	quietMax    float64
	quietMisses int
	quietUntil  int64
}

// SetClipCounter hands the practice view a running count of clipped input
// samples (the live session's ClippedSamples).
func (a *App) SetClipCounter(fn func() int64) { a.clipCount = fn }

// updateInputHint runs once per frame after the live status is read.
func (a *App) updateInputHint() {
	if !a.live {
		return
	}
	if a.clipCount != nil {
		if n := a.clipCount(); n > a.clipSeen {
			a.clipSeen = n
			a.clipUntil = a.frame + inputClipHoldFrames
		}
	}

	// Too quiet only counts while the piece plays and notes were missed: a
	// rest, or a pause to think, never raises it.
	if !a.eng.Playing() {
		a.quietActive = false
		return
	}
	if !a.quietActive {
		a.quietActive = true
		a.quietStart, a.quietMax, a.quietMisses = a.frame, a.levelDB, a.stats.Miss
	}
	if a.levelDB > a.quietMax {
		a.quietMax = a.levelDB
	}
	if a.frame-a.quietStart >= inputQuietWindow {
		if a.stats.Miss > a.quietMisses && a.quietMax < inputQuietDB {
			a.quietUntil = a.frame + inputQuietWindow
		}
		a.quietActive = false
	}
}

// inputHint is the hint to show, if any; clipping wins over too quiet.
func (a *App) inputHint() (string, color.RGBA) {
	switch {
	case a.frame < a.clipUntil:
		return inputClipHint, colMiss
	case a.frame < a.quietUntil:
		return inputQuietHint, colClose
	}
	return "", color.RGBA{}
}
