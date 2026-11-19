package oracle

import "testing"

// CR 601.2b-c, 733.1: an announced-X counter removal whose target bound reads X
// (Quillmane Baku's {1}, {T}, Remove X ki counters: return target creature with
// mana value X or less) with no X any target answers. gorge's xAsk keeps the
// whole payable range and the target ask reverses the activation; the kernel
// filtered every X out and posed a decision with no options (tail-48 pair 20,
// evl-11190551). loyaltyXOptions now keeps 0..the counters, as addXOptions does
// for a mana X.
func TestCounterXUnanswerableKeepsRangeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "counter_x_unanswerable_keeps_range")
}
