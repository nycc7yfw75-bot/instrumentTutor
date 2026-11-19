package ui

import (
	"fmt"
	"strings"

	"github.com/S95F/musicTutor/internal/score"
)

// Technique marks on the practice tab, the way online tab sites draw them:
// h7 hammer-on, p5 pull-off, /9 and \7 slides up and down, 7b a bend with
// its amount written above ("full", "1/2"), ~~ vibrato and PM palm mute
// above the note, x a dead note.

type tabMark struct {
	pre, post string // around the fret number
	above     string // small text over the note
}

// tabMarks works the marks out once per track: slide direction needs the
// note before on the same string.
func (a *App) tabMarks() map[noteKey]tabMark {
	if a.marks != nil {
		return a.marks
	}
	a.marks = map[noteKey]tabMark{}
	last := map[int]int{}
	for _, bar := range a.displayed().Bars {
		for _, beat := range bar.Beats {
			for _, n := range beat.Notes {
				if m := markFor(n, last); m != (tabMark{}) {
					a.marks[noteKey{beat.Start, n.String}] = m
				}
				last[n.String] = n.Fret
			}
		}
	}
	return a.marks
}

func markFor(n score.Note, last map[int]int) tabMark {
	var m tabMark
	switch {
	case n.Tech&score.TechHammer != 0:
		m.pre = "h"
	case n.Tech&score.TechPull != 0:
		m.pre = "p"
	case n.Tech&score.TechSlide != 0:
		m.pre = "/"
		if prev, ok := last[n.String]; ok && n.Fret < prev {
			m.pre = `\`
		}
	}
	var above []string
	if n.Tech&score.TechPalmMute != 0 {
		above = append(above, "PM")
	}
	if n.Tech&score.TechBend != 0 {
		m.post = "b"
		if s := bendText(n.Bend); s != "" {
			above = append(above, s)
		}
	}
	if n.Tech&score.TechVibrato != 0 {
		above = append(above, "~~")
	}
	m.above = strings.Join(above, " ")
	return m
}

// bendText names a bend's amount: "full", "1/2", "pre full rel".
func bendText(b score.Bend) string {
	if b.Peak == 0 {
		return ""
	}
	s := quarterToneText(b.Peak)
	if b.From > 0 {
		s = "pre " + s
	}
	if b.To < b.Peak {
		s += " rel"
	}
	return s
}

func quarterToneText(q uint8) string {
	whole, rest := q/4, q%4
	frac := map[uint8]string{1: "1/4", 2: "1/2", 3: "3/4"}[rest]
	switch {
	case whole == 0:
		return frac
	case whole == 1 && rest == 0:
		return "full"
	case rest == 0:
		return fmt.Sprintf("%d", whole)
	}
	return fmt.Sprintf("%d %s", whole, frac)
}
