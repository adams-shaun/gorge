package oracle

import "testing"

// Bellowing Aegisaur blocks (or is blocked by) Undergrowth Champion and the
// combat damage is divided across them: the two engines part at the Aegisaur's
// Blocks projection (decision 670 of 670, tail-60-c000 pair 73).
func TestMBCBellowingAegisaurReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bellowing_aegisaur_damage_to_prevented_champion")
}
