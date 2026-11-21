package ui

import (
	"errors"
	"math"
	"testing"

	"github.com/S95F/musicTutor/internal/career"
	"github.com/S95F/musicTutor/internal/progress"
	"github.com/S95F/musicTutor/internal/scoring"
)

func gigFixture(t *testing.T, crowd string, load GigLoader) (*GigScreen, *progress.Store) {
	t.Helper()
	st := masteredStore(t, 1)
	g := st.Career().Route()[0][0]
	set := st.Mastered()
	return NewGigScreen(nil, st, g, crowd, false, set, load, func() {}, nil), st
}

func TestTheSceneBandLeavesTheTabRoomAtEverySize(t *testing.T) {
	atEachSize(t, func(t *testing.T) {
		s, _ := gigFixture(t, career.CrowdNormal, nil)
		a := newApp(t, 4)
		a.SetGigMode(50, 1)
		s.withBand(func() {
			if gap := a.stringGap(); gap < 36 {
				t.Errorf("with the scene band the strings are %.0f px apart, want at least 36", gap)
			}
		})
		if b := s.bandH(); b > screenH*0.42+1 || b < 80 {
			t.Errorf("the scene band is %.0f px of %.0f", b, screenH)
		}
		if stageBandH != 0 {
			t.Error("the band stayed in the layout after the gig drew")
		}
	})
}

func TestASongThatWillNotOpenStopsTheGigPlainly(t *testing.T) {
	s, _ := gigFixture(t, career.CrowdNormal, func(progress.MasteredSong, bool) (*App, error) {
		return nil, errors.New("file moved")
	})
	s.startSong()
	if s.phase != gigFailed || s.err == "" {
		t.Errorf("phase %v, message %q; want the failure shown", s.phase, s.err)
	}
}

func TestAFinishedGigIsRatedAndFiled(t *testing.T) {
	s, st := gigFixture(t, career.CrowdNormal, nil)
	s.total = scoring.Summary{Judged: 40, SumScore: 39.2} // 98%
	s.exciteSum, s.exciteTime = 90*60, 60                 // excitement 90 on average
	s.finish(false)
	if s.result == nil || !s.result.scored || s.result.stars != 5 {
		t.Fatalf("result %+v; 98%% with excitement 90 scores 0.96: 5 stars", s.result)
	}
	rec := st.Career().Record(s.gig.ID)
	if rec.BestStars != 5 || st.Career().Fame != 50 || s.result.reward.BonusXP != 250 {
		t.Errorf("filed %+v, Fame %d, bonus %d; want 5 stars, 50 Fame, 250 XP", rec, st.Career().Fame, s.result.reward.BonusXP)
	}
}

func TestBooedOffFilesZeroStarsAndOffersARetry(t *testing.T) {
	s, st := gigFixture(t, career.CrowdTough, nil)
	s.total = scoring.Summary{Judged: 20, SumScore: 19}
	s.finish(true)
	if s.phase != gigBooed || s.result.stars != 0 || st.Career().Fame != 0 {
		t.Errorf("phase %v, %+v, Fame %d; booed off is 0 stars and no Fame", s.phase, s.result, st.Career().Fame)
	}
	if s.stage.mood != moodBooed {
		t.Error("the stage does not show the boo")
	}
	attempts := st.Career().Record(s.gig.ID).Attempts
	s.retry()
	if s.phase != gigIntro || st.Career().Record(s.gig.ID).Attempts != attempts+1 || s.crowd != career.CrowdTough {
		t.Errorf("a retry should start again, count an attempt and keep the tough crowd")
	}
}

func TestAnUnscoredSetEarnsNoRating(t *testing.T) {
	s, st := gigFixture(t, career.CrowdNormal, nil)
	s.total = scoring.Summary{Judged: 40, SumScore: 40}
	s.credible = false
	s.finish(false)
	if s.result.scored || s.result.stars != 0 || st.Career().Record(s.gig.ID).BestStars != 0 {
		t.Errorf("an uncalibrated set was rated: %+v", s.result)
	}
}

func TestTheGigTakesTheKeysAwayFromPractice(t *testing.T) {
	a := newApp(t, 4)
	a.eng.SetTempoScale(0.5)
	a.SetGigMode(30, 1.5)
	if a.eng.TempoScale() != 1 {
		t.Error("a gig is played at full speed")
	}
	if e := a.gigExcitement(); e.Value != 30 || e.DrainMult != 1.5 {
		t.Errorf("a tough gig's excitement starts at %v draining x%v", e.Value, e.DrainMult)
	}
}

func TestTheIntroBringsTheLightsUpAndWalksTheGuitaristOn(t *testing.T) {
	s, _ := gigFixture(t, career.CrowdNormal, nil)
	at := func(sec float64) {
		s.phaseT = sec
		s.choreoIntro(0.016)
	}
	at(0.1)
	if s.stage.lightLevel != 0 || s.stage.walkX != -16 {
		t.Errorf("at 0.1 s: lights %.2f, guitarist at %.0f; the stage starts dark, the guitarist off it", s.stage.lightLevel, s.stage.walkX)
	}
	at(0.5)
	if l := s.stage.lightLevel; l <= 0 || l >= 1 || l*4 != math.Floor(l*4) {
		t.Errorf("at 0.5 s the lights are at %.3f; they come up in four steps", l)
	}
	at(1.0)
	if x := s.stage.walkX; x <= -16 || x >= 140 {
		t.Errorf("at 1.0 s the guitarist is at %.1f, want walking on", x)
	}
	at(1.5)
	if !s.cheered || s.stage.walkX != 0 || s.stage.crowdPose != "jump" {
		t.Errorf("at 1.5 s: cheered %v, x %.0f, crowd %q; the guitarist is on the mark and the crowd cheers", s.cheered, s.stage.walkX, s.stage.crowdPose)
	}
}

func TestTheEndCardWaitsForTheBowAndSpaceSkipsIt(t *testing.T) {
	s, _ := gigFixture(t, career.CrowdNormal, nil)
	s.total = scoring.Summary{Judged: 40, SumScore: 36}
	s.exciteSum, s.exciteTime = 70*60, 60
	s.finish(false)
	if ready := s.endReadyAt(); ready > 2.5 {
		t.Errorf("the end sequence takes %.2f s; CAREER.md §9 allows 2.5", ready)
	}
	if got := rolled(100, endCardAt+5*starStep+durRoll/2, endCardAt+5*starStep); got <= 0 || got >= 100 {
		t.Errorf("halfway through the roll the XP reads %d", got)
	}
	if rolled(100, 0, 1) != 0 || rolled(100, 9, 1) != 100 {
		t.Error("the roll should start at 0 and end on the number")
	}
}

func TestClearingATierShowsTheMomentBeforeTheTour(t *testing.T) {
	s, st := gigFixture(t, career.CrowdNormal, nil)
	st.UpdateCareer(func(c *career.State) {
		r := c.Route()[0]
		c.FinishGig(r[1], career.Result{Stars: 3})
		c.FinishGig(r[2], career.Result{Stars: 3})
	})
	s.total = scoring.Summary{Judged: 40, SumScore: 40}
	s.exciteSum, s.exciteTime = 90*60, 60
	s.finish(false)
	if s.result.reward.TierCleared != 1 {
		t.Fatalf("the third 3-star gig did not clear the Bedroom: %+v", s.result.reward)
	}
	if err := s.backToTour(); err != nil || s.phase != gigTierCleared {
		t.Errorf("Back to tour went to phase %v (err %v); the tier-cleared moment comes first", s.phase, err)
	}
}
