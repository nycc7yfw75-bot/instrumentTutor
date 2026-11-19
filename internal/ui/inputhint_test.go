package ui

import "testing"

// runFrames advances the practice view n frames with the input at levelDB.
func runFrames(a *App, n int, levelDB float64) {
	a.SetLiveStatus(func() (float64, int64) { return levelDB, 0 })
	for i := 0; i < n; i++ {
		a.frame++
		a.syncLive()
	}
}

func TestClippingHintShowsThenClears(t *testing.T) {
	a := newApp(t, 8)
	var clips int64
	a.SetClipCounter(func() int64 { return clips })
	runFrames(a, 5, -20)
	if h, _ := a.inputHint(); h != "" {
		t.Fatalf("clean input shows %q", h)
	}
	clips = 12
	runFrames(a, 1, -2)
	if h, _ := a.inputHint(); h != inputClipHint {
		t.Fatalf("after clipping the hint is %q, want the clipping hint", h)
	}
	runFrames(a, inputClipHoldFrames+1, -20)
	if h, _ := a.inputHint(); h != "" {
		t.Errorf("3 s after the last clip the hint still reads %q", h)
	}
}

func TestTooQuietNeedsMissesWhilePlaying(t *testing.T) {
	a := newApp(t, 8)
	a.eng.Play()

	// A rest: quiet, but nothing missed.
	runFrames(a, inputQuietWindow+2, -70)
	if h, _ := a.inputHint(); h != "" {
		t.Fatalf("a quiet stretch with no misses raised %q", h)
	}

	// Quiet, and notes going by unheard partway through a window.
	runFrames(a, inputQuietWindow/2, -70)
	a.stats.Miss += 3
	runFrames(a, inputQuietWindow+2, -70)
	if h, _ := a.inputHint(); h != inputQuietHint {
		t.Fatalf("quiet input with misses shows %q, want the too-quiet hint", h)
	}

	// A healthy level with misses is a playing problem, not a level one.
	b := newApp(t, 8)
	b.eng.Play()
	runFrames(b, inputQuietWindow/2, -20)
	b.stats.Miss += 3
	runFrames(b, inputQuietWindow+2, -20)
	if h, _ := b.inputHint(); h != "" {
		t.Errorf("a strong signal with misses was called too quiet: %q", h)
	}
}
