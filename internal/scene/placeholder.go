package scene

import (
	"image"
	"image/color"
	"strconv"
	"strings"
)

// Placeholder art, drawn in code (CAREER.md §7): simple, consistent,
// palette-correct pixel shapes, laid out exactly as the real sheets will be,
// so every system works before real art exists. docs/ART_BRIEF.md says what
// replaces each one.

func rgb(v uint32) color.RGBA {
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
}

// The scene palette: the REDESIGN tokens plus a per-tier accent ramp.
var (
	pBG      = rgb(0x111116)
	pSurface = rgb(0x191920)
	pRaised  = rgb(0x25252F)
	pLine    = rgb(0x393843)
	pText    = rgb(0xF4F0E8)
	pText2   = rgb(0xA5A3AF)
	pText3   = rgb(0x6F6D7A)
	pBrand   = rgb(0xF3A33B)
	pMiss    = rgb(0xE56A6A)

	// TierAccent is each venue's light colour: Bedroom lamp orange, Jam
	// room teal, Pub amber, Opening act magenta, Stadium white-blue, and
	// the studio's cool grey.
	TierAccent = map[string]color.RGBA{
		"bedroom": rgb(0xE8954A), "jamroom": rgb(0x3FB8AF), "pub": rgb(0xD49A45),
		"opening": rgb(0xD14FA0), "stadium": rgb(0xCFE0FF), "studio": rgb(0x8FA3B8),
	}
	tierWall = map[string]color.RGBA{
		"bedroom": rgb(0x2B2230), "jamroom": rgb(0x1E2A2E), "pub": rgb(0x3A2A1E),
		"opening": rgb(0x24182A), "stadium": rgb(0x0E1424), "studio": rgb(0x1C2128),
	}
	regionSky = map[string]color.RGBA{
		"south-asia": rgb(0x2A1C33), "east-asia": rgb(0x1A1F38), "europe": rgb(0x1C2230),
		"north-america": rgb(0x1F1A2E), "latin-america": rgb(0x2C1B2A), "oceania": rgb(0x182634),
		"africa": rgb(0x2B2118),
	}
	skinTones = []color.RGBA{rgb(0xF1C9A5), rgb(0xC68C5E), rgb(0x8D5A3B), rgb(0xE0AC83), rgb(0x5C3A26), rgb(0xD9A27A)}
	shirtCols = []color.RGBA{rgb(0x4A6FA5), rgb(0x9A4A4A), rgb(0x4A8A5A), rgb(0x7A5A9A), rgb(0xA58A4A), rgb(0x4A4A5A)}
)

// character presets: hair, top, legs, guitar, skin.
var presetLooks = map[string][5]color.RGBA{
	"rae":  {rgb(0x2A1E18), rgb(0x3E5F8A), rgb(0x23232B), rgb(0xC8742E), skinTones[3]},
	"theo": {rgb(0x6B4A2A), rgb(0x2B2B33), rgb(0x3A3A48), rgb(0x1C1C20), skinTones[0]},
	"mina": {rgb(0x111111), rgb(0x4A2E2E), rgb(0x2A2030), rgb(0xB8353A), skinTones[2]},
	"sol":  {rgb(0x7A3A3A), rgb(0x8A5A2E), rgb(0x2E3A2A), rgb(0xD8B07A), skinTones[4]},
}

// Placeholder draws a sheet for a sprite: its kind is "<what>:<variant>".
func Placeholder(kind string, s Sprite) image.Image {
	what, variant, _ := strings.Cut(kind, ":")
	cols, rows := 1, 1
	for _, a := range s.Anims {
		cols = max(cols, a.Frames)
		rows = max(rows, a.Row+1)
	}
	img := image.NewRGBA(image.Rect(0, 0, cols*s.FrameW, rows*s.FrameH))
	c := &canvas{img: img}
	for _, a := range s.Anims {
		for f := 0; f < a.Frames; f++ {
			c.ox, c.oy = f*s.FrameW, a.Row*s.FrameH
			c.w, c.h = s.FrameW, s.FrameH
			drawPlaceholder(c, what, variant, a.Name, f)
		}
	}
	return img
}

// canvas draws into one frame of a sheet, clipped to it.
type canvas struct {
	img          *image.RGBA
	ox, oy, w, h int
}

func (c *canvas) rect(x, y, w, h int, col color.RGBA) {
	for yy := max(0, y); yy < min(c.h, y+h); yy++ {
		for xx := max(0, x); xx < min(c.w, x+w); xx++ {
			c.img.SetRGBA(c.ox+xx, c.oy+yy, col)
		}
	}
}

func alpha(col color.RGBA, a float64) color.RGBA {
	return color.RGBA{uint8(float64(col.R) * a), uint8(float64(col.G) * a), uint8(float64(col.B) * a), uint8(255 * a)}
}

