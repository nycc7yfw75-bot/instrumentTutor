package career

import (
	"math"
	"math/rand"
	"time"
)

// State is a profile's career (CAREER.md §5.2). It is stored in
// progress.json by the progress package; nothing here touches files.
type State struct {
	Hometown  string                `json:"hometown,omitempty"`
	RouteSeed int64                 `json:"routeSeed,omitempty"`
	Character int                   `json:"character"`
	ToughP    float64               `json:"toughP,omitempty"` // 0 means "not started": ToughStartP
	Gigs      map[string]*GigRecord `json:"gigs,omitempty"`
	Cleared   []int                 `json:"tiersCleared,omitempty"`
	Fame      int                   `json:"fame,omitempty"`
}

// GigRecord is what a profile has done at one gig.
type GigRecord struct {
	Crowd       string   `json:"crowd,omitempty"` // CrowdNormal, CrowdTough, or "" before the first attempt
	BestStars   int      `json:"bestStars"`
	Attempts    int      `json:"attempts"`
	LastSetlist []string `json:"lastSetlist,omitempty"`
}

// Started reports a career set up on first launch (§5.1).
func (s *State) Started() bool { return s.Hometown != "" }

// Setup records the first-launch choices.
func (s *State) Setup(hometown string, character int, seed int64) {
	s.Hometown, s.Character, s.RouteSeed = hometown, character, seed
	if s.ToughP == 0 {
		s.ToughP = ToughStartP
	}
}

// Route is this career's tour.
func (s *State) Route() [][]Gig { return Route(s.Hometown, s.RouteSeed) }

// GigByID finds a gig on this career's route.
func (s *State) GigByID(id string) (Gig, bool) {
	for _, tier := range s.Route() {
		for _, g := range tier {
			if g.ID == id {
				return g, true
			}
		}
	}
	return Gig{}, false
}

func (s *State) record(id string) *GigRecord {
	if s.Gigs == nil {
		s.Gigs = map[string]*GigRecord{}
	}
	r := s.Gigs[id]
	if r == nil {
		r = &GigRecord{}
		s.Gigs[id] = r
	}
	return r
}

// Record is a gig's record (a zero one before the first attempt).
func (s *State) Record(id string) GigRecord {
	if r := s.Gigs[id]; r != nil {
		return *r
	}
	return GigRecord{}
}

// StartGig begins an attempt and returns the gig's crowd. The crowd is
// rolled once, on the first attempt, and kept for good (§3.2). Recording
// sessions have no crowd and are never rolled. rng is the only randomness.
func (s *State) StartGig(g Gig, rng *rand.Rand) (crowd string, firstReveal bool) {
	r := s.record(g.ID)
	r.Attempts++
	if g.Recording {
		return "", false
	}
	if r.Crowd != "" {
		return r.Crowd, false
	}
	if s.ToughP == 0 {
		s.ToughP = ToughStartP
	}
	if rng.Float64() < s.ToughP {
		r.Crowd = CrowdTough
		s.ToughP = ToughStartP
		return r.Crowd, true
	}
	r.Crowd = CrowdNormal
	s.ToughP = math.Min(ToughMaxP, s.ToughP+ToughStepP)
	return r.Crowd, false
}

// ForceTough makes a gig's crowd tough (the HEADLINER_FORCE_TOUGH dev flag).
func (s *State) ForceTough(g Gig) {
	if !g.Recording {
		s.record(g.ID).Crowd = CrowdTough
	}
}

// GigScore is a set's score (§3.1): mostly accuracy, partly excitement.
func GigScore(accuracy, excitementAvg float64) float64 {
	return GigAccuracyWeight*accuracy + GigExcitementWeight*(excitementAvg/100)
}

// Stars is the rating a gig score earns, 0..5.
func Stars(score float64) int {
	n := 0
	for _, t := range StarThresholds {
		if score >= t-1e-9 {
			n++
		}
	}
	return n
}

// Result is a finished gig, as the performance screen reports it.
type Result struct {
	Stars    int
	BooedOff bool
	Setlist  []string
}

