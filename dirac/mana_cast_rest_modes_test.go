package dirac

import "testing"

// TestCastRestPricedModesAllowlist pins the modes payCastPrefixRest prices on
// plannedCastCost: every alternative-cost mode (altPlayCost's cases, kicker)
// stays outside, so a new mode bit is refused until it is audited.
func TestCastRestPricedModesAllowlist(t *testing.T) {
	for name, bit := range map[string]uint8{
		"kicker": 1, "castWarp": castWarp, "castWarpRecast": castWarpRecast,
		"castAirbend": castAirbend, "castFree": castFree, "castFaceDown": castFaceDown,
	} {
		if castRestPricedModes&bit != 0 {
			t.Errorf("%s (%#x) is priced by plannedCastCost's printed read; castRestRefuses must refuse it", name, bit)
		}
	}
	for name, bit := range map[string]uint8{"flashback": 2, "castFace": castFace, "castMayPlay": castMayPlay} {
		if castRestPricedModes&bit != bit {
			t.Errorf("%s (%#x) fell out of the priced modes", name, bit)
		}
	}
}
