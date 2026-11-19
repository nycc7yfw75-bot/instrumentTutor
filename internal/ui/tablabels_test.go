package ui

import (
	"testing"

	"github.com/S95F/musicTutor/internal/score"
)

func TestTabMarksLinkToTheNoteBefore(t *testing.T) {
	const prevAt = 480
	cases := []struct {
		name string
		n    score.Note
		want tabMark
	}{
		{"slide down", score.Note{String: 3, Fret: 7, Tech: score.TechSlide},
			tabMark{link: legatoSlide, prevStart: prevAt, prevFret: 9}},
		{"slide up", score.Note{String: 3, Fret: 11, Tech: score.TechSlide},
			tabMark{link: legatoSlide, prevStart: prevAt, prevFret: 9, up: true}},
		{"hammer-on", score.Note{String: 3, Fret: 11, Tech: score.TechHammer},
			tabMark{link: legatoArc, prevStart: prevAt, prevFret: 9, up: true}},
		{"pull-off", score.Note{String: 3, Fret: 7, Tech: score.TechPull},
			tabMark{link: legatoArc, prevStart: prevAt, prevFret: 9}},
		{"bend", score.Note{String: 2, Fret: 8, Tech: score.TechBend, Bend: score.Bend{Peak: 4, To: 4}},
			tabMark{bent: true, bend: score.Bend{Peak: 4, To: 4}}},
		{"palm mute and vibrato", score.Note{String: 6, Fret: 0, Tech: score.TechPalmMute | score.TechVibrato},
			tabMark{above: "PM ~~"}},
	}
	for _, c := range cases {
		if got := markFor(c.n, prevAt, 9, true); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
	if got := markFor(score.Note{String: 3, Fret: 7, Tech: score.TechHammer}, 0, 0, false); got.link != legatoNone {
		t.Error("a hammer-on with no note before it on the string drew a link to nothing")
	}
}

func TestFretLabelsReadLikeTabSites(t *testing.T) {
	for _, c := range []struct {
		n    score.Note
		want string
	}{
		{score.Note{Fret: 4}, "4"},
		{score.Note{Fret: 4, Tied: true}, "(4)"},
		{score.Note{Fret: 9, Tech: score.TechDead}, "x"},
	} {
		if got := fretLabel(c.n); got != c.want {
			t.Errorf("note %+v reads %q, want %q", c.n, got, c.want)
		}
	}
	for q, want := range map[uint8]string{2: "1/2", 4: "full", 6: "1 1/2", 8: "2"} {
		if got := quarterToneText(q); got != want {
			t.Errorf("%d quarter-tones read %q, want %q", q, got, want)
		}
	}
}
