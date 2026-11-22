package progress

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Battle records (BATTLES.md §8.2), kept per profile.

// BattleTotals is a win / loss / draw count.
type BattleTotals struct {
	Wins   int `json:"wins"`
	Losses int `json:"losses"`
	Draws  int `json:"draws"`
}

// BattleRecord is a profile's battles.
type BattleRecord struct {
	Totals BattleTotals `json:"totals"`
	// PerOpponent is the record against each opponent ("rival:Rusher Rex",
	// "profile:<id>", "ghost", "session:Hard").
	PerOpponent map[string]BattleTotals `json:"perOpponent,omitempty"`
	Best        int                     `json:"best"` // best battle total
	Log         []BattleLog             `json:"log,omitempty"`
}

// BattleLog is one battle in the history.
type BattleLog struct {
	// ID tells one battle from another when two devices' logs merge.
	ID           string    `json:"id,omitempty"`
	Date         time.Time `json:"date"`
	Material     string    `json:"material"`
	Participants []string  `json:"participants"`
	Totals       []int     `json:"totals"`
	Winner       string    `json:"winner"` // a name, or "" for a draw
	// How it went for this profile, kept so records can be rebuilt from
	// the log after a sync merge (ONLINE.md §B4).
	Scored  bool           `json:"scored,omitempty"`
	Outcome int            `json:"outcome,omitempty"`
	Against map[string]int `json:"against,omitempty"`
	Total   int            `json:"total,omitempty"`
}

const maxBattleLog = 50

// Outcomes of a battle for one player.
const (
	OutcomeLoss = -1
	OutcomeDraw = 0
	OutcomeWin  = 1
)

// BattleFiling is one battle as one human player saw it.
type BattleFiling struct {
	Log BattleLog
	// Total is this player's battle total; Scored is false when the
	// battle had no winner because a human turn went unjudged (then only
	// the log is kept: no win, loss or draw, no best).
	Total  int
	Scored bool
	// Overall is OutcomeWin / Draw / Loss; Against is the outcome per
	// opponent key ("rival:<name>", "profile:<id>", "ghost",
	// "session:<level>").
	Overall int
	Against map[string]int
	BonusXP int
}

func (t *BattleTotals) add(outcome int) {
	switch outcome {
	case OutcomeWin:
		t.Wins++
	case OutcomeLoss:
		t.Losses++
	default:
		t.Draws++
	}
}

// RecordBattle files a battle on a profile's record and credits its XP
// bonus to that profile (never negative); it reports a level up. An
// unknown profile is ignored.
func (s *Store) RecordBattle(profileID string, f BattleFiling) (leveledUp bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prof := s.f.Profiles[profileID]
	if prof == nil {
		return false, nil
	}
	entry := f.Log
	entry.Scored, entry.Outcome, entry.Against, entry.Total = f.Scored, f.Overall, f.Against, f.Total
	if entry.ID == "" {
		entry.ID = newBattleID()
	}
	r := &prof.Battles
	if f.Scored {
		r.Totals.add(f.Overall)
		for key, o := range f.Against {
			if r.PerOpponent == nil {
				r.PerOpponent = map[string]BattleTotals{}
			}
			t := r.PerOpponent[key]
			t.add(o)
			r.PerOpponent[key] = t
		}
		r.Best = max(r.Best, f.Total)
	}
	r.Log = append(r.Log, entry)
	if n := len(r.Log); n > maxBattleLog {
		r.Log = append([]BattleLog(nil), r.Log[n-maxBattleLog:]...)
	}
	if f.BonusXP > 0 {
		leveledUp = prof.XP.Add(f.BonusXP)
		prof.Level = prof.XP.Level()
	}
	return leveledUp, s.saveLocked()
}

// BattleRecordOf is a profile's battle record (a copy).
func (s *Store) BattleRecordOf(profileID string) (BattleRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prof := s.f.Profiles[profileID]
	if prof == nil {
		return BattleRecord{}, false
	}
	r := prof.Battles
	r.PerOpponent = make(map[string]BattleTotals, len(prof.Battles.PerOpponent))
	for k, v := range prof.Battles.PerOpponent {
		r.PerOpponent[k] = v
	}
	r.Log = append([]BattleLog(nil), prof.Battles.Log...)
	return r, true
}

// newBattleID is a random id for a battle log entry.
func newBattleID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// logKey identifies a battle: its ID, or for entries from before IDs,
// its date and totals.
func (l BattleLog) logKey() string {
	if l.ID != "" {
		return l.ID
	}
	return fmt.Sprintf("%d|%s|%v", l.Date.UnixNano(), l.Material, l.Totals)
}
