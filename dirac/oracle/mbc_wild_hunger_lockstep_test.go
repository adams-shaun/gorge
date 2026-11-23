package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const wildHungerFlashbackRepro = "wild_hunger_flashback_offer"

// CR 118.9, 602.2, 605.1a, 702.34a: Wild Hunger in a graveyard has flashback
// {3}{R}, and a granted any-colour mana ability is the only red source p1 can
// reach. Gorge's potentialPrefixScript spends its 160-node budget on the
// untapped lands before the granted any-colour tap that alone makes {R}, so
// the flashback cast is priced census_incomplete and not offered, while the
// kernel prices it (tail-15-c000 pair 4, decision 1248, "options options").
// Aligning that search node for node is beyond a bounded fix, so the card is
// refused by name, as Wormhole Serpent is for the same reason, and the
// recorded game still shows the divergence with it implicated.
func TestMBCWildHungerReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Wild Hunger" {
			continue
		}
		found = true
		if e.Repro.Pool != "custom" || e.Repro.File != wildHungerFlashbackRepro {
			t.Fatalf("Wild Hunger's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, wildHungerFlashbackRepro)
		}
		checkRefusedRepro(t, e)
	}
	if !found {
		t.Error("Wild Hunger has no dirac/support/refused entry: gorge's prefix search spends its node budget before the granted any-colour {R} and the kernel offers the flashback cast")
	}
}
