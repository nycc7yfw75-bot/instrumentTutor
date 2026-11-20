package ui

import (
	"testing"

	"github.com/S95F/musicTutor/internal/practice"
	"github.com/S95F/musicTutor/internal/progress"
	"github.com/S95F/musicTutor/internal/score"
	"github.com/S95F/musicTutor/internal/scoring"
)

var ghostTempo = score.TempoMap{{Tick: 0, USPerQuarter: 500000}} // 120 BPM: 1 s = 2 quarters

func judged(offsetMs float64, v practice.Verdict) scoring.Judgement {
	ev := score.NoteEvent{Start: 4 * score.PPQ, String: 3, Fret: 5}
	r := practice.NoteResult{Event: ev, Verdict: v, Matched: v != practice.VerdictMiss, Timed: v != practice.VerdictMiss,
		OnsetErrFrames: int64(offsetMs * 48)}
	return scoring.Judge(r, 48000)
}

func TestGhostsOnlyForEarlyOrLateNotes(t *testing.T) {
	if _, ok := ghostFor(judged(10, practice.VerdictHit), ghostTempo, 1, 4*score.PPQ); ok {
		t.Error("a perfect note got a ghost")
	}
	if _, ok := ghostFor(judged(0, practice.VerdictMiss), ghostTempo, 1, 4*score.PPQ); ok {
		t.Error("a miss got a ghost")
	}
	late, ok := ghostFor(judged(60, practice.VerdictHit), ghostTempo, 1, 4*score.PPQ)
	if !ok || late.offTicks <= 0 {
		t.Fatalf("a note 60 ms late: ghost %+v (made %v), want one to the right", late, ok)
	}
	early, ok := ghostFor(judged(-60, practice.VerdictClose), ghostTempo, 1, 4*score.PPQ)
	if !ok || early.offTicks >= 0 {
		t.Errorf("a wrong-pitch note 60 ms early: ghost %+v (made %v), want one to the left", early, ok)
	}
	// 60 ms at 120 BPM is 0.12 quarter; at half speed the same real lateness
	// is half as far in the tab's own time.
	half, _ := ghostFor(judged(60, practice.VerdictHit), ghostTempo, 0.5, 4*score.PPQ)
	if d := late.offTicks - 2*half.offTicks; d < -1 || d > 1 {
		t.Errorf("at half speed the ghost sits %d ticks off, want half of %d", half.offTicks, late.offTicks)
	}
}

func TestGhostsFadeInThreeStepsOverTwoBars(t *testing.T) {
	gh := ghost{start: 1000, offTicks: 50, fadeLen: 3000}
	var seen []float64
	for _, pos := range []int64{900, 1050, 1100, 2100, 3100, 4050} {
		seen = append(seen, ghostAlphaAt(gh, pos))
	}
	want := []float64{ghostAlpha, ghostAlpha, ghostAlpha, ghostAlpha * 2 / 3, ghostAlpha / 3, 0}
	for i := range want {
		if d := seen[i] - want[i]; d > 1e-9 || d < -1e-9 {
			t.Errorf("alpha steps %v, want %v", seen, want)
			break
		}
	}
}

func TestALateNoteLeavesAGhostAndANewPassClearsIt(t *testing.T) {
	a := newApp(t, 4)
	a.SetProgress(tempStore(t, "Player"), progress.SongID(a.sc), "Riff", "riff.gtab", true)
	a.live = true
	a.startPass(0)
	ev := a.sc.Events()[1]
	a.gameResults([]practice.NoteResult{
		{Event: a.sc.Events()[0], Verdict: practice.VerdictHit, Matched: true, Timed: true},            // perfect
		{Event: ev, Verdict: practice.VerdictHit, Matched: true, Timed: true, OnsetErrFrames: 90 * 48}, // 90 ms late
	})
	if len(a.game.ghosts) != 1 || a.game.ghosts[0].start != ev.Start {
		t.Fatalf("ghosts %+v, want one, on the late note", a.game.ghosts)
	}
	a.startPass(0)
	if len(a.game.ghosts) != 0 {
		t.Error("a new pass kept the last pass's ghosts")
	}
}
