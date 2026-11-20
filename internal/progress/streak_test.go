package progress

import (
	"testing"
	"time"
)

var streakNow = time.Date(2026, 11, 21, 20, 0, 0, 0, time.Local)

func streakDay(back int) string { return streakNow.AddDate(0, 0, -back).Format("2006-01-02") }

func TestStreakCountsPracticeDays(t *testing.T) {
	cases := []struct {
		name    string
		days    map[string]float64
		want    int
		today   bool
		freezes int
	}{
		{"nothing", nil, 0, false, 2},
		{"today only", map[string]float64{streakDay(0): 300}, 1, true, 2},
		{"three days", map[string]float64{streakDay(0): 400, streakDay(1): 300, streakDay(2): 900}, 3, true, 2},
		// Today not reached yet: the streak still stands on yesterday.
		{"today in progress", map[string]float64{streakDay(0): 120, streakDay(1): 300, streakDay(2): 300}, 2, false, 2},
		// A missed day uses a freeze instead of breaking the streak.
		{"a gap uses a freeze", map[string]float64{streakDay(0): 300, streakDay(2): 300}, 2, true, 1},
		{"under 5 minutes is not a day", map[string]float64{streakDay(1): 299}, 0, false, 2},
	}
	for _, c := range cases {
		st := streakOf(c.days, streakNow)
		if st.Days != c.want || st.TodayDone != c.today || st.Freezes != c.freezes {
			t.Errorf("%s: %+v; want %d days (today %v), %d freezes", c.name, st, c.want, c.today, c.freezes)
		}
	}
}

func TestStreakBreaksPastTheFreezesAndRestartsAtOne(t *testing.T) {
	// Played 10, 9 and 8 days ago, then nothing until today: 7 missed days,
	// far more than the 2 freezes.
	days := map[string]float64{streakDay(10): 300, streakDay(9): 300, streakDay(8): 300, streakDay(0): 300}
	st := streakOf(days, streakNow)
	if st.Days != 1 {
		t.Errorf("after a broken streak today counts as %d, want a fresh 1 (never 0)", st.Days)
	}
	if st.Longest != 3 || st.TotalDays != 4 {
		t.Errorf("longest %d, total %d; want 3 and 4 (they never go down)", st.Longest, st.TotalDays)
	}
}

func TestSevenDaysInARowEarnAFreeze(t *testing.T) {
	days := map[string]float64{}
	for i := 0; i < 7; i++ {
		days[streakDay(i)] = 300
	}
	if st := streakOf(days, streakNow); st.Freezes != 3 || st.Days != 7 {
		t.Errorf("seven days in a row: %+v, want 7 days and 3 freezes (the cap)", st)
	}
}

func TestStreakIgnoresAClockRunningBehind(t *testing.T) {
	days := map[string]float64{streakDay(0): 300, streakDay(1): 300}
	behind := streakNow.AddDate(0, 0, -5)
	if st := streakOf(days, behind); st.Days != 2 || st.TotalDays != 2 {
		t.Errorf("a clock 5 days behind gave %+v; it should change nothing", st)
	}
}

func TestUnscoredPracticeCountsTowardTheStreak(t *testing.T) {
	s, err := OpenFile(t.TempDir() + "/progress.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProfile("Me"); err != nil {
		t.Fatal(err)
	}
	// Five minutes of playing with nothing judged (detection off).
	if _, err := s.RecordPass(Pass{SongID: "x", Seconds: 300}); err != nil {
		t.Fatal(err)
	}
	if st := s.Streak(); st.Days != 1 || !st.TodayDone {
		t.Errorf("5 unscored minutes gave %+v, want a 1-day streak", st)
	}
}
