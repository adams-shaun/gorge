package oracle

import "testing"

// CR 702.126a, 601.2g: Archway of Innovation gave p1's next spell improvise, but
// p1 controls no artifact, so gorge plans Wind Sail and Oji, the Exquisite
// Blade plainly. Beside Flooded Grove's hybrid-fee filter (an unrelaxable
// census) its prefix search funds the {G/U} fee from Llanowar Wastes' G and pays
// both from the Grove's UU and the Plains. dirac dropped the improvise casts from
// its any-mana walk as a contribution (decision 261 of tail-15 pair 10).
func TestMBCOjiTheExquisiteBladeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "improvise_alone_cast_listed_in_any_walk")
}
