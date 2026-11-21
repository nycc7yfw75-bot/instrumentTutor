package ui

import (
	"testing"

	"github.com/S95F/musicTutor/internal/career"
	"github.com/S95F/musicTutor/internal/progress"
	"github.com/S95F/musicTutor/internal/scoring"
)

// masteredStore is a profile with a career and n mastered songs.
func masteredStore(t *testing.T, n int) *progress.Store {
	t.Helper()
	st := tempStore(t, "Alex")
	st.UpdateCareer(func(c *career.State) { c.Setup("London", 0, 7) })
	for i := 0; i < n; i++ {
		st.RecordPass(progress.Pass{SongID: string(rune('a' + i)), Title: "Song " + string(rune('A'+i)), TempoScale: 1,
			Credible: true, FullRun: true, Seconds: 30, Difficulty: scoring.Medium,
			Summary: scoring.Summary{Judged: 20, SumScore: 19}})
	}
	return st
}

func TestPerformOpensAtTheCurrentTierAndLocksTheRest(t *testing.T) {
	p := NewPerformScreen(nil, masteredStore(t, 1))
	if p.tier != 1 || !p.unlocked(1) || p.unlocked(2) {
		t.Errorf("tier %d in view; the Bedroom should be open and the Jam room locked", p.tier)
	}
	p.setTier(2, false)
	p.openPicker()
	if p.picker != nil {
		t.Error("a locked tier opened its setlist picker")
	}
	p.setTier(9, false)
	if p.tier != len(career.Tiers) {
		t.Errorf("stepping past the last stop left tier %d", p.tier)
	}
}

func TestSetlistPickerKeepsOrderAndStartsTheGigOnce(t *testing.T) {
	st := masteredStore(t, 3)
	p := NewPerformScreen(nil, st)
	var played []progress.MasteredSong
	var gotCrowd string
	p.SetPlay(func(g career.Gig, crowd string, reveal bool, set []progress.MasteredSong) {
		played, gotCrowd = set, crowd
	})
	p.setTier(1, false)
	p.gig = 0
	p.openPicker()
	sp := p.picker
	if sp == nil || sp.need != 1 || len(sp.songs) != 3 {
		t.Fatalf("picker %+v; a Bedroom gig takes 1 of the 3 mastered songs", sp)
	}
	sp.toggle(2)
	sp.toggle(0) // full: ignored
	if len(sp.chosen) != 1 || sp.chosen[0] != 2 {
		t.Fatalf("chosen %v, want only song 2", sp.chosen)
	}
	sp.start(p)
	if len(played) != 1 || played[0].SongID != sp.songs[2].SongID || (gotCrowd != career.CrowdNormal && gotCrowd != career.CrowdTough) {
		t.Fatalf("the gig started with %v and crowd %q", played, gotCrowd)
	}
	rec := st.Career().Record(sp.gig.ID)
	if rec.Attempts != 1 || rec.Crowd == "" || len(rec.LastSetlist) != 1 {
		t.Errorf("after starting: %+v; one attempt, a crowd, the setlist kept", rec)
	}
	// The next visit remembers the setlist.
	again := newSetlistPicker(st, sp.gig)
	if len(again.chosen) != 1 || again.chosen[0] != 2 {
		t.Errorf("the remembered setlist came back as %v", again.chosen)
	}
}
