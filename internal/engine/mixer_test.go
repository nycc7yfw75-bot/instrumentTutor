package engine

import (
	"math"
	"testing"

	"github.com/S95F/musicTutor/internal/score"
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

func TestLimiterKeepsTheOutputInsideFullScale(t *testing.T) {
	buf := []float32{0.5, -0.89, 0.95, 1.4, -3, 100}
	limit(buf)
	if buf[0] != 0.5 || buf[1] != -0.89 {
		t.Errorf("the limiter touched quiet samples: %v", buf[:2])
	}
	prev := float32(0.89)
	for _, s := range buf[2:] {
		a := float32(math.Abs(float64(s)))
		if a > 1 {
			t.Errorf("a peak left the limiter at %v, past full scale", s)
		}
		if a < prev && s > 0 {
			t.Errorf("louder input came out quieter (%v after %v)", a, prev)
		}
		prev = a
	}
	if buf[4] >= 0 {
		t.Errorf("the limiter flipped a negative peak to %v", buf[4])
	}
}

func TestBendSegsFollowTheTempoAndPracticeSpeed(t *testing.T) {
	e, _ := newFixtureEngine(t, Options{}) // 120 BPM: a quarter is half a second
	ev := score.NoteEvent{Start: 960, End: 1920, Tech: score.TechBend,
		Bends: []score.TimedBend{{Len: 960, Bend: score.Bend{Peak: 2, To: 0}}}}
	if segs := e.bendSegs(&ev); len(segs) != 1 || segs[0].Len != 0.5 || segs[0].Peak != 1 {
		t.Fatalf("at full speed: %+v, want half a second up a semitone (2 quarter-tones)", segs)
	}
	e.SetTempoScale(0.5)
	if segs := e.bendSegs(&ev); segs[0].Len != 1 {
		t.Errorf("at half speed the bend lasts %.2fs, want 1s", segs[0].Len)
	}
	bare := score.NoteEvent{Start: 0, End: 960, Tech: score.TechBend}
	if segs := e.bendSegs(&bare); len(segs) != 1 || segs[0].Peak != 2 || segs[0].To != 2 {
		t.Errorf("a .gtab bend with no amount plays as %+v, want a full bend up, held", segs)
	}
}
