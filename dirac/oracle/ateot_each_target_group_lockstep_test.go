package oracle

import "testing"

// TestEachTargetAtEOTOneGroupTriggerLockstep: Pull returns two creature cards
// and its AtEOT$ Sacrifice rider names both. gorge registers one delayed
// triggered ability over the pair (effects/ateot.go scheduleAtEOTGroup,
// CR 603.7a), so the end step puts one trigger on the stack and asks no
// trigger order; the kernel's each-target loop must join the second target to
// the first one's record instead of registering a second trigger.
func TestEachTargetAtEOTOneGroupTriggerLockstep(t *testing.T) {
	oneOffReproClean(t, "each_target_ateot_one_group_trigger")
}
