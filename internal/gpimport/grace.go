package gpimport

import "github.com/S95F/musicTutor/internal/score"

// graceTicks is a grace note's length: a 32nd note, shortened if that would
// take more than half the note it borrows from.
const graceTicks = score.Quarter / 8

// placeGraces gives each grace note its time. A grace "before the beat"
// borrows the end of the note before it, so the main note still lands on
// its beat; one with no note before it in the bar (or an "on the beat"
// grace) borrows the start of its main note instead. A grace with no main
// note after it in the bar has nothing to ornament and is dropped.
func (im *importer) placeGraces(beats []beatData) []beatData {
	out := beats[:0]
	for i, b := range beats {
		if !b.grace {
			out = append(out, b)
			continue
		}
		main := -1
		for j := i + 1; j < len(beats); j++ {
			if !beats[j].grace {
				main = j
				break
			}
		}
		if main < 0 {
			im.graceSkipped++
			continue
		}
		donor := &beats[main]
		if b.before {
			for j := len(out) - 1; j >= 0; j-- {
				if !out[j].grace {
					donor = &out[j]
					break
				}
			}
		}
		g := min(graceTicks, donor.dur/2)
		if g <= 0 {
			im.graceSkipped++
			continue
		}
		donor.dur -= g
		b.dur = g
		out = append(out, b)
	}
	return out
}
