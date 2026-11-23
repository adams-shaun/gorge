package oracle

import "testing"

// CR 119.4 with CR 702.33a: Scorch Rider's kicked cast ({4}{R}{R}) needs all
// six of p0's lands, and Manabarbs deals 1 damage per land tapped for mana, at
// 4 life. Gorge's potential planner never takes a plan whose summed damage
// reaches the payer's life, so it offers no kicked cast; dirac's census capped
// each colour's lands alone and offered it (tail-34-c000 pair 87, decision 506).
func TestMBCScorchRiderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "manabarbs_kicked_cast_summed_lethal_guard")
}
