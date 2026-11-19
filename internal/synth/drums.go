package synth

import (
	"math"

	"github.com/sinshu/go-meltysynth/meltysynth"
)

// DrumKitProgram asks a Factory for a drum kit instead of a pitched
// instrument: notes are then General MIDI drum keys (36 kick, 38 snare,
// 42 closed hi-hat ...). It sits outside the 0-127 program range.
const DrumKitProgram = 128

// drumHit is how one General MIDI drum is synthesised: a pitched body that
// falls from toneHz to toneEndHz, plus noise, each with its own decay.
type drumHit struct {
	toneHz, toneEndHz float64 // body pitch, sweeping down
	toneLevel         float64
	toneDecay         float64 // seconds to fall to about a third
	noiseLevel        float64
	noiseDecay        float64
	bright            float64 // 0 dark (low-passed noise) .. 1 bright (high-passed)
}

var (
	hitKick      = drumHit{toneHz: 130, toneEndHz: 48, toneLevel: 1.0, toneDecay: 0.22, noiseLevel: 0.12, noiseDecay: 0.012, bright: 0.2}
	hitSnare     = drumHit{toneHz: 200, toneEndHz: 170, toneLevel: 0.45, toneDecay: 0.08, noiseLevel: 0.7, noiseDecay: 0.16, bright: 0.6}
	hitSideStick = drumHit{toneHz: 900, toneEndHz: 700, toneLevel: 0.3, toneDecay: 0.02, noiseLevel: 0.4, noiseDecay: 0.025, bright: 0.8}
	hitClap      = drumHit{noiseLevel: 0.7, noiseDecay: 0.12, bright: 0.6}
	hitHatClosed = drumHit{noiseLevel: 0.35, noiseDecay: 0.045, bright: 1}
	hitHatOpen   = drumHit{noiseLevel: 0.32, noiseDecay: 0.32, bright: 1}
	hitCrash     = drumHit{noiseLevel: 0.38, noiseDecay: 1.1, bright: 0.95}
	hitSplash    = drumHit{noiseLevel: 0.34, noiseDecay: 0.45, bright: 1}
	hitRide      = drumHit{toneHz: 540, toneEndHz: 540, toneLevel: 0.1, toneDecay: 0.6, noiseLevel: 0.2, noiseDecay: 0.7, bright: 0.95}
	hitBell      = drumHit{toneHz: 820, toneEndHz: 820, toneLevel: 0.35, toneDecay: 0.7, noiseLevel: 0.08, noiseDecay: 0.3, bright: 1}
	hitCowbell   = drumHit{toneHz: 560, toneEndHz: 560, toneLevel: 0.45, toneDecay: 0.18, noiseLevel: 0.05, noiseDecay: 0.05, bright: 0.8}
	hitShaker    = drumHit{noiseLevel: 0.25, noiseDecay: 0.06, bright: 0.9}
)

// tomHz is each General MIDI tom's body pitch, low floor tom to high tom.
var tomHz = map[int]float64{41: 82, 43: 98, 45: 115, 47: 135, 48: 158, 50: 185}

func drumFor(key int) drumHit {
	switch key {
	case 35, 36:
		return hitKick
	case 38, 40:
		return hitSnare
	case 37:
		return hitSideStick
	case 39:
		return hitClap
	case 42, 44:
		return hitHatClosed
	case 46:
		return hitHatOpen
	case 49, 52, 57:
		return hitCrash
	case 55:
		return hitSplash
	case 51, 59:
		return hitRide
	case 53:
		return hitBell
	case 56:
		return hitCowbell
	}
	if hz, ok := tomHz[key]; ok {
		return drumHit{toneHz: hz * 1.25, toneEndHz: hz, toneLevel: 0.8, toneDecay: 0.3, noiseLevel: 0.12, noiseDecay: 0.05, bright: 0.3}
	}
	return hitShaker // the rest of the kit: tambourine, maracas, guiro ...
}

const (
	drumMaxHits = 32
	drumGain    = 0.28 // sits the kit just under the rhythm guitar (measured on a full rock mix)
	drumTail    = 6.0  // a hit is done after this many decay times
)

type drumVoice struct {
	key        int
	hit        drumHit
	vel        float64
	t          int // samples since the hit
	length     int
	phase      float64
	lp, hpPrev float64 // noise filter state
}

type drumKit struct {
	sampleRate int
	hits       []drumVoice
	rng        uint64
}

// NewDrumKit is the built-in synthesised General MIDI drum kit.
func NewDrumKit(sampleRate int) Voice {
	return &drumKit{sampleRate: sampleRate, rng: 0x2545F4914F6CDD1D}
}

