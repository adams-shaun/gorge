package oracle

import "testing"

// CR 602.2b with CR 118.3: Hexavus's "{1}, Remove a counter from another
// creature you control" can't be paid while no other creature holds a counter
// (its own six +1/+1 counters don't count). The dirac side matched the cost's
// Creature.Other filter without the source, offered the activation anyway
// (decision 389 of pool tail-46 pair 11). The deal came from tail-46 then; a
// later table build re-bucketed the tail lists and split the deck across
// tail-46 and tail-47, so the recording plays a table of its own (pool custom).
func TestMBCHexavusReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sub_among_cost_other_excludes_source")
}
