package oracle

import "testing"

// CR 616.1 with CR 614.6: two Samurai of the Pale Curtain on the battlefield
// are two exile-instead replacements competing for a sacrificed Treasure's
// move. Pricing an {X} spell's witness plan (xWitnessEats) taps the Treasure on
// a fork, where the move cannot park for gorge's order ask, and the kernel
// stalled (replleave) at the sacrifice instead of moving the token to exile as
// the first-option answer would (dr-mbc-baaa pair 78). Called from
// TestMBCDrMbcBaaaReprosLockstep, the pool's one table.
func replayWitnessSacExileOrder(t *testing.T) {
	t.Helper()
	stdReproNamed(t, "witness_sac_exile_order")
}
