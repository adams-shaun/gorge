package oracle

import "testing"

// CR 603.7: the Ring's level-3 delayed trigger ("sacrifice it at end of
// combat") is created when the bearer becomes blocked and triggers at the end
// of combat whether or not the blocker is still there; a blocker that died in
// combat leaves a trigger that sacrifices nothing. The kernel once dropped
// the promise of a blocker that had left the battlefield, so gorge's
// end-of-combat trigger_order offered one more trigger than the kernel did.
func TestMBCJovenReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ring_blocker_promise_after_blocker_left")
}
