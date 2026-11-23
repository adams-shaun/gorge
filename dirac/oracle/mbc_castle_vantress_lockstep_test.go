package oracle

import "testing"

// Castle Vantress's {2}{U}{U},{T}: Scry 2 ability is offered by both engines at
// the same priority decision (decision 915 of tail-07-c000 pair 151, where only
// dirac offered it).
func TestMBCCastleVantressReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "castle_vantress_scry_activation")
}
