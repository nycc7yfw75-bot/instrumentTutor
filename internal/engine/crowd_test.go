package engine

import (
	"math"
	"testing"
)

func crowdEngine(t *testing.T) *Engine {
	t.Helper()
	e := newTestEngine(t)
	bed := make([]float32, 100)
	for i := range bed {
		bed[i] = 0.5
	}
	e.SetCrowd(bed)
	return e
}

func TestCrowdIsSilentUntilLoadedAndDefaultsToMinus18dB(t *testing.T) {
	e := newTestEngine(t)
	if e.HasCrowd() {
		t.Fatal("a new engine has a crowd")
	}
	e = crowdEngine(t)
	if g := e.Mix().Crowd; math.Abs(g-CrowdDefaultGain) > 1e-9 {
		t.Errorf("crowd volume %.3f, want -18 dB (%.3f)", g, CrowdDefaultGain)
	}
}

func TestCrowdLevelGlidesAndStingersPlayOnce(t *testing.T) {
	e := crowdEngine(t)
	e.SetCrowdLevel(1)
	l, r := make([]float32, 64), make([]float32, 64)
	e.mu.Lock()
	e.mixCrowd(l, r)
	e.mu.Unlock()
	if l[0] >= l[63] || l[63] <= 0 || l[0] != r[0] {
		t.Errorf("the bed did not glide up evenly on both sides: %v .. %v", l[0], l[63])
	}
	e.PlayCrowd([]float32{1, 1, 1})
	l2, r2 := make([]float32, 8), make([]float32, 8)
	e.mu.Lock()
	e.mixCrowd(l2, r2)
	e.mu.Unlock()
	if l2[0] <= l2[5] {
		t.Errorf("the stinger did not sound at the start: %v", l2)
	}
	m := e.Mix()
	m.Crowd = 0
	e.SetMix(m)
	l3, r3 := make([]float32, 8), make([]float32, 8)
	e.mu.Lock()
	e.mixCrowd(l3, r3)
	e.mu.Unlock()
	for _, v := range l3 {
		if v != 0 {
			t.Fatal("the crowd sounds with its volume at 0")
		}
	}
}

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	var reg []*articVoice
	return New(techScore(t, 6, 0, 3, 0), Options{Voices: newArticFactory(&reg)})
}
