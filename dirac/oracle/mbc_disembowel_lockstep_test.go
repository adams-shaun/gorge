package oracle

import "testing"

// Disembowel ({X}{B}, "destroy target creature with mana value X") was offered
// by the kernel at a priority decision where gorge offered only the other
// castable spell (decision 123, tail-69-c001 pair 49).
func TestMBCDisembowelReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "disembowel_x_target_offer")
}
