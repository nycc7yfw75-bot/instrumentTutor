package scoring

import (
	"testing"

	"github.com/S95F/musicTutor/internal/practice"
	"github.com/S95F/musicTutor/internal/score"
)

// chordScore is one bar: a three-note chord on beat 1, then a single note.
func chordScore(t *testing.T) *score.Score {
	t.Helper()
	s := &score.Score{
		Tempos: score.TempoMap{{Tick: 0, USPerQuarter: 500000}},
		Meters: score.MeterMap{{Tick: 0, Num: 4, Den: 4}},
	}
	tr := &score.Track{Name: "Rhythm", Tuning: score.StandardTuning}
	b := tr.AppendBar(4, 4)
	b.AddBeat(score.Half, score.Note{String: 6, Fret: 3}, score.Note{String: 5, Fret: 2}, score.Note{String: 4, Fret: 0})
	b.AddBeat(score.Half, score.Note{String: 3, Fret: 0})
	s.Tracks = []*score.Track{tr}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	return s
}

func chordResults(sc *score.Score, verdicts []practice.Verdict, offFrames int64) []practice.NoteResult {
	var out []practice.NoteResult
	i := 0
	for _, ev := range sc.Events() {
		if ev.Start != 0 {
			continue
		}
		r := practice.NoteResult{Event: ev, Verdict: verdicts[i], Matched: verdicts[i] != practice.VerdictMiss}
		if r.Matched {
			r.OnsetErrFrames, r.Timed = offFrames, true
		}
		out = append(out, r)
		i++
	}
	return out
}

func TestAChordIsOneJudgementTimedFromTheStrum(t *testing.T) {
	sc := chordScore(t)
	cases := []struct {
		name    string
		verdict []practice.Verdict
		want    practice.Verdict
	}{
		{"every note rang", []practice.Verdict{practice.VerdictHit, practice.VerdictHit, practice.VerdictHit}, practice.VerdictHit},
		{"one string missing", []practice.Verdict{practice.VerdictHit, practice.VerdictMiss, practice.VerdictHit}, practice.VerdictClose},
		{"nothing heard", []practice.Verdict{practice.VerdictMiss, practice.VerdictMiss, practice.VerdictMiss}, practice.VerdictMiss},
	}
	for _, c := range cases {
		s := NewSession(sc, 0, sr, true)
		var got []Judgement
		for _, r := range chordResults(sc, c.verdict, 960) { // strummed 20 ms late
			got = append(got, s.Add(r)...)
		}
		if len(got) != 1 {
			t.Fatalf("%s: %d judgements for one chord, want 1", c.name, len(got))
		}
		j := got[0]
		if j.Verdict != c.want {
			t.Errorf("%s: chord judged %v, want %v", c.name, j.Verdict, c.want)
		}
		if c.want != practice.VerdictMiss && (j.Tier != TierPerfect || j.OffsetMs != 20) {
			t.Errorf("%s: timed %v at %.0f ms, want perfect at the strum's 20 ms", c.name, j.Tier, j.OffsetMs)
		}
		t.Logf("%-18s -> one judgement: %s, %v, score %.2f", c.name, [...]string{"hit", "close", "miss"}[j.Verdict], j.Tier, j.Score)
	}
}

func TestSessionTotalsPassesAndRuns(t *testing.T) {
	sc := chordScore(t)
	s := NewSession(sc, 0, sr, true)
	for i := 0; i < 2; i++ {
		for _, r := range chordResults(sc, []practice.Verdict{practice.VerdictHit, practice.VerdictHit, practice.VerdictHit}, 0) {
			s.Add(r)
		}
		pass := s.EndPass()
		if pass.Judged != 1 || pass.SumScore != 1 {
			t.Fatalf("pass %d: judged %d, Σ %.2f, want one perfect chord", i+1, pass.Judged, pass.SumScore)
		}
	}
	if run := s.Run(); run.Judged != 2 {
		t.Errorf("the run judged %d, want both passes' chords", run.Judged)
	}
	if s.Excitement.Value <= ExcitementStart {
		t.Errorf("credible perfect chords left excitement at %.0f", s.Excitement.Value)
	}
}

func TestUncredibleSessionLeavesExcitementAlone(t *testing.T) {
	sc := chordScore(t)
	s := NewSession(sc, 0, sr, false)
	for _, r := range chordResults(sc, []practice.Verdict{practice.VerdictHit, practice.VerdictHit, practice.VerdictHit}, 0) {
		s.Add(r)
	}
	s.Tick(5, Playing)
	if s.Excitement.Value != ExcitementStart {
		t.Errorf("an unscored session moved excitement to %.0f", s.Excitement.Value)
	}
	if pass := s.EndPass(); pass.Judged != 1 {
		t.Errorf("accuracy must still be counted when unscored; judged %d", pass.Judged)
	}
}

func TestFlushJudgesAChordCutShort(t *testing.T) {
	sc := chordScore(t)
	s := NewSession(sc, 0, sr, true)
	rs := chordResults(sc, []practice.Verdict{practice.VerdictHit, practice.VerdictHit, practice.VerdictHit}, 0)
	s.Add(rs[0])
	s.Add(rs[1]) // the pass ends before the third string's result arrives
	if got := s.Flush(); len(got) != 1 || got[0].Verdict != practice.VerdictHit {
		t.Errorf("flushing a part-heard chord gave %+v, want one judgement of what was heard", got)
	}
}
