package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const magmaticForceRepro = "magmatic_force_world_tree_search_budget"

// CR 605.1a, 106.6, 601.2g: with six or more lands The World Tree gives every
// land "{T}: Add one mana of any color", so eight lands (five Swamps, a
// Mountain, a Dwarven Hold holding no counter and The World Tree itself) pay
// Magmatic Force's {5}{R}{R}{R}. Gorge's planner prices none of the granted
// abilities, so its prefix search runs for the cast, spends its 160 nodes on
// colour assignments across the eight sources and finds no script: the play
// is "unsupported census_incomplete" and the full surface withholds it, while
// the kernel offers the cast (tail-08 pair 7, decision 485, "options
// options"). The kernel's mirror of that search finds the script gorge's
// budget runs out before, and the exhaustion is not modelled for an
// eight-source board, so the card is refused by name and the recorded game
// still shows the divergence with it implicated.
func TestMBCMagmaticForceReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Magmatic Force" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != magmaticForceRepro {
			t.Fatalf("Magmatic Force's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, magmaticForceRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Magmatic Force has no dirac/support/refused entry: gorge's prefix search gives up on its cast beside The World Tree's grant")
}