// Reward is what a finished gig earned.
type Reward struct {
	BonusXP     int
	Fame        int
	NewBest     bool
	TierCleared int // the tier this gig cleared, or 0
}

// FinishGig files a gig's result. Booed off is 0 stars and no Fame (the XP
// for notes already played is the caller's, and is kept). Fame is paid
// only for an improvement on the gig's best stars, so a cleared gig cannot
// be farmed (§3.3). The bonus XP is paid on completion.
func (s *State) FinishGig(g Gig, res Result) Reward {
	r := s.record(g.ID)
	stars := res.Stars
	if res.BooedOff {
		stars = 0
	}
	if len(res.Setlist) > 0 {
		r.LastSetlist = append([]string(nil), res.Setlist...)
	}
	var out Reward
	if !res.BooedOff {
		out.BonusXP = GigBonusXP * stars * g.Tier
	}
	if stars > r.BestStars {
		mult := 1.0
		if r.Crowd == CrowdTough {
			mult = ToughFameMult
		}
		out.Fame = int(math.Round(float64((stars-r.BestStars)*TierFame[g.Tier-1]) * mult))
		r.BestStars = stars
		out.NewBest = true
		s.Fame += out.Fame
	}
	if !s.isCleared(g.Tier) && s.TierCleared(g.Tier) {
		s.Cleared = append(s.Cleared, g.Tier)
		out.TierCleared = g.Tier
	}
	return out
}

// TierCleared: any ClearGigs gigs of the tier (the recording counts) at
// ClearStars or more.
func (s *State) TierCleared(tier int) bool {
	n := 0
	for slot := 0; slot <= RecordingSlot; slot++ {
		if r := s.Gigs[GigID(tier, slot)]; r != nil && r.BestStars >= ClearStars {
			n++
		}
	}
	return n >= ClearGigs
}

func (s *State) isCleared(tier int) bool {
	for _, t := range s.Cleared {
		if t == tier {
			return true
		}
	}
	return false
}

// TierProgress is "how many gigs of a tier are at 3 stars".
func (s *State) TierProgress(tier int) int {
	n := 0
	for slot := 0; slot <= RecordingSlot; slot++ {
		if r := s.Gigs[GigID(tier, slot)]; r != nil && r.BestStars >= ClearStars {
			n++
		}
	}
	return n
}

// BooedOff reports a tough crowd booing the player off: excitement at 0
// after the song's grace period. A normal crowd never boos anyone off.
func BooedOff(crowd string, songSeconds, excitement float64) bool {
	return crowd == CrowdTough && songSeconds >= ToughGraceSeconds && excitement <= 0
}

// ExcitementStart and DrainMult are a crowd's excitement settings.
func ExcitementStart(crowd string) float64 {
	if crowd == CrowdTough {
		return ToughExcitement
	}
	return NormalExcitement
}

// DrainMult is how much faster a crowd's excitement drains.
func DrainMult(crowd string) float64 {
	if crowd == CrowdTough {
		return ToughDrainMult
	}
	return 1
}

// Mastery is one mastered song+track (§2).
type Mastery struct {
	Date      time.Time `json:"date"`
	Accuracy  float64   `json:"accuracy"`
	TightMean float64   `json:"tightness"`
}

// Masters reports whether a Practice pass masters its song: a full run
// (first bar to last, no loop) at full speed, credible, at 90% or more.
func Masters(fullRun, credible bool, tempoScale, accuracy float64) bool {
	return fullRun && credible && tempoScale >= MasteryTempo && accuracy >= MasteryAccuracy-1e-9
}

// Character is a preset guitarist (§6.3).
type Character struct {
	Name, Look string
}

// Characters are the four presets.
var Characters = []Character{
	{"Rae", "Short dark hair, denim jacket, sunburst guitar"},
	{"Theo", "Long curly hair, band tee, black guitar"},
	{"Mina", "Shaved sides, leather vest, red guitar"},
	{"Sol", "Beanie and flannel, natural-wood guitar"},
}
