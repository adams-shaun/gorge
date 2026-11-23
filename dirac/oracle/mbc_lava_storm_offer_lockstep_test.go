package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const lavaStormRepro = "lava_storm_instant_offered_without_combat"

// Lava Storm ({3}{R}{R}) is castable on a board with Cascade Bluffs (a hybrid-fee
// filter land) beside Mage-Ring Network (a storage-counter battery): the Mountain
// pays the Bluffs fee and three Swamps pay the {3} (pool tail-55 pair 46, seed
// 839419, decision 715: dirac's options carry cast(#49{Lava Storm}), gorge's do
// not). Gorge's prefix search (rules/potential_script.go) has two gaps there, the
// filter and the battery, and runs out of its 160-node budget before it finds the
// script, so the plan is census_incomplete and the cast is not offered; the
// kernel's mixgaps and ctrScript mirror prices the same board as scripted.
// Mirroring that budget on mixed filter-plus-battery boards is far over 200 lines
// of dirac. Lava Storm is refused by name and its recorded game still diverges
// with it implicated.
func TestMBCLavaStormReprosLockstep(t *testing.T) {
	t.Parallel()
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Lava Storm" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != lavaStormRepro {
			t.Fatalf("Lava Storm's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, lavaStormRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Lava Storm has no dirac/support/refused entry: gorge's prefix search runs out of budget on a filter-plus-battery board and withholds the cast the kernel offers")
	}
}
