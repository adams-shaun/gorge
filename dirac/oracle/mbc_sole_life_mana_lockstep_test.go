package oracle

import "testing"

// CR 602.2, 605.3b: Treasonous Ogre's "Pay 3 life: Add {R}" taps nothing, so
// it can be activated again and again while the payer has the life. With one
// untapped Plains and 17 life, p0's Thermal Navigator ({3}) and Wand of
// Orcus's equip ({3}) are both payable (Plains plus two Ogre activations).
// gorge offered both; the kernel offered only pass (tail-51 focus pair 27).
func TestMBCThermalNavigatorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sole_life_mana_repeated_for_cast")
}
