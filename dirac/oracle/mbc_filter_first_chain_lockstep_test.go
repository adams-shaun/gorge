package oracle

import "testing"

// CR 605.3a, 605.3b: with Unnerve ({3}{B}) in hand and only a Mountain,
// Viridescent Bog ("{1}, {T}: Add {B}{G}") and two Magus of the Coffers ("{2},
// {T}: Add {B} for each Swamp you control", three Swamps) untapped, the
// Mountain funds the Bog, the Bog's {B}{G} funds one Magus and two of its
// {B}{B}{B} fund the other: {B}{B}{B}{B} casts Unnerve. gorge's PotentialMana
// fixpoint admits each Magus once an admitted filter's mana covers its fee;
// dirac admitted a first-only filter only while plain sources covered it, so
// it offered pass alone (decision 466 of tail-12 pair 22, seed 601411).
func TestMBCUnnerveReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_first_chain")
}
