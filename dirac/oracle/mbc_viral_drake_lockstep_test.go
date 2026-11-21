package oracle

import "testing"

// Viral Drake ({3}{U}: Proliferate) with a modes decision at decision 383 of
// tail-34-c000 pair 1: gorge offered mode 0 and 1, dirac only mode 0.
func TestMBCViralDrakeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "viral_drake_proliferate_modes")
}
