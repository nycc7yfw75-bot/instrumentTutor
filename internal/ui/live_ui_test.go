package ui

import (
	"strings"
	"testing"

	"github.com/S95F/musicTutor/internal/practice"
)

const splitDeviceMsg = "capture and playback are different devices (Microphone Array (Intel Smart Sound Technology for Digital Microphones) / Speakers (Realtek(R) Audio)): their clocks drift apart over a session and timing scores wander"

func TestTunerIdleLineIsHonest(t *testing.T) {
	a := newApp(t, 1)

	s, scale := a.tunerIdleLine()
	if strings.Contains(s, "settings") || !strings.Contains(s, "-listen") {
		t.Errorf("with no live input and no settings screen the tuner says %q, want the command-line remedy", s)
	}
	if w := textWScaled(s, scale); w > screenW {
		t.Errorf("the idle line is %.0f px wide at scale %v, past the %.0f px window", w, scale, screenW)
	}
	a.SetSettingsOpener(func() {})
	if s, _ := a.tunerIdleLine(); !strings.Contains(s, "settings") {
		t.Errorf("with settings wired the tuner says %q, want it to point at settings", s)
	}

	a.SetLiveStatus(func() (float64, int64) { return -20, 0 })
	a.syncLive()
	if s, _ := a.tunerIdleLine(); s != "listening..." {
		t.Errorf("a live session's idle tuner says %q, want %q", s, "listening...")
	}
}

func TestDismissedWarningReturnsForANewOccurrence(t *testing.T) {
	a := newApp(t, 1)
	const msg = "could not re-open the piece: truncated archive"

	a.SetLiveWarning(msg)
	a.syncLive()
	if !a.warningVisible() {
		t.Fatal("the first failure raised no banner")
	}
	a.dismissWarning()

	for i := 0; i < 3; i++ {
		a.syncLive()
		if a.warningVisible() {
			t.Fatal("the dismissed banner came back on its own")
		}
		if a.warnMsg != msg {
			t.Fatalf("the message was lost while dismissed: %q", a.warnMsg)
		}
	}

	a.SetLiveWarning(msg)
	a.syncLive()
	if !a.warningVisible() {
		t.Error("the second identical failure raised no banner")
	}
}

func TestRepeatedWarningStaysDismissed(t *testing.T) {
	a := newApp(t, 1)
	const msg = "input stream stopped"

	for i := 0; i < 5; i++ {
		a.SetLiveWarning(msg)
		a.syncLive()
	}
	if !a.warningVisible() {
		t.Fatal("the polled condition raised no banner")
	}
	a.dismissWarning()
	for i := 0; i < 60; i++ {
		a.SetLiveWarning(msg)
		a.syncLive()
		if a.warningVisible() {
			t.Fatalf("the banner came back %d frames after being dismissed", i+1)
		}
	}

	a.SetLiveWarning("")
	a.syncLive()
	a.SetLiveWarning(msg)
	a.syncLive()
	if !a.warningVisible() {
		t.Error("the condition returning after being cleared raised no banner")
	}
}

func TestALongWarningStaysFullyReadable(t *testing.T) {
	a := newApp(t, 1)
	a.SetImportWarnings([]string{"a bend on bar 12 was simplified"})
	a.SetLiveWarning("the saved capture device is not connected; " + splitDeviceMsg + "; timing is not calibrated")
	a.syncLive()
	items := a.bannerItems()
	if len(items) != 3 || !strings.HasPrefix(items[0], "The saved capture") {
		t.Fatalf("banner conditions %q, want the three parts, the first leading", items)
	}
	// Everything the banner cannot show is one click away, in full.
	all := a.warnItems()
	if len(all) != 4 || all[1] != sentenceCase(splitDeviceMsg) || all[3] != "a bend on bar 12 was simplified" {
		t.Errorf("the warnings list holds %q; want every banner part and the import warning", all)
	}
	for _, l := range a.warnListLines() {
		if w := textW(l.text); w > wlCardW-2*wlPadX-wlNumW {
			t.Errorf("a list line is %.0f px wide, past the card: %q", w, l.text)
		}
	}
	if b := bannerRect(); b.x < 0 || b.x+b.w > screenW || b.h < 40 {
		t.Errorf("the banner %+v is off the window or under a 40px target", b)
	}
}

func TestHUDAccuracyEarnsItsPercentage(t *testing.T) {
	a := newApp(t, 1)
	if got := a.hudAccuracy(); got != "—" {
		t.Errorf("with nothing judged the HUD reads %q, want —", got)
	}
	a.stats = practice.Stats{Hit: 1, Miss: 1}
	if got := a.hudAccuracy(); got != "50%" {
		t.Errorf("one hit and one miss should read 50%%, got %q", got)
	}
}
