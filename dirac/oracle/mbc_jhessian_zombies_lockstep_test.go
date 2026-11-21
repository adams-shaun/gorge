package oracle

import "testing"

// Jhessian Zombies' islandcycling and swampcycling ({2}, discard) are offered
// at priority by one engine and not the other (decision 572 of pair 4, seed
// 520726, tail-64-c000).
func TestMBCJhessianZombiesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "typecycling_offered_with_two_untapped_lands")
}
