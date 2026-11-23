package oracle

import "testing"

// CR 106.6 with CR 601.2f: Summoning Materia on a Smokebraider lets its
// controller cast Nissa's Chosen ({G}{G}) from the top of their library, paid
// with a Forest and the Materia's granted {T}: Add {G}, while a second
// Smokebraider's "any combination of colors, Elementals only" mana can pay
// for none of it. The prefix search that finds the granted source drops each
// gap whose mana the cast cannot spend, and it used to drop them only for a
// cast from the hand: for the may-play cast it spent its node budget on the
// restricted Smokebraider's colour branches, gorge withheld the legal cast
// (census_incomplete) and the kernel offered it (decision 507;
// tail-34-c004, mulligans 0, seed 39947).
func TestMBCNissaSChosenReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mayplay_top_library_cast_restricted_gap")
}
