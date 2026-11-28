package oracle

import "testing"

const pathToTheWorldTreeRepro = "path_to_the_world_tree_activation"

// Path to the World Tree's {2}{W}{U}{B}{R}{G} activation (CR 602.2b) is legal
// on a board whose only blue and red mana come from a Lotus Blossom with a
// petal ({T}, sacrifice: one mana of any one colour) and a Celestial Prism
// ({2}, {T}: one mana of any colour), the lands paying the rest (pool tail-74
// pair 24, seed 493426, decision 553: the kernel's options alone carry
// activate(#26{Path to the World Tree})). Gorge's offer gate prices an
// activation's mana through its payment planner, which counts neither a dynamic
// {T}, sacrifice source nor a paid filter as the only source of a colour: on a
// scratch board of Path, nine lands and either source alone, gorge withheld the
// activation when the Prism was the only {R} or the Lotus the only {U}. The
// kernel's census prices both as gaps (mana_dynsacgap.go, mana_filtsubsets.go).
// That was the divergence the card was refused for. At the recorded decision
// both engines now offer the activation of Path to the World Tree, and the
// recorded game replays clean on a table of its own cards, so the card is
// unrefused with this game as its proof (dirac/support/unrefused.tsv).
func TestMBCPathToTheWorldTreeReprosLockstep(t *testing.T) {
	t.Parallel()
	oneOffReproClean(t, pathToTheWorldTreeRepro)
}
