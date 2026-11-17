package oracle

import "testing"

// Collective Resistance is a three-mode escalate charm (Destroy target
// artifact, Destroy target enchantment, hexproof and indestructible on a target
// creature). The recording is the first divergence of dr-mbc-abba pair 154:
// the kernel offered the cast at a priority decision where gorge did not.
// The recording is pooled custom (the table its own decks need): the dr-mbc
// bucket it was found on re-cuts whenever the lowered card set moves.
func TestMBCCollectiveResistanceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_collective_resistance_cast_offer")
}
