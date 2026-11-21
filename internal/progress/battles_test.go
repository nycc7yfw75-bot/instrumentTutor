package progress

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestBattlesAreFiledPerProfileAndSurviveAReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "progress.json")
	s, err := OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := s.CreateProfile("Aarsh")
	b, _ := s.AddProfile("Friend")
	xpBefore := func(id string) int { p, _ := s.Profile(id); return p.XP.TotalXP }
	a0, b0 := xpBefore(a), xpBefore(b)

	// Aarsh beats Friend and Benny: +50 XP to Aarsh, nothing to Friend.
	log := BattleLog{Material: "Riff", Participants: []string{"Aarsh", "Friend", "Basement Benny"}, Totals: []int{900, 700, 600}, Winner: "Aarsh"}
	if _, err := s.RecordBattle(a, BattleFiling{Log: log, Total: 900, Scored: true, Overall: OutcomeWin,
		Against: map[string]int{"profile:" + b: OutcomeWin, "rival:Basement Benny": OutcomeWin}, BonusXP: 50}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordBattle(b, BattleFiling{Log: log, Total: 700, Scored: true, Overall: OutcomeLoss,
		Against: map[string]int{"profile:" + a: OutcomeLoss, "rival:Basement Benny": OutcomeWin}}); err != nil {
		t.Fatal(err)
	}
	if got := xpBefore(a) - a0; got != 50 {
		t.Errorf("the winner got %d XP, want the 50 bonus", got)
	}
	if got := xpBefore(b) - b0; got != 0 {
		t.Errorf("the loser's XP changed by %d; losing never costs or earns a bonus", got)
	}

	s2, err := OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ra, _ := s2.BattleRecordOf(a)
	rb, _ := s2.BattleRecordOf(b)
	if ra.Totals != (BattleTotals{Wins: 1}) || rb.Totals != (BattleTotals{Losses: 1}) {
		t.Errorf("after a reopen: Aarsh %+v, Friend %+v", ra.Totals, rb.Totals)
	}
	if rb.PerOpponent["rival:Basement Benny"].Wins != 1 || rb.PerOpponent["profile:"+a].Losses != 1 {
		t.Errorf("Friend's record per opponent: %+v", rb.PerOpponent)
	}
	if ra.Best != 900 || len(ra.Log) != 1 || ra.Log[0].Winner != "Aarsh" {
		t.Errorf("Aarsh's best %d, log %+v", ra.Best, ra.Log)
	}
	if p, _ := s2.Profile(a); p.Record.Wins != 1 {
		t.Error("the profile card doesn't show the win")
	}
}

func TestAnUnscoredBattleIsLoggedButNotCounted(t *testing.T) {
	s, _ := OpenFile(filepath.Join(t.TempDir(), "progress.json"))
	a, _ := s.CreateProfile("Aarsh")
	s.RecordBattle(a, BattleFiling{Log: BattleLog{Material: "Riff"}, Total: 5000, Overall: OutcomeLoss})
	r, _ := s.BattleRecordOf(a)
	if r.Totals != (BattleTotals{}) || r.Best != 0 || len(r.Log) != 1 {
		t.Errorf("unscored battle: totals %+v, best %d, log %d", r.Totals, r.Best, len(r.Log))
	}
}

func TestTheBattleLogKeepsTheLast50(t *testing.T) {
	s, _ := OpenFile(filepath.Join(t.TempDir(), "progress.json"))
	a, _ := s.CreateProfile("Aarsh")
	for i := 0; i < 60; i++ {
		s.RecordBattle(a, BattleFiling{Log: BattleLog{Material: fmt.Sprint(i)}, Scored: true, Overall: OutcomeDraw})
	}
	r, _ := s.BattleRecordOf(a)
	if len(r.Log) != 50 || r.Log[0].Material != "10" || r.Totals.Draws != 60 {
		t.Errorf("log %d long starting at %q, draws %d", len(r.Log), r.Log[0].Material, r.Totals.Draws)
	}
}
