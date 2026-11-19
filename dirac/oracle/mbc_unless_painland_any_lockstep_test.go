package oracle

import "testing"

// CR 605.1a, 118.12: Darba's upkeep "sacrifice unless you pay {G}{G}" with a
// Forest and a Grand Coliseum untapped (its second ability: one mana of any
// colour, 1 damage to you). gorge's unless census pins that free any-colour
// ability to every colour beside the Coliseum's {C}, so it offered pay and
// dirac the decline alone (modes decision 201, tail-16-c000 pair 47). The
// payment window's "Choose a mana ability" ask carries no cost to owe, so it
// answers the first ability, {C}, and the payment fails: dirac's window must
// tap the Coliseum for {C} as well, not for the owed {G}.
func TestMBCTail16ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_pay_painland_any_colour")
}
