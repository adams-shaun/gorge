package oracle

import "testing"

// Horrid Shadowspinner's attack trigger costs Draw<X/You> with X its power.
// When a penalty has driven that power to 0, the count still resolves: gorge's
// triggeredCostDrawCounts offers "pay" beside decline (drawing no cards is
// payable, CR 118.3), and dirac once priced the resolved-0 Draw unpayable and
// offered decline only (tail-36 pair 5, decision 789).
func TestMBCHorridShadowspinnerZeroDrawCostReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "horrid_shadowspinner_zero_draw_cost")
}
