package oracle

import "testing"

// CR 605.1a, 601.2g-h: Dutiful Thrull's {B} regeneration is payable from
// Tinder Wall's "Sacrifice this creature: Add {R}{R}" feeding Unknown Shores'
// "{1}, {T}: Add one mana of any color" (one {R} pays the {1}, the Shores
// makes {B}, one {R} floats). gorge's planner finds that funding and offers
// the activation; the kernel offered only pass and Deathrender (tail-44 pair
// 713 at decision 468).
func TestMBCDutifulThrullReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sac_two_mana_funds_filter")
}
