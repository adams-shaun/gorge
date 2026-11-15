package oracle

import "testing"

// CR 310.11 timing: Pack Rats defeat Invasion of Eldraine in the combat damage
// step while Merchant of Truth dies blocking, so its untargeted investigate
// trigger is put on the stack in the same SBA/trigger pass. gorge's
// drainTriggers pushes it and grants priority inside that one step(), so the
// defeat cast is asked at the next step's head; the kernel once woke the
// deferred defeat record when the trigger's build finished and asked the
// battle's owner in place of that priority (decision 783 of tail-53 pair 543).
func TestMBCMerchantOfTruthReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "defeat_ask_waits_past_trigger_push")
}
