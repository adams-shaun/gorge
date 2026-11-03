package oracle

import "testing"

// Wolfbat's {B} payment with only Great Divide Guide's granted any-colour
// mana left to fund it: gorge's planner has printed abilities only, so it has
// no witness and the manual window is answered source by source. The kernel
// must not plan the granted sources either.
func TestResolutionPaymentLeavesGrantedSourcesToTheWindowLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "wolfbat_pay", "std-tla", Config{})
}
