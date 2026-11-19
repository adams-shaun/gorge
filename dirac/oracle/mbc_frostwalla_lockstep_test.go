package oracle

import "testing"

// Frostwalla's {S} pump (once each turn) is offered by gorge and must be
// offered by dirac too (evidence loop, tail-56 pair 56, decision 643).
func TestMBCFrostwallaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "frostwalla_snow_pump")
}
