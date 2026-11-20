package ui

import (
	"testing"

	"github.com/S95F/musicTutor/internal/practice"
	"github.com/S95F/musicTutor/internal/progress"
	"github.com/S95F/musicTutor/internal/score"
)

func reviewApp(t *testing.T) (*App, *progress.Store) {
	t.Helper()
	a := newApp(t, 12)
	st := tempStore(t, "Player")
	a.SetProgress(st, progress.SongID(a.sc), "Riff", "riff.gtab", true)
	a.live = true
	return a, st
}

func TestAFullRunOpensTheReview(t *testing.T) {
	a, _ := reviewApp(t)
	playPass(a, 10)
	a.endPass(a.sc.End())
	r := a.game.review
	if r.pass == nil || !r.ov.open || !r.sweep {
		t.Fatalf("a run to the end: review kept %v, open %v, sweep %v; want it open and sweeping", r.pass != nil, r.ov.open, r.sweep)
	}
	if len(r.pass.notes) != 12 || r.pass.xp <= 0 || !r.pass.scored {
		t.Errorf("review has %d notes, %d XP (scored %v); want all 12 and the XP earned", len(r.pass.notes), r.pass.xp, r.pass.scored)
	}
}

func TestALoopPassIsKeptButNotShownUntilAsked(t *testing.T) {
	a, st := reviewApp(t)
	playPass(a, 12)
	a.endPass(4 * 4 * score.PPQ) // mid-song
	if a.game.review.pass == nil || a.game.review.ov.open {
		t.Fatal("a mid-song pass should be kept for P, not opened")
	}
	_, before, _ := st.Active()
	a.toggleReview()
	if !a.game.review.ov.open || a.game.review.sweep {
		t.Error("P should open the review with every dot at once")
	}
	a.toggleReview()
	if _, after, _ := st.Active(); after != before {
		t.Error("reviewing changed the XP")
	}
}

func TestAShortPassIsNotReviewed(t *testing.T) {
	a, _ := reviewApp(t)
	a.startPass(0)
	playPassN(a, 5)
	a.endPass(4 * 4 * score.PPQ)
	if a.game.review.pass != nil {
		t.Error("a 5-note pass was kept for review")
	}
}

func TestReviewDotsCanBeFoundAndTheLayoutFits(t *testing.T) {
	a, _ := reviewApp(t)
	playPass(a, 12)
	a.endPass(a.sc.End())
	p := a.game.review.pass
	x, y := p.dotXY(p.notes[3], reviewStrip())
	if i, ok := a.reviewDotAt(pointer{x: x + 8, y: y - 8}); !ok || i != 3 {
		t.Errorf("a click 8px off dot 3 found %d (%v)", i, ok)
	}
	again, _ := reviewButtons()
	tab, card := reviewTab(), reviewCard()
	if tab.y+tab.h > again.y || card.y < 0 || card.y+card.h > screenH {
		t.Errorf("review layout overlaps: tab ends %.0f, buttons at %.0f, card %+v", tab.y+tab.h, again.y, card)
	}
}

func playPassN(a *App, n int) {
	var rs []practice.NoteResult
	for _, ev := range a.sc.Events()[:n] {
		rs = append(rs, practice.NoteResult{Event: ev, Verdict: practice.VerdictHit, Matched: true, Timed: true})
	}
	a.gameResults(rs)
}
