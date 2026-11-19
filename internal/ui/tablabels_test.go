package ui

import (
	"testing"

	"github.com/S95F/musicTutor/internal/score"
)

func TestTabMarksReadLikeTabSites(t *testing.T) {
	last := map[int]int{3: 9}
	cases := []struct {
		n    score.Note
		want tabMark
	}{
		{score.Note{String: 3, Fret: 7, Tech: score.TechSlide}, tabMark{pre: `\`}},
		{score.Note{String: 3, Fret: 11, Tech: score.TechSlide}, tabMark{pre: "/"}},
		{score.Note{String: 3, Fret: 11, Tech: score.TechHammer}, tabMark{pre: "h"}},
		{score.Note{String: 3, Fret: 7, Tech: score.TechPull}, tabMark{pre: "p"}},
		{score.Note{String: 2, Fret: 8, Tech: score.TechBend, Bend: score.Bend{Peak: 4, To: 4}}, tabMark{post: "b", above: "full"}},
		{score.Note{String: 2, Fret: 8, Tech: score.TechBend, Bend: score.Bend{Peak: 2, To: 0}}, tabMark{post: "b", above: "1/2 rel"}},
		{score.Note{String: 2, Fret: 8, Tech: score.TechBend, Bend: score.Bend{From: 4, Peak: 4, To: 4}}, tabMark{post: "b", above: "pre full"}},
		{score.Note{String: 2, Fret: 8, Tech: score.TechBend}, tabMark{post: "b"}}, // .gtab "b": no amount known
		{score.Note{String: 6, Fret: 0, Tech: score.TechPalmMute | score.TechVibrato}, tabMark{above: "PM ~~"}},
	}
	for _, c := range cases {
		if got := markFor(c.n, last); got != c.want {
			t.Errorf("note %+v marks as %+v, want %+v", c.n, got, c.want)
		}
	}
}
