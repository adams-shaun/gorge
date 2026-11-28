package oracle

import "testing"

const grantedAnyManaRepro = "granted_any_mana_prefix_search_budget_overoffers"

// Wakeroot Elemental's {G}{G}{G}{G}{G} activation beside Dune Chanter's "{T}:
// Add one mana of any color" granted to five untapped lands (pool tail-71,
// surface full, the recorded game; decision 636): CR 605.1a lets the five
// granted abilities pay it, but gorge's composed planner skips granted
// abilities (census_incomplete) and its prefix search, whose plain, phased and
// subsume runs all spend the potentialScriptNodes budget over the granted gaps,
// finds no script, so gorge offers only pass. The kernel's subsume run reaches
// the five-G line at its 162nd node and offers the play. That was the
// divergence the card was refused for. At the recorded decision both engines
// now offer the activation of Wakeroot Elemental, and the recorded game replays
// clean on a table of its own cards, so the card is unrefused with this game as
// its proof (dirac/support/unrefused.tsv).
func TestMBCWakerootElementalRefusalReprosLockstep(t *testing.T) {
	t.Parallel()
	oneOffReproClean(t, grantedAnyManaRepro)
}
