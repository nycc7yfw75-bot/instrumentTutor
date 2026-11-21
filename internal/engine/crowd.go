package engine

import "math"

// The crowd channel (CAREER.md §6.4): a looping crowd bed whose level
// follows the crowd's mood, plus one-shot cheers and boos. It is mixed
// into the output only — never into the input, never near detection —
// under its own mixer volume, and it sounds between songs too.

// CrowdDefaultGain is the crowd's default volume: −18 dB under the song.
var CrowdDefaultGain = math.Pow(10, -18.0/20)

// SetCrowd loads a crowd bed (mono samples at the engine's rate); nil
// removes the crowd.
func (e *Engine) SetCrowd(bed []float32) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.crowdBed = append(e.crowdBed[:0], bed...)
	silenceNonFinite(e.crowdBed)
	e.crowdPos = 0
	if !e.crowdSet {
		e.crowdGain, e.crowdSet = CrowdDefaultGain, true
	}
	if len(bed) == 0 {
		e.crowdBed, e.sting = nil, nil
	}
}

// HasCrowd reports a crowd bed loaded.
func (e *Engine) HasCrowd() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.crowdBed) > 0
}

// SetCrowdLevel sets how loud the bed is within the crowd channel, 0..1
// (the crowd's mood); it glides there, so a change never clicks.
func (e *Engine) SetCrowdLevel(level float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.crowdTarget = clampGain(level)
}

// PlayCrowd plays a one-shot (a cheer or a boo) over the bed.
func (e *Engine) PlayCrowd(s []float32) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.crowdBed) == 0 {
		return
	}
	e.sting = append(e.sting[:0], s...)
	silenceNonFinite(e.sting)
	e.stingPos = 0
}

// mixCrowd adds the crowd to a rendered buffer; called with e.mu held.
func (e *Engine) mixCrowd(left, right []float32) {
	n := len(e.crowdBed)
	if n == 0 || !(e.crowdGain > 0) {
		return
	}
	// Glide the bed's level across the buffer.
	step := (e.crowdTarget - e.crowdLevel) / float64(max(1, len(left)))
	for i := range left {
		e.crowdLevel += step
		v := e.crowdBed[e.crowdPos] * float32(e.crowdLevel)
		e.crowdPos++
		if e.crowdPos >= n {
			e.crowdPos = 0
		}
		if e.stingPos < len(e.sting) {
			v += e.sting[e.stingPos]
			e.stingPos++
		}
		v *= float32(e.crowdGain)
		left[i] += v
		right[i] += v
	}
}
