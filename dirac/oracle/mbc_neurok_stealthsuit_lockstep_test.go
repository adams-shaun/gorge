package oracle

import "testing"

// Neurok Stealthsuit's second option at a modes decision: gorge offers two
// modes where dirac offers one (pool tail-34-c000, pair 5, decision 243).
func TestMBCNeurokStealthsuitReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "neurok_stealthsuit_modes")
}
