package main

import (
	"testing"

	"github.com/S95F/musicTutor/internal/score"
)

func tracksScore(t *testing.T, tracks ...*score.Track) *score.Score {
	t.Helper()
	sc := &score.Score{
		Tempos: score.TempoMap{{Tick: 0, USPerQuarter: 500000}},
		Meters: score.MeterMap{{Tick: 0, Num: 4, Den: 4}},
		Tracks: tracks,
	}
	for _, tr := range tracks {
		tr.Tuning = score.StandardTuning
		tr.AppendBar(4, 4).AddBeat(score.Whole, score.Note{String: 6, Fret: 0})
	}
	if err := sc.Validate(); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return sc
}

func TestDefaultTrackSkipsAVocalLineMarkedAsThePlayers(t *testing.T) {
	for _, c := range []struct {
		name   string
		tracks []*score.Track
		want   int
	}{
		{"plain: first user track", []*score.Track{
			{Name: "Backing", Role: score.RoleBacking}, {Name: "Guitar", Role: score.RoleUser}}, 1},
		{"Sultans: vocals marked as the player's", []*score.Track{
			{Name: "Mark Knopfler | Lead Vocals", Role: score.RoleUser},
			{Name: "Mark Knopfler | Fender Stratocaster | Lead Guitar", Role: score.RoleBacking},
			{Name: "John Illsley | Fender Precision Bass", Role: score.RoleBacking}}, 1},
		{"only vocals: keep the marked one", []*score.Track{
			{Name: "Backing Vocals", Role: score.RoleBacking}, {Name: "Lead Vocals", Role: score.RoleUser}}, 1},
	} {
		if got := defaultTrack(tracksScore(t, c.tracks...)); got != c.want {
			t.Errorf("%s: opened on track %d, want %d", c.name, got, c.want)
		}
	}
}
