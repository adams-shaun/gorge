package oracle

import "testing"

// Root Elemental (Morph {5}{G}{G}) face down on the battlefield: turning it
// face up is a special action its controller may take any time they have
// priority (CR 702.37b), so the priority options must offer it whenever the
// cost is payable. The gorge side offered the turn-up while the kernel did
// not (decision 343 of 343; tail-66-c000 pair 40, seed 118111).
func TestMBCRootElementalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "root_elemental_morph_turnup")
}
