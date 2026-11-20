// Package scoring turns the practice scorer's per-note results into the
// game layer's numbers: note scores with timing, accuracy, tightness,
// excitement, XP and levels (docs/SCORING.md §4). It is pure: no UI, audio
// or engine. Every tunable number lives in this file.
package scoring

// §4.1 Per-note score.
const (
	PitchCreditHit   = 1.0
	PitchCreditClose = 0.5

	PerfectMs = 35.0  // |timing offset| at or under: Perfect
	GoodMs    = 70.0  // ... Good
	LooseMs   = 150.0 // ... Loose (the scorer's match window)

	PerfectMult = 1.00
	GoodMult    = 0.85
	LooseMult   = 0.60
)

// §4.2 Accuracy is not shown below this many judged notes.
const MinJudgedNotes = 8

// §4.3 Tightness labels.
const (
	LockedInMs    = 10.0 // |mean offset| under: "Locked in"
	TightSpreadMs = 20.0 // spread under: "tight"
	LooseSpreadMs = 40.0 // spread under: "loose"; at or over: "uneven"
)

// §4.4 Excitement, 0..100.
const (
	ExcitementStart = 50.0
	ExcitementMax   = 100.0

	GainPerfectHit = 4.0
	GainGoodHit    = 3.0
	GainLooseHit   = 2.0
	GainClose      = 1.0

	StreakEvery = 10  // consecutive non-miss notes
	StreakBonus = 5.0 // added each time a streak reaches a multiple of StreakEvery

	DrainPerSecond = 3.0 // of transport playing time

	WarmFrom   = 25.0
	HotFrom    = 50.0
	OnFireFrom = 75.0
)

// §4.5 XP.
const (
	PassBaseXP = 10.0

	// Slow practice earns more (owner's ground rule 5): the tempo factor
	// rises from 1 at full speed to SlowBonusMax at SlowBonusAt and below.
	SlowBonusMax = 1.5
	SlowBonusAt  = 0.5

	EasyBelowNPS   = 2.0 // notes per second at full tempo
	MediumBelowNPS = 4.0
	HardBelowNPS   = 7.0

	EasyMult   = 1.0
	MediumMult = 1.25
	HardMult   = 1.5
	ExpertMult = 2.0

	RepeatStep  = 0.15 // XP lost per extra pass of the same section today
	RepeatFloor = 0.25

	// CredibleConfidence is the calibration confidence a session needs for
	// XP and excitement to count.
	CredibleConfidence = 0.5
)

// §4.6 Levels: xpToNext(level) = round(LevelXPBase × level^LevelXPPower).
const (
	LevelXPBase  = 100.0
	LevelXPPower = 1.5
)

// Titles by the first level that earns them.
var titles = []struct {
	from  int
	title string
}{
	{1, "Bedroom Player"},
	{5, "Garage Player"},
	{10, "Open-Mic Regular"},
	{15, "Bar Band"},
	{20, "Club Act"},
	{25, "Touring Musician"},
	{30, "Festival Act"},
	{40, "Headliner"},
}
