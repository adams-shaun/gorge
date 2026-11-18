package oracle

import "testing"

// Entrancing Melody's {X}{U}{U} cast: whether a legal cast option exists for
// the current X choice must agree between gorge and the kernel (the kernel
// offered cast where gorge did not in the recorded game).
func TestMBCEntrancingMelodyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "x_sweep_overtallies_fixed_choice")
}
