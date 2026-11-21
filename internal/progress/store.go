package progress

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/S95F/musicTutor/internal/appconfig"
	"github.com/S95F/musicTutor/internal/career"
	"github.com/S95F/musicTutor/internal/scoring"
)

const (
	fileName     = "progress.json"
	version      = 2   // 2: career and mastery (CAREER.md §5.2)
	backupCopies = 3   // progress.json.1 .. .3, rotated on every save
	maxPassLog   = 200 // passes kept per profile
	maxReplays   = 300 // best-pass replays kept per profile
	maxNameRunes = 20
)

// File is progress.json.
type File struct {
	Version       int                 `json:"version"`
	ActiveProfile string              `json:"activeProfile"`
	Profiles      map[string]*Profile `json:"profiles"`
}

// Profile is one player's progress. Nothing is stored outside a profile.
type Profile struct {
	Name    string    `json:"name"`
	Created time.Time `json:"created"`

	XP    scoring.Player `json:"xp"`
	Level int            `json:"level"` // stored for the future venue ladder

	// Bests per song and track ("songID#track").
	Bests map[string]*Best `json:"bests,omitempty"`
	// Daily counts the day's passes per section, for the repeat factor.
	Daily scoring.DailyPasses `json:"daily"`
	// Passes is the history, newest last, capped at maxPassLog.
	Passes []PassLog `json:"passes,omitempty"`
	// Replays keeps the best-accuracy pass per song, track and bar range
	// ("songID#track#from-to"), for Battles' "your own ghost".
	Replays map[string]*Replay `json:"replays,omitempty"`
	// Practice is time spent playing.
	Practice Practice `json:"practice"`
	// Songs remembers where each song was last seen and what it is called.
	Songs map[string]SongInfo `json:"songs,omitempty"`
	// Badges earned, and when (set-only: never removed).
	Badges map[string]time.Time `json:"badges,omitempty"`
	// Totals are the counters the badges read.
	Totals Totals `json:"totals"`
	// Career is the tour (CAREER.md §5.2); Mastered the songs mastered in
	// Practice, by "songID#track", never removed.
	Career   career.State              `json:"career"`
	Mastered map[string]career.Mastery `json:"mastered,omitempty"`
}

// Best is a song track's best scored results.
type Best struct {
	Accuracy  float64 `json:"accuracy"`  // 0..1
	TightMean float64 `json:"tightMean"` // the mean offset closest to 0, ms
	Tempo     float64 `json:"tempo"`     // highest practice speed passed, 1 = 100%
	TightSet  bool    `json:"tightSet"`  // TightMean holds a real value
}

// PassLog is one pass in the history.
type PassLog struct {
	Date        time.Time `json:"date"`
	Song        string    `json:"song"`
	Title       string    `json:"title"`
	Track       int       `json:"track"`
	Bars        string    `json:"bars"`
	Tempo       float64   `json:"tempo"`
	Judged      int       `json:"judged"`
	Accuracy    float64   `json:"accuracy"`
	TightMean   float64   `json:"tightMean"`
	TightSpread float64   `json:"tightSpread"`
	XP          int       `json:"xp"`
	Unscored    bool      `json:"unscored,omitempty"`
	Seconds     float64   `json:"seconds"`
}

// Replay is a stored best pass: each judged note's result.
type Replay struct {
	Accuracy float64      `json:"accuracy"`
	Tempo    float64      `json:"tempo"`
	Notes    []ReplayNote `json:"notes"`
	LastUsed time.Time    `json:"lastUsed"`
}

// ReplayNote is one note of a replay.
type ReplayNote struct {
	Index    int     `json:"i"`     // see NoteIndex
	Verdict  int     `json:"v"`     // practice.Verdict
	OffsetMs float64 `json:"ms"`    // + late, - early
	Timed    bool    `json:"timed"` // false: no timing (a slide's arrival)
}

// Practice is time spent with the song playing.
type Practice struct {
	Days  map[string]float64 `json:"days,omitempty"`  // local date -> seconds
	Songs map[string]float64 `json:"songs,omitempty"` // song ID -> seconds
}

// SongInfo is how a song is shown: its title and where it was last seen.
type SongInfo struct {
	Title      string `json:"title"`
	Path       string `json:"path"`
	Difficulty string `json:"difficulty,omitempty"` // of the track last practised
}

