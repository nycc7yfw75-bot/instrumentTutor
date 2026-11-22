package online

import (
	"encoding/json"
	"errors"
)

// The refresh token lives in Windows Credential Manager (ONLINE.md §B2),
// never in a plain file. tokens_windows.go has the real store; other
// systems keep it in memory for the session only.

// credTarget names the entry in Credential Manager.
const credTarget = "Headliner/account"

// Saved is what is kept between runs.
type Saved struct {
	RefreshToken string `json:"refresh_token"`
	UserID       string `json:"user_id"`
	Email        string `json:"email"`
}

// TokenStore keeps the signed-in account between runs.
type TokenStore interface {
	Load() (Saved, error)
	Save(Saved) error
	Clear() error
}

// ErrNoToken is a store with nobody signed in.
var ErrNoToken = errors.New("not signed in")

// MemoryStore is a TokenStore for tests and non-Windows builds.
type MemoryStore struct {
	s   Saved
	set bool
}

func (m *MemoryStore) Load() (Saved, error) {
	if !m.set {
		return Saved{}, ErrNoToken
	}
	return m.s, nil
}
func (m *MemoryStore) Save(s Saved) error { m.s, m.set = s, true; return nil }
func (m *MemoryStore) Clear() error       { m.s, m.set = Saved{}, false; return nil }

func encodeSaved(s Saved) ([]byte, error) { return json.Marshal(s) }

func decodeSaved(b []byte) (Saved, error) {
	var s Saved
	if err := json.Unmarshal(b, &s); err != nil || s.RefreshToken == "" {
		return Saved{}, ErrNoToken
	}
	return s, nil
}
