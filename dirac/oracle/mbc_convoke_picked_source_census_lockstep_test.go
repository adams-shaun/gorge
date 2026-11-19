package oracle

import "testing"

// CR 702.51a with CR 601.2b: a creature picked for Convoke is tapped to pay
// that cast, so it cannot also tap for mana while X is announced. Sage of the
// Maze is an "{T}: Add two mana in any combination of colors" Combo Any
// census gap; picked as the fourth convoke contributor beside three generic
// picks, it stayed in the kernel's X census and the ask offered X = 10 on a
// board whose six lands and four contributors pay only X = 8. gorge offered
// 3..8 (the picks must each absorb a generic).
func TestMBCIlluminatorVirtuosoReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "convoke_picked_source_census")
}
