package progress

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/S95F/musicTutor/internal/career"
	"github.com/S95F/musicTutor/internal/scoring"
)

var t0 = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func onDay(n int) time.Time { return t0.AddDate(0, 0, n) }

func TestMergeRules(t *testing.T) {
	local := &Profile{
		Name: "Alex", Created: onDay(5), XP: scoring.Player{TotalXP: 900},
		Bests:    map[string]*Best{"s#0": {Accuracy: 0.8, Tempo: 0.7, TightMean: 30, TightSet: true}},
		Replays:  map[string]*Replay{"s#0#1-4": {Accuracy: 0.9}, "only-local": {Accuracy: 0.5}},
		Badges:   map[string]time.Time{"first": onDay(6), "local": onDay(7)},
		Mastered: map[string]career.Mastery{"s#0": {Date: onDay(9), Accuracy: 0.95}},
		Career: career.State{Hometown: "Mumbai", Fame: 40, ToughP: 0.3, Cleared: []int{0},
			Gigs: map[string]*career.GigRecord{
				"g1": {Crowd: career.CrowdNormal, BestStars: 2, Attempts: 1},
				"g2": {Crowd: career.CrowdTough, BestStars: 3, Attempts: 1},
			}},
		Practice: Practice{Days: map[string]float64{"2026-11-01": 600}},
		Passes:   []PassLog{{Date: onDay(1), Song: "s"}, {Date: onDay(3), Song: "s"}},
	}
	remote := &Profile{
		Name: "Alex (laptop 2)", Created: onDay(2), XP: scoring.Player{TotalXP: 1200},
		Bests:    map[string]*Best{"s#0": {Accuracy: 0.85, Tempo: 0.6, TightMean: -12, TightSet: true}},
		Replays:  map[string]*Replay{"s#0#1-4": {Accuracy: 0.92}},
		Badges:   map[string]time.Time{"first": onDay(4)},
		Mastered: map[string]career.Mastery{"s#0": {Date: onDay(8), Accuracy: 0.91}, "t#0": {Date: onDay(10)}},
		Career: career.State{Hometown: "Pune", Fame: 55, ToughP: 0.5, Cleared: []int{1},
			Gigs: map[string]*career.GigRecord{
				"g1": {Crowd: career.CrowdTough, BestStars: 1, Attempts: 3},
			}},
		Practice: Practice{Days: map[string]float64{"2026-11-01": 300, "2026-11-02": 120}},
		Passes:   []PassLog{{Date: onDay(3), Song: "s"}, {Date: onDay(2), Song: "s"}},
	}
	m := MergeProfiles(local, remote)

	checks := []struct {
		rule      string
		got, want any
	}{
		{"XP is the larger", m.XP.TotalXP, 1200},
		{"level follows XP", m.Level, scoring.Player{TotalXP: 1200}.Level()},
		{"the local name stays", m.Name, "Alex"},
		{"created is the earlier", m.Created, onDay(2)},
		{"best accuracy is the larger", m.Bests["s#0"].Accuracy, 0.85},
		{"best tempo is the larger", m.Bests["s#0"].Tempo, 0.7},
		{"tightest timing wins", m.Bests["s#0"].TightMean, -12.0},
		{"the replay with the higher accuracy wins", m.Replays["s#0#1-4"].Accuracy, 0.92},
		{"a replay only one side has stays", m.Replays["only-local"] != nil, true},
		{"badges union, earlier date", m.Badges["first"], onDay(4)},
		{"a badge only one side has stays", m.Badges["local"], onDay(7)},
		{"mastered: union", len(m.Mastered), 2},
		{"mastered: the earlier date wins", m.Mastered["s#0"].Date, onDay(8)},
		{"fame is the larger", m.Career.Fame, 55},
		{"the local career's town stays", m.Career.Hometown, "Mumbai"},
		{"gig best stars is the larger", m.Career.Gigs["g1"].BestStars, 2},
		{"gig crowd: the first recorded (server's) wins", m.Career.Gigs["g1"].Crowd, career.CrowdTough},
		{"a gig only one side played stays", m.Career.Gigs["g2"].BestStars, 3},
		{"tough odds from the side with more rolls", m.Career.ToughP, 0.5},
		{"cleared tiers union", m.Career.Cleared, []int{0, 1}},
		{"practice per day is the larger, never summed", m.Practice.Days["2026-11-01"], 600.0},
		{"a practice day only one side has stays", m.Practice.Days["2026-11-02"], 120.0},
		{"passes union by id, in order", len(m.Passes), 3},
		{"passes sorted by date", m.Passes[0].Date, onDay(1)},
	}
	for _, c := range checks {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s: got %v, want %v", c.rule, c.got, c.want)
		}
	}
	// Nothing was changed in the inputs.
	if local.XP.TotalXP != 900 || remote.Career.Gigs["g1"].BestStars != 1 {
		t.Error("merging changed an input")
	}
	// Deterministic.
	if !reflect.DeepEqual(m, MergeProfiles(local, remote)) {
		t.Error("the same merge came out twice differently")
	}
}

