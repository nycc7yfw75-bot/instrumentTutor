package progress

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/S95F/musicTutor/internal/practice"
	"github.com/S95F/musicTutor/internal/score"
	"github.com/S95F/musicTutor/internal/scoring"
)

var day = time.Date(2026, 11, 20, 18, 0, 0, 0, time.Local)

func clockAt(t *time.Time) func() time.Time { return func() time.Time { return *t } }

func openTemp(t *testing.T) (*Store, string, *time.Time) {
	t.Helper()
	now := day
	path := filepath.Join(t.TempDir(), fileName)
	s, err := openAt(path, clockAt(&now))
	if err != nil {
		t.Fatal(err)
	}
	return s, path, &now
}

func song(t *testing.T, fret int) *score.Score {
	t.Helper()
	s := &score.Score{
		Title:  "Riff",
		Tempos: score.TempoMap{{Tick: 0, USPerQuarter: 500000}},
		Meters: score.MeterMap{{Tick: 0, Num: 4, Den: 4}},
	}
	tr := &score.Track{Name: "Lead", Tuning: score.StandardTuning}
	for b := 0; b < 4; b++ {
		bar := tr.AppendBar(4, 4)
		for i := 0; i < 4; i++ {
			bar.AddBeat(score.Quarter, score.Note{String: 3, Fret: fret + i})
		}
	}
	s.Tracks = []*score.Track{tr}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	return s
}

// passOf makes a pass over the song: the first hits notes perfect, the rest missed.
func passOf(t *testing.T, sc *score.Score, hits int, credible bool) Pass {
	t.Helper()
	var js []scoring.Judgement
	for i, ev := range sc.Events() {
		v := practice.VerdictMiss
		if i < hits {
			v = practice.VerdictHit
		}
		js = append(js, scoring.Judge(practice.NoteResult{Event: ev, Verdict: v, Matched: v != practice.VerdictMiss, Timed: v != practice.VerdictMiss}, 48000))
	}
	return Pass{
		SongID: SongID(sc), Title: sc.Title, Path: "C:/songs/riff.gtab", Track: 0, FromBar: 1, ToBar: 4,
		TempoScale: 1, Difficulty: scoring.Easy, Summary: scoring.Summarise(js), Judgements: js,
		NoteIndex: NoteIndex(sc, 0), Credible: credible, Seconds: 8,
	}
}

func TestFirstLaunchAsksForANameThenRemembersIt(t *testing.T) {
	s, path, now := openTemp(t)
	if !s.NeedsProfile() {
		t.Fatal("a fresh progress file should ask for the player's name")
	}
	if _, err := s.CreateProfile("  "); err == nil {
		t.Error("an empty name was accepted")
	}
	if _, err := s.CreateProfile("Alex the very long-named player"); err != nil {
		t.Fatal(err)
	}
	again, err := openAt(path, clockAt(now))
	if err != nil {
		t.Fatal(err)
	}
	name, _, ok := again.Active()
	if !ok || again.NeedsProfile() || len([]rune(name)) != maxNameRunes {
		t.Errorf("after reopening: profile %q (ok %v); want the name kept, cut to %d letters", name, ok, maxNameRunes)
	}
	if err := again.Rename("Alex"); err != nil {
		t.Fatal(err)
	}
	if name, _, _ := again.Active(); name != "Alex" {
		t.Errorf("renamed profile reads %q", name)
	}
}

func TestACrediblePassEarnsXPAndPersists(t *testing.T) {
	s, path, now := openTemp(t)
	s.CreateProfile("P1")
	sc := song(t, 3)
	rec, err := s.RecordPass(passOf(t, sc, 16, true))
	if err != nil {
		t.Fatal(err)
	}
	if rec.XP != 26 || !rec.Logged || !rec.NewBest { // 10 + 16 perfect hits × Easy × 100%
		t.Errorf("recorded %+v, want 26 XP, logged, a new best", rec)
	}
	again, _ := openAt(path, clockAt(now))
	_, xp, _ := again.Active()
	if xp.TotalXP != 26 {
		t.Errorf("total XP after reopening = %d, want 26", xp.TotalXP)
	}
	prof := again.active()
	if len(prof.Passes) != 1 || prof.Practice.Days["2026-11-20"] != 8 || prof.Practice.Songs[SongID(sc)] != 8 {
		t.Errorf("history %d passes, practice %+v; want one pass and 8 s today on this song", len(prof.Passes), prof.Practice)
	}
	if r, ok := again.Replay(SongID(sc), 0, 1, 4); !ok || len(r.Notes) != 16 || r.Accuracy != 1 {
		t.Errorf("replay %+v (found %v), want 16 perfect notes", r, ok)
	}
}

func TestUnscoredPassesAreLoggedWithoutXPOrBests(t *testing.T) {
	s, _, _ := openTemp(t)
	s.CreateProfile("P1")
	sc := song(t, 3)
	rec, _ := s.RecordPass(passOf(t, sc, 16, false))
	prof := s.active()
	if rec.XP != 0 || prof.XP.TotalXP != 0 {
		t.Errorf("an unscored pass earned %d XP (total %d)", rec.XP, prof.XP.TotalXP)
	}
	if len(prof.Passes) != 1 || !prof.Passes[0].Unscored || prof.Practice.Days["2026-11-20"] != 8 {
		t.Errorf("an unscored pass should be logged as Unscored with its practice time: %+v", prof.Passes)
	}
	if len(prof.Bests) != 0 || len(prof.Replays) != 0 {
		t.Error("an unscored pass set a best or a replay")
	}
}

