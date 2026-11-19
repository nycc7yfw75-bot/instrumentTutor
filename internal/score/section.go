package score

import "fmt"

// Section is a named part of a piece, starting at Tick and running to the
// next section (or the end).
type Section struct {
	Tick int64
	Name string
}

// SectionAt is the index of the section playing at tick, or -1 before the
// first one (or when there are none).
func (s *Score) SectionAt(tick int64) int {
	at := -1
	for i, sec := range s.Sections {
		if sec.Tick > tick {
			break
		}
		at = i
	}
	return at
}

func (s *Score) validateSections() error {
	for i, sec := range s.Sections {
		if sec.Tick < 0 {
			return fmt.Errorf("section %q starts before the piece", sec.Name)
		}
		if i > 0 && sec.Tick <= s.Sections[i-1].Tick {
			return fmt.Errorf("section %q is out of order", sec.Name)
		}
	}
	return nil
}
