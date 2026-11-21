package main

import (
	"path/filepath"
	"testing"

	"github.com/S95F/musicTutor/internal/career"
	"github.com/S95F/musicTutor/internal/progress"
)

func TestTheGrantFlagOnlyWorksWhenSet(t *testing.T) {
	st, err := progress.OpenFile(filepath.Join(t.TempDir(), "progress.json"))
	if err != nil {
		t.Fatal(err)
	}
	st.CreateProfile("Me")
	st.UpdateCareer(func(c *career.State) { c.Setup("Paris", 0, 3) })
	t.Setenv("HEADLINER_GRANT", "")
	grantForTesting(st)
	if st.Career().TierCleared(1) {
		t.Fatal("the Bedroom was cleared without the flag")
	}
	t.Setenv("HEADLINER_GRANT", "tier1")
	grantForTesting(st)
	if !st.Career().TierCleared(1) {
		t.Error("HEADLINER_GRANT=tier1 did not clear the Bedroom")
	}
}
