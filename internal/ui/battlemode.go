package ui

import "github.com/S95F/musicTutor/internal/practice"

// A practice view in a battle (BATTLES.md §6.3): gig mode's locks (the
// battle screen owns the keys, no toolbar, no review popping up), plus
// bot turns: while a bot plays, the live input is not judged and the
// bot's own results are shown on the tab instead, never scored as the
// player's pass.

// SetBotTurn starts or ends a bot's turn on this view.
func (a *App) SetBotTurn(on bool) { a.botTurn = on }

// ShowBotResults puts a bot's results on the tab (verdicts and counts),
// outside the game layer.
func (a *App) ShowBotResults(rs []practice.NoteResult) {
	if a.verdicts == nil {
		a.verdicts = make(map[noteKey]practice.Verdict)
	}
	for _, r := range rs {
		a.verdicts[noteKey{r.Event.Start, r.Event.String}] = r.Verdict
		switch r.Verdict {
		case practice.VerdictHit:
			a.stats.Hit++
		case practice.VerdictClose:
			a.stats.Close++
		default:
			a.stats.Miss++
		}
	}
}

// ClearTurn wipes the tab's verdicts, counts and ghosts for a new turn.
func (a *App) ClearTurn() {
	a.verdicts = nil
	a.stats = practice.Stats{}
	a.game.ghosts = a.game.ghosts[:0]
}
