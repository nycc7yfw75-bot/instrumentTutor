package main

import (
	"math"
	"testing"

	"github.com/S95F/musicTutor/internal/latency"
	"github.com/S95F/musicTutor/internal/synth"
)

// A recording of a player plucking 30 ms after every tap click measures as
// a 30 ms offset, through the same attack detection the scorer uses.
func TestTapCalibrationMeasuresThePlucks(t *testing.T) {
	spacing := latency.TapSpacingFrames(sampleRate)
	lateFrames := 30 * sampleRate / 1000
	rec := make([]float32, latency.TapClicks*spacing+sampleRate)
	l := make([]float32, spacing)
	r := make([]float32, spacing)
	for i := 0; i < latency.TapClicks; i++ {
		p := synth.NewPluck(sampleRate, 27)
		p.NoteOn(45, 0.8) // a plucked A string
		clear(l)
		clear(r)
		p.Render(l[:spacing/3], r[:spacing/3]) // a short, muted pluck
		copy(rec[i*spacing+lateFrames:], l[:spacing/3])
	}
	clicks := make([]int64, latency.TapClicks)
	for i := range clicks {
		clicks[i] = int64(i * spacing)
	}
	off, conf, err := latency.TapEstimate(sampleRate, clicks, pickAttacks(rec))
	if err != nil {
		t.Fatalf("TapEstimate: %v", err)
	}
	if got := float64(off) * 1000 / sampleRate; math.Abs(got-30) > 6 {
		t.Errorf("measured %.1f ms, want ~30", got)
	}
	if conf < 0.7 {
		t.Errorf("confidence %.1f for machine-steady plucks, want steady", conf)
	}
}
