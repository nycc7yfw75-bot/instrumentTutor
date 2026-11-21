package scoring

import (
	"math"
	"testing"
	"time"

	"github.com/S95F/musicTutor/internal/practice"
	"github.com/S95F/musicTutor/internal/score"
)

func passOf(rs ...practice.NoteResult) Summary { return Summarise(judgedAll(rs...)) }

func hits(n int, offMs float64) []practice.NoteResult {
	var out []practice.NoteResult
	for i := 0; i < n; i++ {
		out = append(out, result(practice.VerdictHit, offMs))
	}
	return out
}

func TestPassXPFormula(t *testing.T) {
	s := passOf(hits(10, 0)...) // Σ noteScore = 10
	cases := []struct {
		d     Difficulty
		tempo float64
		want  float64
	}{
		{Easy, 1.0, 10 + 10*1.0},
		{Medium, 1.0, 10 + 10*1.25},
		{Hard, 0.6, 10 + 10*1.5*1.4}, // 60% speed earns 1.4× (slow practice earns more)
		{Easy, 0.5, 10 + 10*1.5},     // half speed: the full 1.5×
		{Easy, 0.25, 10 + 10*1.5},    // slower still: capped at 1.5×
		{Expert, 1.5, 10 + 10*2.0},   // faster than written earns no bonus
	}
	for _, c := range cases {
		if got := PassXP(s, c.d, c.tempo); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%v at %.0f%%: %.2f XP, want %.2f", c.d, c.tempo*100, got, c.want)
		}
	}
	if got := PassXP(passOf(hits(7, 0)...), Easy, 1); got != 0 {
		t.Errorf("a pass of 7 judged notes earned %.1f; fewer than 8 earn nothing", got)
	}
}

// §4.7: Σ noteScore = judgedNotes × accuracy, so a miss adds nothing to XP.
// Inserting a miss anywhere leaves the pass's XP unchanged (its accuracy %
// drops, honestly); XP is never negative; stored total XP only rises.
func TestMissNeverReducesXP(t *testing.T) {
	base := []practice.NoteResult{
		result(practice.VerdictHit, 0), result(practice.VerdictHit, 50),
		result(practice.VerdictClose, 10), result(practice.VerdictHit, 120),
		result(practice.VerdictHit, -20), result(practice.VerdictClose, -90),
		result(practice.VerdictHit, 0), result(practice.VerdictHit, 30),
	}
	want := PassXP(passOf(base...), Hard, 0.8)
	baseAcc := passOf(base...).Accuracy()
	for pos := 0; pos <= len(base); pos++ {
		withMiss := append(append(append([]practice.NoteResult{}, base[:pos]...), result(practice.VerdictMiss, 0)), base[pos:]...)
		p := passOf(withMiss...)
		if got := PassXP(p, Hard, 0.8); got != want {
			t.Errorf("a miss at position %d changed pass XP from %.2f to %.2f", pos, want, got)
		}
		if p.Accuracy() >= baseAcc {
			t.Errorf("a miss at position %d did not lower the accuracy (%.3f -> %.3f)", pos, baseAcc, p.Accuracy())
		}
	}
	// An all-miss pass earns the base, never less.
	var misses []practice.NoteResult
	for i := 0; i < 12; i++ {
		misses = append(misses, result(practice.VerdictMiss, 0))
	}
	if got := PassXP(passOf(misses...), Easy, 1); got < 0 {
		t.Errorf("an all-miss pass earned %.1f XP; XP is never negative", got)
	}
	var p Player
	before := p.TotalXP
	for _, xp := range []int{12, 0, -5, 30} {
		p.Add(xp)
		if p.TotalXP < before {
			t.Fatalf("adding %d lowered total XP from %d to %d", xp, before, p.TotalXP)
		}
		before = p.TotalXP
	}
	if p.TotalXP != 42 {
		t.Errorf("total XP %d, want 42 (the -5 ignored)", p.TotalXP)
	}
}

func TestRepeatedPassesEarnEqualXP(t *testing.T) {
	in := PassInput{Summary: passOf(hits(12, 0)...), Difficulty: Medium, TempoScale: 1, Section: "bars 5-8", Credible: true}
	var d DailyPasses
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.Local)
	first := AwardXP(in, &d, now)
	for i := 2; i <= 10; i++ {
		if xp := AwardXP(in, &d, now); xp != first {
			t.Fatalf("pass %d earned %d XP, want the same %d as the first (no diminishing returns)", i, xp, first)
		}
	}
}

