package progress

import (
	"fmt"
	"testing"
	"time"

	"github.com/S95F/musicTutor/internal/scoring"
)

func badgeStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenFile(t.TempDir() + "/progress.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProfile("Me"); err != nil {
		t.Fatal(err)
	}
	return s
}

// scored is a credible pass of 20 notes at acc accuracy.
func scored(song string, tempo, acc float64) Pass {
	return Pass{SongID: song, FromBar: 1, ToBar: 4, TempoScale: tempo, Credible: true, Seconds: 10,
		Summary: scoring.Summary{Judged: 20, SumScore: 20 * acc}}
}

func has(s *Store, id string) bool {
	for _, b := range s.Earned() {
		if b.ID == id {
			return true
		}
	}
	return false
}

func record(t *testing.T, s *Store, p Pass) []string {
	t.Helper()
	r, err := s.RecordPass(p)
	if err != nil {
		t.Fatal(err)
	}
	return r.NewBadges
}

func TestEveryBadgeHasANameAndHowToEarnIt(t *testing.T) {
	seen := map[string]bool{}
	for _, b := range Badges {
		if b.ID == "" || b.Name == "" || b.How == "" || seen[b.ID] {
			t.Errorf("badge %+v is incomplete or repeated", b)
		}
		seen[b.ID] = true
	}
	if len(Badges) != 11 {
		t.Errorf("%d badges, want the 10 rows of PROGRESS.md (Seven Days and A Month apart): 11", len(Badges))
	}
}

func TestSlowAndClean(t *testing.T) {
	s := badgeStore(t)
	record(t, s, scored("a", 0.8, 0.99)) // too fast
	if has(s, BadgeSlowClean) {
		t.Fatal("80% speed earned Slow and Clean")
	}
	got := record(t, s, scored("a", 0.7, 0.96))
	if len(got) == 0 || got[0] != BadgeSlowClean {
		t.Errorf("a clean pass at 70%% earned %v", got)
	}
	// Unscored never earns a detection badge.
	s2 := badgeStore(t)
	p := scored("a", 0.5, 1)
	p.Credible = false
	record(t, s2, p)
	if has(s2, BadgeSlowClean) {
		t.Error("an unscored pass earned Slow and Clean")
	}
}

func TestSectionMasteredNeedsThreeInARow(t *testing.T) {
	s := badgeStore(t)
	record(t, s, scored("a", 1, 0.96))
	record(t, s, scored("a", 1, 0.96))
	record(t, s, scored("a", 1, 0.80)) // breaks the run
	record(t, s, scored("a", 1, 0.96))
	record(t, s, scored("a", 1, 0.96))
	if has(s, BadgeMastered) {
		t.Fatal("a broken run earned Section Mastered")
	}
	record(t, s, scored("a", 1, 0.96))
	if !has(s, BadgeMastered) {
		t.Error("three clean full-speed passes in a row did not earn Section Mastered")
	}
}

func TestUpToSpeedNeedsARampOfThirtyPercent(t *testing.T) {
	s := badgeStore(t)
	for _, tempo := range []float64{0.7, 0.8, 0.9} {
		record(t, s, scored("a", tempo, 0.92))
	}
	if has(s, BadgeUpToSpeed) {
		t.Fatal("Up to Speed before reaching full speed")
	}
	record(t, s, scored("a", 1, 0.92))
	if !has(s, BadgeUpToSpeed) {
		t.Error("a ramp from 70% to full speed at 92% did not earn Up to Speed")
	}
}

func TestNailedItNeedsTheWholeSong(t *testing.T) {
	s := badgeStore(t)
	record(t, s, scored("a", 1, 0.99)) // a loop, not the whole song
	if has(s, BadgeNailedIt) {
		t.Fatal("a loop earned Nailed It")
	}
	p := scored("a", 1, 0.99)
	p.FullRun = true
	record(t, s, p)
	if !has(s, BadgeNailedIt) {
		t.Error("a 99% full run did not earn Nailed It")
	}
}

func TestRepertoireCountsDifferentSongs(t *testing.T) {
	s := badgeStore(t)
	for i := 0; i < 4; i++ {
		record(t, s, scored(fmt.Sprintf("song%d", i), 1, 0.97))
		record(t, s, scored(fmt.Sprintf("song%d", i), 1, 0.97))
	}
	if has(s, BadgeRepertoire) {
		t.Fatal("four songs earned Repertoire")
	}
	record(t, s, scored("song4", 1, 0.97))
	if !has(s, BadgeRepertoire) {
		t.Error("five clean songs did not earn Repertoire")
	}
}

func TestTimeBadgesNeedNoDetection(t *testing.T) {
	s := badgeStore(t)
	// 20 minutes in one sitting, nothing judged.
	record(t, s, Pass{SongID: "a", Seconds: 20 * 60})
	if !has(s, BadgeFiveMore) {
		t.Error("20 unscored minutes did not earn Five More Minutes")
	}
	record(t, s, Pass{SongID: "a", Seconds: 60, SlowSeconds: 60 * 60, ClickSeconds: 60 * 60})
	if !has(s, BadgeSpeedLimit) || !has(s, BadgeClickFriend) {
		t.Error("an hour slow with the click did not earn Under the Speed Limit and Friend of the Click")
	}
}

func TestBackAtItAfterTwoWeeks(t *testing.T) {
	s := badgeStore(t)
	now := time.Date(2026, 11, 21, 18, 0, 0, 0, time.Local)
	s.now = func() time.Time { return now.AddDate(0, 0, -20) }
	record(t, s, Pass{SongID: "a", Seconds: 60})
	s.now = func() time.Time { return now }
	got := record(t, s, Pass{SongID: "a", Seconds: 60})
	if len(got) != 1 || got[0] != BadgeBackAtIt {
		t.Errorf("coming back after 20 days earned %v, want Back at It", got)
	}
}

func TestStreakBadges(t *testing.T) {
	s := badgeStore(t)
	now := time.Date(2026, 11, 21, 18, 0, 0, 0, time.Local)
	for i := 6; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		s.now = func() time.Time { return day }
		record(t, s, Pass{SongID: "a", Seconds: 300})
	}
	if !has(s, BadgeSevenDays) || has(s, BadgeMonth) {
		t.Errorf("seven days in a row: badges %v", s.Earned())
	}
}

// Badges are set-only: nothing a later pass does takes one away.
func TestBadgesAreNeverTakenAway(t *testing.T) {
	s := badgeStore(t)
	record(t, s, scored("a", 0.6, 1))
	for i := 0; i < 5; i++ {
		record(t, s, scored("a", 1, 0)) // a run of total misses
	}
	if !has(s, BadgeSlowClean) {
		t.Error("misses took Slow and Clean away")
	}
}
