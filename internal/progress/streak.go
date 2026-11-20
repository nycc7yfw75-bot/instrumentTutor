package progress

import "time"

// The practice streak (owner's ground rule 2): it counts practice time,
// not accuracy. A day with at least StreakDaySeconds of the song actually
// playing counts, scored or not, listening or not.

// StreakDaySeconds is how much playing makes a day count: 5 minutes.
const StreakDaySeconds = 5 * 60

// Streak is the player's run of practice days.
type Streak struct {
	Days      int     // days in a row that counted, up to today
	TodaySecs float64 // playing time so far today
	TodayDone bool    // today already counts
}

// Streak is the active profile's streak now.
func (s *Store) Streak() Streak {
	s.mu.Lock()
	defer s.mu.Unlock()
	prof := s.active()
	if prof == nil {
		return Streak{}
	}
	return streakOf(prof.Practice.Days, s.now())
}

// streakOf counts back from today. Today not counting yet does not break
// the streak: it runs to yesterday until today is over.
func streakOf(days map[string]float64, now time.Time) Streak {
	day := now.Local()
	key := func(t time.Time) string { return t.Format("2006-01-02") }
	st := Streak{TodaySecs: days[key(day)]}
	st.TodayDone = st.TodaySecs >= StreakDaySeconds
	if !st.TodayDone {
		day = day.AddDate(0, 0, -1)
	}
	for days[key(day)] >= StreakDaySeconds {
		st.Days++
		day = day.AddDate(0, 0, -1)
	}
	return st
}
