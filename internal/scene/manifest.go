// Package scene is the performance scene's art pipeline (CAREER.md §7): it
// reads asset manifests and builds sprite sheets, either from image files
// or, until real art lands, from placeholder art drawn in code. It knows
// nothing about the UI toolkit: sheets are plain images, so everything here
// is testable without a window.
package scene

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Base is the scene's resolution; it is scaled up by whole numbers.
const (
	BaseW = 480
	BaseH = 200
)

// Layers, back to front (§6.1).
var Layers = []string{"backdrop", "city", "interior", "lights", "crowd-back", "stage", "guitarist", "crowd-front", "fx"}

// Anim is one animation in a sheet: a row of frames.
type Anim struct {
	Name   string `json:"name"`
	Frames int    `json:"frames"`
	Row    int    `json:"row"`
}

// Sprite is one entry in a manifest.
type Sprite struct {
	Name string `json:"name"`
	// Image is a PNG file next to the manifest, or "placeholder:<kind>"
	// for art drawn in code.
	Image  string `json:"image"`
	FrameW int    `json:"frameW"`
	FrameH int    `json:"frameH"`
	Anims  []Anim `json:"anims"`
	// Anchor is the point of a frame placed at Pos (feet for a character).
	Anchor [2]int `json:"anchor"`
	Layer  string `json:"layer"`
	Pos    [2]int `json:"pos"` // at base resolution
}

// Manifest is one asset group's manifest.json.
type Manifest struct {
	Group   string   `json:"group"`
	Sprites []Sprite `json:"sprites"`
	Sounds  []Sound  `json:"sounds,omitempty"`
}

// Sound is a crowd sound: a WAV next to the manifest, or a placeholder.
type Sound struct {
	Name  string  `json:"name"`
	File  string  `json:"file"`
	Loop  bool    `json:"loop"`
	Level float64 `json:"levelDB"`
}

// Library is every manifest, by group ("characters/rae").
type Library struct {
	fsys   fs.FS
	groups map[string]*Manifest
	sheets map[string]image.Image
}

// Load reads every manifest.json under fsys.
func Load(fsys fs.FS) (*Library, error) {
	lib := &Library{fsys: fsys, groups: map[string]*Manifest{}, sheets: map[string]image.Image{}}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Base(p) != "manifest.json" {
			return err
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("scene: %s: %w", p, err)
		}
		if m.Group == "" {
			m.Group = path.Dir(p)
		}
		for _, s := range m.Sprites {
			if err := s.check(); err != nil {
				return fmt.Errorf("scene: %s: %w", p, err)
			}
		}
		lib.groups[m.Group] = &m
		return nil
	})
	return lib, err
}

func (s Sprite) check() error {
	switch {
	case s.Name == "" || s.Image == "":
		return fmt.Errorf("a sprite needs a name and an image")
	case s.FrameW <= 0 || s.FrameH <= 0:
		return fmt.Errorf("%s: frame size %dx%d", s.Name, s.FrameW, s.FrameH)
	case len(s.Anims) == 0:
		return fmt.Errorf("%s: no animations", s.Name)
	}
	known := false
	for _, l := range Layers {
		known = known || l == s.Layer
	}
	if !known {
		return fmt.Errorf("%s: unknown layer %q", s.Name, s.Layer)
	}
	return nil
}

// Groups lists the loaded groups, sorted.
func (l *Library) Groups() []string {
	out := make([]string, 0, len(l.groups))
	for g := range l.groups {
		out = append(out, g)
	}
	sort.Strings(out)
	return out
}

// Group is one group's manifest.
func (l *Library) Group(name string) (*Manifest, bool) {
	m, ok := l.groups[name]
	return m, ok
}

// Sprite finds a sprite by group and name.
func (l *Library) Sprite(group, name string) (Sprite, bool) {
	m, ok := l.groups[group]
	if !ok {
		return Sprite{}, false
	}
	for _, s := range m.Sprites {
		if s.Name == name {
			return s, true
		}
	}
	return Sprite{}, false
}

// Sheet is a sprite's sheet: its PNG, or its placeholder drawn in code.
// Sheets are built once and kept.
func (l *Library) Sheet(group string, s Sprite) (image.Image, error) {
	key := group + "/" + s.Name
	if img, ok := l.sheets[key]; ok {
		return img, nil
	}
	var img image.Image
	if kind, ok := strings.CutPrefix(s.Image, "placeholder:"); ok {
		img = Placeholder(kind, s)
	} else {
		f, err := l.fsys.Open(path.Join(group, s.Image))
		if err != nil {
			return nil, fmt.Errorf("scene: %s: %w", key, err)
		}
		defer f.Close()
		if img, err = png.Decode(f); err != nil {
			return nil, fmt.Errorf("scene: %s: %w", key, err)
		}
	}
	l.sheets[key] = img
	return img, nil
}

// Frame is the rectangle of an animation's frame within its sheet.
func (s Sprite) Frame(anim string, i int) image.Rectangle {
	for _, a := range s.Anims {
		if a.Name == anim {
			i = ((i % a.Frames) + a.Frames) % a.Frames
			return image.Rect(i*s.FrameW, a.Row*s.FrameH, (i+1)*s.FrameW, (a.Row+1)*s.FrameH)
		}
	}
	return image.Rect(0, 0, s.FrameW, s.FrameH)
}

// Has reports whether a sprite has an animation.
func (s Sprite) Has(anim string) bool {
	for _, a := range s.Anims {
		if a.Name == anim {
			return true
		}
	}
	return false
}
