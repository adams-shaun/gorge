package oracle

import "testing"

// CR 601.2f, 602.2b: Heartstone ("activated abilities of creatures cost {1}
// less", no Activator$) lowers the opponent's creature's activation too, so
// Bloodsworn Squire's {1}{B}, discard a card costs {B} and one Sunlit Marsh
// pays it. The kernel only reduced the Heartstone controller's own activations
// and withheld the activate option gorge offered (decision 359 of 359; pair
// 1052 of tail-06 as dealt at land dfc2cd0565).
func TestMBCBloodswornSquireReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "heartstone_reduces_opponents_ability")
}
