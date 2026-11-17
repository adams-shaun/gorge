package oracle

import "testing"

// Magus of the Candelabra ({X},{T}: untap X target lands): the X choice must
// be capped by the same count in both engines (decision 813 of pair 50 of
// tail-32 offered gorge 0..9 and dirac 0..8).
func TestMBCMagusOfTheCandelabraReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "magus_candelabra_x_cap")
}
