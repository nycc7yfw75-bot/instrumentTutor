package scoring

import (
	"fmt"
	"testing"

	"github.com/S95F/musicTutor/internal/practice"
	"github.com/S95F/musicTutor/internal/score"
)

func judgedAll(rs ...practice.NoteResult) []Judgement {
	var out []Judgement
	for _, r := range rs {
		out = append(out, Judge(r, sr))
	}
	return out
}

func TestAccuracyIsHiddenBelowEightNotes(t *testing.T) {
	var rs []practice.NoteResult
	for i := 0; i < 7; i++ {
		rs = append(rs, result(practice.VerdictHit, 0))
	}
	if _, ok := Summarise(judgedAll(rs...)).AccuracyPct(); ok {
		t.Error("accuracy shown after only 7 judged notes; want \"—\"")
	}
	rs = append(rs, result(practice.VerdictMiss, 0))
	pct, ok := Summarise(judgedAll(rs...)).AccuracyPct()
	if !ok || pct != 88 { // 7 / 8 = 87.5 -> 88
		t.Errorf("7 hits and a miss: %d%% (shown %v), want 88%%", pct, ok)
	}
}

func TestAccuracyIgnoresExcludedNotes(t *testing.T) {
	var rs []practice.NoteResult
	for i := 0; i < 8; i++ {
		rs = append(rs, result(practice.VerdictHit, 0))
	}
	bend := result(practice.VerdictMiss, 0)
	bend.Event.Tech = score.TechBend
	rs = append(rs, bend)
	if s := Summarise(judgedAll(rs...)); s.Judged != 8 || s.Accuracy() != 1 {
		t.Errorf("judged %d, accuracy %.2f: a missed bend must count neither way", s.Judged, s.Accuracy())
	}
}

func TestTightnessSignAndLabels(t *testing.T) {
	cases := []struct {
		offs   []float64
		label  string
		spread string
	}{
		{[]float64{2, -3, 4, -1}, "Locked in", "tight"},
		{[]float64{20, 16, 24, 18}, "Dragging +20 ms", "tight"},
		{[]float64{-18, -22, -20, -20}, "Rushing −20 ms", "tight"},
		{[]float64{-30, 30, -30, 30}, "Locked in", "loose"},
		{[]float64{-60, 60, -60, 60}, "Locked in", "uneven"},
	}
	for _, c := range cases {
		var rs []practice.NoteResult
		for _, o := range c.offs {
			rs = append(rs, result(practice.VerdictHit, o))
		}
		rs = append(rs, result(practice.VerdictMiss, 0)) // misses never count toward tightness
		tt := Summarise(judgedAll(rs...)).Tightness()
		if tt.Label() != c.label || tt.SpreadLabel() != c.spread {
			t.Errorf("offsets %v: %q · %q, want %q · %q", c.offs, tt.Label(), tt.SpreadLabel(), c.label, c.spread)
		}
		t.Logf("offsets %-20s -> %s · %s (mean %+.1f, spread %.1f)", fmt.Sprint(c.offs), tt.Label(), tt.SpreadLabel(), tt.MeanMs, tt.SpreadMs)
	}
}
