package oracle

import "testing"

// CR 601.2a-c: a spell leaves the hand for the stack before its targets are
// chosen, so Vanquish the Foul ("destroy target creature with power 4 or
// greater") cast with three other cards in hand can't target a Body of
// Knowledge whose power is its controller's hand size. tail-57 pair 62.
func TestMBCVanquishTheFoulReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hand_sized_power_target_vanquish_the_foul")
}
