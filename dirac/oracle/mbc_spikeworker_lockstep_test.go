package oracle

import "testing"

// Spike Worker's {2}, remove a +1/+1 counter ability stays offered at priority
// after a convoke cast of Sundering Vitae is aborted for want of a legal target
// (CR 601.2c): the aborted cast is reversed whole, so the untapped Sage of the
// Maze is still there to pay {2}. Gorge offered the activation; the kernel
// didn't (decision 332, tail-36 pair 75).
func TestMBCSpikeWorkerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "convoke_aborted_cast_restores_untapped_mana_source")
}
