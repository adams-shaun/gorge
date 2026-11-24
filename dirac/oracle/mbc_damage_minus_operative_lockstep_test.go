package oracle

import "testing"

// CR 616.1: two damage-minus lines (the Lashknife Barrier class) both rewrite
// the combat damage Sword of Fire and Ice's carrier deals to Duskmantle
// Operative, and gorge asks the affected player which applies next (decision
// 554 of tail-55-c000, pair 1). The kernel dealt the damage without the ask and
// the two engines parted at the game's end (projection Over). The kernel now
// stalls loud (StallReplPlus) on two minus lines meeting one event, so the
// recording is a declared stall rather than a divergence.
func TestMBCDuskmantleOperativeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "damage_minus_pair_order_ask_operative")
}
