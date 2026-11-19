package score

// A drum hit is stored as a note whose String is its General MIDI drum
// number less drumKeyBase, so the GM kit (35 kick ... 81 open triangle, and
// the few below) fits the 1-64 lane range, and two drums struck together
// never share a lane.
const drumKeyBase = 26

// Drum keys a track can hold: General MIDI percussion runs 27-87.
const (
	MinDrumKey = drumKeyBase + 1
	MaxDrumKey = drumKeyBase + 64
)

// DrumNote is the note for General MIDI drum key on a Drums track; ok is
// false for a key outside MinDrumKey..MaxDrumKey.
func DrumNote(key int) (n Note, ok bool) {
	if key < MinDrumKey || key > MaxDrumKey {
		return Note{}, false
	}
	return Note{String: key - drumKeyBase}, true
}

// WithoutDrums drops the drum tracks, for places that need every track to
// have strings (the editor). It reports how many it dropped.
func (s *Score) WithoutDrums() int {
	kept := s.Tracks[:0]
	dropped := 0
	for _, tr := range s.Tracks {
		if tr.Drums {
			dropped++
			continue
		}
		kept = append(kept, tr)
	}
	s.Tracks = kept
	return dropped
}