func seedOf(s string) int {
	h := 7
	for _, r := range s {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return h
}

func drawPlaceholder(c *canvas, what, variant, anim string, f int) {
	switch what {
	case "sky":
		drawSky(c, variant)
	case "skyline":
		drawSkyline(c, variant)
	case "interior":
		drawInterior(c, variant)
	case "lights":
		drawLights(c, variant, f)
	case "stage":
		drawStage(c, variant)
	case "guitarist":
		drawGuitarist(c, variant, anim, f)
	case "crowd":
		drawCrowdMember(c, variant, anim, f)
	case "phone":
		c.rect(1, 0, 2, 4, pText3)
		if f == 0 {
			c.rect(1, 0, 2, 2, pText)
		}
	case "beam":
		acc := TierAccent[variant]
		for y := 0; y < c.h; y++ {
			w := 4 + y*(c.w-4)/c.h
			c.rect((c.w-w)/2, y, w, 1, alpha(acc, 0.18))
		}
	case "confetti":
		cols := []color.RGBA{pBrand, TierAccent["jamroom"], TierAccent["opening"]}
		c.rect(f%2, f/2, 2, 2, cols[f%3])
	case "boo":
		drawBoo(c, f)
	case "flames":
		drawFlames(c, f)
	case "star":
		drawStar(c, anim == "lit")
	case "rec":
		if f == 0 {
			c.rect(0, 0, c.w, c.h, pMiss)
		} else {
			c.rect(0, 0, c.w, c.h, alpha(pMiss, 0.35))
		}
	}
}

func drawSky(c *canvas, tier string) {
	wall := tierWall[tier]
	// Night sky in four stepped bands, darkest at the top.
	for i := 0; i < 4; i++ {
		c.rect(0, i*c.h/4, c.w, c.h/4+1, lerp(pBG, wall, float64(i)/4))
	}
	for i := 0; i < 24; i++ { // stars
		s := seedOf(tier + strconv.Itoa(i))
		c.rect(s%c.w, (s/7)%(c.h/2), 1, 1, pText3)
	}
}

func lerp(a, b color.RGBA, t float64) color.RGBA {
	m := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.RGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 255}
}

func drawSkyline(c *canvas, region string) {
	sky := regionSky[region]
	silhouette := lerp(sky, pBG, 0.6)
	s := seedOf(region)
	x := 0
	for x < c.w {
		w := 14 + s%22
		h := 30 + (s/5)%70
		top := c.h - 40 - h
		c.rect(x, top, w, h+40, silhouette)
		for wy := top + 4; wy < c.h-44; wy += 6 { // lit windows
			for wx := x + 3; wx < x+w-3; wx += 5 {
				if (wx*7+wy*13+s)%9 == 0 {
					c.rect(wx, wy, 2, 2, alpha(pBrand, 0.7))
				}
			}
		}
		x += w + 2
		s = s*1103515245 + 12345
		if s < 0 {
			s = -s
		}
	}
}

// drawInterior draws the room with its windows left clear, so the city
// behind shows through; the stadium is open to the sky.
func drawInterior(c *canvas, tier string) {
	wall := tierWall[tier]
	floor := lerp(wall, pBG, 0.4)
	switch tier {
	case "stadium":
		// Stands: rows of dark tiers rising at both sides.
		for i := 0; i < 6; i++ {
			c.rect(0, 120+i*8, 90-i*10, 8, lerp(wall, pSurface, float64(i)/6))
			c.rect(c.w-90+i*10, 120+i*8, 90-i*10, 8, lerp(wall, pSurface, float64(i)/6))
		}
		c.rect(0, 168, c.w, 32, floor)
		return
	}
	c.rect(0, 0, c.w, 168, wall)
	win := pBG
	win.A = 0
	switch tier {
	case "bedroom":
		c.rect(300, 40, 90, 60, win) // one window
		c.rect(296, 36, 98, 4, pLine)
		c.rect(296, 100, 98, 4, pLine)
	case "jamroom", "pub", "studio":
		for i := 0; i < 3; i++ {
			c.rect(60+i*140, 30, 70, 50, win)
			c.rect(58+i*140, 80, 74, 3, pLine)
		}
	case "opening":
		c.rect(0, 0, c.w, 24, lerp(wall, pBG, 0.5)) // the theatre's dark rig
	}
	c.rect(0, 168, c.w, 32, floor)
	c.rect(0, 168, c.w, 1, pLine)
}

func drawLights(c *canvas, tier string, f int) {
	acc := TierAccent[tier]
	n := 3
	for i := 0; i < n; i++ {
		x := 60 + i*(c.w-120)/(n-1)
		if (i+f)%4 == 3 {
			continue // one lamp off per step: lights cycle
		}
		for y := 0; y < 150; y++ {
			w := 6 + y/3
			c.rect(x-w/2, y, w, 1, alpha(acc, 0.10))
		}
		c.rect(x-3, 0, 6, 4, acc)
	}
}

