// Package battle holds the rules of a guitar battle (docs/BATTLES.md):
// phrases, turn order, turn scores with the tempo handicap, totals, the
// winner and its tie-breaks, the crowd meter, and the bots. It is pure: no
// UI, engine or audio, and all randomness comes in through a seeded source.
package battle

// Every number a battle uses, in one place.
const (
	DefaultPhraseBars = 4
	DefaultExchanges  = 4
	MinPhraseBars     = 2
	MaxPhraseBars     = 8
	MinExchanges      = 1
	MaxExchanges      = 8
	MinParticipants   = 2
	MaxParticipants   = 4

	// MinJudgeable is how many judgeable notes a phrase needs for a fair
	// turn; fewer, and setup warns.
	MinJudgeable = 8

	// TurnScoreScale: turnScore = round(TurnScoreScale × accuracy × √tempo).
	TurnScoreScale = 1000

	// The two-player crowd meter: each exchange moves it by the score
	// difference ÷ 1000 × CrowdSwing, clamped to ±CrowdStepMax.
	CrowdSwing   = 1.5
	CrowdStepMax = 0.4

	// XP bonuses (§8.1): the winner gets WinBonusPer for each other
	// participant; a draw gives everyone DrawBonus. Losing earns none.
	WinBonusPer = 25
	DrawBonus   = 10

	// A human's handicap tempo: its range and step.
	HumanTempoMin  = 0.50
	HumanTempoMax  = 1.00
	HumanTempoStep = 0.05
)
