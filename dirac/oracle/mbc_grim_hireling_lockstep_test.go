package oracle

import "testing"

// Grim Hireling's "{B}, Sacrifice X Treasures" sorcery-speed ability: the two
// engines must agree on whether the activation is offered at priority
// (decision 530 of tail-51-c000 pair 6: gorge offered it, the kernel did not).
func TestMBCGrimHirelingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grim_hireling_sacrifice_x_treasure_activation")
}
