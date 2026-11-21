package ui

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/S95F/musicTutor/internal/career"
	"github.com/S95F/musicTutor/internal/scene"
	"github.com/S95F/musicTutor/internal/scoring"
)

// The performance scene (CAREER.md §6): the venue, the crowd and the
// guitarist at the 480×200 base resolution, reacting to what the screen
// already knows — excitement, the song's position, the gig's moments. All
// motion is stepped sprite frames; nothing here reads the audio thread.

// stageMood is what the stage reacts to this frame.
type stageMood int

const (
	moodPlaying stageMood = iota
	moodIdle              // before a song, between songs
	moodSongEnd           // the bow and the cheer
	moodBooed             // booed off
)

// stage draws the scene.
type stage struct {
	tier      string // scene folder
	region    string
	preset    int
	recording bool
	crowd     string // career.CrowdNormal / CrowdTough / "" (recording)

	img   *ebiten.Image
	clock float64
	mood  stageMood
	moodT float64 // seconds in the current mood

	excitement float64
	beats      float64 // the song's position in beats, for the strum
	cheerUntil float64 // a streak cheer, until this clock time
	streak     int     // excitement's non-miss streak count, seen last frame
	booPuffs   []booPuff
}

type booPuff struct{ x, y, t float64 }

func newStage(g career.Gig, crowd string, preset int) *stage {
	return &stage{
		tier: tierSlug(g.Tier, g.Recording), region: g.City.Region, preset: preset,
		recording: g.Recording, crowd: crowd, img: ebiten.NewImage(scene.BaseW, scene.BaseH),
	}
}

func (s *stage) setMood(m stageMood) {
	if s.mood != m {
		s.mood, s.moodT = m, 0
	}
}

// step advances the scene's clock and reads the moment.
func (s *stage) step(dt, excitement, beats float64, streak int) {
	s.clock += dt
	s.moodT += dt
	s.excitement, s.beats = excitement, beats
	// A streak of 10 sends one crowd member into a cheer.
	if streak > 0 && streak%scoring.StreakEvery == 0 && streak != s.streak {
		s.cheerUntil = s.clock + 1
	}
	s.streak = streak
	// A cold tough crowd lets out the odd BOO.
	if s.crowd == career.CrowdTough && s.mood == moodPlaying && excitement < career.BooLowExcitement {
		if int(s.clock*1.5) != int((s.clock-dt)*1.5) {
			s.booPuffs = append(s.booPuffs, booPuff{x: 300 + math.Mod(s.clock*97, 160), y: 120, t: 0})
		}
	}
	if s.mood == moodBooed && int(s.clock*4) != int((s.clock-dt)*4) {
		s.booPuffs = append(s.booPuffs, booPuff{x: 280 + math.Mod(s.clock*131, 180), y: 110 + math.Mod(s.clock*53, 30), t: 0})
	}
	kept := s.booPuffs[:0]
	for _, p := range s.booPuffs {
		p.t += dt
		if p.t < 1.2 {
			kept = append(kept, p)
		}
	}
	s.booPuffs = kept
}

// band is the excitement band the crowd shows.
func (s *stage) band() scoring.Band {
	e := scoring.Excitement{Value: s.excitement}
	return e.Band()
}

// frameAt steps an animation at fps with a phase offset; reduced motion
// holds the first frame.
func (s *stage) frameAt(fps, phase float64) int {
	if reduceMotion {
		return 0
	}
	return int(s.clock*fps + phase)
}

// crowdSize is how many people a tier draws (§4.1's crowds).
func crowdSize(tier string) int {
	switch tier {
	case "bedroom":
		return 4
	case "jamroom":
		return 9
	case "pub":
		return 16
	case "opening":
		return 26
	case "stadium":
		return 40
	}
	return 0
}

// draw renders the scene into its base image.
func (s *stage) draw() *ebiten.Image {
	img := s.img
	img.Clear()
	band := s.band()
	lightFPS := [...]float64{1, 2, 4, 6}[band]
	if s.mood != moodPlaying {
		lightFPS = 1
	}
	g := "scenes/" + s.tier
	drawSprite(img, g, "sky", "still", 0, 0, 0, 1)
	drawSprite(img, "cities/"+regionSlug(s.region), "skyline", "still", 0, 0, 0, 1)
	drawSprite(img, g, "interior", "still", 0, 0, 0, 1)
	lightsAlpha := float32(1)
	if s.mood == moodBooed {
		lightsAlpha = float32(math.Max(0.25, 1-float64(stepIndex(s.moodT, 3))*0.25)) // dims stepped
	}
	drawSprite(img, g, "lights", "cycle", s.frameAt(lightFPS, 0), 0, 0, lightsAlpha)
	if s.tier == "stadium" {
		for i, x := range []float64{90, 240, 390} {
			drawSprite(img, "fx", "beam", "still", 0, x, 0, float32(0.4+0.2*float64((s.frameAt(lightFPS, float64(i)))%2)))
		}
	}
	s.drawCrowd(img, false)
	drawSprite(img, g, "stage", "still", 0, 0, 0, 1)
	s.drawGuitarist(img)
	s.drawCrowd(img, true)
	s.drawFX(img)
	return img
}

