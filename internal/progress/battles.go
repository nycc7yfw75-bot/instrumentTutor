package progress

import "time"

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
	Date         time.Time `json:"date"`
	Material     string    `json:"material"`
	Participants []string  `json:"participants"`
	Totals       []int     `json:"totals"`
	Winner       string    `json:"winner"` // a name, or "" for a draw
}

const maxBattleLog = 50
