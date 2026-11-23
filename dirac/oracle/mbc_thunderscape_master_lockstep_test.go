package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const thunderscapeMasterRepro = "thunderscape_master_gap_search_budget_exhausted"

// CR 601.2g: Thunderscape Master's cast costs {2}{R}{R}. With two Mountains, a
// Domesticated Watercourse, five Islands under Scald and Overflowing Basin's
// {1},{T} filter untapped, gorge's potential-play prefix search for that cast
// walks its whole 160-node budget (plain, phased and subsume runs) and prices
// nothing, so the autopay surface withholds the cast; the kernel's mirror of
// the search prices the payment and offers it (tail-49 pair 60, decision 608,
// "options options"). The card is refused by name and the recorded game still
// shows the divergence with it implicated.
func TestMBCThunderscapeMasterReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Thunderscape Master" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != thunderscapeMasterRepro {
			t.Fatalf("Thunderscape Master's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, thunderscapeMasterRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Thunderscape Master has no dirac/support/refused entry: gorge's prefix search exhausts its node budget on the cast")
}
