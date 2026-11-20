package oracle

import "testing"

// CR 701.38a with CR 603.3c: a triggered Vote's Choices$ name the ballot
// options each voter picks at resolution, not modes chosen when the trigger is
// put on the stack. Orchard Elemental's council's dilemma trigger goes on the
// stack with no mode ask, so priority follows; the kernel's lowering once
// posed a placement mode (decision 504 of 504, tail-59-c000 pair 7: gorge
// offered pass, the kernel mode 0 / mode 1), a copy of a gorge defect gorge
// has since fixed (askTriggerModes returns false for Vote).
func TestMBCMalachiteTalismanReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "triggered_vote_no_placement_mode")
}
