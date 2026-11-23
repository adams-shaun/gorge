package oracle

import "testing"

// CR 107.3, 601.2g-h, 605.1a: Mage-Ring Network's "remove X storage counters"
// ability asks X, and gorge's prefix search for a play the census misses (the
// Bump in the Night flashback {5}{R}, whose red comes from Cascade Bluffs'
// {U/R} filter) branches every X from the counters held down to 0, each a
// subtree of the 160-node budget. The kernel's filter search took the ability
// at its one largest X, so it priced the flashback cast gorge's search gave up
// on (decision 826 of tail-55 pair 143, seed 840391).
func TestMBCBumpInTheNightReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "storage_battery_x_ask_in_filter_prefix_search")
}
