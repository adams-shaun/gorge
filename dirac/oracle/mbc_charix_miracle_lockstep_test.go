package oracle

import "testing"

// Charix, the Raging Isle's "spells your opponents cast that target Charix
// cost {2} more" against a miracle cast: gorge and the kernel must agree on
// whether Charix is on the target menu of Revenge of the Hunted's miracle cast
// (decision 299 of tail-34-c000 pair 46, seed 137611).
func TestMBCCharixMiracleTargetRaiseReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "miracle_target_raise_cost_withheld")
}
