package ui

import (
	"context"
	"errors"
	"testing"
)

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

func TestOffsetNudgeStepsTwoMillisecondsWithin150(t *testing.T) {
	s, _ := newSettingsFixture(t, settingsTunableAudio{newSettingsAudio()})
	if !s.canNudgeOffset() {
		t.Fatal("the fine-tune buttons are not offered with a tunable device")
	}
	s.nudgeOffset(+1)
	s.nudgeOffset(+1)
	if !s.offOK || s.offFrames != 192 {
		t.Fatalf("two steps up from unmeasured: %d frames (stored %v), want 192 (4 ms at 48 kHz)", s.offFrames, s.offOK)
	}
	for i := 0; i < 200; i++ {
		s.nudgeOffset(-1)
	}
	if s.offFrames != -7200 {
		t.Fatalf("stepping far down stopped at %d frames, want -7200 (-150 ms)", s.offFrames)
	}
	for i := 0; i < 400; i++ {
		s.nudgeOffset(+1)
	}
	if s.offFrames != 7200 {
		t.Fatalf("stepping far up stopped at %d frames, want 7200 (+150 ms)", s.offFrames)
	}
}

func TestOffsetNudgeHiddenWithoutASetter(t *testing.T) {
	s, _ := newSettingsFixture(t, newSettingsAudio())
	if s.canNudgeOffset() {
		t.Fatal("offered - / + with an audio service that cannot store a hand-set offset")
	}
}

type settingsTapAudio struct {
	*settingsFakeAudio
	frames int
	conf   float64
	err    error
}

func (a settingsTapAudio) TapCalibrate(ctx context.Context, captureID, playbackID string, progress func(float64)) (int, float64, error) {
	progress(1)
	return a.frames, a.conf, a.err
}

func TestTapCalibrationReportsInPlainWords(t *testing.T) {
	s, _ := newSettingsFixture(t, settingsTapAudio{settingsFakeAudio: newSettingsAudio(), frames: 1632, conf: 0.9})
	if s.rowIndex(srTap) < 0 {
		t.Fatal("no tap calibrate row with a tap-capable audio service")
	}
	if !s.startCalibrationKind(true) {
		t.Fatal("tap calibration did not start")
	}
	sn := settingsWaitPhase(t, s, calDone)
	if txt, _ := s.tapText(sn); txt != "Offset 34 ms · steady" {
		t.Errorf("tap result reads %q, want %q", txt, "Offset 34 ms · steady")
	}

	bad, _ := newSettingsFixture(t, settingsTapAudio{settingsFakeAudio: newSettingsAudio(), err: errors.New("too uneven — try again with muted strings")})
	bad.startCalibrationKind(true)
	sn = settingsWaitPhase(t, bad, calFailed)
	if txt, _ := bad.tapText(sn); txt != "too uneven — try again with muted strings" {
		t.Errorf("a failed tap reads %q", txt)
	}
}
