package oracle

import "testing"

// The pool tail-40 game 15 (focus_00376_UR vs focus_00255_RG, seed
// 7897695009980561635) once stalled on the kernel's `lowering` class: a second
// Aggravated Assault activation in the additional main phase the first grant
// opened was a grant shape the kernel did not model. The recording pins the
// game's whole decision path.
func TestMBCTail40ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lowering")
}
