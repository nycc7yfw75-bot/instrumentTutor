package scoring

import (
	"fmt"
	"math"

	"github.com/S95F/musicTutor/internal/practice"
)

// Summary totals a run of judgements: a pass, a loop section or a song run.
type Summary struct {
	Judged   int     // notes counted: excluded ones are not
	SumScore float64 // Σ noteScore
	offsets  []float64
}

// Summarise totals judgements.
func Summarise(js []Judgement) Summary {
	var s Summary
	for _, j := range js {
		s.Add(j)
	}
	return s
}

// Add counts one judgement.
func (s *Summary) Add(j Judgement) {
	if j.Excluded {
		return
	}
	s.Judged++
	s.SumScore += j.Score
	// Tightness is over hits and closes that carry a timing offset.
	if j.Verdict != practice.VerdictMiss && j.Timed() {
		s.offsets = append(s.offsets, j.OffsetMs)
	}
}

// Accuracy is Σ noteScore / judged notes, 0..1 (§4.2).
func (s Summary) Accuracy() float64 {
	if s.Judged == 0 {
		return 0
	}
	return s.SumScore / float64(s.Judged)
}

// AccuracyPct is the accuracy as a whole percent; ok is false (show "—")
// below MinJudgedNotes.
func (s Summary) AccuracyPct() (pct int, ok bool) {
	if s.Judged < MinJudgedNotes {
		return 0, false
	}
	return int(math.Round(s.Accuracy() * 100)), true
}

// Tightness is how the timed hits sat against the beat (§4.3).
type Tightness struct {
	N        int
	MeanMs   float64 // + dragging, - rushing
	SpreadMs float64 // standard deviation
}

// Tightness of the summary's timed hits and closes.
func (s Summary) Tightness() Tightness {
	t := Tightness{N: len(s.offsets)}
	if t.N == 0 {
		return t
	}
	for _, o := range s.offsets {
		t.MeanMs += o
	}
	t.MeanMs /= float64(t.N)
	for _, o := range s.offsets {
		t.SpreadMs += (o - t.MeanMs) * (o - t.MeanMs)
	}
	t.SpreadMs = math.Sqrt(t.SpreadMs / float64(t.N))
	return t
}

// Label: "Locked in", "Dragging +N ms" or "Rushing −N ms".
func (t Tightness) Label() string {
	switch {
	case math.Abs(t.MeanMs) < LockedInMs:
		return "Locked in"
	case t.MeanMs > 0:
		return fmt.Sprintf("Dragging +%.0f ms", t.MeanMs)
	}
	return fmt.Sprintf("Rushing −%.0f ms", -t.MeanMs)
}

// SpreadLabel: "tight", "loose" or "uneven".
func (t Tightness) SpreadLabel() string {
	switch {
	case t.SpreadMs < TightSpreadMs:
		return "tight"
	case t.SpreadMs < LooseSpreadMs:
		return "loose"
	}
	return "uneven"
}