func drawStage(c *canvas, tier string) {
	if tier == "studio" {
		c.rect(320, 100, 120, 68, alpha(pText2, 0.15)) // the control-room glass
		c.rect(318, 98, 124, 2, pLine)
		c.rect(80, 140, 30, 28, pRaised) // an amp
		return
	}
	stageTop := 168
	if tier != "bedroom" {
		stageTop = 160
		c.rect(0, 160, 300, 8, pSurface) // the stage edge
		c.rect(0, 160, 300, 1, pLine)
	}
	c.rect(40, stageTop-30, 26, 30, pRaised) // amp
	c.rect(42, stageTop-28, 22, 18, pSurface)
	if tier != "bedroom" {
		c.rect(200, stageTop-22, 30, 22, pRaised) // drum kit
		c.rect(196, stageTop-30, 10, 2, pText3)
		c.rect(226, stageTop-34, 10, 2, pText3)
	}
}

// drawGuitarist draws a 32×48 frame of a preset: idle, strum, cheer, bow
// and slump.
func drawGuitarist(c *canvas, preset, anim string, f int) {
	look, ok := presetLooks[preset]
	if !ok {
		look = presetLooks["rae"]
	}
	hair, top, legs, guitar, skin := look[0], look[1], look[2], look[3], look[4]
	headY, lean := 4, 0
	switch anim {
	case "bow":
		headY, lean = 4+f*5, f*2
	case "slump":
		headY = 6 + f*2
	case "idle":
		headY = 4 + f%2
	}
	// Legs and feet.
	c.rect(11, 34, 4, 14, legs)
	c.rect(17, 34, 4, 14, legs)
	c.rect(10, 46, 6, 2, pBG)
	c.rect(17, 46, 6, 2, pBG)
	// Body.
	c.rect(10+lean, 18+lean, 12, 16, top)
	// Head and hair.
	c.rect(12+lean, headY+2, 8, 8, skin)
	c.rect(11+lean, headY, 10, 3, hair)
	// The guitar across the body, and the strumming arm.
	if anim != "bow" && anim != "slump" {
		c.rect(6, 26, 18, 5, guitar)
		c.rect(22, 24, 9, 2, pText3) // the neck
	}
	switch anim {
	case "strum":
		c.rect(18, 22+f*2, 3, 5, skin)
	case "cheer":
		c.rect(22, 6+f*2, 3, 12, skin) // a fist in the air
	default:
		c.rect(18, 26, 3, 5, skin)
	}
}

// drawCrowdMember draws a 16×24 frame of one crowd member.
func drawCrowdMember(c *canvas, variant, anim string, f int) {
	v, _ := strconv.Atoi(variant)
	skin, shirt := skinTones[v%len(skinTones)], shirtCols[(v*5+1)%len(shirtCols)]
	hair := lerp(pBG, skin, 0.3)
	dy := 0
	switch anim {
	case "jump", "armsup":
		dy = -2 * (f % 2)
	case "nod":
		dy = f % 2
	}
	c.rect(4, 14+dy, 8, 10, shirt)
	c.rect(5, 6+dy, 6, 6, skin)
	c.rect(5, 5+dy, 6, 2, hair)
	switch anim {
	case "armsup", "cheer":
		c.rect(2, 4+dy-f, 2, 10, skin)
		c.rect(12, 4+dy-f, 2, 10, skin)
	case "crossed", "boo":
		c.rect(4, 16, 8, 2, skin)
		if anim == "boo" && f == 1 {
			c.rect(7, 10, 2, 2, pBG) // mouth open
		}
	}
}

// drawBoo draws the "BOO" puff: 24×10, growing over three frames.
func drawBoo(c *canvas, f int) {
	col := alpha(pText2, 0.4+0.2*float64(f))
	// B
	c.rect(1, 2, 1, 6, col)
	c.rect(2, 2, 3, 1, col)
	c.rect(2, 4, 3, 1, col)
	c.rect(2, 7, 3, 1, col)
	c.rect(5, 3, 1, 1, col)
	c.rect(5, 5, 1, 2, col)
	for _, x := range []int{8, 15} { // O O
		c.rect(x, 2, 5, 1, col)
		c.rect(x, 7, 5, 1, col)
		c.rect(x, 3, 1, 4, col)
		c.rect(x+4, 3, 1, 4, col)
	}
}

func drawFlames(c *canvas, f int) {
	cols := []color.RGBA{pMiss, pBrand, rgb(0xF6D36B)}
	heights := [3][3]int{{10, 14, 8}, {13, 9, 12}, {8, 12, 14}}[f%3]
	for i, h := range heights {
		x := 2 + i*4
		c.rect(x, 16-h, 4, h, cols[0])
		c.rect(x+1, 16-h+3, 2, h-3, cols[1])
		c.rect(x+1, 16-h/2, 2, h/2, cols[2])
	}
}

func drawStar(c *canvas, lit bool) {
	col := pLine
	if lit {
		col = pBrand
	}
	rows := []string{
		".......##.......",
		".......##.......",
		"......####......",
		"......####......",
		"################",
		".##############.",
		"..############..",
		"...##########...",
		"...##########...",
		"..#####..#####..",
		"..####....####..",
		".###........###.",
	}
	for y, r := range rows {
		for x, ch := range r {
			if ch == '#' {
				c.rect(x, y+2, 1, 1, col)
			}
		}
	}
}
