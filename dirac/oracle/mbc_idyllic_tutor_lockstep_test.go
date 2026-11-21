package oracle

import "testing"

// A sorcery cast whose {2}{W} gorge's prefix script pays by floating a
// Gift-enchanted Plains' printed {W} and a Lotus Blossom's petal mana before
// the cast, then tapping a Forest. The kernel's planner found no plan at the
// cast's mana step.
func TestMBCIdyllicTutorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spell_gap_prefix_lotus_granted_plains")
}
