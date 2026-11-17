package oracle

import "testing"

// Darkwater Catacombs holds the class "projection Cards[42].Flags": gorge's
// side activated mana abilities at priority inside answers the canonical
// stream never carried (pair 165, tail-04-c000: Grove of the Burnwillows
// {T}: add {C}, then Darkwater Catacombs {1},{T}: add {U}{B} paid with the
// floating {C} -- gorge renders priority:[1], choose:[0], priority:[4] at
// events 1984/1987/1992, three asks that consumed no canonical action), so at
// the next aligned ask (the Hulkbuster Armor Equip-{6} target ask) gorge has
// both sources tapped and {U}{B} floating while the kernel, which never saw
// those asks, has neither. The kernel's mana model produces an activation's
// mana at its bsMana stage, after bsTargets, so no priority-phase activation
// of its own can stand in either.
func TestMBCDarkwaterCatacombsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "prio_mana_kernel_unasked")
}
