package scoring

import (
	"github.com/S95F/musicTutor/internal/practice"
	"github.com/S95F/musicTutor/internal/score"
)

// Session accumulates one practice session on one track: it turns the
// practice scorer's results into judgements, a chord's notes into one,
// and keeps the pass and run totals and the excitement bar.
type Session struct {
	sampleRate int
	credible   bool
	chordSize  map[int64]int // notes sounded together at a start tick
	pending    map[int64][]practice.NoteResult

	pass, run  Summary
	Excitement Excitement
}

// NewSession starts scoring track of sc. credible is the credibility gate
// (see Credible): an uncredible session still totals accuracy, but its
// excitement does not move.
func NewSession(sc *score.Score, track, sampleRate int, credible bool) *Session {
	s := &Session{
		sampleRate: sampleRate,
		credible:   credible,
		chordSize:  map[int64]int{},
		pending:    map[int64][]practice.NoteResult{},
		Excitement: NewExcitement(),
	}
	for _, ev := range sc.Events() {
		if ev.Track == track {
			s.chordSize[ev.Start]++
		}
	}
	return s
}

// Credible reports whether this session's XP and excitement count.
func (s *Session) Credible() bool { return s.credible }

// Add takes one result from the practice scorer and returns the judgements
// it completes: one for a single note, none while a chord's other notes
// are still to come, one for the chord once they have all arrived.
func (s *Session) Add(r practice.NoteResult) []Judgement {
	start := r.Event.Start
	if s.chordSize[start] <= 1 {
		return s.record(Judge(r, s.sampleRate))
	}
	s.pending[start] = append(s.pending[start], r)
	if len(s.pending[start]) < s.chordSize[start] {
		return nil
	}
	members := s.pending[start]
	delete(s.pending, start)
	return s.record(s.judgeChord(members))
}

// Flush judges chords still waiting for some of their notes (at the end
// of a pass), from what was heard.
func (s *Session) Flush() []Judgement {
	var out []Judgement
	for start, members := range s.pending {
		delete(s.pending, start)
		out = append(out, s.record(s.judgeChord(members))...)
	}
	return out
}

// judgeChord makes one judgement of a chord (§4.1): a hit when every
// judgeable string rang, a miss when none did, close otherwise; timed from
// the strum, which every matched string of the chord shares.
func (s *Session) judgeChord(members []practice.NoteResult) Judgement {
	var hits, misses, judgeable int
	var strum *practice.NoteResult
	for i := range members {
		r := &members[i]
		if Judge(*r, s.sampleRate).Excluded {
			continue
		}
		judgeable++
		switch r.Verdict {
		case practice.VerdictHit:
			hits++
		case practice.VerdictMiss:
			misses++
		}
		if strum == nil && r.Timed {
			strum = r
		}
	}
	if judgeable == 0 {
		return Judgement{Event: members[0].Event, Excluded: true}
	}
	chord := members[0]
	chord.Event.Tech = 0 // the chord as a whole; its excluded strings were skipped above
	switch {
	case misses == judgeable:
		chord.Verdict, chord.Matched, chord.Timed = practice.VerdictMiss, false, false
	case hits == judgeable:
		chord.Verdict = practice.VerdictHit
	default:
		chord.Verdict = practice.VerdictClose
	}
	if chord.Verdict != practice.VerdictMiss {
		chord.Matched = true
		chord.Timed = strum != nil
		if strum != nil {
			chord.OnsetErrFrames = strum.OnsetErrFrames
		}
	}
	return Judge(chord, s.sampleRate)
}

func (s *Session) record(j Judgement) []Judgement {
	s.pass.Add(j)
	s.run.Add(j)
	if s.credible {
		s.Excitement.Note(j)
	}
	return []Judgement{j}
}

// Tick drains excitement for seconds of transport time (see
// Excitement.Tick); an uncredible session's bar does not move.
func (s *Session) Tick(seconds float64, t Transport) {
	if s.credible {
		s.Excitement.Tick(seconds, t)
	}
}

// EndPass returns the pass just completed (a loop pass, or a run to the
// end) and starts the next.
func (s *Session) EndPass() Summary {
	p := s.pass
	s.pass = Summary{}
	return p
}

// Run is everything judged since the session started.
func (s *Session) Run() Summary { return s.run }
