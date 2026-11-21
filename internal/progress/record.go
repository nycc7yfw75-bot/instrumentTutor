package progress

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/S95F/musicTutor/internal/career"
	"github.com/S95F/musicTutor/internal/scoring"
)

// Pass is one finished pass, as the practice view hands it over: a loop
// wrap, the song's end, or the player pausing (docs/SCORING.md §10).
type Pass struct {
	SongID, Title, Path string
	Track               int
	FromBar, ToBar      int // the bars played, as numbered on screen
	TempoScale          float64
	Difficulty          scoring.Difficulty
	Summary             scoring.Summary
	Judgements          []scoring.Judgement
	NoteIndex           map[NoteKey]int // NoteIndex(sc, Track), for the replay
	Credible            bool            // the credibility gate (scoring.Credible) and not wait mode
	Seconds             float64         // time the song was playing during the pass
	SlowSeconds         float64         // of those, at 75% speed or slower
	ClickSeconds        float64         // of those, with the metronome on
	FullRun             bool            // the pass ran the whole song, start to end
}

// Recorded is what a pass earned.
type Recorded struct {
	XP        int
	LeveledUp bool
	Logged    bool     // a pass long enough to judge (>= 8 notes) went into the history
	NewBest   bool     // it set the song track's best accuracy
	NewBadges []string // badges this pass earned (IDs)
	Mastered  bool     // this pass mastered its song (CAREER.md §2), for the first time
}

// RecordPass files a finished pass under the active profile and saves. It
// always counts the practice time. A pass that judged at least 8 notes is
// logged; when credible it also earns XP and may set bests and the replay.
// An uncredible one is logged as Unscored with 0 XP.
func (s *Store) RecordPass(p Pass) (Recorded, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prof := s.active()
	if prof == nil {
		return Recorded{}, fmt.Errorf("progress: no profile yet")
	}
	now := s.now()
	var out Recorded

	if prof.Songs == nil {
		prof.Songs = map[string]SongInfo{}
	}
	prof.Songs[p.SongID] = SongInfo{Title: p.Title, Path: p.Path, Difficulty: p.Difficulty.String()}
	today := now.Local().Format(dayFormat)
	firstToday, lastBefore := prof.Practice.Days[today] == 0 && p.Seconds > 0, lastDayBefore(prof.Practice.Days, today)
	if p.Seconds > 0 {
		if prof.Practice.Days == nil {
			prof.Practice.Days, prof.Practice.Songs = map[string]float64{}, map[string]float64{}
		}
		prof.Practice.Days[today] += p.Seconds
		prof.Practice.Songs[p.SongID] += p.Seconds
	}
	out.NewBadges = s.evalBadges(prof, p, now, lastBefore, firstToday)
	if p.Summary.Judged >= scoring.MinJudgedNotes && career.Masters(p.FullRun, p.Credible, p.TempoScale, p.Summary.Accuracy()) {
		key := fmt.Sprintf("%s#%d", p.SongID, p.Track)
		if prof.Mastered == nil {
			prof.Mastered = map[string]career.Mastery{}
		}
		if _, had := prof.Mastered[key]; !had {
			prof.Mastered[key] = career.Mastery{Date: now, Accuracy: p.Summary.Accuracy(), TightMean: p.Summary.Tightness().MeanMs}
			out.Mastered = true
		}
	}

	if p.Summary.Judged >= scoring.MinJudgedNotes {
		bars := fmt.Sprintf("%d-%d", p.FromBar, p.ToBar)
		section := fmt.Sprintf("%s#%d#%s", p.SongID, p.Track, bars)
		out.XP = scoring.AwardXP(scoring.PassInput{
			Summary:    p.Summary,
			Difficulty: p.Difficulty,
			TempoScale: p.TempoScale,
			Section:    section,
			Credible:   p.Credible,
		}, &prof.Daily, now)
		out.LeveledUp = prof.XP.Add(out.XP)
		prof.Level = prof.XP.Level()

		tt := p.Summary.Tightness()
		prof.Passes = append(prof.Passes, PassLog{
			Date: now, Song: p.SongID, Title: p.Title, Track: p.Track, Bars: bars,
			Tempo: p.TempoScale, Judged: p.Summary.Judged, Accuracy: p.Summary.Accuracy(),
			TightMean: tt.MeanMs, TightSpread: tt.SpreadMs, XP: out.XP,
			Unscored: !p.Credible, Seconds: p.Seconds,
		})
		if n := len(prof.Passes); n > maxPassLog {
			prof.Passes = append([]PassLog(nil), prof.Passes[n-maxPassLog:]...)
		}
		out.Logged = true
		if p.Credible {
			out.NewBest = updateBest(prof, fmt.Sprintf("%s#%d", p.SongID, p.Track), p, tt)
			keepReplay(prof, section, p, now)
		}
	}
	return out, s.saveLocked()
}

