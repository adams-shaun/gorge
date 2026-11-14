package dirac

import "testing"

// Split second (CR 702.61a) bars every activated ability that is not a mana
// ability, Station (CR 702.184a) and Crew (CR 702.122a) included; the
// kernel's mana taps are not AkActivate and stay offered.
func TestSplitSecondBarsEveryActivation(t *testing.T) {
	var s State
	for _, mode := range []uint8{0, 1, 0x80} {
		if !s.splitSecondBars(nil, MakeAction(AkActivate, 3, TCode(mode), 0)) {
			t.Errorf("an activation of mode %#x is not barred", mode)
		}
	}
	if s.splitSecondBars(nil, MakeAction(AkPass, 0, 0, 0)) {
		t.Error("pass is barred")
	}
}
