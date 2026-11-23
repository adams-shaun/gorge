package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const krosanDruidRepro = "kicked_cast_gap_search_budget_exhausted"

// CR 702.33a, 605.3: Krosan Druid's kicked cast costs {6}{G}{G}. With a floating
// {U}{U}{U}{U}{G}, six Islands under Scald, four Forests and Overflowing Basin's
// {1},{T} filter untapped, gorge's potential-play prefix search for that cast
// walks its whole 160-node budget (plain, phased and subsume runs) and prices
// nothing, so the autopay surface withholds the kicked cast; the kernel's
// mirror of the search finds a payment and offers it
// (tail-49-c000 pair 25, decision 861, "options options"). The card is refused
// by name and the recorded game still shows the divergence with it implicated.
func TestMBCKrosanDruidReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Krosan Druid" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != krosanDruidRepro {
			t.Fatalf("Krosan Druid's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, krosanDruidRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Krosan Druid has no dirac/support/refused entry: gorge's prefix search exhausts its node budget on the kicked cast")
}
