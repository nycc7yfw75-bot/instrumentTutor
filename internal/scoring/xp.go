package scoring

import (
	"math"
	"time"

	"github.com/S95F/musicTutor/internal/score"
)

// Difficulty of a track, from how many notes it asks for per second at full
// tempo (§4.5).
type Difficulty int

const (
	Easy Difficulty = iota
	Medium
	Hard
	Expert
)

func (d Difficulty) String() string { return [...]string{"Easy", "Medium", "Hard", "Expert"}[d] }

// Mult is the difficulty's XP multiplier.
func (d Difficulty) Mult() float64 {
	return [...]float64{EasyMult, MediumMult, HardMult, ExpertMult}[d]
}

// DifficultyOf rates a track from its attacks per second at full tempo: a
// chord is one attack. The caller caches it per piece and track.
func DifficultyOf(sc *score.Score, track int) Difficulty {
	starts := map[int64]bool{}
	first, last := int64(-1), int64(0)
	for _, ev := range sc.Events() {
		if ev.Track != track {
			continue
		}
		starts[ev.Start] = true
		if first < 0 || ev.Start < first {
			first = ev.Start
		}
		last = max(last, ev.End)
	}
	span := sc.Tempos.TimeAt(last) - sc.Tempos.TimeAt(first)
	if first < 0 || span <= 0 {
		return Easy
	}
	switch nps := float64(len(starts)) / span; {
	case nps < EasyBelowNPS:
		return Easy
	case nps < MediumBelowNPS:
		return Medium
	case nps < HardBelowNPS:
		return Hard
	}
	return Expert
}

// PassXP is a completed pass's XP before the repeat factor:
// PassBaseXP + Σ noteScore × difficulty × TempoFactor(tempo). A pass of
// fewer than MinJudgedNotes earns nothing. A miss adds 0 to Σ noteScore, so
// it never lowers XP.
func PassXP(s Summary, d Difficulty, tempoScale float64) float64 {
	if s.Judged < MinJudgedNotes {
		return 0
	}
	return PassBaseXP + s.SumScore*d.Mult()*TempoFactor(tempoScale)
}

// TempoFactor rewards slow practice (owner's ground rule 5): 1 at full
// speed or faster, rising in a straight line to SlowBonusMax at half speed,
// and staying there below it.
func TempoFactor(tempoScale float64) float64 {
	switch {
	case tempoScale >= 1:
		return 1
	case tempoScale <= SlowBonusAt:
		return SlowBonusMax
	}
	return 1 + (SlowBonusMax-1)*(1-tempoScale)/(1-SlowBonusAt)
}

// DailyPasses counts today's passes of each section ("bars 5-8", or the
// whole song); the counts restart at local midnight.
type DailyPasses struct {
	Day    string         `json:"day"` // local date, YYYY-MM-DD
	Counts map[string]int `json:"counts"`
}

// Record counts one pass of section at now and returns how many times it
// has been passed today, this one included.
func (d *DailyPasses) Record(section string, now time.Time) int {
	if day := now.Local().Format("2006-01-02"); d.Day != day || d.Counts == nil {
		d.Day, d.Counts = day, map[string]int{}
	}
	d.Counts[section]++
	return d.Counts[section]
}

// Credible reports whether a session's numbers can count toward XP and
// excitement: the input is calibrated with enough confidence and no
// capture samples were dropped.
func Credible(calibrationConfidence float64, droppedSamples int64) bool {
	return calibrationConfidence >= CredibleConfidence && droppedSamples == 0
}

// PassInput is a completed pass, for awarding XP.
type PassInput struct {
	Summary    Summary
	Difficulty Difficulty
	TempoScale float64
	Section    string
	Credible   bool
}

// AwardXP is the XP a completed pass earns: 0 for an unscored (uncredible)
// pass or one too short to judge; otherwise PassXP, rounded. Repeats earn
// the same every time (SCORING.md §4.5: no diminishing returns); the daily
// count is still kept for the pass log.
func AwardXP(in PassInput, daily *DailyPasses, now time.Time) int {
	if !in.Credible || in.Summary.Judged < MinJudgedNotes {
		return 0
	}
	daily.Record(in.Section, now)
	return int(math.Round(PassXP(in.Summary, in.Difficulty, in.TempoScale)))
}
