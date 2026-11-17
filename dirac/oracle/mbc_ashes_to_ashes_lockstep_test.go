package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const relicOfSauronRepro = "relic_of_sauron_two_unit_combo_unplanned"

// CR 106.4, 605.3: Relic of Sauron's "{T}: Add two mana in any combination of
// {U}, {B} and/or {R}" lets the player choose each of the two mana. Gorge's
// payment planner does not price a Produced$ Combo source with Amount$ above
// one (the census leaves it as a gap: "unsupported census_incomplete"), so on
// the autopay surface it withholds every spell only that source's second mana
// pays for, while the kernel offers it: tail-47 pairs 0 and 14 part at
// "options options" on Ashes to Ashes ({1}{B}{B} from a Swamp and the Relic) and
// Blessed Wind, and pair 2 at the Relic's own tap flags. The Relic is refused by
// name and the recorded game still shows the divergence with it implicated.
func TestMBCAshesToAshesReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Relic of Sauron" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != relicOfSauronRepro {
			t.Fatalf("Relic of Sauron's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, relicOfSauronRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Relic of Sauron has no dirac/support/refused entry: gorge's planner leaves its two-unit Combo mana unpriced")
}
