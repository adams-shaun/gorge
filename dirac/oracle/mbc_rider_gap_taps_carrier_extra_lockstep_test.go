package oracle

import (
	"testing"
)

// CR 605.3a, 601.2g-h: Wormwood Treefolk's "{G}{G}: gains forestwalk and
// deals 2 damage to you" is payable from a board whose only green comes from
// Remote Farm's depletion rider ("{T}, Remove a depletion counter: Add
// {W}{W}") with Wild Growth's TapsForMana carrier on it (the rider's tap adds
// one more {G}) and Heap Gate's "{1}, {T}: Add one mana of any color" filter,
// its {1} paid from the floating {W}. gorge's priority list offered the
// activation (decision 536 of pair 166, pool tail-51, surface autopay); the
// kernel offered pass only.
func TestMBCWormwoodTreefolkReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "rider_gap_taps_carrier_extra")
}
