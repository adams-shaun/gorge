package oracle

import "testing"

// Webspinner Cuff's Reconfigure pair (CR 702.151a) on tail-71 pair 284: with
// the Cuff attached to Hidden Gibbons and an empty pool, the only mana the
// offer planner can raise is Ashnod's Altar's "{T}, Sacrifice a creature:
// add {C}{C}". Its first scripted prefix sacrifices the bearer, which detaches
// the Cuff and makes the unattach half (ability 1) unofferable; gorge then
// dropped the option the kernel still offered. The search now rejects that
// dead-end prefix and finds one that sacrifices another creature, so both
// engines offer the unattach.
func TestMBCWebspinnerCuffReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reconfigure_unattach_script_keeps_bearer")
}
