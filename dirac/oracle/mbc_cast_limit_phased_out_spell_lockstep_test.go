package oracle

import "testing"

// CR 702.26b hides a phased-out permanent from battlefield reads, not a spell
// that was cast: Kaito Shizuki was cast this turn and phased out at the end
// step, so Eidolon of Rhetoric ("each player can't cast more than one spell
// each turn", CR 601.3) still stops a second cast, Contagion for its
// alternative cost included. Gorge's cast-limit count read the phased-out
// Kaito as not a Card and offered the Contagion cast the kernel withheld
// (tail-67-c001 pair 10, decision 300).
func TestMBCContagionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cast_limit_counts_phased_out_spell")
}
