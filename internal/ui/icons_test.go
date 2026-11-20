package ui

import "testing"

func TestEveryIconIsSixteenBySixteen(t *testing.T) {
	for id := iconID(0); id < iconCount; id++ {
		lit := 0
		for y, row := range iconGrids[id] {
			if len(row) != iconPx {
				t.Errorf("icon %s row %d is %d wide, want %d", iconNames[id], y, len(row), iconPx)
			}
			for _, c := range row {
				switch c {
				case '.':
				case '#':
					lit++
				case '1', '2', '3':
					if id != icSpotlight {
						t.Errorf("icon %s uses colour %q; only the spotlight is multicolour", iconNames[id], c)
					}
					lit++
				default:
					t.Errorf("icon %s has a stray %q", iconNames[id], c)
				}
			}
		}
		if lit == 0 {
			t.Errorf("icon %s is blank", iconNames[id])
		}
		if iconNames[id] == "" {
			t.Errorf("icon %d has no name", id)
		}
	}
}
