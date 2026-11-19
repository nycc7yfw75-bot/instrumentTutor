package gpimport

import (
	"math"

	"github.com/S95F/musicTutor/internal/score"
)

// Guitar Pro slide flags: a shift or legato slide carries into the next note
// on the string. Slides into or out of nothing (4, 8, 16, 32) have no target
// note and are not marked.
const (
	gpSlideShift  = 1
	gpSlideLegato = 2
)

// techProps collects one note's playing techniques from its properties.
type techProps struct {
	dead, palmMute, hopoDest, vibrato bool
	slide                             int
	bended                            bool
	bendFrom, bendMid, bendTo         *float64
}

// read takes a property it knows and reports whether it did.
func (t *techProps) read(p gpProperty) bool {
	switch p.Name {
	case "Muted":
		t.dead = p.Enable != nil
	case "PalmMuted":
		t.palmMute = p.Enable != nil
	case "HopoDestination":
		t.hopoDest = p.Enable != nil
	case "Slide":
		if p.Flags != nil {
			t.slide = *p.Flags
		}
	case "Bended":
		t.bended = p.Enable != nil
	case "BendOriginValue":
		t.bendFrom = p.Float
	case "BendMiddleValue":
		t.bendMid = p.Float
	case "BendDestinationValue":
		t.bendTo = p.Float
	case "HopoOrigin", "Midi",
		"BendOriginOffset", "BendMiddleOffset1", "BendMiddleOffset2", "BendDestinationOffset":
		// The origin is implied by the destination, the MIDI number by the
		// fret, and bend timing is not drawn.
	default:
		return false
	}
	return true
}

// quarterTones converts a Guitar Pro bend value (100 = a full bend, one
// whole tone) into quarter-tones, capped at two whole tones.
func quarterTones(v *float64) uint8 {
	if v == nil || math.IsNaN(*v) || *v <= 0 {
		return 0
	}
	q := math.Round(*v / 25)
	if q > 8 {
		q = 8
	}
	return uint8(q)
}

// apply marks n with its techniques. Notes must arrive in playing order per
// track, since hammer-on vs pull-off and slides depend on the note before.
func (t techProps) apply(n *score.Note, tc *trackConv) {
	if tc.lastFret == nil {
		tc.lastFret, tc.slideNext = map[int]int{}, map[int]bool{}
	}
	if t.dead {
		n.Tech |= score.TechDead
	}
	if t.palmMute {
		n.Tech |= score.TechPalmMute
	}
	if t.vibrato {
		n.Tech |= score.TechVibrato
	}
	if t.hopoDest {
		if last, ok := tc.lastFret[n.String]; ok && n.Fret < last {
			n.Tech |= score.TechPull
		} else {
			n.Tech |= score.TechHammer
		}
	}
	if tc.slideNext[n.String] {
		n.Tech |= score.TechSlide
		delete(tc.slideNext, n.String)
	}
	if t.slide&(gpSlideShift|gpSlideLegato) != 0 {
		tc.slideNext[n.String] = true
	}
	if t.bended {
		n.Tech |= score.TechBend
		from, to := quarterTones(t.bendFrom), quarterTones(t.bendTo)
		peak := max(from, to)
		if t.bendMid != nil {
			peak = max(peak, quarterTones(t.bendMid))
		}
		n.Bend = score.Bend{From: from, Peak: peak, To: to}
	}
	tc.lastFret[n.String] = n.Fret
}
