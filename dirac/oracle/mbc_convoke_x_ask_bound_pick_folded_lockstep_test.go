package oracle

import "testing"

// Chord of Calling's convoke cast announces X (CR 601.2b), and its three
// creature picks (Barkhide Troll, Spike Breeder, Dualcaster Mage) each pay one
// generic mana (CR 702.51a), so X = 5 is reachable and gorge offers choose:x
// 2..5. The kernel stopped at 4 because its X-ask bound priced the cost without
// the announced picks folded, one generic short of the cast window. Recorded
// from tail-75 pair 18 (seed 1986184142480234361, surface full).
func TestMBCDualcasterMageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "convoke_x_ask_bound_pick_folded")
}
