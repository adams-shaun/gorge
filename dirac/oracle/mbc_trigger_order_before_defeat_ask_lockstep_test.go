package oracle

import "testing"

// CR 310.12b with CR 603.3b: a Siege defeated by combat damage that also
// damaged several of the opponent's creatures (each one a Repercussion trigger
// for the same player) is asked after the triggers are ordered and pushed, not
// between the ordering picks. Gorge posed the whole trigger_order batch (decision
// 37, then 38) and kept the defeat's transformed-cast ask for the next step's
// head; the kernel's stepRaw head woke the deferred defeat while the batch was
// still being ordered and asked the cast at decision 38. The recording plays on
// the table its own decks need (pool custom).
func TestMBCRepercussionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_order_before_defeat_ask")
}
