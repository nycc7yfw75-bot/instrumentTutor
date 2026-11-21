package ui

import (
	"errors"
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
