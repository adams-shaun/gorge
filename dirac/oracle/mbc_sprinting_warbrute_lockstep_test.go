package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const worldTreeRedPrefixBudgetRepro = "world_tree_any_colour_red_prefix_budget"

// CR 106.1, 605.1a, 601.2g: once a Rain of Filth turn has made every other
// land sacrifice for {B}, The World Tree's "lands you control have '{T}: Add
// one mana of any color'" is the only red p0 can reach, so Sprinting Warbrute
// ({4}{R}, dash {3}{R}) is payable by tapping a land for {R} and sacrificing
// lands for the rest. Gorge's potentialPrefixScript walks the sacrifice
// answers first and spends its 160-clone budget before it reaches the granted
// any-colour answer, so both casts are priced census_incomplete and not
// offered, while the kernel's mirrored grant search offers them (tail-08 pair
// 62, decision 406, "options options"). It is the cause already recorded for
// O'aka, Traveling Merchant and Wormhole Serpent (a {U} from the same grant),
// and aligning that search node for node is beyond a bounded fix, so the card
// is refused by name and the recorded game still shows the divergence with it
// implicated.
func TestMBCSprintingWarbruteReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Sprinting Warbrute" {
			continue
		}
		found = true
		if e.Repro.Pool != "custom" || e.Repro.File != worldTreeRedPrefixBudgetRepro {
			t.Fatalf("Sprinting Warbrute's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, worldTreeRedPrefixBudgetRepro)
		}
		checkRefusedRepro(t, e)
	}
	if !found {
		t.Error("Sprinting Warbrute has no dirac/support/refused entry: gorge's prefix search spends its node budget before The World Tree's granted any-colour answer and the kernel offers the cast")
	}
}
