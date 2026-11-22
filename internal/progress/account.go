package progress

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/S95F/musicTutor/internal/appconfig"
	"github.com/S95F/musicTutor/internal/scoring"
)

// The online account's side of the progress file (ONLINE.md §B3–B4).

// SchemaVersion is progress.json's schema, sent with every synced doc.
const SchemaVersion = version

// AccountLink ties a local profile to an online account.
type AccountLink struct {
	UserID    string    `json:"userId"`
	Email     string    `json:"email"`
	ProfileID string    `json:"profileId"`
	LastSync  time.Time `json:"lastSync,omitempty"`
}

// OnSave sets a function run after every save (in its own goroutine).
func (s *Store) OnSave(fn func()) {
	s.mu.Lock()
	s.onSave = fn
	s.mu.Unlock()
}

// Linked is the account link, if a profile here is linked.
func (s *Store) Linked() (AccountLink, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f.Account == nil || s.f.Profiles[s.f.Account.ProfileID] == nil {
		return AccountLink{}, false
	}
	return *s.f.Account, true
}

// Link ties profile id to the account (replacing any earlier link).
func (s *Store) Link(userID, email, profileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f.Profiles[profileID] == nil {
		return fmt.Errorf("no profile %s", profileID)
	}
	s.f.Account = &AccountLink{UserID: userID, Email: email, ProfileID: profileID}
	return s.saveLocked()
}

// Unlink forgets the account here (signing out); the profile stays.
func (s *Store) Unlink() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f.Account == nil {
		return nil
	}
	s.f.Account = nil
	return s.saveLocked()
}

// ProfileJSON is a profile as the synced document holds it.
func (s *Store) ProfileJSON(id string) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.f.Profiles[id]
	if p == nil {
		return nil, fmt.Errorf("no profile %s", id)
	}
	return json.Marshal(p)
}

// ErrNewerDoc is a synced document from a newer Headliner.
var ErrNewerDoc = errors.New("your online progress was saved by a newer Headliner: update to sync")

// DecodeProfile reads a synced document.
func DecodeProfile(doc []byte, schema int) (*Profile, error) {
	if schema > version {
		return nil, ErrNewerDoc
	}
	var p Profile
	if err := json.Unmarshal(doc, &p); err != nil {
		return nil, fmt.Errorf("the online progress is damaged: %w", err)
	}
	return &p, nil
}

// ApplyRemote merges the server's copy into the linked profile id
// (MergeProfiles). When anything changes, the saves are backed up first
// (ONLINE.md §A7). It reports whether the local profile changed.
func (s *Store) ApplyRemote(id string, remote *Profile) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	local := s.f.Profiles[id]
	if local == nil {
		return false, fmt.Errorf("no profile %s", id)
	}
	merged := MergeProfiles(local, remote)
	before, _ := json.Marshal(local)
	after, _ := json.Marshal(merged)
	if bytes.Equal(before, after) {
		return false, nil
	}
	if s.readOnly {
		return false, errNewerSave
	}
	if _, err := appconfig.BackupSaves(filepath.Dir(s.path), "sync"); err != nil {
		return false, fmt.Errorf("backing up before the sync: %w", err)
	}
	s.f.Profiles[id] = merged
	return true, s.quietSaveLocked()
}

// MarkSynced records a finished sync.
func (s *Store) MarkSynced(at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f.Account != nil {
		s.f.Account.LastSync = at
		_ = s.quietSaveLocked()
	}
}

// AddProfileFrom adds a downloaded profile as a new local one (a second
// device that chose not to merge) and returns its id.
func (s *Store) AddProfileFrom(p *Profile) (string, error) {
	if p == nil {
		return "", errors.New("nothing to add")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.readOnly {
		return "", errNewerSave
	}
	id := newID()
	cp := cloneProfile(p)
	cp.Tint = s.freeTintLocked()
	s.f.Profiles[id] = cp
	if s.f.ActiveProfile == "" {
		s.f.ActiveProfile = id
	}
	return id, s.saveLocked()
}

// CardOf is what friends see of a profile: name, character, level and
// title, XP, Fame, tour tier and battle record.
type CardOf struct {
	Name                string
	Character           int
	Level               int
	Title               string
	XP, Fame, Tier      int
	Wins, Losses, Draws int
}

// Card is profile id's card.
func (s *Store) Card(id string) (CardOf, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.f.Profiles[id]
	if p == nil {
		return CardOf{}, false
	}
	lvl := p.XP.Level()
	return CardOf{Name: p.Name, Character: p.Career.Character, Level: lvl, Title: scoring.Title(lvl),
		XP: p.XP.TotalXP, Fame: p.Career.Fame, Tier: len(p.Career.Cleared),
		Wins: p.Battles.Totals.Wins, Losses: p.Battles.Totals.Losses, Draws: p.Battles.Totals.Draws}, true
}

// quietSaveLocked saves without running OnSave.
func (s *Store) quietSaveLocked() error {
	s.quiet = true
	defer func() { s.quiet = false }()
	return s.saveLocked()
}
