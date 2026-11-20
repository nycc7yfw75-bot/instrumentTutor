// Package progress keeps each player profile's progress — XP, level,
// bests, pass history, best-pass replays and practice time — in
// progress.json beside config.json (docs/SCORING.md §5 and §10).
package progress

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"

	"github.com/S95F/musicTutor/internal/score"
)

// SongID fingerprints a song by what it plays: every note's track, timing,
// pitch, string and fret, plus the tempo and time-signature maps. The title,
// track names and the file's name and place play no part, so renaming or
// moving a file keeps its progress; editing its notes makes it a new song.
func SongID(sc *score.Score) string {
	h := sha256.New()
	put := func(vs ...int64) {
		var b [8]byte
		for _, v := range vs {
			binary.LittleEndian.PutUint64(b[:], uint64(v))
			h.Write(b[:])
		}
	}
	for _, t := range sc.Tempos {
		put(t.Tick, int64(t.USPerQuarter))
	}
	for _, m := range sc.Meters {
		put(m.Tick, int64(m.Num), int64(m.Den))
	}
	for _, ev := range sc.Events() {
		put(int64(ev.Track), ev.Start, ev.End, int64(ev.Key), int64(ev.String), int64(ev.Fret))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// NoteKey names one note of a track: its start tick and string.
type NoteKey struct {
	Start  int64
	String int
}

// NoteIndex numbers a track's notes in playing order, for replays: a note's
// index stays the same as long as the song's notes do.
func NoteIndex(sc *score.Score, track int) map[NoteKey]int {
	out := map[NoteKey]int{}
	for _, ev := range sc.Events() {
		if ev.Track == track {
			out[NoteKey{ev.Start, ev.String}] = len(out)
		}
	}
	return out
}
