package oracle

import "testing"

// Rustspore Ram's card types agree between gorge and dirac (tail-08-c000,
// decision 383: gorge Artifact|Creature, dirac carried one more type bit).
func TestMBCRustsporeRamReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "rustspore_ram_types")
}
