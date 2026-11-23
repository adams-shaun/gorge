package oracle

import "testing"

// Hag Hedge-Mage's priority cast offer at decision 564 of tail-55 pair 160
// (evidence loop, seed 105341): gorge offered the cast and the kernel, at land
// ec5795494d, did not. Later dirac mana-offer fixes made the kernel offer it
// too; the recording keeps both engines' priority menus equal.
//
// CR 601.2b with CR 605.1a and CR 106.5: a {B/G} pip's black face is offered
// when the only black source left is Incubation Druid. Its reflected "one mana
// of any type that a land you control could produce" counts the tapped Swamp,
// and its three counters' worth of {B} covers the face beside the floating
// {G}{G}{C}. gorge priced the face through the cast window probe (one
// alternative per reflected colour); the kernel's window view holds the source
// as {C} alone and offered only the green face (decision 440 of tail-55-c000
// pair 34, seed 419595).
func TestMBCHagHedgeMageReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"hag_hedge_mage_priority_cast_offer",
		"hybrid_pip_face_reflected_amount_source",
	} {
		t.Run(name, func(t *testing.T) { stdReproNamed(t, name) })
	}
}
