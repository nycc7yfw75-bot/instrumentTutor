package battle

import (
	"errors"
	"fmt"
	"math"

	"github.com/S95F/musicTutor/internal/score"
	"github.com/S95F/musicTutor/internal/scoring"
)

// Kind is what a participant is (§1).
type Kind int

const (
	Human   Kind = iota // a local profile
	Rival               // a named rival bot
	Ghost               // a profile's stored best pass
	Session             // the difficulty slider bot
)

// IsBot reports a participant the computer plays.
func (k Kind) IsBot() bool { return k != Human }

// Participant is one player in a battle.
type Participant struct {
	Name       string
	Kind       Kind
	ID         string  // a profile's id, a rival's name, a session level
	TempoScale float64 // a human's handicap tempo, fixed for the battle
	Tint       int     // a human's profile tint
}

// Tempo is the participant's tempo: a human's chosen handicap, else full.
func (p Participant) Tempo() float64 {
	if p.Kind == Human && p.TempoScale > 0 {
		return math.Max(HumanTempoMin, math.Min(HumanTempoMax, p.TempoScale))
	}
	return 1
}

// Material is what is battled: a track's bar range.
type Material struct {
	Score    *score.Score
	Track    int
	Title    string
	SongID   string
	Path     string
	StartBar int // index into the track's bars
	EndBar   int // exclusive; 0 or less means to the end
}

// Setup is a battle as chosen at setup (§4.1).
type Setup struct {
	Material     Material
	PhraseBars   int
	Exchanges    int
	Participants []Participant
}

// Phrase is one exchange's bars.
type Phrase struct {
	Index            int
	FromBar, ToBar   int   // bar indexes, ToBar exclusive
	FromTick, ToTick int64 // the phrase's ticks, ToTick exclusive
	Judgeable        int   // notes (a chord counts once) the scorer can judge
}

// Plan is a setup laid out into phrases.
type Plan struct {
	Phrases   []Phrase
	Shortened bool     // fewer exchanges than asked, to fit the section
	Warnings  []string // plain words, shown at setup
}

// Errors a setup can have.
var (
	ErrPlayers  = errors.New("a battle needs 2 to 4 players")
	ErrTooShort = errors.New("the section is shorter than one phrase")
	ErrNoTrack  = errors.New("pick a track to battle on")
)

// NewPlan splits the section into phrases: phrase k covers bars
// start + k·phraseBars. A section too short for every exchange gets fewer
// exchanges, and says so; a phrase with too few judgeable notes warns.
func NewPlan(s Setup) (Plan, error) {
	var p Plan
	if n := len(s.Participants); n < MinParticipants || n > MaxParticipants {
		return p, ErrPlayers
	}
	m := s.Material
	if m.Score == nil || m.Track < 0 || m.Track >= len(m.Score.Tracks) {
		return p, ErrNoTrack
	}
	bars := m.Score.Tracks[m.Track].Bars
	end := min(m.EndBar, len(bars))
	if end <= m.StartBar {
		end = len(bars)
	}
	pb := max(MinPhraseBars, min(MaxPhraseBars, s.PhraseBars))
	ex := max(MinExchanges, min(MaxExchanges, s.Exchanges))
	have := (end - m.StartBar) / pb
	if m.StartBar < 0 || have < 1 {
		return p, ErrTooShort
	}
	if have < ex {
		p.Shortened = true
		p.Warnings = append(p.Warnings, fmt.Sprintf("The section has room for %d %s of %d bars, so the battle has %d %s instead of %d.",
			have, plural(have, "phrase", "phrases"), pb, have, plural(have, "exchange", "exchanges"), ex))
		ex = have
	}
	for k := 0; k < ex; k++ {
		from := m.StartBar + k*pb
		ph := Phrase{Index: k, FromBar: from, ToBar: from + pb, FromTick: bars[from].Start}
		last := bars[from+pb-1]
		ph.ToTick = last.Start + last.Len()
		ph.Judgeable = judgeableIn(m.Score, m.Track, ph.FromTick, ph.ToTick)
		if ph.Judgeable < MinJudgeable {
			p.Warnings = append(p.Warnings, fmt.Sprintf("Phrase %d has only %d notes the app can judge; longer phrases make fairer turns.", k+1, ph.Judgeable))
		}
		p.Phrases = append(p.Phrases, ph)
	}
	return p, nil
}

