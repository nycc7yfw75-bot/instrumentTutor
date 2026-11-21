package progress

import (
	"path/filepath"
	"testing"
)

func TestSeveralProfilesShareTheLaptop(t *testing.T) {
	s, err := OpenFile(filepath.Join(t.TempDir(), "progress.json"))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := s.CreateProfile("Aarsh")
	b, err := s.AddProfile("Friend")
	if err != nil {
		t.Fatal(err)
	}
	ps := s.Profiles()
	if len(ps) != 2 || !ps[0].Active || ps[0].ID != a {
		t.Fatalf("profiles %+v; Aarsh stays active, first", ps)
	}
	if ps[0].Tint == ps[1].Tint {
		t.Error("two profiles share a tint")
	}
	if err := s.SetActive(b); err != nil {
		t.Fatal(err)
	}
	if name, _, _ := s.Active(); name != "Friend" {
		t.Errorf("active is %q after switching", name)
	}
	if err := s.RenameProfile(a, "Aarsh V"); err != nil {
		t.Fatal(err)
	}
	if long, _ := s.AddProfile("A name that is far too long to fit"); long != "" {
		if p, _ := s.Profile(long); len([]rune(p.Name)) > 20 {
			t.Errorf("a name was kept at %d characters, over 20", len([]rune(p.Name)))
		}
	}
	if err := s.DeleteProfile(b); err != nil {
		t.Fatal(err)
	}
	if name, _, ok := s.Active(); !ok || name == "Friend" {
		t.Errorf("deleting the active profile left %q active (ok %v)", name, ok)
	}
}
