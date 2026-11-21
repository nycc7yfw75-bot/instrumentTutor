package career

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/S95F/musicTutor/internal/scoring"
)

func TestMasteryNeedsAFullCredibleRunAtFullSpeed(t *testing.T) {
	cases := []struct {
		name            string
		full, credible  bool
		tempo, accuracy float64
		want            bool
	}{
		{"full run at 100% with 90%", true, true, 1, 0.90, true},
		{"a loop, not the whole song", false, true, 1, 0.99, false},
		{"unscored", true, false, 1, 0.99, false},
		{"at 95% speed", true, true, 0.95, 0.99, false},
		{"89% accuracy", true, true, 1, 0.89, false},
	}
	for _, c := range cases {
		if got := Masters(c.full, c.credible, c.tempo, c.accuracy); got != c.want {
			t.Errorf("%s: mastered %v, want %v", c.name, got, c.want)
		}
	}
}

// riggedRand is a source whose Float64 returns the values given, in turn.
type riggedRand struct{ vals []float64 }

func (r *riggedRand) Int63() int64 {
	v := r.vals[0]
	r.vals = r.vals[1:]
	return int64(v * (1 << 63))
}
func (r *riggedRand) Seed(int64) {}

func TestToughCrowdOddsRiseAndReset(t *testing.T) {
	var s State
	s.Setup("Mumbai", 0, 1)
	route := s.Route()
	// Rolls 0.99 (normal) three times: odds go 20, 30, 40, 50.
	rng := rand.New(&riggedRand{vals: []float64{0.99, 0.99, 0.99, 0.0}})
	want := []float64{0.30, 0.40, 0.50}
	for i := 0; i < 3; i++ {
		crowd, _ := s.StartGig(route[0][i], rng)
		if crowd != CrowdNormal {
			t.Fatalf("roll %d came up %q", i, crowd)
		}
		if diff := s.ToughP - want[i]; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("after normal roll %d the odds are %.2f, want %.2f", i, s.ToughP, want[i])
		}
	}
	crowd, reveal := s.StartGig(route[0][3], rng) // rolls 0.0: tough
	if crowd != CrowdTough || !reveal || s.ToughP != ToughStartP {
		t.Errorf("a tough roll gave %q (reveal %v) with odds %.2f, want tough, revealed, back to 0.20", crowd, reveal, s.ToughP)
	}
	// Never rolled twice for one gig.
	again, reveal := s.StartGig(route[0][3], rand.New(rand.NewSource(1)))
	if again != CrowdTough || reveal {
		t.Errorf("a second attempt re-rolled the crowd: %q, reveal %v", again, reveal)
	}
	// Never for a recording session.
	before := s.ToughP
	if crowd, _ := s.StartGig(route[0][RecordingSlot], rand.New(rand.NewSource(1))); crowd != "" || s.ToughP != before {
		t.Errorf("a recording session rolled a crowd (%q) or moved the odds", crowd)
	}
}

func TestGigStars(t *testing.T) {
	for score, want := range map[float64]int{0.59: 0, 0.60: 1, 0.70: 2, 0.79: 2, 0.80: 3, 0.88: 4, 0.94: 5, 1: 5} {
		if got := Stars(score); got != want {
			t.Errorf("score %.2f: %d stars, want %d", score, got, want)
		}
	}
	if got := GigScore(0.9, 60); got < 0.824 || got > 0.826 {
		t.Errorf("gig score for 90%% and excitement 60 is %.3f, want 0.825", got)
	}
}

func TestBooedOffKeepsXPButGivesNoStarsOrFame(t *testing.T) {
	var s State
	s.Setup("London", 0, 7)
	g := s.Route()[2][0]
	s.ForceTough(g)
	r := s.FinishGig(g, Result{Stars: 4, BooedOff: true})
	if r.Fame != 0 || r.BonusXP != 0 || s.Record(g.ID).BestStars != 0 {
		t.Errorf("booed off earned %+v and best %d; want nothing", r, s.Record(g.ID).BestStars)
	}
	if !BooedOff(CrowdTough, 12, 0) || BooedOff(CrowdTough, 9, 0) || BooedOff(CrowdNormal, 60, 0) {
		t.Error("booing: only a tough crowd, only after the 10 s grace, only at 0")
	}
}

