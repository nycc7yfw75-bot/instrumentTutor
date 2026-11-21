package scoring

import (
	"testing"

	"github.com/S95F/musicTutor/internal/practice"
)

func TestExcitementStartsAtFiftyAndGainsByTier(t *testing.T) {
	cases := []struct {
		r    practice.NoteResult
		gain float64
	}{
		{result(practice.VerdictHit, 0), 4},
		{result(practice.VerdictHit, 50), 3},
		{result(practice.VerdictHit, 120), 2},
		{result(practice.VerdictClose, 0), 1},
		{result(practice.VerdictClose, 120), 1},
		{result(practice.VerdictMiss, 0), 0},
	}
	for _, c := range cases {
		e := NewExcitement()
		e.Note(Judge(c.r, sr))
		if got := e.Value - ExcitementStart; got != c.gain {
			t.Errorf("verdict %v at %+d frames: +%.0f, want +%.0f", c.r.Verdict, c.r.OnsetErrFrames, got, c.gain)
		}
	}
}

func TestExcitementStreakBonusAtTenAndTwenty(t *testing.T) {
	e := NewExcitement()
	e.Value = 0
	for i := 1; i <= 20; i++ {
		before := e.Value
		e.Note(Judge(result(practice.VerdictClose, 0), sr)) // +1 each
		gain := e.Value - before
		want := 1.0
		if i == 10 || i == 20 {
			want = 1 + StreakBonus
		}
		if gain != want {
			t.Errorf("note %d gained %.0f, want %.0f", i, gain, want)
		}
	}
	// A miss breaks the streak: the bonus waits another ten notes.
	e.Note(Judge(result(practice.VerdictMiss, 0), sr))
	for i := 1; i <= 9; i++ {
		e.Note(Judge(result(practice.VerdictClose, 0), sr))
	}
	if e.streak != 9 {
		t.Errorf("streak after a miss and nine notes = %d, want 9", e.streak)
	}
}

func TestExcitementCapsAtZeroAndHundred(t *testing.T) {
	e := NewExcitement()
	for i := 0; i < 40; i++ {
		e.Note(Judge(result(practice.VerdictHit, 0), sr))
	}
	if e.Value != ExcitementMax {
		t.Errorf("after forty perfect hits: %.0f, want capped at 100", e.Value)
	}
	e.Tick(1000, Playing)
	if e.Value != 0 {
		t.Errorf("after a long drain: %.0f, want floored at 0", e.Value)
	}
}

func TestExcitementDrainsOnlyWhilePlaying(t *testing.T) {
	for _, st := range []Transport{Paused, Waiting, CountingIn} {
		e := NewExcitement()
		e.Tick(10, st)
		if e.Value != ExcitementStart {
			t.Errorf("%v for 10 s drained to %.0f; want frozen at 50", st, e.Value)
		}
	}
	e := NewExcitement()
	e.Tick(2, Playing)
	if e.Value != ExcitementStart-2*DrainPerSecond {
		t.Errorf("2 s playing: %.0f, want %.0f", e.Value, ExcitementStart-2*DrainPerSecond)
	}
}

func TestMissesNeverLowerExcitement(t *testing.T) {
	e := NewExcitement()
	e.Note(Judge(result(practice.VerdictMiss, 0), sr))
	if e.Value != ExcitementStart {
		t.Errorf("a miss moved excitement to %.0f; misses never subtract", e.Value)
	}
}

func TestExcitementBands(t *testing.T) {
	for v, want := range map[float64]Band{0: Cold, 24.9: Cold, 25: Warm, 49: Warm, 50: Hot, 74: Hot, 75: OnFire, 100: OnFire} {
		if got := (Excitement{Value: v}).Band(); got != want {
			t.Errorf("excitement %.1f is %v, want %v", v, got, want)
		}
	}
}

func TestAToughCrowdStartsLowerAndDrainsFaster(t *testing.T) {
	e := NewExcitementAt(30, 1.5)
	e.Tick(2, Playing)
	if want := 30 - DrainPerSecond*1.5*2; e.Value != want {
		t.Errorf("after 2 s a tough crowd is at %.1f, want %.1f", e.Value, want)
	}
	n := NewExcitement()
	n.Tick(2, Playing)
	if n.Value != ExcitementStart-DrainPerSecond*2 {
		t.Errorf("a normal crowd drains at %.1f", ExcitementStart-n.Value)
	}
}
