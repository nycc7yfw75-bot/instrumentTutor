package latency

import (
	"errors"
	"math"
	"sort"
)

// Tap calibration, for interfaces that cannot loop their output back in
// (a Marshall CODE over USB): the app plays clicks, the player plucks a
// muted string on each one, and the offset is how late the plucks arrive.

const (
	TapClicks     = 16 // clicks played
	TapBPM        = 90
	TapSkip       = 4 // the first taps, while the player settles in, are ignored
	TapMinUsed    = 8 // fewest matched taps an estimate is made from
	tapEarliestMs = 150
	tapLatestMs   = 350
)

// Confidence from the spread (interquartile range) of the used taps.
const (
	tapSteadyMs  = 10 // -> 0.9
	tapGoodMs    = 20 // -> 0.7
	tapUsableMs  = 35 // -> 0.5; wider is refused
	tapSteadyCnf = 0.9
	tapGoodCnf   = 0.7
	tapUsableCnf = 0.5
)

var (
	ErrTooFewTaps = errors.New("not enough taps heard — pluck a muted string on every click, a little harder")
	ErrTooUneven  = errors.New("too uneven — try again with muted strings, plucking right on each click")
)

// TapSpacingFrames is the gap between tap clicks.
func TapSpacingFrames(sampleRate int) int { return sampleRate * 60 / TapBPM }

// TapEstimate works out the offset from the click times and the pick
// attacks heard (both in the same stream's frames). Each click after the
// first TapSkip takes the nearest attack from 150 ms before to 350 ms
// after it; the offset is the median of (attack - click), and the
// confidence comes from how evenly the taps landed.
func TapEstimate(sampleRate int, clicks, attacks []int64) (offsetFrames int, confidence float64, err error) {
	early := int64(sampleRate) * tapEarliestMs / 1000
	late := int64(sampleRate) * tapLatestMs / 1000
	var diffs []float64
	for i, c := range clicks {
		if i < TapSkip {
			continue
		}
		best, found := int64(0), false
		for _, a := range attacks {
			d := a - c
			if d < -early || d > late {
				continue
			}
			if !found || abs64(d) < abs64(best) {
				best, found = d, true
			}
		}
		if found {
			diffs = append(diffs, float64(best))
		}
	}
	if len(diffs) < TapMinUsed {
		return 0, 0, ErrTooFewTaps
	}
	sort.Float64s(diffs)
	iqrMs := (quantile(diffs, 0.75) - quantile(diffs, 0.25)) * 1000 / float64(sampleRate)
	switch {
	case iqrMs <= tapSteadyMs:
		confidence = tapSteadyCnf
	case iqrMs <= tapGoodMs:
		confidence = tapGoodCnf
	case iqrMs <= tapUsableMs:
		confidence = tapUsableCnf
	default:
		return 0, 0, ErrTooUneven
	}
	return int(math.Round(quantile(diffs, 0.5))), confidence, nil
}

// quantile of sorted values, interpolating between neighbours.
func quantile(sorted []float64, q float64) float64 {
	pos := q * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	return sorted[lo] + (sorted[hi]-sorted[lo])*(pos-float64(lo))
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
