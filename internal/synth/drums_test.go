package synth

import (
	"math"
	"testing"
)

func energyOf(v Voice, frames int) float64 {
	l, r := make([]float32, frames), make([]float32, frames)
	v.Render(l, r)
	var e float64
	for i := range l {
		e += math.Abs(float64(l[i])) + math.Abs(float64(r[i]))
	}
	return e
}

func TestDrumKitHitsRingOutAndStop(t *testing.T) {
	for _, key := range []int{36, 38, 42, 46, 49, 51, 45, 56, 70} {
		k := NewBuiltin(48000, DrumKitProgram)
		k.NoteOn(key, 1)
		k.NoteOff(key) // a drum ignores note-off and rings out
		if e := energyOf(k, 4800); e == 0 {
			t.Errorf("drum %d made no sound", key)
		}
		energyOf(k, 48000*10) // let it ring out
		if e := energyOf(k, 4800); e != 0 {
			t.Errorf("drum %d still sounds after ten seconds (%v)", key, e)
		}
	}
}

func TestClosedHiHatChokesTheOpenOne(t *testing.T) {
	k := NewDrumKit(48000)
	k.NoteOn(46, 1)
	energyOf(k, 2400)
	k.NoteOn(42, 1)
	energyOf(k, 48000) // the closed hat's own short tail is long gone
	if e := energyOf(k, 4800); e != 0 {
		t.Errorf("the open hi-hat kept ringing through a closed one (%v)", e)
	}
}

func TestDrumKitStaysInsideFullScale(t *testing.T) {
	k := NewDrumKit(48000)
	for _, key := range []int{36, 38, 42, 49, 51} {
		k.NoteOn(key, 1)
	}
	l, r := make([]float32, 4800), make([]float32, 4800)
	k.Render(l, r)
	for i := range l {
		if math.Abs(float64(l[i])) > 1 {
			t.Fatalf("a full kit hit reached %v, past full scale", l[i])
		}
	}
}