func TestFameOnlyForABetterBest(t *testing.T) {
	var s State
	s.Setup("Austin", 0, 3)
	g := s.Route()[1][0] // jam room: 20 Fame a star
	if r := s.FinishGig(g, Result{Stars: 3}); r.Fame != 60 || r.BonusXP != 50*3*2 {
		t.Errorf("first 3-star gig earned %+v, want 60 Fame and 300 XP", r)
	}
	if r := s.FinishGig(g, Result{Stars: 3}); r.Fame != 0 {
		t.Errorf("replaying a 3-star gig at 3 stars earned %d Fame", r.Fame)
	}
	if r := s.FinishGig(g, Result{Stars: 5}); r.Fame != 40 {
		t.Errorf("improving 3 -> 5 stars earned %d Fame, want 40", r.Fame)
	}
	tough := s.Route()[1][1]
	s.ForceTough(tough)
	if r := s.FinishGig(tough, Result{Stars: 2}); r.Fame != 60 {
		t.Errorf("2 stars at a tough jam room earned %d Fame, want 60 (1.5x)", r.Fame)
	}
}

func TestTierClearsWithThreeGigsAtThreeStarsIncludingRecording(t *testing.T) {
	var s State
	s.Setup("Tokyo", 0, 9)
	tier := s.Route()[0]
	s.FinishGig(tier[0], Result{Stars: 3})
	s.FinishGig(tier[1], Result{Stars: 2})
	if s.TierCleared(1) {
		t.Fatal("one gig at 3 stars cleared the Bedroom")
	}
	s.FinishGig(tier[2], Result{Stars: 4})
	r := s.FinishGig(tier[RecordingSlot], Result{Stars: 3})
	if !s.TierCleared(1) || r.TierCleared != 1 {
		t.Errorf("three gigs at 3+ stars (one of them the recording) did not clear the tier: %+v", r)
	}
}

func TestRouteIsStableAndNeverUsesRealVenues(t *testing.T) {
	a, b := Route("Bangalore", 42), Route("Bangalore", 42)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("the same seed gave two routes")
	}
	if reflect.DeepEqual(a, Route("Bangalore", 43)) {
		t.Error("a different seed gave the same route")
	}
	home, _ := CityByName("Bangalore")
	seen := map[string]bool{}
	for ti, tier := range a {
		if len(tier) != GigsPerTier+1 || !tier[RecordingSlot].Recording {
			t.Errorf("tier %d has %d gigs; want 4 and a recording session", ti+1, len(tier))
		}
		for _, g := range tier {
			if isRealVenue(g.Venue) {
				t.Errorf("venue %q uses a real venue's name", g.Venue)
			}
			if seen[g.Venue] {
				t.Errorf("venue %q appears twice", g.Venue)
			}
			seen[g.Venue] = true
			switch {
			case ti < 2 && g.City != home:
				t.Errorf("%s is in %s; the first two tiers are at home", g.Card(), g.City.Name)
			case ti == 2 && g.City.Region != home.Region:
				t.Errorf("pub gig %s is outside the home region", g.Card())
			case ti >= 3 && g.City.Region == home.Region:
				t.Errorf("%s should be in another region", g.Card())
			}
		}
	}
}

func TestRequirementsAndTheNextStep(t *testing.T) {
	var s State
	s.Setup("Dublin", 0, 1)
	if got := s.NextRequirement(1, nil); got != "Master 1 song to play your first gig" {
		t.Errorf("a new player is told %q", got)
	}
	one := []MasteredSong{{scoring.Easy}}
	if !s.Unlocked(Tiers[0], 1, one) {
		t.Error("one mastered song at level 1 should book the Bedroom")
	}
	if s.Unlocked(Tiers[1], 5, []MasteredSong{{}, {}, {}}) {
		t.Error("the Jam room opened before the Bedroom was cleared")
	}
	reqs := s.Requirements(Tiers[3], 20, one)
	met := 0
	for _, r := range reqs {
		if r.Met {
			met++
		}
	}
	if len(reqs) != 4 || met != 1 {
		t.Errorf("Opening act requirements %+v; want 4 with only the level met", reqs)
	}
}
