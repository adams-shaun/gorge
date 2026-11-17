package oracle

import "testing"

// CR 106.6 with CR 107.4: A-Elven Bow costs {G}, and the only untapped mana
// source its controller has is Nardole, Resourceful Cyborg's "{T}: Add {U}
// for each counter on Nardole. Spend this mana only to cast noncreature
// spells." Blue mana can't pay a green pip, so Bow isn't castable. dirac
// priced the restricted mana as if it could pay any colour and offered the
// cast at a priority decision gorge doesn't (decision 408).
func TestMBCAElvenBowReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restricted_mana_noncreature_cant_pay_coloured")
}
