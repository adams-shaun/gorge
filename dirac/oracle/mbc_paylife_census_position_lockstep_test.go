package oracle

import "testing"

// A mana ability whose activation cost pays life ("{T}, Pay 1 life: Add {G}",
// Blightsoil Druid) is outside gorge's shared census, which lists free-cost
// abilities only (pay.ManaFreeCost rejects PayLife): paymentPlanManaUnits
// appends such a source's unit after the shared census's battlefield-order
// units. The kernel's census listed it at its battlefield position (the
// deterministic pain unit's pass 0), so the plan tie-break (gorge key 8,
// pay.CompareSteps) compared a different source sequence and paid a {1}{G}
// Dryad Greenseeker from a Swamp where gorge paid from a Vivid Crag
// (decision 588; tail-61 pair 240 seed 1, the game the evidence loop filed
// under Vivid Crag).
func TestMBCLifeCostCensusPositionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "paylife_mana_unit_census_position")
}
