package appconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const EnvConfigDir = "HEADLINER_CONFIG_DIR"

// LegacyEnvConfigDir is still honoured when EnvConfigDir is unset.
const LegacyEnvConfigDir = "MUSICTUTOR_CONFIG_DIR"

const CurrentVersion = 4

var ErrNewerVersion = errors.New("config file is from a newer version of musicTutor")

const (
	legacyDirName = "musictutor"
	oldDirName    = "guitartutor"
	fileName      = "config.json"
)

type Config struct {
	Version int `json:"version"`

	CaptureDeviceID  string `json:"captureDeviceID,omitempty"`
	PlaybackDeviceID string `json:"playbackDeviceID,omitempty"`

	LatencyOffsets map[string]int `json:"latencyOffsets,omitempty"`

	LatencyConfidence map[string]float64 `json:"latencyConfidence,omitempty"`

	SoundFontPath string `json:"soundFontPath,omitempty"`

	Recents []string `json:"recents,omitempty"`

	CountInBeats int `json:"countInBeats,omitempty"`

	LastBrowseDir string `json:"lastBrowseDir,omitempty"`

	WindowWidth  int `json:"windowWidth,omitempty"`
	WindowHeight int `json:"windowHeight,omitempty"`

	Created []string `json:"created,omitempty"`

	HideStartHint bool `json:"hideStartHint,omitempty"`

	SyncTrimMS int `json:"syncTrimMS,omitempty"`

	// ReduceMotion skips UI transitions (MOTION §A4); unset follows Windows'
	// "Show animations" setting.
	ReduceMotion *bool `json:"reduceMotion,omitempty"`

	// Pieces remembers each piece's practice setup, keyed by file path.
	Pieces map[string]PieceSetup `json:"pieces,omitempty"`

	// UpdateChannel is "stable" (the default) or "beta" (ONLINE.md §A6).
	UpdateChannel string `json:"updateChannel,omitempty"`
	// SkipVersion is an update the player chose to skip.
	SkipVersion string `json:"skipVersion,omitempty"`
}

// PieceSetup is where a piece was left: reopening it starts here. Every
// value is re-checked against the piece when applied, so a file that
// changed on disk (fewer tracks, shorter) cannot carry a bad value in.
type PieceSetup struct {
	Track int       `json:"track"`
	Tempo float64   `json:"tempo,omitempty"`
	Pos   int64     `json:"pos,omitempty"`
	LoopA int64     `json:"loopA,omitempty"`
	LoopB int64     `json:"loopB,omitempty"`
	Muted []bool    `json:"muted,omitempty"`
	Solo  int       `json:"solo,omitempty"`
	Mix   *PieceMix `json:"mix,omitempty"`
	// Used is when the piece was last left (Unix seconds), for pruning.
	Used int64 `json:"used"`
}

// PieceMix is the mixer: each volume 0 (silent) to 1.
type PieceMix struct {
	Master  float64   `json:"master"`
	Click   float64   `json:"click"`
	Backing float64   `json:"backing"`
	Tracks  []float64 `json:"tracks"`
}

// MaxPieces is how many pieces' setups are kept; the least recently used
// go first.
const MaxPieces = 100

// SetPiece records a piece's setup and drops the oldest beyond MaxPieces.
func (c *Config) SetPiece(path string, s PieceSetup) {
	if c.Pieces == nil {
		c.Pieces = make(map[string]PieceSetup)
	}
	c.Pieces[filepath.Clean(path)] = s
	for len(c.Pieces) > MaxPieces {
		oldest, at := "", int64(0)
		for k, v := range c.Pieces {
			if oldest == "" || v.Used < at {
				oldest, at = k, v.Used
			}
		}
		delete(c.Pieces, oldest)
	}
}

// Piece returns a piece's remembered setup.
func (c Config) Piece(path string) (PieceSetup, bool) {
	s, ok := c.Pieces[filepath.Clean(path)]
	return s, ok
}

const MaxSyncTrimMS = 250

const (
	MaxCountInBeats     = 8
	DefaultCountInBeats = 2
)

func Path() (string, error) {
	if dir := envDir(); dir != "" {
		return filepath.Join(dir, fileName), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("appconfig: locating user config dir: %w", err)
	}
	return filepath.Join(base, dirName, fileName), nil
}

func migrateOldDir(base string) {
	newDir := filepath.Join(base, legacyDirName)
	if _, err := os.Stat(newDir); err == nil || !os.IsNotExist(err) {
		return
	}
	oldDir := filepath.Join(base, oldDirName)
	if _, err := os.Stat(oldDir); err != nil {
		return
	}
	_ = os.Rename(oldDir, newDir)
}

