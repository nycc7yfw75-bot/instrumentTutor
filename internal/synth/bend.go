package synth

// BendSeg is one bend within a note, in seconds from its start and
// semitones above its fret: it rises From -> Peak, holds, and (for a
// release) falls Peak -> To.
type BendSeg struct {
	At, Len        float64
	From, Peak, To float64
}

// Where in a bend segment the pitch moves, as fractions of its length: up
// over the first 40%, held to the middle, and back down by 90%.
const (
	bendRiseEnd    = 0.4
	bendReleaseAt  = 0.5
	bendReleaseEnd = 0.9
)

// bendOffset is how many semitones the note is bent t seconds after it
// started. Between and after segments the last bend's end holds, as a
// finger keeps a string bent until it lets go; done reports that every
// segment is over and the offset will not change again.
func bendOffset(segs []BendSeg, t float64) (semis float64, done bool) {
	held := 0.0
	for _, s := range segs {
		if t < s.At {
			return held, false
		}
		if s.Len <= 0 || t >= s.At+s.Len {
			held = s.To
			continue
		}
		u := (t - s.At) / s.Len
		switch {
		case u < bendRiseEnd:
			return s.From + (s.Peak-s.From)*smooth(u/bendRiseEnd), false
		case u < bendReleaseAt || s.To == s.Peak:
			return s.Peak, false
		case u < bendReleaseEnd:
			return s.Peak + (s.To-s.Peak)*smooth((u-bendReleaseAt)/(bendReleaseEnd-bendReleaseAt)), false
		}
		return s.To, false
	}
	return held, true
}

// smooth eases 0..1 in and out, as a finger pushes a string.
func smooth(x float64) float64 { return x * x * (3 - 2*x) }
