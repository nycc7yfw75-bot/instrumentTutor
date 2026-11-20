package progress

import (
	"fmt"
	"sort"
	"time"
)

// Badges (docs/PROGRESS.md §Badges; owner's choice 2026-11-21). They are
// set-only: a criterion is a threshold over counters and best-ever values,
// evaluation only ever sets a badge, and nothing can take one away. Half
// need no detection at all, so they are earned with listening off.

// BadgeDef is one badge: its name and how it is earned, in plain words.
type BadgeDef struct {
	ID, Name, How string
	// Detection: earned only from scored (credible) passes.
	Detection bool
}

// Badge IDs.
const (
	BadgeSlowClean   = "slow-clean"
	BadgeUpToSpeed   = "up-to-speed"
	BadgeMastered    = "section-mastered"
	BadgeNailedIt    = "nailed-it"
	BadgeRepertoire  = "repertoire"
	BadgeFiveMore    = "five-more-minutes"
	BadgeSpeedLimit  = "under-the-speed-limit"
	BadgeClickFriend = "friend-of-the-click"
	BadgeSevenDays   = "seven-days"
	BadgeMonth       = "a-month"
	BadgeBackAtIt    = "back-at-it"
)

// Badges is the catalog, in the order the start screen shows it.
var Badges = []BadgeDef{
	{BadgeSlowClean, "Slow and Clean", "A scored pass at 70% speed or slower with 95% accuracy or more", true},
	{BadgeUpToSpeed, "Up to Speed", "Ramp a section up by 30% or more to full speed, staying at 90% accuracy", true},
	{BadgeMastered, "Section Mastered", "Three scored passes of one loop in a row at full speed, each 95% or more", true},
	{BadgeNailedIt, "Nailed It", "Play a whole song at full speed with 98% accuracy", true},
	{BadgeRepertoire, "Repertoire", "A clean scored pass (95% or more) on five different songs", true},
	{BadgeFiveMore, "Five More Minutes", "20 minutes of playing in one sitting", false},
	{BadgeSpeedLimit, "Under the Speed Limit", "60 minutes in total at 75% speed or slower", false},
	{BadgeClickFriend, "Friend of the Click", "60 minutes in total with the metronome on", false},
	{BadgeSevenDays, "Seven Days", "A 7-day practice streak", false},
	{BadgeMonth, "A Month", "A 30-day practice streak", false},
	{BadgeBackAtIt, "Back at It", "Come back to practise after two weeks or more away", false},
}

// BadgeByID looks a badge up in the catalog.
func BadgeByID(id string) (BadgeDef, bool) {
	for _, b := range Badges {
		if b.ID == id {
			return b, true
		}
	}
	return BadgeDef{}, false
}

// Badge thresholds.
const (
	slowCleanTempo   = 0.7
	cleanAccuracy    = 0.95
	rampSpan         = 0.3
	rampAccuracy     = 0.9
	masteredRun      = 3
	nailedAccuracy   = 0.98
	repertoireSongs  = 5
	fiveMoreSecs     = 20 * 60
	speedLimitTempo  = 0.75
	speedLimitSecs   = 60 * 60
	clickFriendSecs  = 60 * 60
	backAtItGapDays  = 14
	fullSpeed        = 0.999
	sevenDays, month = 7, 30
)

// Totals are the counters the badges read. Each only grows, except the
// per-section runs, which feed set-only badges.
type Totals struct {
	SlowSecs   float64         `json:"slowSecs,omitempty"`  // playing at <= 75% speed
	ClickSecs  float64         `json:"clickSecs,omitempty"` // playing with the metronome on
	CleanSongs map[string]bool `json:"cleanSongs,omitempty"`
	// Mastery: a section's run of full-speed scored passes at >= 95%.
	Mastery map[string]int `json:"mastery,omitempty"`
	// RampLow: the slowest tempo in a section's current run of scored
	// passes at >= 90%, for Up to Speed.
	RampLow map[string]float64 `json:"rampLow,omitempty"`
}

// EarnedBadge is a badge the player has, and when they earned it.
type EarnedBadge struct {
	BadgeDef
	At time.Time
}

