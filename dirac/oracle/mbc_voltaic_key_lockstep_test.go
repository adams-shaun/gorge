package oracle

import "testing"

// Voltaic Key's activation ({1},{T}: untap target artifact) reached a choose:x
// decision (decision 520 of tail-02-c000 pair 7, seed 656357, autopay) where
// dirac offered one more answer than gorge.
func TestMBCVoltaicKeyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "voltaic_key_choose_options")
}
