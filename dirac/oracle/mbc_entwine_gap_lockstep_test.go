package oracle

import "testing"

// Grab the Reins (CR 702.42a) is an entwine spell. Dr-mbc-bbbb pair 67: Plains,
// three Mountains and two artifacts (The Rack, Krark-Clan Ironworks) cannot
// pay the entwined cast ({3}{R} plus entwine {2}{R}) by gorge's potential
// bound, which counts the fixed-colour sacrifice ability {C}{C} once. The
// kernel's potential census marked that source open and folded one {C}{C} per
// sacrifice candidate, so it offered the entwined cast gorge never lists.
// Pair 4 (seed 522826, decision 301): beside one Mountain, two Plains and
// Patriar's Seal the census still swapped one of the Ironworks' two mana for
// its candidate count and reached the seven the entwined cast needs.
func TestMBCGrabTheReinsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "entwine_gap")
	stdReproNamed(t, "ironworks_fixed_production_counts_once")
}
