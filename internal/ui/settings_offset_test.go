package ui

import "testing"

type settingsTunableAudio struct{ *settingsFakeAudio }

func (a settingsTunableAudio) SetOffsetManually(captureID, playbackID string, frames int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.offsets == nil {
		a.offsets = map[string]settingsOffset{}
	}
	a.offsets[captureID+"|"+playbackID] = settingsOffset{frames: frames, ok: true}
	return nil
}

func TestOffsetNudgeStepsFiveMillisecondsAndNeverGoesNegative(t *testing.T) {
	s, _ := newSettingsFixture(t, settingsTunableAudio{newSettingsAudio()})
	if !s.canNudgeOffset() {
		t.Fatal("the - / + offset buttons are not offered with a tunable device")
	}
	s.nudgeOffset(+1)
	s.nudgeOffset(+1)
	if !s.offOK || s.offFrames != 480 {
		t.Fatalf("two + steps from unmeasured: %d frames (stored %v), want 480 (10 ms at 48 kHz)", s.offFrames, s.offOK)
	}
	s.nudgeOffset(-1)
	s.nudgeOffset(-1)
	s.nudgeOffset(-1)
	if s.offFrames != 0 {
		t.Fatalf("stepping below zero left %d frames, want 0", s.offFrames)
	}
}

func TestOffsetNudgeHiddenWithoutASetter(t *testing.T) {
	s, _ := newSettingsFixture(t, newSettingsAudio())
	if s.canNudgeOffset() {
		t.Fatal("offered - / + with an audio service that cannot store a hand-set offset")
	}
}
