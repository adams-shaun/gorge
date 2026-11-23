package oracle

import "testing"

// Photon, Lady of Light at a modes decision: gorge offers one mode where
// dirac offers two (pool tail-44-c000, pair 26, decision 516).
func TestMBCPhotonLadyOfLightReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "photon_lady_of_light_modes")
}
