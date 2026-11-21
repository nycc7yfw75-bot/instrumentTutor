package battle

import (
	"math"
	"math/rand"
	"sort"

	"github.com/S95F/musicTutor/internal/practice"
	"github.com/S95F/musicTutor/internal/score"
	"github.com/S95F/musicTutor/internal/scoring"
)

// The bots (BATTLES.md §5). A bot's turn is a made-up set of results for
// the phrase's notes, drawn from its profile with a seeded source, then
// judged by the very same scoring functions a human's notes go through.

// BotProfile is how a bot plays.
type BotProfile struct {
	HitP, CloseP, MissP float64 // sum to 1
	BiasMs, SpreadMs    float64 // timing offset ~ Normal(bias, spread), clamped to ±150
	SpeedChoke          float64 // extra miss chance per note/second above 4
	ChordPenalty        float64 // extra miss chance on a chord
}

// chokeFrom is the speed above which a bot starts to choke (notes/second).
const chokeFrom = 4.0

// Rival is a named rival (all original characters).
type Rival struct {
	Name     string
	Unlock   int // the player's level that unlocks the rival
	Bio      string
	Portrait string // the pixel portrait's name (the UI draws it)
	Profile  BotProfile
}

// Rivals is the roster, in unlock order (§5.2).
var Rivals = []Rival{
	{"Basement Benny", 1, "Drags, sloppy, loves it anyway", "benny", BotProfile{0.60, 0.20, 0.20, 40, 55, 0.04, 0.06}},
	{"Rusher Rex", 1, "Rushes everything, apologises for nothing", "rex", BotProfile{0.70, 0.15, 0.15, -35, 40, 0.04, 0.06}},
	{"Metronome Mira", 3, "Dead on time, misses notes", "mira", BotProfile{0.72, 0.18, 0.10, 0, 20, 0.04, 0.06}},
	{"Velvet Vik", 6, "Laid back, behind the beat on purpose", "vik", BotProfile{0.80, 0.12, 0.08, 15, 30, 0.03, 0.05}},
	{"Clockwork Kai", 10, "Machine-tight, chokes on chords", "kai", BotProfile{0.85, 0.10, 0.05, 0, 15, 0.03, 0.18}},
	{"Shredder Sol", 15, "Fast, and the speed doesn't rattle", "sol", BotProfile{0.88, 0.08, 0.04, -20, 25, 0.01, 0.05}},
	{"Ghost-Note Gwen", 20, "Clean, rarely rattled", "gwen", BotProfile{0.92, 0.06, 0.02, 5, 15, 0.015, 0.03}},
	{"The Headliner", 30, "The final boss", "headliner", BotProfile{0.96, 0.03, 0.01, 0, 10, 0.01, 0.02}},
}

// RivalNamed looks a rival up.
func RivalNamed(name string) (Rival, bool) {
	for _, r := range Rivals {
		if r.Name == name {
			return r, true
		}
	}
	return Rival{}, false
}

// Unlocked reports a rival the player's level has unlocked.
func (r Rival) Unlocked(level int) bool { return level >= r.Unlock }

// SessionLevels are the Session Player's settings and the rivals whose
// playing they borrow (§5.2).
var SessionLevels = []struct {
	Name  string
	Rival string
}{
	{"Easy", "Basement Benny"},
	{"Medium", "Velvet Vik"},
	{"Hard", "Ghost-Note Gwen"},
	{"Expert", "The Headliner"},
}

// SessionProfile is the Session Player's profile at a level name.
func SessionProfile(level string) (BotProfile, bool) {
	for _, s := range SessionLevels {
		if s.Name == level {
			r, ok := RivalNamed(s.Rival)
			return r.Profile, ok
		}
	}
	return BotProfile{}, false
}

// phraseGroups are a phrase's judgeable notes grouped by start (a chord
// is one group, judged once), in order.
func phraseGroups(sc *score.Score, track int, ph Phrase) [][]score.NoteEvent {
	by := map[int64][]score.NoteEvent{}
	for _, ev := range sc.Events() {
		if ev.Track == track && ev.Start >= ph.FromTick && ev.Start < ph.ToTick && scoring.Judgeable(ev) {
			by[ev.Start] = append(by[ev.Start], ev)
		}
	}
	starts := make([]int64, 0, len(by))
	for s := range by {
		starts = append(starts, s)
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i] < starts[j] })
	out := make([][]score.NoteEvent, len(starts))
	for i, s := range starts {
		out[i] = by[s]
	}
	return out
}

