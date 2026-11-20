package progress

import (
	"testing"
	"time"
)

func TestStreakCountsPracticeDays(t *testing.T) {
	now := time.Date(2026, 11, 21, 20, 0, 0, 0, time.Local)
	day := func(back int) string { return now.AddDate(0, 0, -back).Format("2006-01-02") }
	cases := []struct {
		name  string
		days  map[string]float64
		want  int
		today bool
	}{
		{"nothing", nil, 0, false},
		{"today only", map[string]float64{day(0): 300}, 1, true},
		{"three days", map[string]float64{day(0): 400, day(1): 300, day(2): 900}, 3, true},
		// Today not reached yet: the streak still stands on yesterday.
		{"today in progress", map[string]float64{day(0): 120, day(1): 300, day(2): 300}, 2, false},
		{"a gap breaks it", map[string]float64{day(0): 300, day(2): 300}, 1, true},
		{"under 5 minutes is not a day", map[string]float64{day(1): 299}, 0, false},
	}
	for _, c := range cases {
		st := streakOf(c.days, now)
		if st.Days != c.want || st.TodayDone != c.today {
			t.Errorf("%s: %d days (today %v), want %d (today %v)", c.name, st.Days, st.TodayDone, c.want, c.today)
		}
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
