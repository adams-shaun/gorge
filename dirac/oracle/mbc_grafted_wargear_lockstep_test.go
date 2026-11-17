package oracle

import "testing"

// CR 603.2 with CR 701.3b: when the creature a Grafted Wargear is attached to
// dies, the Wargear becomes unattached from it and its "becomes unattached"
// trigger goes on the stack, beside Vat of Rebirth's own death trigger, even
// though the creature is gone and the sacrifice finds nothing (decision 653 of
// tail-29 pair 38: one engine left Grafted Wargear out of the trigger order).
func TestMBCGraftedWargearReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grafted_wargear_unattached_on_leave_trigger")
}
