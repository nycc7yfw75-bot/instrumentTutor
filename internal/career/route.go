package career

import (
	"fmt"
	"math/rand"
	"strings"
)

// The tour's route (CAREER.md §4.2): real cities, fictional venues.

// City is a hometown or a stop on the tour.
type City struct {
	Name, Region string
}

// Cities is the hometown list, grouped by region.
var Cities = []City{
	{"Mumbai", "South Asia"}, {"Delhi", "South Asia"}, {"Bangalore", "South Asia"},
	{"Tokyo", "East Asia"}, {"Seoul", "East Asia"}, {"Singapore", "East Asia"},
	{"London", "Europe"}, {"Berlin", "Europe"}, {"Paris", "Europe"}, {"Dublin", "Europe"},
	{"New York", "North America"}, {"Los Angeles", "North America"}, {"Austin", "North America"}, {"Toronto", "North America"},
	{"São Paulo", "Latin America"}, {"Mexico City", "Latin America"}, {"Buenos Aires", "Latin America"},
	{"Sydney", "Oceania"}, {"Melbourne", "Oceania"},
	{"Lagos", "Africa"}, {"Cape Town", "Africa"},
}

// CityByName looks a city up.
func CityByName(name string) (City, bool) {
	for _, c := range Cities {
		if c.Name == name {
			return c, true
		}
	}
	return City{}, false
}

// Gig is one booking on the route.
type Gig struct {
	ID        string // stable: "t<tier>g<slot>"
	Tier      int
	Slot      int // 0..3 gigs, 4 the recording session
	Recording bool
	Venue     string
	City      City
}

// Card is how a gig card names it: "Pub gigs · The Copper Kettle · Bangalore".
func (g Gig) Card() string {
	t, _ := TierByNum(g.Tier)
	return t.Name + " · " + g.Venue + " · " + g.City.Name
}

// GigID is the stable ID of a tier's slot.
func GigID(tier, slot int) string { return fmt.Sprintf("t%dg%d", tier, slot) }

// Route lays the tour out for a hometown and seed. The same inputs always
// give the same route, so a profile's tour never reshuffles.
func Route(hometown string, seed int64) [][]Gig {
	home, ok := CityByName(hometown)
	if !ok {
		home = Cities[0]
	}
	rng := rand.New(rand.NewSource(seed))
	var sameRegion, otherRegions []City
	for _, c := range Cities {
		switch {
		case c.Name == home.Name:
		case c.Region == home.Region:
			sameRegion = append(sameRegion, c)
		default:
			otherRegions = append(otherRegions, c)
		}
	}
	rng.Shuffle(len(sameRegion), func(i, j int) { sameRegion[i], sameRegion[j] = sameRegion[j], sameRegion[i] })
	rng.Shuffle(len(otherRegions), func(i, j int) { otherRegions[i], otherRegions[j] = otherRegions[j], otherRegions[i] })

	// Pub gigs: home plus up to two cities from the same region (a region
	// with fewer cities repeats home).
	pub := []City{home}
	for i := 0; i < 2; i++ {
		if i < len(sameRegion) {
			pub = append(pub, sameRegion[i])
		} else {
			pub = append(pub, home)
		}
	}
	// Opening acts and stadiums: three cities each from other regions, all
	// different, from as many regions as possible.
	spread := pickSpread(otherRegions, 6)
	cities := [5][]City{{home}, {home}, pub, spread[:3], spread[3:6]}

	names := newVenueNamer(rng)
	route := make([][]Gig, len(Tiers))
	for ti, t := range Tiers {
		for slot := 0; slot <= RecordingSlot; slot++ {
			c := cities[ti][slot%len(cities[ti])]
			g := Gig{ID: GigID(t.Num, slot), Tier: t.Num, Slot: slot, Recording: slot == RecordingSlot, City: c}
			if g.Recording {
				g.Venue = names.studio()
			} else {
				g.Venue = names.venue(t.Num)
			}
			route[ti] = append(route[ti], g)
		}
	}
	return route
}

