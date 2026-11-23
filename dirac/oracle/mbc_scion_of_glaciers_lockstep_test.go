package oracle

import "testing"

// Scion of Glaciers is castable at decision 1256 of tail-15 pair 20: gorge
// offers the cast and dirac once did not (options priority/cast). The recording
// pins that both engines now offer the same options there.
func TestMBCScionOfGlaciersReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "scion_of_glaciers_after_equip")
}
