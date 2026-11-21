package battle

import (
	"math"
	"testing"

	"github.com/S95F/musicTutor/internal/practice"
	"github.com/S95F/musicTutor/internal/score"
	"github.com/S95F/musicTutor/internal/scoring"
)

// riff is a one-track piece of n bars, four quarter notes a bar.
func riff(t *testing.T, bars int) *score.Score {
	t.Helper()
	sc := &score.Score{
		Tempos: score.TempoMap{{Tick: 0, USPerQuarter: 500000}},
		Meters: score.MeterMap{{Tick: 0, Num: 4, Den: 4}},
	}
	tr := &score.Track{Name: "lead", Tuning: score.StandardTuning}
	for i := 0; i < bars; i++ {
		b := tr.AppendBar(4, 4)
		for n := 0; n < 4; n++ {
			b.AddBeat(score.Quarter, score.Note{String: 3, Fret: 5 + n})
		}
	}
	sc.Tracks = []*score.Track{tr}
	if err := sc.Validate(); err != nil {
		t.Fatal(err)
	}
	return sc
}

func players(n int) []Participant {
	out := make([]Participant, n)
	for i := range out {
		out[i] = Participant{Name: string(rune('A' + i)), Kind: KindHuman, TempoScale: 1}
	}
	return out
}

func TestPhrasesSplitTheSectionAndShortenToFit(t *testing.T) {
	sc := riff(t, 16)
	p, err := NewPlan(Setup{Material: Material{Score: sc}, PhraseBars: 4, Exchanges: 4, Participants: players(2)})
	if err != nil || len(p.Phrases) != 4 || p.Shortened {
		t.Fatalf("16 bars in 4-bar phrases: %+v (err %v)", p, err)
	}
	if ph := p.Phrases[2]; ph.FromBar != 8 || ph.ToBar != 12 || ph.Judgeable != 16 {
		t.Errorf("phrase 3 is %+v; want bars 8-12 with 16 notes", ph)
	}
	short, _ := NewPlan(Setup{Material: Material{Score: riff(t, 10)}, PhraseBars: 4, Exchanges: 4, Participants: players(2)})
	if len(short.Phrases) != 2 || !short.Shortened || len(short.Warnings) == 0 {
		t.Errorf("10 bars asked for 4 exchanges: %d phrases, shortened %v, warnings %v", len(short.Phrases), short.Shortened, short.Warnings)
	}
	enough, _ := NewPlan(Setup{Material: Material{Score: riff(t, 4)}, PhraseBars: 2, Exchanges: 2, Participants: players(2)})
	if len(enough.Warnings) != 0 {
		t.Errorf("phrases of 8 notes each (the minimum) warned: %v", enough.Warnings)
	}
	sparse := riff(t, 4)
	for _, b := range sparse.Tracks[0].Bars {
		b.Beats = b.Beats[:1] // one note a bar
	}
	thin, _ := NewPlan(Setup{Material: Material{Score: sparse}, PhraseBars: 2, Exchanges: 2, Participants: players(2)})
	if len(thin.Warnings) != 2 {
		t.Errorf("phrases of 2 notes should each warn: %v", thin.Warnings)
	}
	if _, err := NewPlan(Setup{Material: Material{Score: riff(t, 1)}, PhraseBars: 2, Exchanges: 1, Participants: players(2)}); err != ErrTooShort {
		t.Errorf("a 1-bar section made a 2-bar phrase: %v", err)
	}
	if _, err := NewPlan(Setup{Material: Material{Score: sc}, PhraseBars: 4, Exchanges: 4, Participants: players(1)}); err != ErrPlayers {
		t.Errorf("one player: %v", err)
	}
}

func TestTurnOrderRotates(t *testing.T) {
	want := [][]int{{0, 1, 2}, {1, 2, 0}, {2, 0, 1}, {0, 1, 2}}
	for k, w := range want {
		got := TurnOrder(3, k)
		for i := range w {
			if got[i] != w[i] {
				t.Errorf("exchange %d order %v, want %v", k, got, w)
				break
			}
		}
	}
}

func TestTurnScoreWithHandicaps(t *testing.T) {
	if got := TurnScore(0.95, 0.60); got != 736 {
		t.Errorf("95%% at 60%% speed scored %d, want 736", got)
	}
	if got := TurnScore(0.75, 1.0); got != 750 {
		t.Errorf("75%% at full speed scored %d, want 750", got)
	}
	if got := TurnScore(1, 1.4); got != 1000 {
		t.Errorf("faster than written scored %d; the handicap caps at full speed", got)
	}
}

// sum is a scored summary: n notes at acc, timed at the offsets given.
func sum(n int, acc float64, offsets ...float64) scoring.Summary {
	var s scoring.Summary
	for i := 0; i < n; i++ {
		j := scoring.Judgement{Verdict: practice.VerdictHit, Tier: scoring.TierPerfect, Score: acc}
		if i < len(offsets) {
			j.OffsetMs = offsets[i]
		}
		s.Add(j)
	}
	return s
}