func battle(id string, outcome int, opp string) BattleLog {
	return BattleLog{ID: id, Date: t0.Add(time.Duration(len(id)) * time.Minute), Scored: true, Outcome: outcome,
		Against: map[string]int{opp: outcome}}
}

func TestBattleRecordsAreRebuiltFromTheMergedLog(t *testing.T) {
	shared := battle("s", OutcomeWin, "rival:Benny")
	a := BattleRecord{Totals: BattleTotals{Wins: 2}, Best: 900,
		PerOpponent: map[string]BattleTotals{"rival:Benny": {Wins: 2}},
		Log:         []BattleLog{shared, battle("aa", OutcomeWin, "rival:Benny")}}
	b := BattleRecord{Totals: BattleTotals{Wins: 1, Losses: 1}, Best: 700,
		PerOpponent: map[string]BattleTotals{"rival:Benny": {Wins: 1}, "ghost": {Losses: 1}},
		Log:         []BattleLog{shared, battle("bbb", OutcomeLoss, "ghost")}}
	m := mergeBattles(a, b)
	if m.Totals != (BattleTotals{Wins: 2, Losses: 1}) {
		t.Errorf("totals %+v: the shared win must count once", m.Totals)
	}
	if m.PerOpponent["rival:Benny"].Wins != 2 || m.PerOpponent["ghost"].Losses != 1 {
		t.Errorf("per opponent %+v", m.PerOpponent)
	}
	if len(m.Log) != 3 || m.Best != 900 {
		t.Errorf("log %d, best %d", len(m.Log), m.Best)
	}
	// Merging the result with itself again changes nothing (repeat syncs).
	if again := mergeBattles(m, m); !reflect.DeepEqual(again, m) {
		t.Errorf("a repeat sync changed the record: %+v", again.Totals)
	}
}

func TestOlderBattlesBeyondTheLogAreNotDoubleCounted(t *testing.T) {
	var log []BattleLog
	for i := 0; i < maxBattleLog; i++ {
		log = append(log, BattleLog{ID: fmt.Sprint(i), Date: t0.Add(time.Duration(i) * time.Hour), Scored: true, Outcome: OutcomeWin})
	}
	// 70 wins, 20 of them older than the 50-entry log; both devices agree.
	r := BattleRecord{Totals: BattleTotals{Wins: 70}, Log: log}
	if m := mergeBattles(r, r); m.Totals.Wins != 70 {
		t.Errorf("70 wins on both sides merged to %d", m.Totals.Wins)
	}
	// One more win on one device only.
	r2 := BattleRecord{Totals: BattleTotals{Wins: 71}, Log: append(append([]BattleLog(nil), log[1:]...),
		BattleLog{ID: "new", Date: t0.Add(100 * time.Hour), Scored: true, Outcome: OutcomeWin})}
	if m := mergeBattles(r, r2); m.Totals.Wins != 71 || len(m.Log) != maxBattleLog {
		t.Errorf("one new win: %d wins, log %d", m.Totals.Wins, len(m.Log))
	}
	// Battles from before logs carried outcomes keep their counts.
	old := BattleRecord{Totals: BattleTotals{Wins: 3, Draws: 1}, Log: []BattleLog{{Material: "x", Date: t0}}}
	if m := mergeBattles(old, old); m.Totals != old.Totals {
		t.Errorf("pre-outcome records merged to %+v", m.Totals)
	}
}

func TestMergingWithNothing(t *testing.T) {
	p := &Profile{Name: "Alex", XP: scoring.Player{TotalXP: 5}}
	if m := MergeProfiles(p, nil); m.Name != "Alex" || m == p {
		t.Error("merge with no remote should be a copy of local")
	}
	if m := MergeProfiles(nil, p); m.XP.TotalXP != 5 {
		t.Error("merge with no local should be the remote")
	}
}
