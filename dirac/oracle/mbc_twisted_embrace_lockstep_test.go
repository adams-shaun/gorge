package oracle

import "testing"

// Twisted Embrace (Enchant creature or artifact you control) is offered as a
// cast at decision 689 of tail-49 pair 74 by gorge but not by dirac.
func TestMBCTwistedEmbraceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "twisted_embrace_aura_cast_offer")
}
