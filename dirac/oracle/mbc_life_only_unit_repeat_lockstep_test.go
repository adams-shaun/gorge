package oracle

import "testing"

// CR 605.1a, 601.2h: Treasonous Ogre's "Pay 3 life: Add {R}" taps nothing. With
// one untapped Plains and the Ogre, gorge's planner proves Wand of Orcus's
// Equip {3} insufficient but its PotentialMana bound affords it, and the prefix
// search activates the Ogre and prices the equip over the floating {R} with the
// Ogre's unit still in the census. The kernel's filter search never ran for an
// activation on a board with no filter, so the equip was left out (tail-51
// pair 27, decision 400).
func TestMBCWandOfOrcusReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_only_mana_unit_pays_equip_repeatedly")
}
