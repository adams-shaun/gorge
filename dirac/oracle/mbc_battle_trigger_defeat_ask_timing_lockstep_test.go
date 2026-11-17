package oracle

import "testing"

// A Siege defeated by combat damage during the regular damage step waits for
// the step's priority round before its owner's transformed-cast ask: gorge
// queues the offer inside the step's action and next drains it at its next
// step() head, after p1's pass. The kernel once woke the queued record inside
// finishBuild when the defeat-adjacent Anax trigger was PLACED on the stack,
// and asked the cast in place of that priority round. A cast or activation
// build is different: gorge's next step() head follows it directly, so the
// kernel wakes there. The recording is the divergence of dr-ev-tail-10 pair
// 36 (the same sweep as the Radiant Purge re-offer, mirrored: the kernel
// asked, gorge gave priority).
func TestMBCBattleTriggerDefeatAskTimingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_battle_trigger_defeat_ask_timing")
}