// Earned lists the active profile's badges, newest first.
func (s *Store) Earned() []EarnedBadge {
	s.mu.Lock()
	defer s.mu.Unlock()
	prof := s.active()
	if prof == nil {
		return nil
	}
	var out []EarnedBadge
	for id, at := range prof.Badges {
		if d, ok := BadgeByID(id); ok {
			out = append(out, EarnedBadge{d, at})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].At.Equal(out[j].At) {
			return out[i].At.After(out[j].At)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// award sets a badge, once; it reports whether it is new.
func award(prof *Profile, id string, now time.Time) bool {
	if prof.Badges == nil {
		prof.Badges = map[string]time.Time{}
	}
	if _, ok := prof.Badges[id]; ok {
		return false
	}
	prof.Badges[id] = now
	return true
}

// evalBadges updates the counters with a pass and awards what it earns.
// firstToday is whether this is the day's first playing time; lastBefore
// the previous day anything was played ("" for none).
func (s *Store) evalBadges(prof *Profile, p Pass, now time.Time, lastBefore string, firstToday bool) []string {
	var earned []string
	give := func(id string) {
		if award(prof, id, now) {
			earned = append(earned, id)
		}
	}
	t := &prof.Totals

	// Time-based: no detection needed.
	t.SlowSecs += p.SlowSeconds
	t.ClickSecs += p.ClickSeconds
	s.sessionSecs += p.Seconds
	if s.sessionSecs >= fiveMoreSecs {
		give(BadgeFiveMore)
	}
	if t.SlowSecs >= speedLimitSecs {
		give(BadgeSpeedLimit)
	}
	if t.ClickSecs >= clickFriendSecs {
		give(BadgeClickFriend)
	}
	st := streakOf(prof.Practice.Days, now)
	if st.Days >= sevenDays {
		give(BadgeSevenDays)
	}
	if st.Days >= month {
		give(BadgeMonth)
	}
	if firstToday && lastBefore != "" {
		if last, err := time.ParseInLocation(dayFormat, lastBefore, time.Local); err == nil {
			if dayDiff(last, now.Local()) >= backAtItGapDays {
				give(BadgeBackAtIt)
			}
		}
	}

	// Scored passes only.
	if !p.Credible || p.Summary.Judged < minBadgeNotes {
		return earned
	}
	acc := p.Summary.Accuracy()
	section := fmt.Sprintf("%s#%d#%d-%d", p.SongID, p.Track, p.FromBar, p.ToBar)
	if p.TempoScale <= slowCleanTempo+1e-9 && acc >= cleanAccuracy {
		give(BadgeSlowClean)
	}
	if acc >= cleanAccuracy {
		if t.CleanSongs == nil {
			t.CleanSongs = map[string]bool{}
		}
		t.CleanSongs[p.SongID] = true
		if len(t.CleanSongs) >= repertoireSongs {
			give(BadgeRepertoire)
		}
	}
	if t.Mastery == nil {
		t.Mastery = map[string]int{}
	}
	if p.TempoScale >= fullSpeed && acc >= cleanAccuracy {
		t.Mastery[section]++
		if t.Mastery[section] >= masteredRun {
			give(BadgeMastered)
		}
	} else {
		t.Mastery[section] = 0
	}
	if p.FullRun && p.TempoScale >= fullSpeed && acc >= nailedAccuracy {
		give(BadgeNailedIt)
	}
	if t.RampLow == nil {
		t.RampLow = map[string]float64{}
	}
	if acc >= rampAccuracy {
		low, ok := t.RampLow[section]
		if !ok || p.TempoScale < low {
			low = p.TempoScale
			t.RampLow[section] = low
		}
		if p.TempoScale >= fullSpeed && 1-low >= rampSpan-1e-9 {
			give(BadgeUpToSpeed)
		}
	} else {
		delete(t.RampLow, section)
	}
	return earned
}

// minBadgeNotes: a scored pass shorter than this proves nothing.
const minBadgeNotes = 8

const dayFormat = "2006-01-02"

// dayDiff is the number of calendar days from a to b.
func dayDiff(a, b time.Time) int {
	da := time.Date(a.Year(), a.Month(), a.Day(), 12, 0, 0, 0, time.Local)
	db := time.Date(b.Year(), b.Month(), b.Day(), 12, 0, 0, 0, time.Local)
	return int((db.Sub(da).Hours() + 12) / 24)
}
