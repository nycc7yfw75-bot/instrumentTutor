package progress

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/S95F/musicTutor/internal/career"
	"github.com/S95F/musicTutor/internal/scoring"
)

// A version-1 file (SCORING.md's schema) loads into version 2 with every
// field kept, and a copy of the old file is left beside it.
func TestMigrationKeepsAllScoringData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "progress.json")
	v1 := `{"version":1,"activeProfile":"abc","profiles":{"abc":{"name":"Aarsh","xp":{"totalXP":1234},"level":4,
	"bests":{"s#0":{"accuracy":0.91,"tightMean":4,"tempo":1}},"passes":[{"song":"s","xp":42}],
	"practice":{"days":{"2026-11-20":600}},"songs":{"s":{"title":"Riff","path":"C:/r.gtab"}},
	"badges":{"slow-clean":"2026-11-20T10:00:00Z"}}}}`
	if err := os.WriteFile(path, []byte(v1), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	name, xp, ok := s.Active()
	if !ok || name != "Aarsh" || xp.TotalXP != 1234 {
		t.Fatalf("after migration the player is %q with %d XP", name, xp.TotalXP)
	}
	if acc, ok := s.BestAccuracy("s", 0); !ok || acc != 0.91 {
		t.Errorf("the best accuracy became %.2f", acc)
	}
	if len(s.Earned()) != 1 || s.Streak().TotalDays != 1 {
		t.Error("badges or practice days were lost")
	}
	if _, err := os.Stat(path + ".v1"); err != nil {
		t.Errorf("no copy of the version-1 file was kept: %v", err)
	}
	if err := s.UpdateCareer(func(c *career.State) { c.Setup("Mumbai", 1, 5) }); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var f File
	if err := json.Unmarshal(data, &f); err != nil || f.Version != version {
		t.Fatalf("the saved file is version %d (%v)", f.Version, err)
	}
	if f.Profiles["abc"].XP.TotalXP != 1234 || len(f.Profiles["abc"].Passes) != 1 {
		t.Error("saving after the migration dropped data")
	}
}

func TestANewerSaveIsNeverWrittenOver(t *testing.T) {
	path := filepath.Join(t.TempDir(), "progress.json")
	newer := `{"version":99,"activeProfile":"a","profiles":{"a":{"name":"Future","xp":{"totalXP":5}}}}`
	os.WriteFile(path, []byte(newer), 0o644)
	s, err := OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !s.ReadOnly() {
		t.Fatal("a version-99 file opened read-write")
	}
	if name, _, _ := s.Active(); name != "Future" {
		t.Errorf("the newer save's player reads %q", name)
	}
	if _, err := s.RecordPass(Pass{SongID: "x", Seconds: 10}); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Errorf("recording into a newer save gave %v", err)
	}
	if data, _ := os.ReadFile(path); string(data) != newer {
		t.Error("the newer save was changed on disk")
	}
}

func TestAFullRunMastersTheSongOnce(t *testing.T) {
	s := badgeStore(t)
	p := Pass{SongID: "song", Track: 1, TempoScale: 1, Credible: true, FullRun: true, Seconds: 30,
		Summary: scoring.Summary{Judged: 20, SumScore: 18.4}}
	r, _ := s.RecordPass(p)
	if !r.Mastered || !s.IsMastered("song", 1) {
		t.Fatal("a credible 92% full run at full speed did not master the song")
	}
	if r, _ := s.RecordPass(p); r.Mastered {
		t.Error("mastering the same song again announced it twice")
	}
	loop := p
	loop.SongID, loop.FullRun = "other", false
	if r, _ := s.RecordPass(loop); r.Mastered {
		t.Error("a loop mastered a song")
	}
	if got := s.Mastered(); len(got) != 1 || got[0].SongID != "song" || got[0].Track != 1 {
		t.Errorf("mastered list %+v", got)
	}
}
