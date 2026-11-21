package ui

import (
	"strings"
	"testing"

	"github.com/S95F/musicTutor/internal/battle"
	"github.com/S95F/musicTutor/internal/score"
)

func battleFixture(t *testing.T, calibrated bool) (*BattleSetup, *App) {
	t.Helper()
	st := tempStore(t, "Aarsh")
	a := newApp(t, 8)
	deps := BattleDeps{
		Songs:      func() []PieceInfo { return []PieceInfo{{Path: "riff.gtab", Title: "Riff"}} },
		Riffs:      func() []PieceInfo { return nil },
		LoadScore:  func(string) (*score.Score, error) { return a.sc, nil },
		Calibrated: func() bool { return calibrated },
		Open:       func(string, int) (*App, error) { return a, nil },
		Close:      func() {},
	}
	return NewBattleSetup(nil, st, deps), a
}

func TestStartBattleSaysWhyItCannotGo(t *testing.T) {
	b, _ := battleFixture(t, false)
	why, needsCal := b.blocked()
	if why == "" || !needsCal {
		t.Errorf("uncalibrated with a human playing: %q (calibrate button %v)", why, needsCal)
	}
	b, _ = battleFixture(t, true)
	if why, _ := b.blocked(); why != "" {
		t.Fatalf("you vs Basement Benny, calibrated, is blocked: %q", why)
	}
	b.slots[1] = battleSlot{kind: slotProfile, tempo: 1}
	if why, _ := b.blocked(); !strings.Contains(why, "two slots") {
		t.Errorf("the same profile twice: %q", why)
	}
	b.slots[1] = battleSlot{kind: slotRival, choice: len(battle.Rivals) - 1}
	if why, _ := b.blocked(); !strings.Contains(why, "unlocks at level 30") {
		t.Errorf("The Headliner at level 1: %q", why)
	}
	b.slots[1] = battleSlot{kind: slotGhost}
	if why, _ := b.blocked(); !strings.Contains(why, "create a ghost") {
		t.Errorf("a ghost with no stored pass: %q", why)
	}
}

func TestTempoHandicapShowsWhatItCountsAs(t *testing.T) {
	b, _ := battleFixture(t, true)
	for i := 0; i < 8; i++ {
		b.adjust(bsRowSlot0, -1, true) // 100% down to 60%
	}
	_, second := b.slotText(0)
	if !strings.Contains(second, "60%|") || !strings.Contains(second, "×0.77") {
		t.Errorf("at 60%% the slot reads %q; want the handicap ×0.77", second)
	}
	for i := 0; i < 20; i++ {
		b.adjust(bsRowSlot0, -1, true)
	}
	if b.slots[0].tempo != battle.HumanTempoMin {
		t.Errorf("the tempo went down to %.2f; 50%% is the floor", b.slots[0].tempo)
	}
}

// A battle of two bots runs from the VS intro to the results with every
// turn filed, the engine advanced by hand.
func TestABotBattleRunsToTheResults(t *testing.T) {
	b, a := battleFixture(t, true)
	b.slots[0] = battleSlot{kind: slotRival}
	b.slots[1] = battleSlot{kind: slotSession, choice: 2}
	b.phraseBars, b.exchanges = 2, 2
	s, _, err := b.setup()
	if err != nil {
		t.Fatal(err)
	}
	scr, err := NewBattleScreen(nil, b.st, b.deps, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	l, r := make([]float32, 4800), make([]float32, 4800)
	for i := 0; i < 4000 && scr.phase != bpResults; i++ {
		scr.t += 10 // skip every wait
		scr.Update()
		a.eng.RenderFrames(l, r)
	}
	if scr.phase != bpResults {
		t.Fatalf("the battle stopped in phase %v", scr.phase)
	}
	for k := 0; k < 2; k++ {
		for p := 0; p < 2; p++ {
			if tn, ok := scr.b.Turn(k, p); !ok || !tn.Scored {
				t.Errorf("turn %d/%d missing or unscored", k, p)
			}
		}
	}
	if res := scr.result; res.Unscored {
		t.Error("a bot battle came out unscored")
	}
}

// The results file the battle once on each human's record, with the
// winner's XP bonus; the bots keep nothing.
func TestABattleIsFiledOnTheHumansRecord(t *testing.T) {
	b, _ := battleFixture(t, true)
	s, _, err := b.setup() // you vs Basement Benny
	if err != nil {
		t.Fatal(err)
	}
	bt, err := battle.New(s)
	if err != nil {
		t.Fatal(err)
	}
	me := s.Participants[0].ID
	before, _ := b.st.Profile(me)
	scr := &BattleScreen{st: b.st, b: bt, result: battle.Result{Totals: []int{900, 600}, Winner: 0}}
	scr.bonuses = scr.result.Bonuses()
	scr.fileBattle()
	scr.fileBattle() // the results screen may ask again: still one battle
	r, _ := b.st.BattleRecordOf(me)
	if r.Totals.Wins != 1 || len(r.Log) != 1 || r.PerOpponent["rival:Basement Benny"].Wins != 1 || r.Best != 900 {
		t.Errorf("record after one win: %+v", r)
	}
	after, _ := b.st.Profile(me)
	if got := after.XP.TotalXP - before.XP.TotalXP; got != battle.WinBonusPer {
		t.Errorf("the win paid %d XP, want %d", got, battle.WinBonusPer)
	}
	if r.Log[0].Winner != s.Participants[0].Name || r.Log[0].Material == "" {
		t.Errorf("log entry %+v", r.Log[0])
	}
}
