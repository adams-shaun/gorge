package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const pathToTheWorldTreeRepro = "path_to_the_world_tree_activation"

// Path to the World Tree's {2}{W}{U}{B}{R}{G} activation (CR 602.2b) is legal on
// a board whose only blue and red mana come from a Lotus Blossom with a petal
// ({T}, sacrifice: one mana of any one colour) and a Celestial Prism ({2}, {T}:
// one mana of any colour), the lands paying the rest (pool tail-74 pair 24,
// seed 493426, decision 553: the kernel's options alone carry
// activate(#26{Path to the World Tree})). Gorge's offer gate prices an
// activation's mana through its payment planner, which counts neither a
// dynamic {T}, sacrifice source nor a paid filter as the only source of a
// colour: on a scratch board of Path, nine lands and either source alone,
// gorge withheld the activation when the Prism was the only {R} or the Lotus
// the only {U}. The kernel's census prices both as gaps (mana_dynsacgap.go,
// mana_filtsubsets.go). Matching both gaps on one activation is a prefix
// search and a fee class past bounded dirac code, so Path to the World Tree is
// refused by name and its recorded game still diverges with it implicated.
func TestMBCPathToTheWorldTreeReprosLockstep(t *testing.T) {
	t.Parallel()
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Path to the World Tree" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != pathToTheWorldTreeRepro {
			t.Fatalf("Path to the World Tree's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, pathToTheWorldTreeRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Path to the World Tree has no dirac/support/refused entry: gorge withholds its activation when a Lotus Blossom and a Celestial Prism are the only sources of two of its colours and the kernel offers it")
	}
}
