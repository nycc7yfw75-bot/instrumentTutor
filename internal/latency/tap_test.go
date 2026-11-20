package latency

import (
	"errors"
	"testing"
)

const tapSR = 48000

func ms(v float64) int64 { return int64(v * tapSR / 1000) }

// tapsAt places one attack per click at the given lateness (ms).
func tapsAt(late []float64) (clicks, attacks []int64) {
	sp := int64(TapSpacingFrames(tapSR))
	for i := 0; i < TapClicks; i++ {
		c := int64(i) * sp
		clicks = append(clicks, c)
		if i < len(late) {
			attacks = append(attacks, c+ms(late[i]))
		}
	}
	return clicks, attacks
}

func TestTapEstimateTakesTheMedianAfterTheSettlingTaps(t *testing.T) {
	// The first four taps are wild (settling in); the rest sit near 34 ms.
	late := []float64{200, -120, 300, 90, 30, 34, 35, 33, 36, 34, 32, 38, 34, 35, 33, 34}
	clicks, attacks := tapsAt(late)
	off, conf, err := TapEstimate(tapSR, clicks, attacks)
	if err != nil {
		t.Fatalf("TapEstimate: %v", err)
	}
	if got := float64(off) * 1000 / tapSR; got < 33.5 || got > 34.5 {
		t.Errorf("offset %.1f ms, want ~34 (the first four taps ignored)", got)
	}
	if conf != 0.9 {
		t.Errorf("confidence %.1f for taps within a few ms, want 0.9 (steady)", conf)
	}
}

func TestTapConfidenceFollowsTheSpread(t *testing.T) {
	cases := []struct {
		spread float64 // taps over 40 ± spread ms; the twelve used give an IQR of 1.25 × spread
		conf   float64
		err    error
	}{
		{4, 0.9, nil}, {12, 0.7, nil}, {24, 0.5, nil}, {40, 0, ErrTooUneven}, // IQR 5, 15, 30, 50 ms
	}
	for _, c := range cases {
		late := make([]float64, TapClicks)
		for i := range late {
			late[i] = 40 + c.spread*(float64(i%5)-2)/2
		}
		clicks, attacks := tapsAt(late)
		_, conf, err := TapEstimate(tapSR, clicks, attacks)
		if !errors.Is(err, c.err) || conf != c.conf {
			t.Errorf("spread ±%.0f ms: confidence %.1f err %v, want %.1f %v", c.spread, conf, err, c.conf, c.err)
		}
	}
}

func TestTapEstimateNeedsEnoughTaps(t *testing.T) {
	clicks, attacks := tapsAt([]float64{30, 30, 30, 30, 30, 30, 30, 30, 30, 30}) // only 6 after settling
	if _, _, err := TapEstimate(tapSR, clicks, attacks); !errors.Is(err, ErrTooFewTaps) {
		t.Errorf("err %v, want ErrTooFewTaps with only six usable taps", err)
	}
}
