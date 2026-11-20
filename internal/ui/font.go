package ui

import (
	"bytes"
	_ "embed"
	"log"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"image/color"
)

// The three type families (docs/REDESIGN.md §3), embedded with their
// licences in assets/fonts: Pixel Operator (CC0) for display, Inter (OFL)
// for the UI, JetBrains Mono (OFL) for numbers.
var (
	//go:embed assets/fonts/Inter-Regular.ttf
	ttfInter []byte
	//go:embed assets/fonts/Inter-Medium.ttf
	ttfInterMedium []byte
	//go:embed assets/fonts/JetBrainsMono-Regular.ttf
	ttfMono []byte
	//go:embed assets/fonts/JetBrainsMono-Bold.ttf
	ttfMonoBold []byte
	//go:embed assets/fonts/PixelOperator.ttf
	ttfPixel []byte
)

const (
	fontBody  = sizeUI
	fontSmall = sizeUISmall

	uiTextH = 18.0
)

var (
	srcBody     = mustFaceSource(ttfInter)
	srcMedium   = mustFaceSource(ttfInterMedium)
	srcMono     = mustFaceSource(ttfMono)
	srcMonoBold = mustFaceSource(ttfMonoBold)
	srcPixel    = mustFaceSource(ttfPixel)

	faceCache    = map[faceKey]*text.GoTextFace{}
	displayCache = map[float64]text.Face{}
)

// displaySize snaps a scale of the body size to the pixel font's sizes: it
// is crisp only at 16, 24, 32 and 48.
func displaySize(scale float64) float64 {
	want := fontBody * scale
	best := sizeDisplayS
	for _, s := range []float64{sizeDisplayS, sizeDisplayM, sizeDisplayL, sizeDisplayX} {
		if math.Abs(s-want) <= math.Abs(best-want) {
			best = s
		}
	}
	return best
}

// displayFace is the pixel font at the size for scale, falling back to
// Inter for any character it lacks.
func displayFace(scale float64) text.Face {
	size := displaySize(scale)
	if f, ok := displayCache[size]; ok {
		return f
	}
	f, err := text.NewMultiFace(faceOf(srcPixel, size), faceOf(srcMedium, size*0.75))
	if err != nil {
		log.Fatalf("ui: display face: %v", err)
	}
	displayCache[size] = f
	return f
}

type faceKey struct {
	src  *text.GoTextFaceSource
	size float64
}

func mustFaceSource(ttf []byte) *text.GoTextFaceSource {
	s, err := text.NewGoTextFaceSource(bytes.NewReader(ttf))
	if err != nil {
		log.Fatalf("ui: embedded font failed to parse: %v", err)
	}
	return s
}

func faceOf(src *text.GoTextFaceSource, size float64) *text.GoTextFace {
	k := faceKey{src, size}
	if f, ok := faceCache[k]; ok {
		return f
	}
	f := &text.GoTextFace{Source: src, Size: size}
	faceCache[k] = f
	return f
}

func drawText(dst *ebiten.Image, s string, x, y float64, col color.RGBA) {
	drawFace(dst, s, faceOf(srcBody, fontBody), x, y, col)
}

func drawTextSmall(dst *ebiten.Image, s string, x, y float64, col color.RGBA) {
	drawFace(dst, s, faceOf(srcSmallSource(), fontSmall), x, y, col)
}

func srcSmallSource() *text.GoTextFaceSource { return srcBody }

func drawTextMono(dst *ebiten.Image, s string, x, y float64, col color.RGBA) {
	drawFace(dst, s, faceOf(srcMono, sizeMono), x, y, col)
}

// drawTextScaled draws in the display (pixel) font, on whole pixels.
func drawTextScaled(dst *ebiten.Image, s string, x, y, scale float64, col color.RGBA) {
	drawFace(dst, s, displayFace(scale), math.Round(x), math.Round(y), col)
}

func drawFace(dst *ebiten.Image, s string, face text.Face, x, y float64, col color.RGBA) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(col)
	text.Draw(dst, s, face, op)
}

func textW(s string) float64 { return text.Advance(s, faceOf(srcBody, fontBody)) }

func textWScaled(s string, scale float64) float64 {
	return text.Advance(s, displayFace(scale))
}

func textWMono(s string) float64 { return text.Advance(s, faceOf(srcMono, sizeMono)) }

func textWSmall(s string) float64 { return text.Advance(s, faceOf(srcSmallSource(), fontSmall)) }

func fitToWidth(s string, maxPx float64, face text.Face, keepTail bool) string {
	if maxPx <= 0 {
		return ""
	}
	if text.Advance(s, face) <= maxPx {
		return s
	}
	const ell = "…"
	r := []rune(s)

	for n := len(r) - 1; n > 0; n-- {
		var candidate string
		if keepTail {
			candidate = ell + string(r[len(r)-n:])
		} else {
			candidate = string(r[:n]) + ell
		}
		if text.Advance(candidate, face) <= maxPx {
			return candidate
		}
	}

	if text.Advance(ell, face) <= maxPx {
		return ell
	}
	return ""
}

func truncateW(s string, maxPx float64) string {
	return fitToWidth(s, maxPx, faceOf(srcBody, fontBody), false)
}

func truncateWScaled(s string, maxPx, scale float64) string {
	return fitToWidth(s, maxPx, displayFace(scale), false)
}

func ellipsizeW(s string, maxPx float64) string {
	return fitToWidth(s, maxPx, faceOf(srcBody, fontBody), true)
}

func ellipsizeWSmall(s string, maxPx float64) string {
	return fitToWidth(s, maxPx, faceOf(srcSmallSource(), fontSmall), true)
}

func wrapTextW(s string, maxPx float64) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	face := faceOf(srcBody, fontBody)
	var out []string
	line := words[0]
	for _, w := range words[1:] {
		if text.Advance(line+" "+w, face) > maxPx {
			out = append(out, line)
			line = w
			continue
		}
		line += " " + w
	}
	return append(out, line)
}
