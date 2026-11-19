package score

import "sort"

type NoteEvent struct {
	Track      int
	Start, End int64
	Key        int
	Velocity   float64
	String     int
	Fret       int
	Tech       Technique
	// Bends are the bends sounded over the note, one per tied segment that
	// bends: a held note can be bent and released more than once.
	Bends []TimedBend
}

// TimedBend is one bend within a note: At and Len are ticks from the
// note's start.
type TimedBend struct {
	At, Len int64
	Bend    Bend
}

const DefaultVelocity = 0.8

func (s *Score) Events() []NoteEvent {
	var evs []NoteEvent
	for ti, tr := range s.Tracks {

		ringing := map[int]int{}
		for _, bar := range tr.Bars {
			for _, beat := range bar.Beats {
				sounded := map[int]bool{}
				for _, n := range beat.Notes {
					sounded[n.String] = true
					if n.Tied {
						if ei, ok := ringing[n.String]; ok && evs[ei].Fret == n.Fret && evs[ei].End == beat.Start {
							evs[ei].End = beat.Start + beat.Dur

							evs[ei].Tech |= n.Tech
							if n.Tech&TechBend != 0 {
								evs[ei].Bends = append(evs[ei].Bends, TimedBend{At: beat.Start - evs[ei].Start, Len: beat.Dur, Bend: n.Bend})
							}
							continue
						}

					}
					evs = append(evs, NoteEvent{
						Track:    ti,
						Start:    beat.Start,
						End:      beat.Start + beat.Dur,
						Key:      tr.Pitch(n),
						Velocity: DefaultVelocity,
						String:   n.String,
						Fret:     n.Fret,
						Tech:     n.Tech,
					})
					if n.Tech&TechBend != 0 {
						evs[len(evs)-1].Bends = []TimedBend{{Len: beat.Dur, Bend: n.Bend}}
					}
					ringing[n.String] = len(evs) - 1
				}

				for str, ei := range ringing {
					if !sounded[str] && evs[ei].End <= beat.Start {
						delete(ringing, str)
					}
				}
			}
		}
	}
	sort.SliceStable(evs, func(i, j int) bool {
		if evs[i].Start != evs[j].Start {
			return evs[i].Start < evs[j].Start
		}
		return evs[i].Track < evs[j].Track
	})
	return evs
}