func TestAShortPassCountsOnlyItsPracticeTime(t *testing.T) {
	s, _, _ := openTemp(t)
	s.CreateProfile("P1")
	p := passOf(t, song(t, 3), 16, true)
	p.Summary = scoring.Summarise(p.Judgements[:5]) // paused after 5 notes
	rec, _ := s.RecordPass(p)
	prof := s.active()
	if rec.Logged || rec.XP != 0 || len(prof.Passes) != 0 || prof.Practice.Days["2026-11-20"] != 8 {
		t.Errorf("a 5-note pass: %+v, %d logged; want no log or XP, but 8 s of practice", rec, len(prof.Passes))
	}
}

func TestReplayKeepsTheBestPass(t *testing.T) {
	s, _, _ := openTemp(t)
	s.CreateProfile("P1")
	sc := song(t, 3)
	s.RecordPass(passOf(t, sc, 16, true))
	s.RecordPass(passOf(t, sc, 10, true)) // worse: the stored replay stays
	r, _ := s.Replay(SongID(sc), 0, 1, 4)
	if r.Accuracy != 1 {
		t.Errorf("a worse pass replaced the replay (accuracy %.2f)", r.Accuracy)
	}
	if b := s.active().Bests[SongID(sc)+"#0"]; b.Accuracy != 1 {
		t.Errorf("a worse pass lowered the best to %.2f", b.Accuracy)
	}
}

func TestCapsOnHistoryAndReplays(t *testing.T) {
	s, _, now := openTemp(t)
	s.CreateProfile("P1")
	sc := song(t, 3)
	for i := 0; i < maxPassLog+20; i++ {
		*now = now.Add(time.Minute)
		p := passOf(t, sc, 16, true)
		p.FromBar, p.ToBar = i+1, i+4 // a new bar range each time
		s.RecordPass(p)
	}
	prof := s.active()
	if len(prof.Passes) != maxPassLog {
		t.Errorf("history holds %d passes, want the cap of %d", len(prof.Passes), maxPassLog)
	}
	for i := len(prof.Replays); i <= maxReplays+5; i++ {
		*now = now.Add(time.Minute)
		p := passOf(t, sc, 16, true)
		p.FromBar, p.ToBar = 1000+i, 1004+i
		s.RecordPass(p)
	}
	if len(prof.Replays) != maxReplays {
		t.Fatalf("%d replays kept, want the cap of %d", len(prof.Replays), maxReplays)
	}
	if _, ok := s.Replay(SongID(sc), 0, 1, 4); ok {
		t.Error("the least recently used replay survived the cap")
	}
}

func TestSavesRotateThreeBackups(t *testing.T) {
	s, path, _ := openTemp(t)
	s.CreateProfile("P1")
	for i := 0; i < 5; i++ {
		s.Rename(fmt.Sprintf("P%d", i))
	}
	for i := 1; i <= backupCopies; i++ {
		if _, err := os.Stat(fmt.Sprintf("%s.%d", path, i)); err != nil {
			t.Errorf("backup .%d missing: %v", i, err)
		}
	}
	if _, err := os.Stat(path + ".4"); err == nil {
		t.Error("a fourth backup was kept")
	}
	data, _ := os.ReadFile(path + ".1")
	if !strings.Contains(string(data), `"P3"`) {
		t.Errorf("backup .1 is not the previous save: %s", data)
	}
}

func TestDamagedFileIsSetAsideAndAMissingOneFallsBackToABackup(t *testing.T) {
	s, path, now := openTemp(t)
	s.CreateProfile("Kept")
	s.Rename("Kept") // a second save, so .1 exists
	os.Remove(path)
	fallback, err := openAt(path, clockAt(now))
	if err != nil {
		t.Fatal(err)
	}
	if name, _, ok := fallback.Active(); !ok || name != "Kept" {
		t.Errorf("with the main file gone, opened %q (ok %v); want the newest backup", name, ok)
	}

	os.WriteFile(path, []byte("{not json"), 0o644)
	fresh, err := openAt(path, clockAt(now))
	if err != nil {
		t.Fatalf("a damaged file crashed the open: %v", err)
	}
	if !fresh.NeedsProfile() {
		t.Error("a damaged file should start fresh")
	}
	matches, _ := filepath.Glob(path + ".bad-*")
	if len(matches) != 1 {
		t.Errorf("the damaged file was not set aside: %v", matches)
	}
}

func TestSongIDFollowsTheNotesNotTheName(t *testing.T) {
	a, b := song(t, 3), song(t, 3)
	b.Title, b.Tracks[0].Name = "Renamed", "Other"
	if SongID(a) != SongID(b) {
		t.Error("renaming the song or its track changed its identity")
	}
	if SongID(a) == SongID(song(t, 4)) {
		t.Error("different notes gave the same identity")
	}
}
