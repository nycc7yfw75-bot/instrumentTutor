package battle

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/S95F/musicTutor/internal/practice"
)

func phraseOf(t *testing.T, bars int) (Plan, Material) {
	t.Helper()
	m := Material{Score: riff(t, bars)}
	p, err := NewPlan(Setup{Material: m, PhraseBars: bars, Exchanges: 1, Participants: players(2)})
	if err != nil {
		t.Fatal(err)
	}
	return p, m
}

func TestABotTurnIsTheSameForTheSameSeed(t *testing.T) {
	p, m := phraseOf(t, 4)
	benny, _ := RivalNamed("Basement Benny")
	s1, j1 := BotTurn(benny.Profile, m.Score, 0, p.Phrases[0], rand.New(rand.NewSource(7)), 48000)
	s2, j2 := BotTurn(benny.Profile, m.Score, 0, p.Phrases[0], rand.New(rand.NewSource(7)), 48000)
	if !reflect.DeepEqual(j1, j2) || s1.Accuracy() != s2.Accuracy() {
		t.Fatal("the same seed played two different turns")
	}
	if len(j1) != 16 {
		t.Errorf("a 4-bar riff of quarter notes gave %d results, want 16", len(j1))
	}
}

func TestBetterRivalsScoreHigherOnAverage(t *testing.T) {
	p, m := phraseOf(t, 4)
	avg := func(name string) float64 {
		r, _ := RivalNamed(name)
		rng := rand.New(rand.NewSource(1))
		total := 0.0
		for i := 0; i < 200; i++ {
			s, _ := BotTurn(r.Profile, m.Score, 0, p.Phrases[0], rng, 48000)
			total += s.Accuracy()
		}
		return total / 200
	}
	benny, gwen, boss := avg("Basement Benny"), avg("Ghost-Note Gwen"), avg("The Headliner")
	if !(benny < gwen && gwen < boss) {
		t.Errorf("average accuracy Benny %.2f, Gwen %.2f, Headliner %.2f; should rise", benny, gwen, boss)
	}
}

func TestRivalsUnlockByLevelAndSessionLevelsBorrowThem(t *testing.T) {
	open := 0
	for _, r := range Rivals {
		if r.Unlocked(1) {
			open++
		}
	}
	if open != 2 {
		t.Errorf("%d rivals open at level 1, want Benny and Rex", open)
	}
	if r, _ := RivalNamed("The Headliner"); r.Unlocked(29) || !r.Unlocked(30) {
		t.Error("The Headliner unlocks at level 30")
	}
	hard, ok := SessionProfile("Hard")
	gwen, _ := RivalNamed("Ghost-Note Gwen")
	if !ok || hard != gwen.Profile {
		t.Error("Session Player Hard should play like Ghost-Note Gwen")
	}
}

func TestTheGhostReplaysTheStoredPassExactly(t *testing.T) {
	p, m := phraseOf(t, 2)
	index := map[[2]int64]int{}
	var notes []GhostNote
	i := 0
	for _, ev := range m.Score.Events() {
		index[[2]int64{ev.Start, int64(ev.String)}] = i
		v := practice.VerdictHit
		if i%4 == 3 {
			v = practice.VerdictMiss
		}
		notes = append(notes, GhostNote{Index: i, Verdict: int(v), OffsetMs: 20, Timed: true})
		i++
	}
	s1, _, ok := GhostTurn(notes, index, m.Score, 0, p.Phrases[0])
	s2, _, _ := GhostTurn(notes, index, m.Score, 0, p.Phrases[0])
	if !ok || s1.Judged != 8 || s1.Accuracy() != s2.Accuracy() {
		t.Fatalf("ghost: ok %v, judged %d, accuracy %.3f vs %.3f", ok, s1.Judged, s1.Accuracy(), s2.Accuracy())
	}
	if a := s1.Accuracy(); a < 0.7499 || a > 0.7501 {
		t.Errorf("6 hits at 20 ms (Perfect) and 2 misses of 8 should be 75%%, got %.3f", s1.Accuracy())
	}
	if _, _, ok := GhostTurn(notes[:3], index, m.Score, 0, p.Phrases[0]); ok {
		t.Error("a stored pass that misses part of the phrase still made a ghost")
	}
}
