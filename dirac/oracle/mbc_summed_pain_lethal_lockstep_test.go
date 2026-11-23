package oracle

import "testing"

// CR 119.4 and CR 120.3a: Hollowhenge Overlord ({4}{G}{G}) with the caster at
// 2 life and exactly two green sources, Hashep Oasis ({T}, pay 1 life: {G})
// and Grand Coliseum ({T}: any colour, 1 damage to you). Each source alone is
// survivable, but together their pain reaches the caster's life, and gorge's
// planner refuses a plan whose summed life and damage is lethal, so the cast
// isn't offered (tail-16 pair 133, decision 392 of 392).
func TestMBCHollowhengeOverlordReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "summed_pain_lethal_guard_two_painlands_cast")
}
