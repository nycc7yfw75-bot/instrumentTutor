package ui

import "testing"

func TestSoundCheckNeedsThreeSeparatePlucks(t *testing.T) {
	var c soundCheck
	gate := soundCheckGate
	// One long note is one pluck, not three.
	for i := 0; i < 30; i++ {
		c.update(gate+10, 0.05)
	}
	if c.onsets != 1 || c.done {
		t.Fatalf("a held note counted %d plucks (done %v)", c.onsets, c.done)
	}
	// Two more plucks, each after the level falls away.
	for p := 0; p < 2; p++ {
		c.update(gate-20, 0.05)
		c.update(gate+10, 0.05)
	}
	if !c.done || !c.passed {
		t.Errorf("three plucks: done %v passed %v", c.done, c.passed)
	}
}

func TestSoundCheckFailsQuietlyAndRetries(t *testing.T) {
	var c soundCheck
	for i := 0; i < 70; i++ {
		c.update(-80, 0.05)
	}
	if !c.done || c.passed {
		t.Fatalf("silence for 3.5 s: done %v passed %v; want a fail", c.done, c.passed)
	}
	c.retry()
	if c.done || c.onsets != 0 || c.t != 0 {
		t.Error("retry did not start over")
	}
}
