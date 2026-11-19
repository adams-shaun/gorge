package oracle

import "testing"

// Ezuri's Brigade and Gnottvold Hermit were offered as casts by the kernel
// while gorge offered only Krosan Avenger (tail-34-c000 pair 104, decision
// 180 of 180), with Contaminated Ground and Manabarbs on the table.
func TestMBCEzuriSBrigadeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ezuris_brigade_life_loss_lands")
}
