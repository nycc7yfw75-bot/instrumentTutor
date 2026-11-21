package career

import (
	"fmt"

	"github.com/S95F/musicTutor/internal/scoring"
)

// Tier is one stage of the tour (CAREER.md §4.1).
type Tier struct {
	Num      int // 1..5
	Name     string
	SetSize  int  // songs per gig
	Encore   bool // a stadium set ends with an encore (one more song)
	Level    int  // book it from this level
	Mastered int  // mastered songs needed
	HardPlus int  // of those, Hard or Expert
	Crowd    string
}

// Tiers is the tour, in order.
var Tiers = []Tier{
	{1, "Bedroom", 1, false, 1, 1, 0, "Friends on the floor"},
	{2, "Jam room", 2, false, 5, 3, 0, "Other musicians"},
	{3, "Pub gigs", 3, false, 10, 5, 0, "Bar crowd"},
	{4, "Opening act", 4, false, 20, 8, 2, "Theatre crowd"},
	{5, "Stadium", 6, true, 30, 12, 3, "Stadium crowd"},
}

// TierByNum looks a tier up; ok is false out of range.
func TierByNum(n int) (Tier, bool) {
	if n < 1 || n > len(Tiers) {
		return Tier{}, false
	}
	return Tiers[n-1], true
}

// Songs is the number of songs a gig at this tier plays.
func (t Tier) Songs() int {
	if t.Encore {
		return t.SetSize + 1
	}
	return t.SetSize
}

// Requirement is one condition for booking a tier, and whether it is met.
type Requirement struct {
	Text string
	Met  bool
}

// MasteredSong is what the requirements need to know of a mastered song.
type MasteredSong struct {
	Difficulty scoring.Difficulty
}

// Requirements lists what a tier needs and which are met.
func (s *State) Requirements(t Tier, level int, mastered []MasteredSong) []Requirement {
	hard := 0
	for _, m := range mastered {
		if m.Difficulty >= scoring.Hard {
			hard++
		}
	}
	reqs := []Requirement{
		{fmt.Sprintf("Level %d", t.Level), level >= t.Level},
		{fmt.Sprintf("%d mastered %s", t.Mastered, plural(t.Mastered, "song", "songs")), len(mastered) >= t.Mastered},
	}
	if t.HardPlus > 0 {
		reqs = append(reqs, Requirement{fmt.Sprintf("%d of them Hard or Expert", t.HardPlus), hard >= t.HardPlus})
	}
	if t.Num > 1 {
		prev := Tiers[t.Num-2]
		reqs = append(reqs, Requirement{prev.Name + " cleared", s.TierCleared(prev.Num)})
	}
	return reqs
}

// Unlocked reports whether every requirement of a tier is met.
func (s *State) Unlocked(t Tier, level int, mastered []MasteredSong) bool {
	for _, r := range s.Requirements(t, level, mastered) {
		if !r.Met {
			return false
		}
	}
	return true
}

// CurrentTier is the highest tier the player can book.
func (s *State) CurrentTier(level int, mastered []MasteredSong) Tier {
	cur := Tiers[0]
	for _, t := range Tiers {
		if s.Unlocked(t, level, mastered) {
			cur = t
		}
	}
	return cur
}

// NextRequirement is the first unmet requirement of the next tier, in
// words ("Master 1 more song to play the Jam room"), or "" at the top.
func (s *State) NextRequirement(level int, mastered []MasteredSong) string {
	cur := s.CurrentTier(level, mastered)
	if !s.Unlocked(cur, level, mastered) {
		// Not even the Bedroom yet: it needs a mastered song.
		return fmt.Sprintf("Master %d song to play your first gig", cur.Mastered)
	}
	next, ok := TierByNum(cur.Num + 1)
	if !ok {
		return ""
	}
	hard := 0
	for _, m := range mastered {
		if m.Difficulty >= scoring.Hard {
			hard++
		}
	}
	switch {
	case !s.TierCleared(cur.Num):
		return fmt.Sprintf("Play %d %s gigs at 3 stars to clear it", ClearGigs, cur.Name)
	case len(mastered) < next.Mastered:
		n := next.Mastered - len(mastered)
		return fmt.Sprintf("Master %d more %s to play the %s", n, plural(n, "song", "songs"), next.Name)
	case hard < next.HardPlus:
		n := next.HardPlus - hard
		return fmt.Sprintf("Master %d more Hard %s to play the %s", n, plural(n, "song", "songs"), next.Name)
	case level < next.Level:
		return fmt.Sprintf("Reach level %d to play the %s", next.Level, next.Name)
	}
	return ""
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