// judgeableIn counts the judgeable notes in a tick range (a chord once).
func judgeableIn(sc *score.Score, track int, from, to int64) int {
	starts := map[int64]bool{}
	for _, ev := range sc.Events() {
		if ev.Track == track && ev.Start >= from && ev.Start < to && scoring.Judgeable(ev) {
			starts[ev.Start] = true
		}
	}
	return len(starts)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// TurnOrder is the order players go in exchange k: the setup order first,
// rotated by one each exchange so nobody always goes first.
func TurnOrder(players, exchange int) []int {
	out := make([]int, players)
	for i := range out {
		out[i] = (i + exchange) % players
	}
	return out
}

// HandicapFactor is how much a tempo counts for: √tempo (60% → 0.77).
func HandicapFactor(tempo float64) float64 { return math.Sqrt(math.Max(0, math.Min(tempo, 1))) }

// TurnScore is a turn's score: 1000 × accuracy × √tempo, rounded.
func TurnScore(accuracy, tempo float64) int {
	return int(math.Round(TurnScoreScale * accuracy * HandicapFactor(tempo)))
}

// Turn is one participant's go at one phrase.
type Turn struct {
	Player     int
	Exchange   int
	Summary    scoring.Summary
	Judgements []scoring.Judgement // the notes, for the tab and a ghost
	Tempo      float64
	Scored     bool // judged: credible for a human; always for a bot
	Score      int
}

// NewTurn scores a turn. An unscored turn has no score at all: a score is
// never shown for a turn that was not judged (§0).
func NewTurn(player, exchange int, sum scoring.Summary, tempo float64, scored bool) Turn {
	t := Turn{Player: player, Exchange: exchange, Summary: sum, Tempo: tempo, Scored: scored}
	if scored {
		t.Score = TurnScore(sum.Accuracy(), tempo)
	}
	return t
}

// Battle is a battle in progress, or finished.
type Battle struct {
	Setup Setup
	Plan  Plan
	turns map[[2]int]Turn // (exchange, player)
}

// New starts a battle.
func New(s Setup) (*Battle, error) {
	p, err := NewPlan(s)
	if err != nil {
		return nil, err
	}
	return &Battle{Setup: s, Plan: p, turns: map[[2]int]Turn{}}, nil
}

// Players is how many play.
func (b *Battle) Players() int { return len(b.Setup.Participants) }

// Exchanges is how many exchanges the battle has.
func (b *Battle) Exchanges() int { return len(b.Plan.Phrases) }

// Record files a turn.
func (b *Battle) Record(t Turn) { b.turns[[2]int{t.Exchange, t.Player}] = t }

// Turn is a filed turn.
func (b *Battle) Turn(exchange, player int) (Turn, bool) {
	t, ok := b.turns[[2]int{exchange, player}]
	return t, ok
}

// Next is the next turn to play: its exchange and player, in turn order;
// ok is false once every turn is in.
func (b *Battle) Next() (exchange, player int, ok bool) {
	for k := 0; k < b.Exchanges(); k++ {
		for _, p := range TurnOrder(b.Players(), k) {
			if _, done := b.Turn(k, p); !done {
				return k, p, true
			}
		}
	}
	return 0, 0, false
}

// Totals are each player's scores so far.
func (b *Battle) Totals() []int {
	out := make([]int, b.Players())
	for _, t := range b.turns {
		out[t.Player] += t.Score
	}
	return out
}

// Leader is the player ahead on points, or -1 when the top is shared or
// nobody has scored.
func (b *Battle) Leader() int {
	best, who := 0, -1
	for i, t := range b.Totals() {
		switch {
		case t > best:
			best, who = t, i
		case t == best && t > 0:
			who = -1
		}
	}
	return who
}

// Result is how a battle ended.
type Result struct {
	Totals   []int
	Winner   int    // -1 for a draw, or when there can be no winner
	Unscored bool   // a human turn was not judged: no winner (§3)
	TieBreak string // "" or how a tie at the top was broken
}

// Result decides the battle (§4.2): the highest total wins; a tie goes to
// the smaller average timing offset, then the smaller spread, then it is
// a draw. Any unscored human turn means no winner.
func (b *Battle) Result() Result {
	r := Result{Totals: b.Totals(), Winner: -1}
	for _, t := range b.turns {
		if !t.Scored && b.Setup.Participants[t.Player].Kind == Human {
			r.Unscored = true
			return r
		}
	}
	best := -1
	var top []int
	for i, tot := range r.Totals {
		switch {
		case tot > best:
			best, top = tot, []int{i}
		case tot == best:
			top = append(top, i)
		}
	}
	if len(top) == 1 {
		r.Winner = top[0]
		return r
	}
	tight := make([]scoring.Tightness, b.Players())
	for i := range tight {
		var sum scoring.Summary
		for k := 0; k < b.Exchanges(); k++ {
			if t, ok := b.Turn(k, i); ok {
				sum.Merge(t.Summary)
			}
		}
		tight[i] = sum.Tightness()
	}
	pick := func(key func(i int) float64) []int {
		low := math.Inf(1)
		var keep []int
		for _, i := range top {
			v := math.Round(key(i)*10) / 10 // tenths of a millisecond
			switch {
			case v < low:
				low, keep = v, []int{i}
			case v == low:
				keep = append(keep, i)
			}
		}
		return keep
	}
	if top = pick(func(i int) float64 { return math.Abs(tight[i].MeanMs) }); len(top) == 1 {
		r.Winner, r.TieBreak = top[0], "Tied on points: the tighter average timing wins"
		return r
	}
	if top = pick(func(i int) float64 { return tight[i].SpreadMs }); len(top) == 1 {
		r.Winner, r.TieBreak = top[0], "Tied on points and timing: the steadier timing wins"
		return r
	}
	r.TieBreak = "A draw: tied on points and on timing"
	return r
}

// Crowd2 is the two-player crowd meter after the exchanges played so far:
// positive leans to player 0, negative to player 1. Each exchange moves it
// by clamp((s0 − s1) / 1000 × 1.5, ±0.4); it stays within ±1. Visual only.
func (b *Battle) Crowd2() float64 {
	c := 0.0
	for k := 0; k < b.Exchanges(); k++ {
		t0, ok0 := b.Turn(k, 0)
		t1, ok1 := b.Turn(k, 1)
		if !ok0 || !ok1 {
			break
		}
		c = CrowdStep(c, t0.Score, t1.Score)
	}
	return c
}

// CrowdStep moves the two-player meter by one exchange.
func CrowdStep(c float64, s0, s1 int) float64 {
	step := float64(s0-s1) / TurnScoreScale * CrowdSwing
	return math.Max(-1, math.Min(1, c+math.Max(-CrowdStepMax, math.Min(CrowdStepMax, step))))
}

// CrowdShares is the 3–4 player crowd bar: each player's share of the
// running totals (even shares before anyone scores).
func (b *Battle) CrowdShares() []float64 {
	tot := b.Totals()
	sum := 0
	for _, t := range tot {
		sum += t
	}
	out := make([]float64, len(tot))
	for i, t := range tot {
		if sum == 0 {
			out[i] = 1 / float64(len(tot))
		} else {
			out[i] = float64(t) / float64(sum)
		}
	}
	return out
}

// Bonuses are the battle's XP bonuses per player (§8.1): the winner +25
// for each other player; a draw +10 each; nothing for losing, never
// negative; nothing without a winner or a draw.
func (r Result) Bonuses() []int {
	out := make([]int, len(r.Totals))
	switch {
	case r.Unscored:
	case r.Winner >= 0:
		out[r.Winner] = WinBonusPer * (len(r.Totals) - 1)
	default:
		for i := range out {
			out[i] = DrawBonus
		}
	}
	return out
}
