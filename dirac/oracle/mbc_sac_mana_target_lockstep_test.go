package oracle

import (
	"testing"
)

// CR 601.2c-h: Double Cleave's only legal target is Catalyst Elemental, the
// one permanent p0's mana can come from (Sacrifice it: {R}{R}) with every land
// tapped. Gorge's walk refuses the cast: its witness payment sacrifices the
// whole target pool (witness_unreachable), and dirac offered it anyway
// (tail-12 pair 196, decision 114).
func TestMBCDoubleCleaveReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sac_mana_only_target")
}
