package gpimport

import (
	"fmt"
	"strings"

	"github.com/S95F/musicTutor/internal/score"
	"github.com/S95F/musicTutor/internal/score/textfmt"
)

// buildDrumTrack turns a Guitar Pro drum kit track into a score drum track:
// backing that plays along, never shown as tab or scored.
func (im *importer) buildDrumTrack(orig int, gt *gpTrack) *score.Track {
	tc := &trackConv{drums: true}
	for _, el := range gt.InstrumentSet.Elements {
		for _, art := range el.Articulations {
			key := -1
			if art.OutputMidiNumber != nil {
				key = *art.OutputMidiNumber
			}
			tc.kit = append(tc.kit, key)
		}
	}
	name, _ := textfmt.CleanLabel(strings.TrimSpace(gt.Name))
	if name == "" {
		name = "Drums"
	}
	tr := &score.Track{Name: name, Drums: true, Role: score.RoleBacking}
	for mi := range im.doc.MasterBars {
		bar := tr.AppendBar(im.barNums[mi], im.barDens[mi])
		im.fillBar(orig, mi, bar, tc)
	}
	if tc.dropped > 0 {
		im.warnf("%s: %d drum hit(s) name no General MIDI drum; left out", trackLabel(gt, orig), tc.dropped)
	}
	return tr
}

// drumBeatNotes reads one beat of a drum track. A note's sound comes from
// its kit articulation; failing that, Guitar Pro also stores the drum's
// MIDI number as the note's "fret".
func (im *importer) drumBeatNotes(gbt *gpBeat, tc *trackConv) []score.Note {
	var out []score.Note
	seen := map[int]bool{}
	for _, nid := range im.ids(gbt.Notes, fmt.Sprintf("beat %d <Notes>", gbt.ID)) {
		gn := im.notes[nid]
		if gn == nil {
			continue
		}
		if gn.Tie != nil && strings.EqualFold(gn.Tie.Destination, "true") {
			continue // a tied drum hit is the same hit still ringing
		}
		key := -1
		if a := gn.InstrumentArticulation; a != nil && *a >= 0 && *a < len(tc.kit) {
			key = tc.kit[*a]
		}
		if key < 0 {
			for _, p := range gn.Properties {
				if p.Name == "Fret" && p.Fret != nil {
					key = *p.Fret
				}
			}
		}
		n, ok := score.DrumNote(key)
		if !ok {
			tc.dropped++
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, n)
	}
	return out
}
