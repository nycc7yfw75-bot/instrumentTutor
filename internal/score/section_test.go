package score

import "testing"

func TestSectionAtAndOrder(t *testing.T) {
	s := &Score{Sections: []Section{{Tick: 0, Name: "Intro"}, {Tick: 3840, Name: "Verse"}, {Tick: 7680, Name: "Solo"}}}
	for tick, want := range map[int64]int{0: 0, 3839: 0, 3840: 1, 9000: 2} {
		if got := s.SectionAt(tick); got != want {
			t.Errorf("SectionAt(%d) = %d, want %d", tick, got, want)
		}
	}
	late := &Score{Sections: []Section{{Tick: 1920, Name: "Verse"}}}
	if got := late.SectionAt(0); got != -1 {
		t.Errorf("before the first section SectionAt = %d, want -1", got)
	}
	s.Sections[2].Tick = 3840
	if err := s.validateSections(); err == nil {
		t.Error("two sections on one tick passed validation")
	}
}
