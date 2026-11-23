package oracle

import "testing"

// Kumano, Master Yamabushi's {1}{R} activation is offered with only untapped
// lands left whose mana comes from The World Tree's granted "{T}: Add one mana
// of any color" (six or more lands), after Rain of Filth resolved and gave the
// lands a sacrifice-for-{B} ability. Gorge offered only pass; dirac offered the
// activation (decision 570 of tail-08-c000 pair 30).
func TestMBCKumanoMasterYamabushiReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_any_color_mana_pays_activation")
}
