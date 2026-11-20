package scoring

import (
	"math"
	"testing"

	"github.com/S95F/musicTutor/internal/practice"
	"github.com/S95F/musicTutor/internal/score"
)

const sr = 48000

// result builds a matched note result, offMs late (negative: early).
func result(v practice.Verdict, offMs float64) practice.NoteResult {
	return practice.NoteResult{
		Event:          score.NoteEvent{Key: 52},
		Verdict:        v,
		Matched:        v != practice.VerdictMiss,
		OnsetErrFrames: int64(math.Round(offMs * sr / 1000)),
		Timed:          v != practice.VerdictMiss,
	}
}

func TestNoteScoreEveryPitchAndTimingTier(t *testing.T) {
	cases := []struct {
		name  string
		v     practice.Verdict
		offMs float64
		tier  Tier
		score float64
	}{
		{"hit perfect, on time", practice.VerdictHit, 0, TierPerfect, 1.00},
		{"hit perfect, 35 ms late (edge)", practice.VerdictHit, 35, TierPerfect, 1.00},
		{"hit good, 36 ms late", practice.VerdictHit, 36, TierGood, 0.85},
		{"hit good, 70 ms early (edge)", practice.VerdictHit, -70, TierGood, 0.85},
		{"hit loose, 71 ms late", practice.VerdictHit, 71, TierLoose, 0.60},
		{"hit loose, 150 ms early (edge)", practice.VerdictHit, -150, TierLoose, 0.60},
		{"close perfect", practice.VerdictClose, 10, TierPerfect, 0.50},
		{"close good", practice.VerdictClose, -50, TierGood, 0.425},
		{"close loose", practice.VerdictClose, 120, TierLoose, 0.30},
		{"miss", practice.VerdictMiss, 0, TierNone, 0},
	}
	for _, c := range cases {
		j := Judge(result(c.v, c.offMs), sr)
		if j.Excluded {
			t.Fatalf("%s: excluded", c.name)
		}
		if j.Tier != c.tier || math.Abs(j.Score-c.score) > 1e-9 {
			t.Errorf("%-32s tier %-7v score %.3f, want %-7v %.3f", c.name, j.Tier, j.Score, c.tier, c.score)
		}
		t.Logf("%-32s tier %-7v score %.3f", c.name, j.Tier, j.Score)
	}
}

func TestUnjudgeableNotesAreExcluded(t *testing.T) {
	for _, c := range []struct {
		name string
		tech score.Technique
	}{{"dead note", score.TechDead}, {"bend", score.TechBend}, {"grace note", score.TechGrace}} {
		r := result(practice.VerdictHit, 0)
		r.Event.Tech = c.tech
		if j := Judge(r, sr); !j.Excluded {
			t.Errorf("%s was judged; it should count neither for nor against", c.name)
		}
	}
}

func TestUntimedHitsScoreOnPitchAlone(t *testing.T) {
	r := result(practice.VerdictHit, 0)
	r.Timed = false // a slide's arrival: matched, but with no pick to time
	j := Judge(r, sr)
	if j.Tier != TierUntimed || j.Score != 1 {
		t.Errorf("untimed hit: tier %v score %.2f, want untimed, full pitch credit", j.Tier, j.Score)
	}
}

func TestWaitModeNotesAreUnscored(t *testing.T) {
	r := result(practice.VerdictHit, 0)
	r.Timed, r.Waited = false, true
	if j := Judge(r, sr); !j.Excluded {
		t.Errorf("a wait-mode note was scored (%+v); wait mode is practice and counts neither way", j)
	}
}

func TestOffsetSignIsLatePositive(t *testing.T) {
	if j := Judge(result(practice.VerdictHit, 20), sr); j.OffsetMs <= 0 {
		t.Errorf("a late note reads %.1f ms, want positive (dragging)", j.OffsetMs)
	}
	if j := Judge(result(practice.VerdictHit, -20), sr); j.OffsetMs >= 0 {
		t.Errorf("an early note reads %.1f ms, want negative (rushing)", j.OffsetMs)
	}
}