// Store is the open progress file. Its methods are safe to call from any
// goroutine; every change is saved at once.
type Store struct {
	mu   sync.Mutex
	path string
	f    File
	now  func() time.Time

	// sessionSecs is this run of the app's playing time (Five More
	// Minutes); it is not saved.
	sessionSecs float64

	// readOnly: the file was saved by a newer HEADLINER; never write it.
	readOnly bool
}

// Path is where progress.json lives: beside config.json.
func Path() (string, error) {
	cfg, err := appconfig.Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(cfg), fileName), nil
}

// Open loads progress.json. A missing file starts empty (no profile yet:
// the player names one on first launch). A damaged one is renamed aside to
// progress.json.bad-<timestamp> and a fresh one started; if the main file
// is missing but a backup exists, the newest backup is used.
func Open() (*Store, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	return openAt(p, time.Now)
}

// OpenFile opens progress kept at path (tests, and a portable install).
func OpenFile(path string) (*Store, error) { return openAt(path, time.Now) }

func openAt(path string, now func() time.Time) (*Store, error) {
	s := &Store{path: path, now: now, f: File{Version: version, Profiles: map[string]*Profile{}}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		data, err = os.ReadFile(path + ".1")
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("progress: reading %s: %w", path, err)
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil || f.Version < 1 {
		bad := fmt.Sprintf("%s.bad-%s", path, now().Format("20060102-150405"))
		_ = os.Rename(path, bad)
		return s, nil
	}
	if f.Version > version {
		// Saved by a newer HEADLINER: read it, never write over it.
		s.readOnly = true
	} else if f.Version < version {
		// An older schema: keep a copy, then carry every field forward. New
		// fields start empty; nothing is dropped.
		_ = os.WriteFile(fmt.Sprintf("%s.v%d", path, f.Version), data, 0o644)
		f.Version = version
	}
	if f.Profiles == nil {
		f.Profiles = map[string]*Profile{}
	}
	if f.Profiles[f.ActiveProfile] == nil {
		f.ActiveProfile = ""
		for id := range f.Profiles {
			f.ActiveProfile = id
			break
		}
	}
	s.f = f
	return s, nil
}

// NeedsProfile reports that no profile exists yet: the app asks the player's
// name on first launch.
func (s *Store) NeedsProfile() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.f.ActiveProfile == ""
}

// CreateProfile adds a profile, makes it active and saves.
func (s *Store) CreateProfile(name string) (id string, err error) {
	name, err = cleanName(name)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id = newID()
	s.f.Profiles[id] = &Profile{Name: name, Created: s.now(), Level: 1}
	s.f.ActiveProfile = id
	return id, s.saveLocked()
}

// Rename renames the active profile and saves.
func (s *Store) Rename(name string) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.active()
	if p == nil {
		return errors.New("progress: no profile to rename")
	}
	p.Name = name
	return s.saveLocked()
}

// Active returns a copy of the active profile's headline numbers.
func (s *Store) Active() (name string, xp scoring.Player, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.active(); p != nil {
		return p.Name, p.XP, true
	}
	return "", scoring.Player{}, false
}

func (s *Store) active() *Profile { return s.f.Profiles[s.f.ActiveProfile] }

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("a profile needs a name")
	}
	if r := []rune(name); len(r) > maxNameRunes {
		name = string(r[:maxNameRunes])
	}
	return name, nil
}

func newID() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	var b [8]byte
	_, _ = rand.Read(b[:])
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b[:])
}

// saveLocked writes progress.json atomically, first rotating the previous
// copies (progress.json -> .1 -> .2 -> .3).
func (s *Store) saveLocked() error {
	if s.readOnly {
		return errNewerSave
	}
	data, err := json.MarshalIndent(s.f, "", "  ")
	if err != nil {
		return fmt.Errorf("progress: encoding: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("progress: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), fileName+".tmp-*")
	if err != nil {
		return fmt.Errorf("progress: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("progress: writing: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("progress: writing: %w", err)
	}
	for i := backupCopies - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", s.path, i), fmt.Sprintf("%s.%d", s.path, i+1))
	}
	_ = os.Rename(s.path, s.path+".1")
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("progress: saving: %w", err)
	}
	return nil
}

// errNewerSave stops a write over a progress file from a newer HEADLINER.
var errNewerSave = errors.New("progress: this progress was saved by a newer HEADLINER; update to keep playing with it")

// ReadOnly reports progress saved by a newer HEADLINER, which is shown but
// never written over.
func (s *Store) ReadOnly() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readOnly
}
