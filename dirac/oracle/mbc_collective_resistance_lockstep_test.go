package oracle

import "testing"

// Collective Resistance is a three-mode escalate charm (Destroy target
// artifact, Destroy target enchantment, hexproof and indestructible on a target
// creature). The recording is the first divergence of dr-mbc-abba pair 154:
// the kernel offered the cast at a priority decision where gorge did not.
func TestMBCCollectiveResistanceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_collective_resistance_cast_offer")
}
