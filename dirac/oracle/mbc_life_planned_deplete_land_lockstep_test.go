package oracle

import "testing"

// A plain {1}{R} cast beside an untapped Peat Bog ("{T}, Remove a depletion
// counter: Add {B}{B}") and Treasonous Ogre ("Pay 3 life: Add {R}"): gorge's
// planner has the Ogre's life unit but no unit for the Bog, so it cannot reach
// two mana and the autopay surface drops the cast; the kernel's life-planned
// keep must not count the Bog through the potential walk either (pair 23,
// decision 461, pool dr-mbc-bbbb, surface autopay).
func TestMBCLifePlannedWithoutDepleteLandUnitLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "life_planned_without_deplete_land_unit")
}
