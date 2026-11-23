package oracle

import "testing"

// Bamboo Grove Archer in hand at priority: the kernel offered the cast while
// gorge did not (decision 482, tail-15 pair 29 seed 202859).
func TestMBCBambooGroveArcherReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bamboo_grove_archer_cast_offer")
}
