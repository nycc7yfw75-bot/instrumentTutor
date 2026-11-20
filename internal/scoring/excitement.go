package scoring

import "github.com/S95F/musicTutor/internal/practice"

// Transport is what the player's song is doing, for the excitement drain.
type Transport int

const (
	Paused Transport = iota
	Playing
	Waiting    // wait mode, holding for the player's note
	CountingIn // the count-in before playback
)

func (t Transport) String() string {
	return [...]string{"paused", "playing", "waiting", "counting in"}[t]
}

// Band names the excitement level for the HUD.
type Band int

const (
	Cold Band = iota
	Warm
	Hot
	OnFire
)

func (b Band) String() string { return [...]string{"Cold", "Warm", "Hot", "On fire"}[b] }

// Excitement is the crowd's mood, 0..100 (§4.4): it rises with each judged
// note and drains while the song plays. Misses never subtract; it only
// falls by draining.
type Excitement struct {
	Value  float64
	Peak   float64
	streak int // consecutive non-miss notes
}

// NewExcitement starts the bar for a fresh playback.
func NewExcitement() Excitement {
	return Excitement{Value: ExcitementStart, Peak: ExcitementStart}
}

// Note raises the bar for one judged note.
func (e *Excitement) Note(j Judgement) {
	if j.Excluded {
		return
	}
	if j.Verdict == practice.VerdictMiss {
		e.streak = 0
		return
	}
	gain := GainClose
	if j.Verdict == practice.VerdictHit {
		switch j.Tier {
		case TierPerfect, TierUntimed:
			gain = GainPerfectHit
		case TierGood:
			gain = GainGoodHit
		case TierLoose:
			gain = GainLooseHit
		}
	}
	e.streak++
	if e.streak%StreakEvery == 0 {
		gain += StreakBonus
	}
	e.set(e.Value + gain)
}

// Tick drains the bar for seconds of transport time, but only while the
// song is actually playing: a pause, wait mode or count-in freezes it. The
// seconds come from the audio frame count, never the wall clock.
func (e *Excitement) Tick(seconds float64, t Transport) {
	if t != Playing || seconds <= 0 {
		return
	}
	e.set(e.Value - DrainPerSecond*seconds)
}

func (e *Excitement) set(v float64) {
	e.Value = max(0, min(ExcitementMax, v))
	e.Peak = max(e.Peak, e.Value)
}

// Band is the bar's named level.
func (e Excitement) Band() Band {
	switch {
	case e.Value >= OnFireFrom:
		return OnFire
	case e.Value >= HotFrom:
		return Hot
	case e.Value >= WarmFrom:
		return Warm
	}
	return Cold
}
