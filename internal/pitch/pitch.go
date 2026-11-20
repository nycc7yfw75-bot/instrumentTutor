package pitch

import "math"

type Config struct {
	SampleRate int

	Window int

	Hop int

	MinHz, MaxHz float64

	ClarityThreshold float64

	NoiseFloorDB float64

	OnsetDipDB float64

	OnsetDipRecoverHops int

	Strums bool

	StrumWindowHops int

	Estimator F0Estimator
}

func DefaultConfig(sampleRate int) Config {
	return Config{
		SampleRate:       sampleRate,
		Window:           2048,
		Hop:              480,
		MinHz:            60,
		MaxHz:            1500,
		ClarityThreshold: 0.80,
		NoiseFloorDB:     -55,
		StrumWindowHops:  defaultStrumWindowHops,
	}
}

const windOnsetDipDB = 5

func ConfigForKeys(sampleRate, lowKey, highKey int) Config {
	cfg := DefaultConfig(sampleRate)
	cfg.MinHz = keyHz(lowKey - 2)
	cfg.MaxHz = keyHz(highKey + 5)
	cfg.OnsetDipDB = windOnsetDipDB
	return cfg
}

func keyHz(key int) float64 {
	return 440 * math.Pow(2, float64(key-69)/12)
}

func (cfg Config) withDefaults() Config {
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 48000
	}
	def := DefaultConfig(cfg.SampleRate)
	if cfg.Window <= 0 {
		cfg.Window = def.Window
	}
	if cfg.Hop <= 0 {
		cfg.Hop = def.Hop
	}
	if cfg.Hop > cfg.Window {
		cfg.Hop = cfg.Window
	}
	if cfg.MinHz <= 0 {
		cfg.MinHz = def.MinHz
	}
	if cfg.MaxHz <= 0 {
		cfg.MaxHz = def.MaxHz
	}
	if cfg.MaxHz < cfg.MinHz {
		cfg.MaxHz = cfg.MinHz
	}

	minTau := int(float64(cfg.SampleRate) / cfg.MaxHz)
	if minTau < 2 {
		minTau = 2
	}
	if cfg.Window < minTau+2 {
		cfg.Window = minTau + 2
	}
	if cfg.ClarityThreshold <= 0 {
		cfg.ClarityThreshold = def.ClarityThreshold
	}
	if cfg.NoiseFloorDB == 0 {
		cfg.NoiseFloorDB = def.NoiseFloorDB
	}

	if cfg.OnsetDipDB > 0 && cfg.OnsetDipRecoverHops <= 0 {
		cfg.OnsetDipRecoverHops = defaultOnsetDipRecoverHops
	}
	if cfg.StrumWindowHops <= 0 {
		cfg.StrumWindowHops = def.StrumWindowHops
	}
	return cfg
}

type Frame struct {
	Frame int64

	F0 float64

	Clarity float64

	RMS float64

	Onset bool
}

const PitchClasses = 12

type Chroma [PitchClasses]float32

type Strum struct {
	Frame int64

	// Attack is the strum's pick attack: Frame is stamped at the centre of
	// the onset window, the attack is in its newest hop.
	Attack int64

	Chroma Chroma

	RMS float64

	Clarity float64
}

type F0Estimator interface {
	Name() string

	EstimateF0(window []float32) (f0 float64, clarity float64)
}

func ChromaOf(key int) int { return ((key % PitchClasses) + PitchClasses) % PitchClasses }

type Note struct {
	Start, End int64

	// Onset is the pick attack that began the note, when one was heard
	// (HasOnset): the time to judge timing by. Start is when the pitch was
	// first recognised, later by up to ~25 ms on low strings. A note begun
	// without a pick (hammer-on, slide) has no onset.
	Onset    int64
	HasOnset bool

	Key int

	Cents float64

	MinCents, MaxCents, EndCents float64

	Clarity float64
}