// drawCrowd draws the back rows or the front row, right of the stage.
func (s *stage) drawCrowd(img *ebiten.Image, front bool) {
	n := crowdSize(s.tier)
	if s.recording || n == 0 {
		return
	}
	anim, fps := "idle", 2.0
	switch s.band() {
	case scoring.Warm:
		anim, fps = "nod", 3
	case scoring.Hot:
		anim, fps = "jump", 4
	case scoring.OnFire:
		anim, fps = "armsup", 5
	}
	switch {
	case s.mood == moodBooed:
		anim, fps = "boo", 3
	case s.mood == moodSongEnd && s.moodT < 2:
		anim, fps = "cheer", 4
	case s.mood != moodPlaying:
		anim, fps = "idle", 1.5
	case s.crowd == career.CrowdTough && s.excitement < career.BooLowExcitement:
		anim, fps = "crossed", 1
	}
	frontN := min(n, 6)
	for i := 0; i < n; i++ {
		isFront := i < frontN
		if isFront != front {
			continue
		}
		var x, y float64
		if isFront {
			x, y = 300+float64(i)*28, 196
		} else {
			row := (i - frontN) / 10
			col := (i - frontN) % 10
			x, y = 290+float64(col)*18+float64(row%2)*9, 178-float64(row)*12
		}
		a := anim
		if s.clock < s.cheerUntil && i == 2 && s.mood == moodPlaying {
			a = "cheer"
		}
		phase := float64(i) * 0.37 // never in sync
		drawSprite(img, "crowd", "member"+itoa(i%6), a, s.frameAt(fps, phase), x, y, 1)
		if a == "armsup" && i%3 == 0 {
			drawSprite(img, "fx", "phone", "glow", s.frameAt(2, phase), x+5, y-24, 1)
		}
	}
}

func itoa(i int) string { return string(rune('0' + i%10)) }

// drawGuitarist draws the player: strumming in time with the song, idle
// between songs, a bow at the end, a slump when booed off.
func (s *stage) drawGuitarist(img *ebiten.Image) {
	group := "characters/" + presetSlug(s.preset)
	anim, frame := "idle", s.frameAt(2, 0)
	switch s.mood {
	case moodPlaying:
		anim, frame = "strum", int(math.Floor(s.beats*2))%4 // eighth notes, in time
		if s.band() == scoring.OnFire && int(s.beats)%8 == 7 {
			anim, frame = "cheer", int(math.Floor(s.beats*2))%2
		}
	case moodSongEnd:
		anim, frame = "bow", min(2, stepIndex(s.moodT, 3)*1)
	case moodBooed:
		anim, frame = "slump", min(2, int(s.moodT*3))
	}
	if reduceMotion && s.mood == moodPlaying {
		frame = 0
	}
	drawSprite(img, group, "guitarist", anim, frame, 140, 168, 1)
}

func (s *stage) drawFX(img *ebiten.Image) {
	if s.recording {
		on := s.mood == moodPlaying && (reduceMotion || int(s.clock*2)%2 == 0)
		f := 1
		if on {
			f = 0
		}
		drawSprite(img, "fx", "rec", "blink", f, 452, 24, 1)
	}
	for _, p := range s.booPuffs {
		drawSprite(img, "fx", "boo", "puff", min(2, int(p.t*3)), p.x, p.y-p.t*8, float32(1-p.t/1.2))
	}
	if s.mood == moodSongEnd && s.moodT < 2 && !s.recording {
		for i := 0; i < 24; i++ { // confetti, falling in whole-pixel steps
			x := math.Mod(float64(i)*53+s.moodT*float64(10+i%5), scene.BaseW)
			y := math.Mod(float64(i)*29+s.moodT*60, 170)
			drawSprite(img, "fx", "confetti", "fall", i%3, math.Round(x), math.Round(y), 1)
		}
	}
}
