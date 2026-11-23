package oracle

import "testing"

// CR 605.3a, 601.2g-h: gorge's prefix search for an entwined Road of Return
// ({G}{G} plus entwine {2}) activates Plaza of Heroes as a funding source and
// answers its "choose a mana ability" ask with the restricted any-colour
// ability too (spend only on a legendary spell, so no use to this cast), a
// node and a colour ask of the 160-node budget each. The kernel's funding
// pass tapped the printed {C} alone, found the Centaur's Paradise Mantle
// {G} within the budget and offered the entwined cast gorge left unpriced
// (tail-26 pair 3, decision 301).
func TestMBCRoadOfReturnReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restricted_funder_search_nodes_entwined_cast")
}
