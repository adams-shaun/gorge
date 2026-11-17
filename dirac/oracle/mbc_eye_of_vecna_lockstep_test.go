package oracle

import "testing"

// Eye of Vecna's upkeep trigger cost: gorge offered trigger_cost_pay where the
// kernel offered only the decline (decision 459 of the recorded game).
func TestMBCEyeOfVecnaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "eye_of_vecna_trigger_cost_pay_offer")
}
