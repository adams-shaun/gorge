//go:build dirac_mp

package oracle

import "testing"

// TestFourSeatGluntchLockstep plays four-seat pods of Gluntch, the Bestower
// (gluntchDeck at every seat): the three choices name three different
// players, the third of whom creates two Treasures, in lockstep with gorge,
// and the pick, the draw and the Treasures each happen.
func TestFourSeatGluntchLockstep(t *testing.T) {
	y, acts := playGluntch(t, 4, 8)
	checkGluntch(t, y, acts)
	if y.threeWay == 0 || y.treasures == 0 {
		t.Errorf("four seats never chose a third player or made Treasures: %+v", y)
	}
}
