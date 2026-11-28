package oracle

import "testing"

const fireDragonRepro = "chromatic_star_gap_script_budget_exhausted"

// CR 605.3b, 601.2g: Fire Dragon ({6}{R}{R}{R}) is payable by nine mana: four
// Forests, three Mountains and a Forest wearing A-Find the Path ("{T}: Add two
// mana of any one color"). Gorge's planner prices no granted ability, so the
// cast is found by its prefix search, and Chromatic Star's {1},{T},sacrifice
// filter (funding one source per activation) is a census gap beside the grant:
// the search walks the Star's colour and funding branches, exhausts its
// potentialScriptNodes budget (plain and phased) with no script, and offers
// pass and Enormous Baloth alone at decision 450 (tail-62 pair 22, seed
// 489595). Without the Star gap the same search finds the three-step script;
// the kernel offers the cast. That was the divergence the card was refused for.
// At the recorded decision both engines now offer the cast of Fire Dragon, and
// the recorded game replays clean on a table of its own cards, so the card is
// unrefused with this game as its proof (dirac/support/unrefused.tsv).
func TestMBCFireDragonReprosLockstep(t *testing.T) {
	t.Parallel()
	oneOffReproClean(t, fireDragonRepro)
}
