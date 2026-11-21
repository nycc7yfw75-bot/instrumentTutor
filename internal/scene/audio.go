package scene

import (
	"math"
	"math/rand"
)

// Placeholder crowd sounds, synthesised in code until real recordings land
// (docs/ART_BRIEF.md §3). Mono, deterministic, about −6 dBFS at most.

// CrowdSound is a crowd sound by its manifest name ("bed", "cheer",
// "boo") at a sample rate; nil for an unknown name.
func CrowdSound(name string, rate int) []float32 {
	if rate <= 0 {
		return nil
	}
	switch name {
	case "bed":
		return crowdBed(rate)
	case "cheer":
		return crowdCheer(rate)
	case "boo":
		return crowdBoo(rate)
	}
	return nil
}

// lowpass is a one-pole low-pass filter.
type lowpass struct{ y, a float64 }

func newLowpass(cutoff float64, rate int) lowpass {
	return lowpass{a: 1 - math.Exp(-2*math.Pi*cutoff/float64(rate))}
}

func (l *lowpass) step(x float64) float64 {
	l.y += l.a * (x - l.y)
	return l.y
}

// crowdBed is a 6-second murmur that loops without a seam: filtered noise
// whose loudness drifts, its ends crossfaded into each other.
func crowdBed(rate int) []float32 {
	n := 6 * rate
	fade := rate / 2
	rng := rand.New(rand.NewSource(1))
	raw := make([]float64, n+fade)
	lo, hi := newLowpass(900, rate), newLowpass(250, rate)
	drift := 0.6
	for i := range raw {
		if i%(rate/8) == 0 {
			drift = 0.45 + 0.4*rng.Float64()
		}
		x := rng.Float64()*2 - 1
		band := lo.step(x) - hi.step(x) // a voice-like band, 250–900 Hz
		raw[i] = band * drift * 2.4
	}
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		v := raw[i]
		if i < fade { // crossfade the tail over the head
			t := float64(i) / float64(fade)
			v = v*t + raw[n+i]*(1-t)
		}
		out[i] = float32(math.Max(-0.5, math.Min(0.5, v)))
	}
	return out
}

// crowdCheer is a 2.5-second cheer: a bright noise swell with a couple of
// whistles.
func crowdCheer(rate int) []float32 {
	n := rate * 5 / 2
	rng := rand.New(rand.NewSource(2))
	lp := newLowpass(3000, rate)
	out := make([]float32, n)
	for i := range out {
		t := float64(i) / float64(rate)
		env := math.Min(1, t/0.25) * math.Exp(-math.Max(0, t-0.6)*1.6)
		v := lp.step(rng.Float64()*2-1) * 0.9
		for _, w := range []struct{ start, f float64 }{{0.3, 2200}, {0.9, 2600}} {
			if d := t - w.start; d > 0 && d < 0.5 {
				v += 0.12 * math.Sin(2*math.Pi*(w.f+400*d)*d) * math.Sin(math.Pi*d/0.5)
			}
		}
		out[i] = float32(math.Max(-0.5, math.Min(0.5, v*env)))
	}
	return out
}

// crowdBoo is a 2.5-second boo: low voices on "ooo", wavering.
func crowdBoo(rate int) []float32 {
	n := rate * 5 / 2
	lp := newLowpass(500, rate)
	voices := []float64{132, 147, 165, 178}
	out := make([]float32, n)
	for i := range out {
		t := float64(i) / float64(rate)
		env := math.Min(1, t/0.3) * math.Min(1, (2.5-t)/0.6)
		v := 0.0
		for k, f := range voices {
			ph := f*t + 0.8*math.Sin(2*math.Pi*(4+float64(k)*0.7)*t)/(2*math.Pi)
			v += 2*(ph-math.Floor(ph)) - 1 // a sawtooth, wavering
		}
		out[i] = float32(math.Max(-0.5, math.Min(0.5, lp.step(v/float64(len(voices)))*env*1.2)))
	}
	return out
}