// BotTurn plays a phrase as a bot: one result per note group, verdicts
// and timing drawn from the profile, judged by scoring.Judge. The same
// source and inputs always give the same turn.
func BotTurn(p BotProfile, sc *score.Score, track int, ph Phrase, rng *rand.Rand, sampleRate int) (scoring.Summary, []scoring.Judgement) {
	groups := phraseGroups(sc, track, ph)
	secs := sc.Tempos.TimeAt(ph.ToTick) - sc.Tempos.TimeAt(ph.FromTick)
	nps := 0.0
	if secs > 0 {
		nps = float64(len(groups)) / secs
	}
	var sum scoring.Summary
	var js []scoring.Judgement
	for _, g := range groups {
		miss := p.MissP + p.SpeedChoke*math.Max(0, nps-chokeFrom)
		if len(g) > 1 {
			miss += p.ChordPenalty
		}
		miss = math.Min(0.95, miss)
		rest := p.HitP + p.CloseP
		hit := 0.0
		if rest > 0 {
			hit = (1 - miss) * p.HitP / rest // renormalised
		}
		v := practice.VerdictClose
		switch roll := rng.Float64(); {
		case roll < miss:
			v = practice.VerdictMiss
		case roll < miss+hit:
			v = practice.VerdictHit
		}
		off := math.Max(-150, math.Min(150, p.BiasMs+rng.NormFloat64()*p.SpreadMs))
		r := practice.NoteResult{Event: g[0], Verdict: v, Matched: v != practice.VerdictMiss, Timed: v != practice.VerdictMiss,
			OnsetErrFrames: int64(math.Round(off * float64(sampleRate) / 1000))}
		j := scoring.Judge(r, sampleRate)
		sum.Add(j)
		js = append(js, j)
	}
	return sum, js
}

// GhostNote is one note of a stored best pass: which note of the track (its
// index among the track's notes in order), how it went and when.
type GhostNote struct {
	Index    int
	Verdict  int // practice.Verdict
	OffsetMs float64
	Timed    bool
}

// GhostTurn replays a stored pass's notes that fall in the phrase. It has
// no randomness; ok is false when the pass does not cover the phrase.
// index maps each of the track's notes (start, string) to its index.
func GhostTurn(notes []GhostNote, index map[[2]int64]int, sc *score.Score, track int, ph Phrase) (scoring.Summary, []scoring.Judgement, bool) {
	byIdx := make(map[int]GhostNote, len(notes))
	for _, n := range notes {
		byIdx[n.Index] = n
	}
	var sum scoring.Summary
	var js []scoring.Judgement
	for _, g := range phraseGroups(sc, track, ph) {
		ev := g[0]
		i, ok := index[[2]int64{ev.Start, int64(ev.String)}]
		if !ok {
			return sum, js, false
		}
		n, ok := byIdx[i]
		if !ok {
			return sum, js, false // the stored pass never played this note
		}
		v := practice.Verdict(n.Verdict)
		r := practice.NoteResult{Event: ev, Verdict: v, Matched: v != practice.VerdictMiss, Timed: n.Timed}
		j := scoring.Judge(r, 1000)
		if n.Timed && v != practice.VerdictMiss {
			j.OffsetMs = n.OffsetMs
			j.Tier = scoring.TierFor(n.OffsetMs)
			j = rescore(j, v)
		}
		sum.Add(j)
		js = append(js, j)
	}
	return sum, js, true
}

// rescore recomputes a judgement's score after its timing tier changed.
func rescore(j scoring.Judgement, v practice.Verdict) scoring.Judgement {
	r := practice.NoteResult{Event: j.Event, Verdict: v, Matched: true, Timed: true,
		OnsetErrFrames: int64(math.Round(j.OffsetMs))}
	out := scoring.Judge(r, 1000) // 1000 frames a second: frames are milliseconds
	return out
}
