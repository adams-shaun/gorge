package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const worldTreePrefixBudgetRepro = "world_tree_any_colour_prefix_budget"

// CR 106.1, 605.1a, 601.2g: The World Tree's "lands you control have '{T}: Add
// one mana of any color'" (six or more lands) is the only blue p0 can reach
// once a Rain of Filth turn has made every other land sacrifice for {B}, so
// O'aka, Traveling Merchant ({1}{U}) and Wormhole Serpent ({4}{U}) are payable
// by tapping Swamp or Dwarven Hold for {U} and sacrificing lands for the rest.
// Gorge's potentialPrefixScript walks the sacrifice answers first and spends
// its 160-clone budget before it reaches the granted any-colour answer, so
// both casts are priced census_incomplete and not offered, while the kernel's
// mirrored grant search offers them (tail-08 pair 8, decision 473 of 473,
// "options options"). Aligning that search node for node is beyond a bounded
// fix, so both cards are refused by name, as Kor Haven is for the same
// reason, and the recorded game still shows the divergence with each implicated.
func TestMBCOAkaTravelingMerchantReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for _, card := range []string{"O'aka, Traveling Merchant", "Wormhole Serpent"} {
		found := false
		for i := range entries {
			e := &entries[i]
			if e.Card != card {
				continue
			}
			found = true
			if e.Repro.Pool != "custom" || e.Repro.File != worldTreePrefixBudgetRepro {
				t.Fatalf("%s's entry replays %s/%s, want custom/%s", card, e.Repro.Pool, e.Repro.File, worldTreePrefixBudgetRepro)
			}
			checkRefusedRepro(t, e)
		}
		if !found {
			t.Errorf("%s has no dirac/support/refused entry: gorge's prefix search spends its node budget before The World Tree's granted any-colour answer and the kernel offers the cast", card)
		}
	}
}
