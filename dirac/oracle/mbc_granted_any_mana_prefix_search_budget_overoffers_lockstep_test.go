package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const grantedAnyManaRepro = "granted_any_mana_prefix_search_budget_overoffers"

// Wakeroot Elemental's {G}{G}{G}{G}{G} activation beside Dune Chanter's
// "{T}: Add one mana of any color" granted to five untapped lands (pool
// tail-71, surface full, the recorded game; decision 636): CR 605.1a lets the
// five granted abilities pay it, but gorge's composed planner skips granted
// abilities (census_incomplete) and its prefix search, whose plain, phased and
// subsume runs all spend the 164-node budget over the granted gaps, finds no
// script, so gorge offers only pass. The kernel's subsume run reaches the
// five-G line at its 162nd node and offers the play. Wakeroot Elemental is
// refused by name; the recorded game still shows the divergence with it
// implicated.
func TestMBCWakerootElementalRefusalReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Wakeroot Elemental" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != grantedAnyManaRepro {
			t.Fatalf("Wakeroot Elemental's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, grantedAnyManaRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Wakeroot Elemental has no dirac/support/refused entry: gorge's prefix search spends its node budget over the granted any-colour gaps")
}
