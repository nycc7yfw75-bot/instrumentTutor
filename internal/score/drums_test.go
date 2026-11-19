package score

import "testing"

func drumScore(t *testing.T, keys ...int) *Score {
	t.Helper()
	s := &Score{
		Tempos: TempoMap{{Tick: 0, USPerQuarter: 500000}},
		Meters: MeterMap{{Tick: 0, Num: 4, Den: 4}},
	}
	gtr := &Track{Name: "Guitar", Tuning: StandardTuning}
	gtr.AppendBar(4, 4).AddBeat(Whole, Note{String: 6, Fret: 0})
	kit := &Track{Name: "Drums", Drums: true, Role: RoleBacking}
	var notes []Note
	for _, k := range keys {
		n, ok := DrumNote(k)
		if !ok {
			t.Fatalf("DrumNote(%d) refused a General MIDI drum", k)
		}
		notes = append(notes, n)
	}
	kit.AppendBar(4, 4).AddBeat(Whole, notes...)
	s.Tracks = []*Track{gtr, kit}
	return s
}

func TestDrumHitsSoundTheirGeneralMIDIKey(t *testing.T) {
	s := drumScore(t, 36, 38, 42) // kick, snare and hi-hat struck together
	if err := s.Validate(); err != nil {
		t.Fatalf("a kick, snare and hi-hat together failed validation: %v", err)
	}
	var keys []int
	for _, ev := range s.Events() {
		if ev.Track == 1 {
			keys = append(keys, ev.Key)
		}
	}
	if len(keys) != 3 || keys[0] != 36 || keys[1] != 38 || keys[2] != 42 {
		t.Errorf("the kit sounds %v, want [36 38 42]", keys)
	}
	if _, ok := DrumNote(20); ok {
		t.Error("DrumNote took a key below the General MIDI kit")
	}
}

func TestWithoutDrumsKeepsTheStringedTracks(t *testing.T) {
	s := drumScore(t, 36)
	if n := s.WithoutDrums(); n != 1 || len(s.Tracks) != 1 || s.Tracks[0].Drums {
		t.Errorf("dropped %d, left %d tracks; want the drum kit gone and the guitar kept", n, len(s.Tracks))
	}
}
