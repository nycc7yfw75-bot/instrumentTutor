package ui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Dev-only screenshots (docs/REDESIGN.md §2): with HEADLINER_SHOT=<dir>
// set, F12 saves the frame as <dir>/<screen>-<w>x<h>.png. Unset, nothing
// here runs.

var shotDir = os.Getenv("HEADLINER_SHOT")

// screenName names a screen for its screenshot file.
func screenName(sc Screen) string {
	switch s := sc.(type) {
	case *Browser:
		return "start"
	case *App:
		switch {
		case s.helpOpen:
			return "practice-help"
		case s.mixer.open:
			return "practice-mixer"
		case s.game.review.ov.open:
			return "practice-review"
		case s.tunerView:
			return "practice-tuner"
		}
		return "practice"
	case *Settings:
		return "settings"
	case *Editor:
		return "editor"
	case *NamePrompt:
		return "name"
	}
	n := fmt.Sprintf("%T", sc)
	return strings.ToLower(n[strings.LastIndex(n, ".")+1:])
}

// shotWanted is set in Update when F12 is pressed; the next Draw saves.
var shotWanted bool

// noteShotKey runs in Update: input is read there, not in Draw.
func noteShotKey() {
	if shotDir != "" && inpututil.IsKeyJustPressed(ebiten.KeyF12) {
		shotWanted = true
	}
}

// maybeShoot saves dst if F12 was pressed.
func maybeShoot(dst *ebiten.Image, sc Screen) {
	if !shotWanted {
		return
	}
	shotWanted = false
	b := dst.Bounds()
	pix := make([]byte, 4*b.Dx()*b.Dy())
	dst.ReadPixels(pix)
	img := &image.RGBA{Pix: pix, Stride: 4 * b.Dx(), Rect: image.Rect(0, 0, b.Dx(), b.Dy())}
	ww, wh := ebiten.WindowSize()
	name := fmt.Sprintf("%s-%dx%d.png", screenName(sc), ww, wh)
	if err := os.MkdirAll(shotDir, 0o755); err != nil {
		return
	}
	f, err := os.Create(filepath.Join(shotDir, name))
	if err != nil {
		return
	}
	defer f.Close()
	_ = png.Encode(f, img)
}
