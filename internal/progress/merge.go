package progress

import (
	"encoding/json"
	"math"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/S95F/musicTutor/internal/career"
)

// Merging a profile that changed on two devices (ONLINE.md §B4). The rules
// are deterministic and never delete anything:
//
//	XP, Fame, level        the larger
//	mastered songs         union; the earlier date wins
//	gig best stars         the larger
//	gig crowd type         the first recorded (the server's) wins
//	tough-crowd odds p     from the side with more gig rolls
//	pass / battle logs     union by ID, capped
//	best-pass replays      the higher accuracy
//	battle records         rebuilt: both sides' counts less the battles
//	                       both logs share
//
// Everything else keeps the larger or earlier value, or unions.
// local is this device's copy, remote the server's.

// MergeProfiles returns the merge of local and remote; neither is changed.
func MergeProfiles(local, remote *Profile) *Profile {
	switch {
	case local == nil && remote == nil:
		return nil
	case local == nil:
		return cloneProfile(remote)
	case remote == nil:
		return cloneProfile(local)
	}
	a, b := cloneProfile(local), cloneProfile(remote)
	m := a // start from the local copy (name, tint, today's counts)
	if m.Name == "" {
		m.Name = b.Name
	}
	if !b.Created.IsZero() && (m.Created.IsZero() || b.Created.Before(m.Created)) {
		m.Created = b.Created
	}
	if b.XP.TotalXP > m.XP.TotalXP {
		m.XP = b.XP
	}
	m.Level = m.XP.Level()

	// Bests: the best of each.
	m.Bests = mergeMap(a.Bests, b.Bests, func(x, y *Best) *Best {
		out := *x
		out.Accuracy = math.Max(x.Accuracy, y.Accuracy)
		out.Tempo = math.Max(x.Tempo, y.Tempo)
		if y.TightSet && (!x.TightSet || math.Abs(y.TightMean) < math.Abs(x.TightMean)) {
			out.TightMean, out.TightSet = y.TightMean, true
		}
		return &out
	})
	if b.Daily.Day > m.Daily.Day {
		m.Daily = b.Daily
	}
	m.Passes = mergePasses(a.Passes, b.Passes)
	m.Replays = mergeMap(a.Replays, b.Replays, func(x, y *Replay) *Replay {
		if y.Accuracy > x.Accuracy {
			return y
		}
		return x
	})
	m.Practice.Days = mergeMap(a.Practice.Days, b.Practice.Days, math.Max)
	m.Practice.Songs = mergeMap(a.Practice.Songs, b.Practice.Songs, math.Max)
	m.Songs = mergeMap(a.Songs, b.Songs, func(x, _ SongInfo) SongInfo { return x })
	m.Badges = mergeMap(a.Badges, b.Badges, earlier)
	m.Totals = mergeTotals(a.Totals, b.Totals)
	m.Career = mergeCareer(a.Career, b.Career)
	m.Mastered = mergeMap(a.Mastered, b.Mastered, func(x, y career.Mastery) career.Mastery {
		if y.Date.Before(x.Date) {
			return y
		}
		return x
	})
	m.Battles = mergeBattles(a.Battles, b.Battles)
	return m
}

func earlier(x, y time.Time) time.Time {
	if y.Before(x) {
		return y
	}
	return x
}

// mergeMap unions two maps, combining keys both have with both(local, remote).
func mergeMap[K comparable, V any](a, b map[K]V, both func(V, V) V) map[K]V {
	if len(a) == 0 && len(b) == 0 {
		return a
	}
	out := make(map[K]V, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if x, ok := out[k]; ok {
			out[k] = both(x, v)
		} else {
			out[k] = v
		}
	}
	return out
}

