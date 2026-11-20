package scoring

import "math"

// XPToNext is the XP that takes a player from level to level+1 (§4.6).
func XPToNext(level int) int {
	return int(math.Round(LevelXPBase * math.Pow(float64(level), LevelXPPower)))
}

// LevelFor is the level a total XP reaches, the XP earned into it, and the
// XP it takes to reach the next.
func LevelFor(totalXP int) (level, into, toNext int) {
	level, into = 1, max(0, totalXP)
	for into >= XPToNext(level) {
		into -= XPToNext(level)
		level++
	}
	return level, into, XPToNext(level)
}

// Title is the name shown with a level.
func Title(level int) string {
	t := titles[0].title
	for _, row := range titles {
		if level >= row.from {
			t = row.title
		}
	}
	return t
}

// Player is a profile's XP. Stored XP only ever rises.
type Player struct {
	TotalXP int `json:"totalXP"`
}

// Add credits xp (a negative or zero amount changes nothing) and reports
// whether it took the player up a level.
func (p *Player) Add(xp int) (leveledUp bool) {
	if xp <= 0 {
		return false
	}
	before, _, _ := LevelFor(p.TotalXP)
	p.TotalXP += xp
	after, _, _ := LevelFor(p.TotalXP)
	return after > before
}

// Level is the player's current level.
func (p Player) Level() int {
	l, _, _ := LevelFor(p.TotalXP)
	return l
}
