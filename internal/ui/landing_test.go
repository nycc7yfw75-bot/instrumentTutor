package ui

import (
	"strings"
	"testing"

	"github.com/S95F/musicTutor/internal/career"
)

func landingFixture(t *testing.T) *Landing {
	t.Helper()
	st := tempStore(t, "Alex")
	st.UpdateCareer(func(c *career.State) { c.Setup("Mumbai", 1, 42) })
	l := NewLanding(NewShell(Services{Prefs: &browserFakePrefs{}, Progress: st}, nil))
	l.SetPractice(func() {})
	l.SetPerform(func() {})
	l.SetSettings(func() {})
	return l
}

func TestLandingShowsPracticeAndPerformButNoBattlesYet(t *testing.T) {
	l := landingFixture(t)
	var ids []string
	for _, d := range l.doors() {
		ids = append(ids, d.id)
	}
	if strings.Join(ids, ",") != "practice,perform" {
		t.Errorf("doors %v; Battles must be absent until it is built (no coming soon)", ids)
	}
	l.SetBattles(func() {})
	if ds := l.doors(); len(ds) != 3 || ds[1].id != "perform" {
		t.Errorf("with Battles the doors are %v; Perform belongs in the middle", ds)
	}
}

func TestLandingFitsAtEverySize(t *testing.T) {
	atEachSize(t, func(t *testing.T) {
		l := landingFixture(t)
		ll := l.layout()
		ds := ll.doors
		if ds[1].w <= ds[0].w {
			t.Errorf("Perform (%.0f px) should be the biggest door (Practice %.0f)", ds[1].w, ds[0].w)
		}
		for i, r := range ds {
			if r.x < pgMargin || r.x+r.w > screenW-pgMargin+1 || r.y+r.h > screenH-pgMargin+1 {
				t.Errorf("door %d %+v leaves the page", i, r)
			}
			if r.y < ll.medals[0].y+ll.medals[0].h {
				t.Errorf("door %d starts under the medal row", i)
			}
		}
		if len(ll.medals) != landingMedals || ll.medals[0].x < screenW/2 {
			t.Errorf("the medals %v belong at the top right with the name", ll.medals)
		}
		if ll.settings.x+ll.settings.w > ll.help.x+1 {
			t.Error("Settings and Help overlap")
		}
	})
}

func TestPerformDoorSaysWhereTheCareerIs(t *testing.T) {
	l := landingFixture(t)
	where, next := l.performLines()
	if !strings.HasPrefix(where, "Your career: ") || !strings.Contains(where, "Mumbai") {
		t.Errorf("the Perform door reads %q", where)
	}
	if next != "Master 1 song to play your first gig" {
		t.Errorf("the next step reads %q", next)
	}
}

func TestCareerSetupSavesNameHometownAndGuitarist(t *testing.T) {
	st := tempStore(t, "Old name")
	c := NewCareerSetup(nil, st)
	if c.editing {
		t.Fatal("a profile without a career opened the setup in edit mode")
	}
	c.name, c.city, c.char = "Alex", 2, 3 // Bangalore, Sol
	if err := c.trySave(); err != errQuit {
		t.Fatalf("saving returned %v", err)
	}
	cs := st.Career()
	if name, _, _ := st.Active(); name != "Alex" || cs.Hometown != "Bangalore" || cs.Character != 3 || cs.RouteSeed == 0 {
		t.Errorf("saved %q, %+v", name, cs)
	}
	// Changing the hometown later asks first.
	edit := NewCareerSetup(nil, st)
	edit.city = 0
	if err := edit.trySave(); err != nil || !edit.confirm {
		t.Fatalf("a hometown change saved without asking (err %v)", err)
	}
	if edit.confirmFocus != 0 {
		t.Error("Cancel should have the default focus")
	}
	seed := cs.RouteSeed
	if err := edit.save(); err != errQuit || st.Career().Hometown != "Mumbai" || st.Career().RouteSeed != seed {
		t.Errorf("after confirming: %+v (err %v); the hometown changes, the seed stays", st.Career(), err)
	}
}
