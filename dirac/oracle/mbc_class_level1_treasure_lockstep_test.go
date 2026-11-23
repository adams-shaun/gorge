package oracle

import "testing"

// CR 716.2a: Alchemist's Talent's "Treasures you control have '{T}, Sacrifice:
// add two mana of any one color'" is a level-2 ability, so at level 1 the
// controller's untapped Plains and two Treasures make three mana and a
// {3}{R} Thrash of Raptors is not castable. Only one engine offered the cast
// (decision 404, tail-16-c000 pair 15).
func TestMBCThrashOfRaptorsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "class_level1_treasure_mana_cast")
}
