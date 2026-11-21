package progress

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/S95F/musicTutor/internal/career"
)

// The active profile's career and mastered songs (CAREER.md §2, §5).

// Career is a copy of the active profile's career.
func (s *Store) Career() career.State {
	s.mu.Lock()
	defer s.mu.Unlock()
	prof := s.active()
	if prof == nil {
		return career.State{}
	}
	c := prof.Career
	c.Gigs = map[string]*career.GigRecord{}
	for k, v := range prof.Career.Gigs {
		r := *v
		c.Gigs[k] = &r
	}
	c.Cleared = append([]int(nil), prof.Career.Cleared...)
	return c
}

// UpdateCareer changes the active profile's career and saves.
func (s *Store) UpdateCareer(fn func(*career.State)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prof := s.active()
	if prof == nil {
		return fmt.Errorf("progress: no profile yet")
	}
	fn(&prof.Career)
	return s.saveLocked()
}

// AddXP credits XP outside a pass (a gig's bonus); it reports a level up.
func (s *Store) AddXP(xp int) (leveledUp bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prof := s.active()
	if prof == nil || xp <= 0 {
		return false, nil
	}
	leveledUp = prof.XP.Add(xp)
	prof.Level = prof.XP.Level()
	return leveledUp, s.saveLocked()
}

// MasteredSong is one mastered song+track.
type MasteredSong struct {
	SongID string
	Track  int
	Title  string
	Path   string
	career.Mastery
}

// Mastered lists the active profile's mastered songs.
func (s *Store) Mastered() []MasteredSong {
	s.mu.Lock()
	defer s.mu.Unlock()
	prof := s.active()
	if prof == nil {
		return nil
	}
	var out []MasteredSong
	for key, m := range prof.Mastered {
		i := strings.LastIndex(key, "#")
		if i < 0 {
			continue
		}
		track, _ := strconv.Atoi(key[i+1:])
		id := key[:i]
		info := prof.Songs[id]
		out = append(out, MasteredSong{SongID: id, Track: track, Title: info.Title, Path: info.Path, Mastery: m})
	}
	return out
}

// IsMastered reports a song+track the active profile has mastered.
func (s *Store) IsMastered(songID string, track int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	prof := s.active()
	if prof == nil {
		return false
	}
	_, ok := prof.Mastered[fmt.Sprintf("%s#%d", songID, track)]
	return ok
}

// BestAccuracy is a song track's best scored accuracy, if any.
func (s *Store) BestAccuracy(songID string, track int) (float64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prof := s.active()
	if prof == nil {
		return 0, false
	}
	b := prof.Bests[fmt.Sprintf("%s#%d", songID, track)]
	if b == nil {
		return 0, false
	}
	return b.Accuracy, true
}
