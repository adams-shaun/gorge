package oracle

import (
	"path/filepath"
	"testing"
)

// A Spree cast's target census beside a rider mana source (CR 601.2g, 106.12,
// 605.3): gorge's census walks the CR 601.2g window, which offers every source
// the player could tap, so Requisition Raid's mode cost is payable from a
// floating {W} plus Cabal Pit's {B} at 1 life (its damage is an effect, not a
// cost) or Gilanra's {G} (a TriggersWhenSpent$ source the payment planner
// defers, the window offers). The kernel's census counted neither and reversed
// the cast after the mode answer. The games were recorded by kfuzz over
// dr-mbc-baab (pairs 34 and 159), gorge leading.
func TestSpreeRiderManaLockstep(t *testing.T) {
	reg := synthReg(t)
	for _, name := range []string{
		"spree_census_counts_a_pain_source_at_one_life",
		"spree_census_counts_a_deferred_mana_source",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			stallCheck(t, r, replayReproValue(t, reg, globSet(t, r.Pool, r), r), false)
		})
	}
}
