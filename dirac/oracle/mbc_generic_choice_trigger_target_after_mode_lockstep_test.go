package oracle

import "testing"

// CR 603.3c-d with CR 601.2c: a triggered ability's controller announces
// its mode when putting it on the stack and then chooses the targets the
// ability requires. Inspirit, Flagship Vessel's granted trigger is a
// GenericChoice whose own ValidTgts$ ("up to one other target artifact")
// sits on the choice, not inside either mode, and its chosen mode's
// Defined$ Targeted reads that ability target. gorge asked the mode and
// then resolved without ever offering the target, so the kernel's target
// ask (decision 134, tail-38 focus game) had no gorge counterpart.
func TestMBCTail38ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "generic_choice_trigger_target_after_mode")
}