// pickSpread takes n cities, cycling through regions so they spread out.
func pickSpread(cs []City, n int) []City {
	byRegion := map[string][]City{}
	var order []string
	for _, c := range cs {
		if _, ok := byRegion[c.Region]; !ok {
			order = append(order, c.Region)
		}
		byRegion[c.Region] = append(byRegion[c.Region], c)
	}
	var out []City
	for len(out) < n {
		took := false
		for _, r := range order {
			if len(byRegion[r]) > 0 && len(out) < n {
				out = append(out, byRegion[r][0])
				byRegion[r] = byRegion[r][1:]
				took = true
			}
		}
		if !took {
			break
		}
	}
	for len(out) < n { // never happens with the city list, but never panics
		out = append(out, cs[len(out)%len(cs)])
	}
	return out
}

// Venue names are built from made-up parts (never a real venue, brand,
// band or landmark), each tier with its own flavour.
var (
	venueWords = [5][]string{
		{"Spare Room", "Loft", "Attic", "Back Bedroom"},
		{"Rehearsal Room", "Garage", "Basement", "Practice Space", "Lock-up"},
		{"Kettle", "Lantern", "Anchor", "Fox", "Fiddle", "Crown & Feather", "Tin Cup", "Compass"},
		{"Hall", "Theatre", "Playhouse", "Arcade", "Picturehouse", "Social"},
		{"Arena", "Bowl", "Grounds", "Park", "Field"},
	}
	venueAdjectives = []string{"Copper", "Velvet", "Rusty", "Silver", "Blue", "Hollow", "Golden", "Quiet",
		"Riverside", "Northside", "Old Mill", "Harbour", "Lamplight", "Oak", "Ember", "Juniper"}
	studioNames = []string{"Tapehouse", "Reel Room", "Needle Drop", "Echo Chamber", "Low Shelf", "Warm Valve"}
)

type venueNamer struct {
	rng  *rand.Rand
	used map[string]bool
}

func newVenueNamer(rng *rand.Rand) *venueNamer {
	return &venueNamer{rng: rng, used: map[string]bool{}}
}

func (v *venueNamer) pick(make func() string) string {
	for i := 0; i < 50; i++ {
		if n := make(); !v.used[n] {
			v.used[n] = true
			return n
		}
	}
	n := make() + fmt.Sprintf(" %d", len(v.used)+1)
	v.used[n] = true
	return n
}

func (v *venueNamer) venue(tier int) string {
	words := venueWords[tier-1]
	return v.pick(func() string {
		adj := venueAdjectives[v.rng.Intn(len(venueAdjectives))]
		w := words[v.rng.Intn(len(words))]
		switch tier {
		case 1:
			return "The " + w + " on " + adj + " Street"
		case 2:
			if v.rng.Intn(2) == 0 {
				return fmt.Sprintf("%s %d", w, 2+v.rng.Intn(18))
			}
			return "The " + adj + " " + w
		case 3:
			return "The " + adj + " " + w
		case 4:
			return adj + " " + w
		}
		return "The " + adj + " " + w
	})
}

func (v *venueNamer) studio() string {
	return v.pick(func() string {
		return studioNames[v.rng.Intn(len(studioNames))] + " Studio"
	})
}

// realVenueWords are parts of real, well-known venue and brand names that
// the generator must never produce (checked by the tests).
var realVenueWords = []string{"Madison", "Wembley", "Albert", "Cavern", "Apollo", "Hard Rock", "Red Rocks",
	"O2", "Fillmore", "Troubadour", "Whisky", "Ryman", "Budokan", "Olympia", "Paradiso", "Bowery"}

// isRealVenue reports a generated name that contains a real venue's name.
func isRealVenue(name string) bool {
	for _, w := range realVenueWords {
		if strings.Contains(strings.ToLower(name), strings.ToLower(w)) {
			return true
		}
	}
	return false
}
