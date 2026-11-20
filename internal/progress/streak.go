package progress

import (
	"sort"
	"time"
)

// The practice streak (owner's ground rule 2, docs/PROGRESS.md invariant
// 6). It counts practice time, never accuracy: a day with at least
// StreakDaySeconds of the song actually playing counts, scored or not,
// listening or not. It forgives: a missed day uses a freeze instead of
// breaking the streak (two to start, one more for every seven days in a
// row, at most three). Past the freezes the streak starts again at 1 on the
// next day played, never 0. Total days practised and the longest streak
// only ever grow. A clock that reads earlier than the last day played
// changes nothing.

// StreakDaySeconds is how much playing makes a day count: 5 minutes.
const StreakDaySeconds = 5 * 60

const (
	streakFreezesStart = 2
	streakFreezesMax   = 3
	streakFreezeEvery  = 7 // days in a row that earn a freeze back
)

// Streak is the player's practice record.
type Streak struct {
	Days      int     // the current streak, in days played
	Longest   int     // the longest streak ever
	TotalDays int     // every day that counted, ever
	Freezes   int     // missed days the streak can still absorb
	TodaySecs float64 // playing time so far today
	TodayDone bool    // today already counts
}

// Streak is the active profile's streak now.
func (s *Store) Streak() Streak {
	s.mu.Lock()
	defer s.mu.Unlock()
	prof := s.active()
	if prof == nil {
		return Streak{Freezes: streakFreezesStart}
	}
	return streakOf(prof.Practice.Days, s.now())
}

// streakOf replays the practice days up to now. It is a pure function of
// the day log, which only grows, so its totals cannot go down.
func streakOf(days map[string]float64, now time.Time) Streak {
	var played []string
	for d, secs := range days {
		if secs >= StreakDaySeconds {
			played = append(played, d)
		}
	}
	sort.Strings(played)
	st := Streak{Freezes: streakFreezesStart, TotalDays: len(played)}
	today := now.Local().Format(dayFormat)
	st.TodaySecs = days[today]
	st.TodayDone = st.TodaySecs >= StreakDaySeconds
	if len(played) == 0 {
		return st
	}
	if last := played[len(played)-1]; today < last {
		today = last // a clock running behind punishes nothing
	}
	isPlayed := make(map[string]bool, len(played))
	for _, d := range played {
		isPlayed[d] = true
	}
	day, err := time.ParseInLocation(dayFormat, played[0], time.Local)
	if err != nil {
		return st
	}
	run := 0 // days in a row toward the next freeze
	for {
		key := day.Format(dayFormat)
		switch {
		case isPlayed[key]:
			st.Days++
			run++
			if run == streakFreezeEvery {
				st.Freezes, run = min(streakFreezesMax, st.Freezes+1), 0
			}
		case key == today:
			// Today is not over: not playing yet is not a miss.
		case st.Days > 0 && st.Freezes > 0:
			st.Freezes-- // a missed day, absorbed
			run = 0
		default:
			st.Days, run = 0, 0 // broken: the next day played starts at 1
		}
		st.Longest = max(st.Longest, st.Days)
		if key >= today {
			break
		}
		day = time.Date(day.Year(), day.Month(), day.Day()+1, 12, 0, 0, 0, time.Local)
	}
	return st
}
