package oracle

import "testing"

// Ichor Explosion ({5}{B}{B}, sacrifice a creature) is castable from seven
// lands where Baldur's Gate's paid ability makes the second {B} and Treasonous
// Ogre's "Pay 3 life: Add {R}" supplies the seventh mana. gorge's prefix search
// walks the Ogre as a covered funding tap in battlefield order and finds the
// script in 120 nodes; the kernel's filter search walked it only at a
// last-resort life pass, repeating it below every funding tap, and spent its
// 160-node budget first (decision 511 of tail-51-c000 pair 34).
func TestMBCIchorExplosionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ichor_explosion_life_mana_beside_filter")
}
