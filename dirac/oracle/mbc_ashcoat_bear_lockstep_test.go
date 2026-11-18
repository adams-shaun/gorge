package oracle

import "testing"

// Ayula, Queen Among Bears: the fight mode ("target Bear you control fights
// target creature you don't control") asks for its second target after the
// first (Ashcoat Bear). Decision 185 of the recording: the kernel offers
// Goblin Firebug where gorge offered only a pass.
func TestMBCAshcoatBearReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ayula_fight_second_target_after_first")
}
