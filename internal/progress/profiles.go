package progress

import (
	"errors"
	"sort"

	"github.com/S95F/musicTutor/internal/scoring"
)

// Several local profiles (BATTLES.md §3): players sharing the laptop.

// TintCount is how many profile tints there are (tokens p1..p6).
const TintCount = 6

// ProfileInfo is what a profile card shows.
type ProfileInfo struct {
	ID     string
	Name   string
	XP     scoring.Player
	Level  int
	Tint   int
	Active bool
	Record BattleTotals
}

// Profiles lists every profile, the active one first, then by name.
func (s *Store) Profiles() []ProfileInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []ProfileInfo
	for id, p := range s.f.Profiles {
		out = append(out, ProfileInfo{ID: id, Name: p.Name, XP: p.XP, Level: p.XP.Level(), Tint: p.Tint,
			Active: id == s.f.ActiveProfile, Record: p.Battles.Totals})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Active != out[j].Active {
			return out[i].Active
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// freeTintLocked is the first tint no profile uses (or the least used).
func (s *Store) freeTintLocked() int {
	used := make([]int, TintCount)
	for _, p := range s.f.Profiles {
		used[((p.Tint%TintCount)+TintCount)%TintCount]++
	}
	best := 0
	for i := range used {
		if used[i] < used[best] {
			best = i
		}
	}
	return best
}

// AddProfile adds a profile without making it active.
func (s *Store) AddProfile(name string) (string, error) {
	name, err := cleanName(name)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f.Profiles == nil {
		s.f.Profiles = map[string]*Profile{}
	}
	id := newID()
	s.f.Profiles[id] = &Profile{Name: name, Created: s.now(), Level: 1, Tint: s.freeTintLocked()}
	if s.f.ActiveProfile == "" {
		s.f.ActiveProfile = id
	}
	return id, s.saveLocked()
}

// SetActive makes a profile the active one.
func (s *Store) SetActive(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f.Profiles[id] == nil {
		return errors.New("progress: no such profile")
	}
	s.f.ActiveProfile = id
	return s.saveLocked()
}

// RenameProfile renames any profile.
func (s *Store) RenameProfile(id, name string) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.f.Profiles[id]
	if p == nil {
		return errors.New("progress: no such profile")
	}
	p.Name = name
	return s.saveLocked()
}

// DeleteProfile removes a profile and everything in it. The saved backups
// (progress.json.1..3) still hold it until they rotate out.
func (s *Store) DeleteProfile(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f.Profiles[id] == nil {
		return errors.New("progress: no such profile")
	}
	delete(s.f.Profiles, id)
	if s.f.ActiveProfile == id {
		s.f.ActiveProfile = ""
		ids := make([]string, 0, len(s.f.Profiles))
		for other := range s.f.Profiles {
			ids = append(ids, other)
		}
		sort.Strings(ids)
		if len(ids) > 0 {
			s.f.ActiveProfile = ids[0]
		}
	}
	return s.saveLocked()
}

// Profile looks one profile up.
func (s *Store) Profile(id string) (ProfileInfo, bool) {
	for _, p := range s.Profiles() {
		if p.ID == id {
			return p, true
		}
	}
	return ProfileInfo{}, false
}
