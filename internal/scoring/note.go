package scoring

import (
	"math"

	"github.com/S95F/musicTutor/internal/practice"
	"github.com/S95F/musicTutor/internal/score"
)

// Tier is how well a note was timed.
type Tier int

const (
	TierNone    Tier = iota // a miss: nothing to time
	TierPerfect             // |offset| <= PerfectMs
	TierGood                // <= GoodMs
	TierLoose               // <= LooseMs (and anything past it: the match window caps it)
	TierUntimed             // matched with no timing measured (wait mode): pitch credit only
)

func (t Tier) String() string {
	return [...]string{"none", "perfect", "good", "loose", "untimed"}[t]
}

// Judgement is one judged note (or chord).
type Judgement struct {
	Event    score.NoteEvent
	Excluded bool // a note the detector cannot judge fairly: counts neither way
	Verdict  practice.Verdict
	Tier     Tier
	OffsetMs float64 // + late (dragging), - early (rushing); 0 when untimed
	Score    float64 // pitch credit × timing multiplier, 0..1
}

// Timed reports whether the judgement carries a real timing offset.
func (j Judgement) Timed() bool { return j.Tier >= TierPerfect && j.Tier <= TierLoose }

// unjudgeable are the notes the scorer cannot fairly judge (§4.1): dead
// notes have no pitch, bends sound above their fret, grace notes are over
// too quickly.
const unjudgeable = score.TechDead | score.TechBend | score.TechGrace

// Judge scores one result from the practice scorer.
func Judge(r practice.NoteResult, sampleRate int) Judgement {
	j := Judgement{Event: r.Event, Verdict: r.Verdict}
	if r.Event.Tech&unjudgeable != 0 {
		j.Excluded = true
		return j
	}
	credit := PitchCreditHit
	switch r.Verdict {
	case practice.VerdictMiss:
		return j // TierNone, score 0
	case practice.VerdictClose:
		credit = PitchCreditClose
	}
	if !r.Timed {
		j.Tier, j.Score = TierUntimed, credit
		return j
	}
	j.OffsetMs = float64(r.OnsetErrFrames) * 1000 / float64(sampleRate)
	j.Tier = TierFor(j.OffsetMs)
	j.Score = credit * tierMult(j.Tier)
	return j
}

// TierFor is the timing tier of an offset.
func TierFor(offsetMs float64) Tier {
	switch a := math.Abs(offsetMs); {
	case a <= PerfectMs:
		return TierPerfect
	case a <= GoodMs:
		return TierGood
	}
	return TierLoose
}

func tierMult(t Tier) float64 {
	switch t {
	case TierPerfect:
		return PerfectMult
	case TierGood:
		return GoodMult
	case TierLoose:
		return LooseMult
	}
	return 1
}
