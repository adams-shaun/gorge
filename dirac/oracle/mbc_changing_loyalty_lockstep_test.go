package oracle

import "testing"

// CR 702.56a, 106.1: a replicated cast of Changing Loyalty ({1}{B} plus a
// replicate payment, {3}{B} in all) beside a board whose only black source is
// Paradise Mantle's granted any-colour tap, a dozen covered lands and a Henge.
// Gorge's prefix search over the sources' activations once spent its 160-node
// budget on the other colours' floats and withheld the play ("unsupported
// census_incomplete") while the kernel's census offered it (tail-26-s000 pair
// 16, decision 598, "options options"). The recorded 598-action game now
// replays in lockstep with both sides offering the cast.
func TestMBCChangingLoyaltyReprosLockstep(t *testing.T) {
	stdReproNamedServed(t, "changing_loyalty_replicate_search_budget")
}
