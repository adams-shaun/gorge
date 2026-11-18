package oracle

import "testing"

// Lotus Blossom's "{T}, Sacrifice: add X mana of any one color" beside an
// Island in hand: gorge offered cast(Azure Mage) and cast(Preordain) at
// priority and the kernel offered neither (tail-74-c000 decision 200).
func TestMBCAzureMageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lotus_blossom_cast_offer")
}