func battleOf(t *testing.T, n int) *Battle {
	t.Helper()
	b, err := New(Setup{Material: Material{Score: riff(t, 8)}, PhraseBars: 4, Exchanges: 2, Participants: players(n)})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestTheHighestTotalWinsAndTiesBreakOnTiming(t *testing.T) {
	b := battleOf(t, 2)
	for k := 0; k < 2; k++ {
		b.Record(NewTurn(0, k, sum(10, 0.9, 5, -5), 1, true))
		b.Record(NewTurn(1, k, sum(10, 0.8, 1, -1), 1, true))
	}
	if r := b.Result(); r.Winner != 0 || r.TieBreak != "" {
		t.Errorf("player 0 scored more: %+v", r)
	}
	// Tie on points: the smaller average offset wins (player 1: -20 vs +3).
	tie := battleOf(t, 2)
	for k := 0; k < 2; k++ {
		tie.Record(NewTurn(0, k, sum(10, 0.9, 20, 20), 1, true))
		tie.Record(NewTurn(1, k, sum(10, 0.9, 3, 3), 1, true))
	}
	if r := tie.Result(); r.Winner != 1 || r.TieBreak == "" {
		t.Errorf("tied on points, the tighter timing should win: %+v", r)
	}
	// Same points, same mean: the smaller spread wins.
	spread := battleOf(t, 2)
	for k := 0; k < 2; k++ {
		spread.Record(NewTurn(0, k, sum(10, 0.9, 30, -30), 1, true))
		spread.Record(NewTurn(1, k, sum(10, 0.9, 2, -2), 1, true))
	}
	if r := spread.Result(); r.Winner != 1 {
		t.Errorf("same mean, the steadier player should win: %+v", r)
	}
	// Identical play: a draw, and everyone gets the draw bonus.
	draw := battleOf(t, 2)
	for k := 0; k < 2; k++ {
		draw.Record(NewTurn(0, k, sum(10, 0.9, 4, -4), 1, true))
		draw.Record(NewTurn(1, k, sum(10, 0.9, 4, -4), 1, true))
	}
	r := draw.Result()
	if r.Winner != -1 || r.Unscored {
		t.Errorf("identical play is a draw: %+v", r)
	}
	if b := r.Bonuses(); b[0] != DrawBonus || b[1] != DrawBonus {
		t.Errorf("draw bonuses %v", b)
	}
}

func TestAnUnscoredHumanTurnBlocksAWinner(t *testing.T) {
	b := battleOf(t, 2)
	b.Record(NewTurn(0, 0, sum(10, 1), 1, false))
	b.Record(NewTurn(1, 0, sum(10, 0.5), 1, true))
	r := b.Result()
	if !r.Unscored || r.Winner != -1 {
		t.Errorf("an unscored human turn still produced %+v", r)
	}
	if t0, _ := b.Turn(0, 0); t0.Score != 0 {
		t.Errorf("an unscored turn shows a score of %d", t0.Score)
	}
	for _, bonus := range r.Bonuses() {
		if bonus != 0 {
			t.Error("an unscored battle paid a bonus")
		}
	}
}

func TestTheCrowdMeterClampsEachSwingAndTheWhole(t *testing.T) {
	if c := CrowdStep(0, 1000, 0); math.Abs(c-0.4) > 1e-9 {
		t.Errorf("a 1000-point exchange moved the crowd %.2f; each swing caps at 0.4", c)
	}
	if c := CrowdStep(0, 600, 500); math.Abs(c-0.15) > 1e-9 {
		t.Errorf("a 100-point exchange moved the crowd %.2f, want 0.15", c)
	}
	c := 0.0
	for i := 0; i < 5; i++ {
		c = CrowdStep(c, 900, 0)
	}
	if c != 1 {
		t.Errorf("five big wins left the crowd at %.2f; it stops at 1", c)
	}
	four := battleOf(t, 4)
	four.Record(NewTurn(0, 0, sum(10, 1), 1, true))
	four.Record(NewTurn(1, 0, sum(10, 0.5), 1, true))
	shares := four.CrowdShares()
	if math.Abs(shares[0]-2.0/3) > 1e-9 || shares[2] != 0 {
		t.Errorf("crowd shares %v", shares)
	}
}

func TestTheWinnerBonusCountsTheOthers(t *testing.T) {
	r := Result{Totals: []int{900, 800, 700}, Winner: 0}
	if b := r.Bonuses(); b[0] != 50 || b[1] != 0 || b[2] != 0 {
		t.Errorf("bonuses %v; the winner of 3 gets +50, losing gets nothing", b)
	}
}

func TestNextFollowsTheRotatingOrder(t *testing.T) {
	b := battleOf(t, 2)
	var order [][2]int
	for {
		k, p, ok := b.Next()
		if !ok {
			break
		}
		order = append(order, [2]int{k, p})
		b.Record(NewTurn(p, k, sum(10, 1), 1, true))
	}
	want := [][2]int{{0, 0}, {0, 1}, {1, 1}, {1, 0}}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("turns went %v, want %v", order, want)
		}
	}
}