func mergePasses(a, b []PassLog) []PassLog {
	seen := map[string]bool{}
	var out []PassLog
	for _, p := range append(append([]PassLog(nil), a...), b...) {
		k := p.key()
		if !seen[k] {
			seen[k] = true
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	if len(out) > maxPassLog {
		out = out[len(out)-maxPassLog:]
	}
	return out
}

// key identifies a pass: when it ended, on what.
func (p PassLog) key() string {
	return p.Date.UTC().Format(time.RFC3339Nano) + "|" + p.Song + "|" + p.Bars + "|" + strconv.Itoa(p.Track)
}

func mergeTotals(a, b Totals) Totals {
	out := a
	out.SlowSecs = math.Max(a.SlowSecs, b.SlowSecs)
	out.ClickSecs = math.Max(a.ClickSecs, b.ClickSecs)
	out.CleanSongs = mergeMap(a.CleanSongs, b.CleanSongs, func(x, y bool) bool { return x || y })
	out.Mastery = mergeMap(a.Mastery, b.Mastery, func(x, y int) int { return max(x, y) })
	out.RampLow = mergeMap(a.RampLow, b.RampLow, math.Min)
	return out
}

func mergeCareer(a, b career.State) career.State {
	out := a
	if !out.Started() && b.Started() {
		out.Hometown, out.RouteSeed, out.Character = b.Hometown, b.RouteSeed, b.Character
	}
	out.Fame = max(a.Fame, b.Fame)
	if rolls(b) > rolls(a) {
		out.ToughP = b.ToughP
	}
	out.Gigs = mergeMap(a.Gigs, b.Gigs, func(x, y *career.GigRecord) *career.GigRecord {
		g := *x
		g.BestStars = max(x.BestStars, y.BestStars)
		g.Attempts = max(x.Attempts, y.Attempts)
		if y.Crowd != "" { // the server's first recorded crowd stands
			g.Crowd = y.Crowd
		}
		if y.Attempts > x.Attempts {
			g.LastSetlist = y.LastSetlist
		}
		return &g
	})
	for _, t := range b.Cleared {
		if !slices.Contains(out.Cleared, t) {
			out.Cleared = append(out.Cleared, t)
		}
	}
	slices.Sort(out.Cleared)
	return out
}

// rolls is how many gigs were played: each attempt rolled the crowd.
func rolls(s career.State) int {
	n := 0
	for _, g := range s.Gigs {
		n += g.Attempts
	}
	return n
}

func mergeBattles(a, b BattleRecord) BattleRecord {
	seen := map[string]bool{}
	var log []BattleLog
	for _, l := range append(append([]BattleLog(nil), a.Log...), b.Log...) {
		if k := l.logKey(); !seen[k] {
			seen[k] = true
			log = append(log, l)
		}
	}
	sort.SliceStable(log, func(i, j int) bool { return log[i].Date.Before(log[j].Date) })
	if len(log) > maxBattleLog {
		log = log[len(log)-maxBattleLog:]
	}
	// The record is the merged log's battles plus the older ones: for each
	// side, what it counted beyond the part of its log still in the merged
	// log; the larger of the two (both sides share their older history).
	inFinal := map[string]bool{}
	for _, l := range log {
		inFinal[l.logKey()] = true
	}
	kept := func(r BattleRecord) []BattleLog {
		var out []BattleLog
		for _, l := range r.Log {
			if inFinal[l.logKey()] {
				out = append(out, l)
			}
		}
		return out
	}
	aT, aOpp := countLog(kept(a))
	bT, bOpp := countLog(kept(b))
	old := maxTotals(subTotals(a.Totals, aT), subTotals(b.Totals, bT))
	oldOpp := mergeMap(diffOpp(a.PerOpponent, aOpp), diffOpp(b.PerOpponent, bOpp), maxTotals)
	logT, logOpp := countLog(log)
	out := BattleRecord{Log: log, Best: max(a.Best, b.Best)}
	out.Totals = addTotals(old, logT)
	out.PerOpponent = mergeMap(oldOpp, logOpp, addTotals)
	if len(out.PerOpponent) == 0 {
		out.PerOpponent = nil
	}
	return out
}

// countLog counts the scored battles in a log, overall and per opponent.
func countLog(log []BattleLog) (BattleTotals, map[string]BattleTotals) {
	var t BattleTotals
	opp := map[string]BattleTotals{}
	for _, l := range log {
		if !l.Scored {
			continue
		}
		t.add(l.Outcome)
		for k, o := range l.Against {
			x := opp[k]
			x.add(o)
			opp[k] = x
		}
	}
	return t, opp
}

func addTotals(x, y BattleTotals) BattleTotals {
	return BattleTotals{Wins: x.Wins + y.Wins, Losses: x.Losses + y.Losses, Draws: x.Draws + y.Draws}
}

func subTotals(x, y BattleTotals) BattleTotals {
	return BattleTotals{Wins: max(0, x.Wins-y.Wins), Losses: max(0, x.Losses-y.Losses), Draws: max(0, x.Draws-y.Draws)}
}

func maxTotals(x, y BattleTotals) BattleTotals {
	return BattleTotals{Wins: max(x.Wins, y.Wins), Losses: max(x.Losses, y.Losses), Draws: max(x.Draws, y.Draws)}
}

func diffOpp(all, logged map[string]BattleTotals) map[string]BattleTotals {
	out := map[string]BattleTotals{}
	for k, v := range all {
		if d := subTotals(v, logged[k]); d != (BattleTotals{}) {
			out[k] = d
		}
	}
	return out
}

// cloneProfile deep-copies through JSON (what is synced is what is saved).
func cloneProfile(p *Profile) *Profile {
	b, err := json.Marshal(p)
	if err != nil {
		return p
	}
	var out Profile
	if err := json.Unmarshal(b, &out); err != nil {
		return p
	}
	return &out
}
