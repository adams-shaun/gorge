package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const shatteringSpreeRepro = "shattering_spree_replicate_search_budget"

// CR 702.56a, 106.1: a replicated cast of Shattering Spree ({R} plus a
// replicate {R}) is paid here only through the gap sources: a Forest wearing
// Sheltered Aerie ("{T}: Add two mana of any one color") and a Chromatic Star
// ("{1}, {T}, Sacrifice: Add one mana of any color"). Gorge's planner prices
// neither from its census, so its prefix search runs for the replicated play,
// spends its whole 160-node budget on the any-colour branches of the two gaps
// and finds no script: the play is "unsupported census_incomplete" and the
// autopay surface withholds it, while the kernel offers the cast (tail-62
// pair 51, decision 273, "options options"). The search is not modelled for
// a repeated-cost cast with mixed gaps, so the card is refused by name and the
// recorded game still shows the divergence with it implicated.
func TestMBCShatteringSpreeReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Shattering Spree" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != shatteringSpreeRepro {
			t.Fatalf("Shattering Spree's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, shatteringSpreeRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Shattering Spree has no dirac/support/refused entry: gorge's prefix search gives up on its replicated cast beside two gap sources")
}
