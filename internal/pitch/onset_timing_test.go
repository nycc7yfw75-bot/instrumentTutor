package pitch

import (
	"math"
	"testing"

	"github.com/S95F/musicTutor/internal/synth"
)

// pluckedAt renders one plucked note starting at sample attack.
func pluckedAt(sr, key, attack int) []float32 {
	p := synth.NewPluck(sr, 27)
	buf := make([]float32, sr*2)
	l, r := make([]float32, len(buf)-attack), make([]float32, len(buf)-attack)
	p.NoteOn(key, 0.8)
	p.Render(l, r)
	copy(buf[attack:], l)
	return buf
}

// Timing is judged from the pick attack: Start (pitch recognition) runs up
// to ~25 ms late on low strings, which no single calibration offset can
// cancel; the corrected onset lands on the attack on every string.
func TestNoteOnsetLandsOnThePickAttack(t *testing.T) {
	const sr = 48000
	cfg := DefaultConfig(sr)
	for _, key := range []int{40, 45, 50, 55, 59, 64, 76} {
		var worst float64
		for trial := 0; trial < 6; trial++ {
			attack := sr/2 + trial*137 // vary the phase against the 10 ms hop
			buf := pluckedAt(sr, key, attack)
			d, tr := NewDetector(cfg), NewTracker(cfg)
			var notes []Note
			for pos := 0; pos+cfg.Hop <= len(buf); pos += cfg.Hop {
				notes = append(notes, tr.Feed(d.Process(buf[pos:pos+cfg.Hop]))...)
			}
			notes = append(notes, tr.Flush()...)
			var got *Note
			for i := range notes {
				if notes[i].Key == key {
					got = &notes[i]
					break
				}
			}
			if got == nil || !got.HasOnset {
				t.Fatalf("key %d trial %d: no note with a pick attack (notes %+v)", key, trial, notes)
			}
			err := float64(got.Onset-int64(attack)) * 1000 / sr
			worst = math.Max(worst, math.Abs(err))
		}
		if worst > 8 {
			t.Errorf("key %d: the onset misses the pick by up to %.1f ms, want within 8", key, worst)
		}
	}
}