func TestDailyPassCountResetsAtMidnight(t *testing.T) {
	day := time.Date(2026, 11, 20, 23, 50, 0, 0, time.Local)
	var d DailyPasses
	for i := 1; i <= 3; i++ {
		if n := d.Record("bars 5-8", day); n != i {
			t.Fatalf("pass %d recorded as %d", i, n)
		}
	}
	if n := d.Record("bars 9-12", day); n != 1 {
		t.Errorf("a different section starts its own count; got %d", n)
	}
	if n := d.Record("bars 5-8", day.Add(15*time.Minute)); n != 1 {
		t.Errorf("after local midnight the count should restart; got pass %d", n)
	}
}

func TestCredibilityGate(t *testing.T) {
	s := passOf(hits(12, 0)...)
	var d DailyPasses
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.Local)
	in := PassInput{Summary: s, Difficulty: Easy, TempoScale: 1, Section: "song"}
	for _, c := range []struct {
		name    string
		conf    float64
		dropped int64
	}{{"uncalibrated", 0, 0}, {"weak calibration", 0.4, 0}, {"dropped audio", 0.9, 120}} {
		in.Credible = Credible(c.conf, c.dropped)
		var p Player
		if xp := AwardXP(in, &d, now); xp != 0 {
			t.Errorf("%s: awarded %d XP, want 0", c.name, xp)
		}
		p.Add(AwardXP(in, &d, now))
		if p.TotalXP != 0 {
			t.Errorf("%s: stored XP moved to %d", c.name, p.TotalXP)
		}
	}
	in.Credible = Credible(0.7, 0)
	if xp := AwardXP(in, &d, now); xp != 22 {
		t.Errorf("a credible pass of 12 perfect hits awarded %d XP, want 22", xp)
	}
	if n := d.Record("song", now); n != 2 {
		t.Errorf("uncredible passes were counted toward the repeat factor (next pass is %d, want 2)", n)
	}
}

func TestLevelCurveAndTitles(t *testing.T) {
	for lvl, want := range map[int]int{1: 100, 5: 1118, 10: 3162} {
		if got := XPToNext(lvl); got != want {
			t.Errorf("XP from level %d to %d: %d, want %d", lvl, lvl+1, got, want)
		}
	}
	for xp, want := range map[int]int{0: 1, 99: 1, 100: 2, 100 + 283: 3} {
		if lvl, _, _ := LevelFor(xp); lvl != want {
			t.Errorf("%d XP is level %d, want %d", xp, lvl, want)
		}
	}
	for lvl, want := range map[int]string{1: "Bedroom Player", 4: "Bedroom Player", 5: "Garage Player", 9: "Garage Player",
		10: "Open-Mic Regular", 15: "Bar Band", 20: "Club Act", 25: "Touring Musician", 29: "Touring Musician",
		30: "Festival Act", 39: "Festival Act", 40: "Headliner", 99: "Headliner"} {
		if got := Title(lvl); got != want {
			t.Errorf("level %d is %q, want %q", lvl, got, want)
		}
	}
	var p Player
	if up := p.Add(150); !up {
		t.Error("crossing 100 XP did not report a level-up")
	}
}

func TestDifficultyFromNotesPerSecond(t *testing.T) {
	// 120 BPM: a quarter is half a second.
	mk := func(dur int64) *score.Score {
		s := &score.Score{
			Tempos: score.TempoMap{{Tick: 0, USPerQuarter: 500000}},
			Meters: score.MeterMap{{Tick: 0, Num: 4, Den: 4}},
		}
		tr := &score.Track{Name: "Lead", Tuning: score.StandardTuning}
		for b := 0; b < 4; b++ {
			bar := tr.AppendBar(4, 4)
			for i := int64(0); i < score.Whole/dur; i++ {
				bar.AddBeat(dur, score.Note{String: 3, Fret: 5})
			}
		}
		s.Tracks = []*score.Track{tr}
		return s
	}
	for _, c := range []struct {
		dur  int64
		want Difficulty
	}{{score.Half, Easy}, {score.Quarter, Medium}, {score.Eighth, Hard}, {score.Sixteenth, Expert}} {
		if got := DifficultyOf(mk(c.dur), 0); got != c.want {
			t.Errorf("notes every %d ticks at 120 BPM: %v, want %v", c.dur, got, c.want)
		}
	}
}
