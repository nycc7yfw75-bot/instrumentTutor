package engine

import (
	"math"
	"testing"
)

// energy plays the fixture for half a second with the given mix and sums |sample|.
func energy(t *testing.T, m func(*Mix)) float64 {
	t.Helper()
	e, _ := newFixtureEngine(t, Options{})
	mix := e.Mix()
	m(&mix)
	e.SetMix(mix)
	e.Play()
	l := make([]float32, 480)
	r := make([]float32, 480)
	var sum float64
	for i := 0; i < 50; i++ {
		e.RenderFrames(l, r)
		for j := range l {
			sum += math.Abs(float64(l[j])) + math.Abs(float64(r[j]))
		}
	}
	return sum
}

func TestTrackVolumeScalesTheTrack(t *testing.T) {
	full := energy(t, func(*Mix) {})
	half := energy(t, func(m *Mix) { m.Tracks[0] = 0.5 })
	silent := energy(t, func(m *Mix) { m.Tracks[0] = 0 })
	if full == 0 {
		t.Fatal("the fixture made no sound at full volume")
	}
	if r := half / full; r < 0.45 || r > 0.55 {
		t.Errorf("track at 50%% played at %.2f of full", r)
	}
	if silent != 0 {
		t.Errorf("track at 0 still made sound (%v)", silent)
	}
}

func TestMasterVolumeScalesEverything(t *testing.T) {
	full := energy(t, func(*Mix) {})
	if q := energy(t, func(m *Mix) { m.Master = 0.25 }) / full; q < 0.2 || q > 0.3 {
		t.Errorf("master at 25%% played at %.2f of full", q)
	}
}

func TestSetMixClampsAndIgnoresAWrongTrackCount(t *testing.T) {
	e, _ := newFixtureEngine(t, Options{})
	e.SetMix(Mix{Master: 3, Click: -1, Backing: math.NaN(), Tracks: []float64{0.5, 0.5}})
	m := e.Mix()
	if m.Master != 1 || m.Click != 0 || m.Backing != 0 {
		t.Errorf("volumes not clamped to 0..1: %+v", m)
	}
	if m.Tracks[0] != 1 {
		t.Errorf("a mix for 2 tracks changed this 1-track piece to %v", m.Tracks)
	}
}
