package oracle

import "testing"

// CR 616.1 with CR 119.4: two Worships on one battlefield both floor the same
// combat damage to the controller's life at 1. Applying either first leaves
// the other with nothing to modify (the floors compose to the larger one in
// every order), so the order ask changes nothing. Gorge asked the affected
// player which Worship applies first and the kernel took them in scan order
// (decision 793 of tail-55-c000, seed 858391).
func TestMBCFirebornKnightReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_floor_pair_no_order_ask")
}

// CR 616.1 with CR 119.4: two Worships floor the same combat damage to the
// same player. The floors take min(amount, life-1) in either order, so the
// affected player's pick changes nothing; gorge once posed the order ask
// anyway (decision 926 of the recording) and the kernel, which takes the
// floors in scan order, has no such ask.
func TestMBCGreelMindRakerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_floor_pair_commutes")
}