func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{CountInBeats: DefaultCountInBeats}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("appconfig: reading %s: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("appconfig: parsing %s: %w", path, err)
	}
	if c.Version > CurrentVersion {
		return Config{}, fmt.Errorf("appconfig: %s is schema version %d, this build reads %d: %w",
			path, c.Version, CurrentVersion, ErrNewerVersion)
	}
	c.migrate()
	c.rehomePaths()
	return c, nil
}

func (c *Config) rehomePaths() {
	if envDir() != "" {
		return
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return
	}
	// guitartutor was renamed to musictutor: follow it once the old
	// folder is gone.
	oldDir := filepath.Join(base, oldDirName)
	legacy := filepath.Join(base, legacyDirName)
	if _, err := os.Stat(oldDir); os.IsNotExist(err) {
		if _, err := os.Stat(legacy); err == nil {
			c.rehome(oldDir, legacy, false)
		}
	}
	// The Headliner folder is a copy of an earlier one (Prepare): point at
	// the copies, wherever the copied file is really there.
	for _, from := range seedSources() {
		c.rehome(filepath.Join(base, from), filepath.Join(base, dirName), true)
	}
}

// rehome moves every remembered path under from to the same place under
// to; with mustExist only when that file exists there.
func (c *Config) rehome(from, to string, mustExist bool) {
	prefix := from + string(filepath.Separator)
	move := func(p string) string {
		rest, ok := strings.CutPrefix(p, prefix)
		if !ok {
			return p
		}
		np := filepath.Join(to, rest)
		if mustExist {
			if _, err := os.Stat(np); err != nil {
				return p
			}
		}
		return np
	}
	for i, p := range c.Recents {
		c.Recents[i] = move(p)
	}
	for i, p := range c.Created {
		c.Created[i] = move(p)
	}
	c.LastBrowseDir = move(c.LastBrowseDir)
	c.SoundFontPath = move(c.SoundFontPath)
	if len(c.Pieces) > 0 {
		pieces := make(map[string]PieceSetup, len(c.Pieces))
		for k, v := range c.Pieces {
			pieces[move(k)] = v
		}
		c.Pieces = pieces
	}
}

func (c *Config) migrate() {
	c.Version = CurrentVersion

	c.Recents = sanitizeRecents(c.Recents)
	c.Created = sanitizeCreated(c.Created)
	if c.CountInBeats < 0 {
		c.CountInBeats = 0
	}
	if c.CountInBeats > MaxCountInBeats {
		c.CountInBeats = MaxCountInBeats
	}
	if c.WindowWidth < 0 {
		c.WindowWidth = 0
	}
	if c.SyncTrimMS < -MaxSyncTrimMS {
		c.SyncTrimMS = -MaxSyncTrimMS
	}
	if c.SyncTrimMS > MaxSyncTrimMS {
		c.SyncTrimMS = MaxSyncTrimMS
	}
	if c.WindowHeight < 0 {
		c.WindowHeight = 0
	}
}

func (c Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("appconfig: creating %s: %w", dir, err)
	}
	if c.Version == 0 {
		c.Version = CurrentVersion
	}
	if on := onDiskVersion(path); on > c.Version {
		return fmt.Errorf("appconfig: %s is schema version %d, refusing to overwrite it with version %d: %w",
			path, on, c.Version, ErrNewerVersion)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("appconfig: encoding config: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, fileName+".tmp-*")
	if err != nil {
		return fmt.Errorf("appconfig: creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("appconfig: writing %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("appconfig: closing %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("appconfig: replacing %s: %w", path, err)
	}
	return nil
}

func onDiskVersion(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var head struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return 0
	}
	return head.Version
}

func pairKey(captureID, playbackID string) string {
	return captureID + "|" + playbackID
}

func (c Config) OffsetFor(captureID, playbackID string) (int, bool) {
	off, ok := c.LatencyOffsets[pairKey(captureID, playbackID)]
	return off, ok
}

func (c Config) ConfidenceFor(captureID, playbackID string) (float64, bool) {
	conf, ok := c.LatencyConfidence[pairKey(captureID, playbackID)]
	return conf, ok
}

func (c *Config) SetOffset(captureID, playbackID string, offsetFrames int, confidence float64) {
	if c.LatencyOffsets == nil {
		c.LatencyOffsets = make(map[string]int)
	}
	if c.LatencyConfidence == nil {
		c.LatencyConfidence = make(map[string]float64)
	}
	key := pairKey(captureID, playbackID)
	c.LatencyOffsets[key] = offsetFrames
	c.LatencyConfidence[key] = confidence
}
