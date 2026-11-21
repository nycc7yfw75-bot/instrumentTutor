package ui

import (
	"embed"
	"image"
	"io/fs"
	"math"
	"strings"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/S95F/musicTutor/internal/career"
	"github.com/S95F/musicTutor/internal/scene"
)

// The performance scene's art (CAREER.md §6–7): the manifests and sheets
// from internal/scene, turned into images the screen can draw. Everything
// is built once, on first use.

//go:embed assets/career
var careerAssets embed.FS

var (
	sceneOnce   sync.Once
	sceneLib    *scene.Library
	sceneSheets = map[string]*ebiten.Image{}
)

// sceneLibrary is the loaded art, or nil if it cannot be read (the scene
// then draws nothing rather than taking the screen down).
func sceneLibrary() *scene.Library {
	sceneOnce.Do(func() {
		sub, err := fs.Sub(careerAssets, "assets/career")
		if err != nil {
			return
		}
		if lib, err := scene.Load(sub); err == nil {
			sceneLib = lib
		}
	})
	return sceneLib
}

// sceneSheet is a sprite's sheet as an image, with the sprite.
func sceneSheet(group, name string) (*ebiten.Image, scene.Sprite, bool) {
	lib := sceneLibrary()
	if lib == nil {
		return nil, scene.Sprite{}, false
	}
	s, ok := lib.Sprite(group, name)
	if !ok {
		return nil, scene.Sprite{}, false
	}
	key := group + "/" + name
	if img, ok := sceneSheets[key]; ok {
		return img, s, true
	}
	src, err := lib.Sheet(group, s)
	if err != nil {
		return nil, scene.Sprite{}, false
	}
	img := ebiten.NewImageFromImage(src)
	sceneSheets[key] = img
	return img, s, true
}

// drawSprite draws one frame of a sprite onto a base-resolution image,
// its anchor at (x, y), on whole pixels.
func drawSprite(dst *ebiten.Image, group, name, anim string, frame int, x, y float64, alpha float32) {
	img, s, ok := sceneSheet(group, name)
	if !ok {
		return
	}
	r := s.Frame(anim, frame)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(math.Round(x)-float64(s.Anchor[0]), math.Round(y)-float64(s.Anchor[1]))
	if alpha < 1 {
		op.ColorScale.ScaleAlpha(alpha)
	}
	dst.DrawImage(img.SubImage(r).(*ebiten.Image), op)
}

// tierSlug names a tier's scene folder; 0 is the recording studio.
func tierSlug(tier int, recording bool) string {
	if recording {
		return "studio"
	}
	return [...]string{"bedroom", "bedroom", "jamroom", "pub", "opening", "stadium"}[max(0, min(5, tier))]
}

// regionSlug names a region's city folder.
func regionSlug(region string) string {
	return strings.ToLower(strings.ReplaceAll(region, " ", "-"))
}

// presetSlug names a character's folder.
func presetSlug(i int) string {
	if i < 0 || i >= len(career.Characters) {
		i = 0
	}
	return strings.ToLower(career.Characters[i].Name)
}

// drawVenue draws a venue's still layers (sky, city, interior, stage) at
// base resolution: the backdrop of the scene and of the Perform door.
func drawVenue(dst *ebiten.Image, tier string, region string, lightsFrame int) {
	g := "scenes/" + tier
	drawSprite(dst, g, "sky", "still", 0, 0, 0, 1)
	if region != "" {
		drawSprite(dst, "cities/"+regionSlug(region), "skyline", "still", 0, 0, 0, 1)
	}
	drawSprite(dst, g, "interior", "still", 0, 0, 0, 1)
	drawSprite(dst, g, "lights", "cycle", lightsFrame, 0, 0, 1)
	drawSprite(dst, g, "stage", "still", 0, 0, 0, 1)
}

// sceneScale is the largest whole-number scale at which the base scene
// fits w×h (never below 1).
func sceneScale(w, h float64) int {
	return max(1, int(math.Min(w/scene.BaseW, h/scene.BaseH)))
}

// rectImage is a rect as an image rectangle, for clipping.
func rectImage(r rect) image.Rectangle {
	return image.Rect(int(r.x), int(r.y), int(math.Ceil(r.x+r.w)), int(math.Ceil(r.y+r.h)))
}

// drawSceneFilled draws a base-resolution scene into area at the largest
// whole-number scale that fits its width (cropping the top if it is too
// tall, so the stage stays in view), centred, and fills any space left by
// stretching the scene's edge rows and columns outward (CAREER.md §6.1: no
// black bars).
func drawSceneFilled(dst, src *ebiten.Image, area rect) {
	clip := dst.SubImage(rectImage(area)).(*ebiten.Image)
	sw, sh := float64(src.Bounds().Dx()), float64(src.Bounds().Dy())
	scale := max(1, int(area.w/sw))
	w, h := sw*float64(scale), sh*float64(scale)
	x := math.Round(area.x + (area.w-w)/2)
	y := math.Round(area.y + (area.h-h)/2)
	if h > area.h {
		y = math.Round(area.y + area.h - h) // keep the floor and stage
	}
	edge := func(sub image.Rectangle, tx, ty, sx, sy float64) {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(sx, sy)
		op.GeoM.Translate(tx, ty)
		clip.DrawImage(src.SubImage(sub).(*ebiten.Image), op)
	}
	iw, ih := src.Bounds().Dx(), src.Bounds().Dy()
	if y > area.y { // above: the corner's colour (the top row may hold lamps)
		edge(image.Rect(0, 0, 1, 1), x, area.y, w, y-area.y)
	}
	if b := y + h; b < area.y+area.h { // below: the bottom row
		edge(image.Rect(0, ih-1, iw, ih), x, b, float64(scale), area.y+area.h-b)
	}
	if x > area.x { // left and right: the edge columns, full height
		edge(image.Rect(0, 0, 1, ih), area.x, y, x-area.x, float64(scale))
		edge(image.Rect(iw-1, 0, iw, ih), x+w, y, area.x+area.w-(x+w), float64(scale))
		if y > area.y {
			edge(image.Rect(0, 0, 1, 1), area.x, area.y, x-area.x, y-area.y)
			edge(image.Rect(iw-1, 0, iw, 1), x+w, area.y, area.x+area.w-(x+w), y-area.y)
		}
		if b := y + h; b < area.y+area.h {
			edge(image.Rect(0, ih-1, 1, ih), area.x, b, x-area.x, area.y+area.h-b)
			edge(image.Rect(iw-1, ih-1, iw, ih), x+w, b, area.x+area.w-(x+w), area.y+area.h-b)
		}
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(scale), float64(scale))
	op.GeoM.Translate(x, y)
	clip.DrawImage(src, op)
}
