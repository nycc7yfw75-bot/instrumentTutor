package online

import (
	"errors"
	"testing"
	"time"
)

func TestTheCredentialManagerKeepsTheAccount(t *testing.T) {
	cs := CredStore{Target: "Headliner/test-" + time.Now().Format("150405.000000")}
	defer cs.Clear()
	want := Saved{RefreshToken: "rt-test", UserID: "u", Email: "alex@example.com"}
	if err := cs.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := cs.Load()
	if err != nil || got != want {
		t.Fatalf("round trip: %+v, %v", got, err)
	}
	if err := cs.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.Load(); !errors.Is(err, ErrNoToken) {
		t.Errorf("after Clear: %v", err)
	}
}
