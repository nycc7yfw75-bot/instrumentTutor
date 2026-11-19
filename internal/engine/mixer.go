package engine

import "math"

// Mix is the volume of everything the engine plays, each 0 (silent) to 1
// (as loud as the piece was written). Tracks is indexed like the score's
// tracks.
type Mix struct {
	Master  float64
	Click   float64
	Backing float64
	Tracks  []float64
}

func clampGain(g float64) float64 {
	if math.IsNaN(g) || g < 0 {
		return 0
	}
	if g > 1 {
		return 1
	}
	return g
}

// Mix returns the current volumes.
func (e *Engine) Mix() Mix {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Mix{
		Master:  e.masterGain,
		Click:   e.clickGain,
		Backing: e.backGain,
		Tracks:  append([]float64(nil), e.trackGain...),
	}
}

// SetMix applies volumes; a Tracks slice of the wrong length leaves the
// track volumes alone, so a mix saved for another version of a file is safe.
func (e *Engine) SetMix(m Mix) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.masterGain = clampGain(m.Master)
	e.clickGain = clampGain(m.Click)
	e.backGain = clampGain(m.Backing)
	if len(m.Tracks) == len(e.trackGain) {
		for i, g := range m.Tracks {
			e.trackGain[i] = clampGain(g)
		}
	}
}

// HasBacking reports whether a backing-track recording is loaded.
func (e *Engine) HasBacking() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.backL) > 0
}

// mixVoices renders each track's voice, scaled by its volume.
func (e *Engine) mixVoices(left, right []float32) {
	n := len(left)
	if len(e.mixL) < n {
		e.mixL = make([]float32, n)
		e.mixR = make([]float32, n)
	}
	for i, v := range e.voices {
		g := e.trackGain[i]
		if g == 1 {
			v.Render(left, right)
			continue
		}
		sl, sr := e.mixL[:n], e.mixR[:n]
		clear(sl)
		clear(sr)
		// Rendered even at zero volume, so the voice's notes keep time.
		v.Render(sl, sr)
		gf := float32(g)
		for j := range sl {
			left[j] += sl[j] * gf
			right[j] += sr[j] * gf
		}
	}
}

func scaleBuf(buf []float32, g float64) {
	if g == 1 {
		return
	}
	gf := float32(g)
	for i := range buf {
		buf[i] *= gf
	}
}
