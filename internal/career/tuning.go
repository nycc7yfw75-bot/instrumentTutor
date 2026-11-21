// Package career holds the career tour's rules (docs/CAREER.md): mastery,
// tiers and their requirements, the route through cities with fictional
// venues, gig ratings, tough crowds and Fame. It is pure: no UI, audio,
// engine or file access; randomness comes in through a seeded source.
package career

// Every number the career uses, in one place (CAREER.md §3–4).

// §2 Mastery: one full credible run at full speed with this accuracy.
const (
	MasteryAccuracy = 0.90
	MasteryTempo    = 0.999 // "100% tempo", with float slack
)

// §3.1 Gig rating.
const (
	GigAccuracyWeight   = 0.75
	GigExcitementWeight = 0.25
)

// StarThresholds: a gig score at or above StarThresholds[i] earns i+1 stars.
var StarThresholds = [5]float64{0.60, 0.70, 0.80, 0.88, 0.94}

// §3.2 Tough crowds.
const (
	ToughStartP         = 0.20 // the odds a first attempt rolls a tough crowd
	ToughStepP          = 0.10 // added after every normal roll
	ToughMaxP           = 1.00
	ToughExcitement     = 30.0 // where excitement starts (normal: 50)
	ToughDrainMult      = 1.5
	ToughGraceSeconds   = 10.0 // per song, before booing can happen
	NormalExcitement    = 50.0
	BooLowExcitement    = 15.0 // the crowd crosses its arms below this
	CrowdNormal         = "normal"
	CrowdTough          = "tough"
	BetweenSongsSeconds = 4.0
)

// §3.3 Rewards.
const (
	GigBonusXP    = 50 // × stars × tier number
	ToughFameMult = 1.5
)

// TierFame is the Fame per star at each tier (index 0 = Bedroom).
var TierFame = [5]int{10, 20, 40, 80, 160}

// §4.1 Tier clearing: any ClearGigs gigs of a tier at ClearStars or more.
const (
	ClearGigs     = 3
	ClearStars    = 3
	GigsPerTier   = 4 // plus one recording session
	RecordingSlot = GigsPerTier
)
