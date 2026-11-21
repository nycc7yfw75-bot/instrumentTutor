package scene

import (
	"image"
	"os"
	"strings"
	"testing"
)

func loadAssets(t *testing.T) *Library {
	t.Helper()
	lib, err := Load(os.DirFS("../ui/assets/career"))
	if err != nil {
		t.Fatal(err)
	}
	return lib
}

func TestEveryManifestLoadsAndEverySheetBuilds(t *testing.T) {
	lib := loadAssets(t)
	if n := len(lib.Groups()); n < 20 {
		t.Fatalf("%d asset groups, want the 20 manifests", n)
	}
	for _, g := range lib.Groups() {
		m, _ := lib.Group(g)
		for _, s := range m.Sprites {
			img, err := lib.Sheet(g, s)
			if err != nil {
				t.Fatalf("%s/%s: %v", g, s.Name, err)
			}
			for _, a := range s.Anims {
				r := s.Frame(a.Name, a.Frames-1)
				if !r.In(img.Bounds()) {
					t.Errorf("%s/%s: frame %s#%d at %v is outside the %v sheet", g, s.Name, a.Name, a.Frames-1, r, img.Bounds())
				}
				if !inked(img, s.Frame(a.Name, 0)) {
					t.Errorf("%s/%s: animation %s draws nothing", g, s.Name, a.Name)
				}
			}
		}
	}
}

func inked(img image.Image, r image.Rectangle) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a > 0 {
				return true
			}
		}
	}
	return false
}

func TestTheArtCoversEveryTierRegionAndPreset(t *testing.T) {
	lib := loadAssets(t)
	want := []string{"crowd", "fx", "audio"}
	for _, tier := range []string{"bedroom", "jamroom", "pub", "opening", "stadium", "studio"} {
		want = append(want, "scenes/"+tier)
	}
	for _, r := range []string{"south-asia", "east-asia", "europe", "north-america", "latin-america", "oceania", "africa"} {
		want = append(want, "cities/"+r)
	}
	for _, p := range []string{"rae", "theo", "mina", "sol"} {
		want = append(want, "characters/"+p)
	}
	for _, g := range want {
		if _, ok := lib.Group(g); !ok {
			t.Errorf("no manifest for %s", g)
		}
	}
	gs, _ := lib.Sprite("characters/rae", "guitarist")
	for _, a := range []string{"idle", "strum", "cheer", "bow", "slump"} {
		if !gs.Has(a) {
			t.Errorf("the guitarist has no %s animation (§6.3)", a)
		}
	}
}

func TestABadManifestIsRefused(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir+"/x", 0o755)
	os.WriteFile(dir+"/x/manifest.json", []byte(`{"sprites":[{"name":"a","image":"placeholder:sky:pub","frameW":0,"frameH":1,"anims":[{"name":"s","frames":1}],"layer":"backdrop"}]}`), 0o644)
	if _, err := Load(os.DirFS(dir)); err == nil || !strings.Contains(err.Error(), "frame size") {
		t.Errorf("a zero-width frame loaded: %v", err)
	}
}