func updateBest(prof *Profile, key string, p Pass, tt scoring.Tightness) (newBest bool) {
	if prof.Bests == nil {
		prof.Bests = map[string]*Best{}
	}
	b := prof.Bests[key]
	if b == nil {
		b = &Best{}
		prof.Bests[key] = b
	}
	if acc := p.Summary.Accuracy(); acc > b.Accuracy {
		b.Accuracy, newBest = acc, true
	}
	if tt.N > 0 && (!b.TightSet || math.Abs(tt.MeanMs) < math.Abs(b.TightMean)) {
		b.TightMean, b.TightSet = tt.MeanMs, true
	}
	b.Tempo = max(b.Tempo, p.TempoScale)
	return newBest
}

// keepReplay stores the pass as the section's replay when it is the most
// accurate yet, and keeps at most maxReplays, dropping the least recently
// used.
func keepReplay(prof *Profile, section string, p Pass, now time.Time) {
	if prof.Replays == nil {
		prof.Replays = map[string]*Replay{}
	}
	acc := p.Summary.Accuracy()
	if old := prof.Replays[section]; old != nil && old.Accuracy >= acc {
		old.LastUsed = now
		return
	}
	r := &Replay{Accuracy: acc, Tempo: p.TempoScale, LastUsed: now}
	for _, j := range p.Judgements {
		if j.Excluded {
			continue
		}
		idx, ok := p.NoteIndex[NoteKey{j.Event.Start, j.Event.String}]
		if !ok {
			continue
		}
		r.Notes = append(r.Notes, ReplayNote{Index: idx, Verdict: int(j.Verdict), OffsetMs: j.OffsetMs, Timed: j.Timed()})
	}
	prof.Replays[section] = r
	if len(prof.Replays) > maxReplays {
		keys := make([]string, 0, len(prof.Replays))
		for k := range prof.Replays {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(a, b int) bool { return prof.Replays[keys[a]].LastUsed.Before(prof.Replays[keys[b]].LastUsed) })
		for _, k := range keys[:len(prof.Replays)-maxReplays] {
			delete(prof.Replays, k)
		}
	}
}

// Replay returns a copy of the active profile's stored best pass for a song
// track's bar range, marking it used.
func (s *Store) Replay(songID string, track, fromBar, toBar int) (Replay, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prof := s.active()
	if prof == nil {
		return Replay{}, false
	}
	r := prof.Replays[fmt.Sprintf("%s#%d#%d-%d", songID, track, fromBar, toBar)]
	if r == nil {
		return Replay{}, false
	}
	r.LastUsed = s.now()
	cp := *r
	cp.Notes = append([]ReplayNote(nil), r.Notes...)
	return cp, true
}

// lastDayBefore is the latest day before today with any playing, or "".
func lastDayBefore(days map[string]float64, today string) string {
	last := ""
	for d, secs := range days {
		if secs > 0 && d < today && d > last {
			last = d
		}
	}
	return last
}