func (d *drumKit) NoteOn(key int, velocity float64) {
	if velocity <= 0 {
		return
	}
	if velocity > 1 {
		velocity = 1
	}
	if key == 42 || key == 44 {
		// A closed hi-hat cuts off a ringing open one, as on a real kit.
		for i := range d.hits {
			if d.hits[i].key == 46 {
				d.hits[i].t = d.hits[i].length
			}
		}
	}
	h := drumFor(key)
	longest := math.Max(h.toneDecay, h.noiseDecay)
	v := drumVoice{key: key, hit: h, vel: velocity, length: int(longest * drumTail * float64(d.sampleRate))}
	if len(d.hits) >= drumMaxHits {
		d.hits = d.hits[1:]
	}
	d.hits = append(d.hits, v)
}

// NoteOff does nothing: a drum rings out on its own.
func (d *drumKit) NoteOff(int) {}

func (d *drumKit) AllNotesOff() { d.hits = d.hits[:0] }

func (d *drumKit) noise() float64 {
	d.rng ^= d.rng << 13
	d.rng ^= d.rng >> 7
	d.rng ^= d.rng << 17
	return float64(d.rng>>11)/float64(1<<52) - 1
}

func (d *drumKit) Render(left, right []float32) {
	if len(d.hits) == 0 {
		return
	}
	sr := float64(d.sampleRate)
	for i := range left {
		var sum float64
		for hi := range d.hits {
			v := &d.hits[hi]
			if v.t >= v.length {
				continue
			}
			sec := float64(v.t) / sr
			h := v.hit
			var s float64
			if h.toneLevel > 0 {
				// The body's pitch falls exponentially toward toneEndHz.
				hz := h.toneEndHz + (h.toneHz-h.toneEndHz)*math.Exp(-sec/0.03)
				v.phase += hz / sr
				s += h.toneLevel * math.Sin(2*math.Pi*v.phase) * math.Exp(-sec/h.toneDecay)
			}
			if h.noiseLevel > 0 {
				n := d.noise()
				v.lp += (n - v.lp) * 0.35 // a low-passed copy
				hp := n - v.hpPrev        // a high-passed copy
				v.hpPrev = n
				shaped := v.lp*(1-h.bright) + hp*0.5*h.bright
				s += h.noiseLevel * shaped * math.Exp(-sec/h.noiseDecay)
			}
			// A 2 ms attack ramp keeps the start from clicking.
			if a := sec / 0.002; a < 1 {
				s *= a
			}
			sum += s * v.vel
			v.t++
		}
		out := float32(sum * drumGain)
		left[i] += out
		right[i] += out
	}
	live := d.hits[:0]
	for _, v := range d.hits {
		if v.t < v.length {
			live = append(live, v)
		}
	}
	d.hits = live
}

var _ Voice = (*drumKit)(nil)

// sfDrums plays a SoundFont's drum kit: General MIDI percussion is MIDI
// channel 10 (9 counting from zero).
type sfDrums struct {
	ms       *meltysynth.Synthesizer
	scratchL []float32
	scratchR []float32
}

const sfDrumChannel = 9

func newSoundFontDrums(sf *meltysynth.SoundFont, sampleRate int) Voice {
	settings := meltysynth.NewSynthesizerSettings(int32(sampleRate))
	settings.EnableReverbAndChorus = false
	ms, err := meltysynth.NewSynthesizer(sf, settings)
	if err != nil {
		return NewDrumKit(sampleRate)
	}
	return &sfDrums{ms: ms, scratchL: make([]float32, sfChunk), scratchR: make([]float32, sfChunk)}
}

func (v *sfDrums) NoteOn(key int, velocity float64) {
	vel := int32(math.Round(velocity * 127))
	if vel < 1 {
		return
	}
	if vel > 127 {
		vel = 127
	}
	v.ms.NoteOn(sfDrumChannel, int32(key), vel)
}

func (v *sfDrums) NoteOff(key int) { v.ms.NoteOff(sfDrumChannel, int32(key)) }

func (v *sfDrums) AllNotesOff() { v.ms.NoteOffAll(true) }

func (v *sfDrums) Render(left, right []float32) {
	for len(left) > 0 {
		n := min(len(left), sfChunk)
		v.ms.Render(v.scratchL[:n], v.scratchR[:n])
		for i := 0; i < n; i++ {
			left[i] += v.scratchL[i]
			right[i] += v.scratchR[i]
		}
		left, right = left[n:], right[n:]
	}
}

var _ Voice = (*sfDrums)(nil)
