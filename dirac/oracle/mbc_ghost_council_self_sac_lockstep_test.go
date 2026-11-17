package oracle

import (
	"testing"
)

// Ghost Council of Orzhova's "{1}, Sacrifice a creature: Exile Ghost Council.
// Return it at the beginning of the next end step" activated twice, the second
// time sacrificing Ghost Council itself while the first is on the stack: the
// exiling abilities find nothing to exile, so no return is delayed. Gorge offers
// pass at the end step (after Screeching Bat) where the kernel asked for a
// trigger order of two delayed returns (decision 344 of pair 6, pool tail-08,
// surface autopay).
func TestMBCGhostCouncilSelfSacReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sacrificed_source_exile_cost_delayed_return")
}
