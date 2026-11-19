package synth

import (
	"math"
	"testing"
)

func TestBendShapeRisesHoldsAndReleases(t *testing.T) {
	// A full bend and release over one second, then a second bend held.
	segs := []BendSeg{{At: 0, Len: 1, Peak: 2, To: 0}, {At: 2, Len: 1, Peak: 1, To: 1}}
	for _, c := range []struct {
		t, want float64
	}{
		{0, 0}, {0.2, 1}, {0.4, 2}, {0.45, 2}, {0.7, 1}, {0.95, 0},
		{1.5, 0},            // between bends: the release holds
		{2.4, 1}, {2.95, 1}, // the second bend goes up and stays
	} {
		if got, _ := bendOffset(segs, c.t); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("at %.2fs the bend is %.3f semitones, want %.3f", c.t, got, c.want)
		}
	}
	if _, done := bendOffset(segs, 3.5); !done {
		t.Error("after the last bend ends, the shape should report it is done")
	}
	pre := []BendSeg{{Len: 1, From: 2, Peak: 2, To: 0}}
	if got, _ := bendOffset(pre, 0); got != 2 {
		t.Errorf("a pre-bend starts at %.2f semitones, want 2 (already bent)", got)
	}
}

func TestPluckedNoteBendsUpAFullTone(t *testing.T) {
	p := NewPluck(48000, 25).(*pluck)
	p.NoteOnSpec(NoteSpec{Key: 64, Velocity: 0.8, Bends: []BendSeg{{Len: 0.2, Peak: 2, To: 2}}})
	var v *ksVoice
	for i := range p.voices {
		if p.voices[i].active {
			v = &p.voices[i]
		}
	}
	base := v.d
	l, r := make([]float32, 48000/5), make([]float32, 48000/5)
	p.Render(l, r) // the whole bend: up a whole tone and held
	if want := base * math.Pow(2, -2.0/12); math.Abs(v.d-want) > 0.01 {
		t.Errorf("after the bend the string delay is %.3f, want %.3f (a whole tone up from %.3f)", v.d, want, base)
	}
}
