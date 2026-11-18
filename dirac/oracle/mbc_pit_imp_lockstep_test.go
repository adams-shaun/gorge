package oracle

import "testing"

// Pit Imp wearing Lightning Prowess: gorge offered the granted "{T}: deals 1
// damage to any target" at a priority decision where the kernel offered
// nothing for it (decision 382, tail-26-c002 pair 29).
func TestMBCPitImpReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "pit_imp_granted_tap_ability_offer")
}
