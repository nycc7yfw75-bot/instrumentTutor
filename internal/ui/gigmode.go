package ui

import "github.com/S95F/musicTutor/internal/scoring"

// gigMode is how a practice view plays a gig (CAREER.md §3): one take at
// full speed, the crowd's excitement settings, no review popping up.
type gigMode struct {
	start, mult float64 // excitement at playback start, and its drain speed
}

// SetGigMode turns the view into a gig's: excitement starts at start and
// drains mult times as fast; the toolbar and every practice key go.
func (a *App) SetGigMode(start, mult float64) {
	a.gig = &gigMode{start: start, mult: mult}
	a.eng.SetTempoScale(1)
	a.eng.ClearLoop()
	a.wait = false
}

// gigExcitement is a fresh excitement bar for the gig's crowd.
func (a *App) gigExcitement() scoring.Excitement {
	if a.gig == nil {
		return scoring.NewExcitement()
	}
	return scoring.NewExcitementAt(a.gig.start, a.gig.mult)
}
