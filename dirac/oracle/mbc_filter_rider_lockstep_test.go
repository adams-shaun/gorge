package oracle

import "testing"

// CR 605.3a, 601.2g-h: Specter's Wail ({1}{B}) beside an untapped Dwarven
// Mine, Plains, Heap Gate ("{1}, {T}: Add one mana of any color") and Remote
// Farm ("{T}, remove a depletion counter: Add {W}{W}", a rider gorge's planner
// defers) is castable: the lands pay Heap Gate's {1} for its {B}. gorge's one
// prefix search activates the filter and the rider as census gaps; the
// kernel's filter search left the rider out, so it never offered the cast
// (decision 287 of 287: gorge offered cast(#70), the kernel did not).
func TestMBCSpecterSWailReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_gap_beside_depletion_rider")
}
