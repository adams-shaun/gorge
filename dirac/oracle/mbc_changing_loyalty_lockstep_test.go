package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const changingLoyaltyRepro = "changing_loyalty_replicate_search_budget"

// CR 702.56a, 106.1: a replicated cast of Changing Loyalty ({1}{B} plus a
// replicate payment, {3}{B} in all) is priced by gorge's planner for no
// optional-cost shape, so its census is incomplete and a prefix search over
// the activations of the sources runs for the replicated play. The only black
// source on that board is Paradise Mantle's granted any-colour tap, with a
// dozen covered lands and a Henge beside it: the 160-node budget is spent on
// the other colours' floats before a pool holding {B} and three more mana is
// reached, so gorge withholds the play ("unsupported census_incomplete") while
// the kernel's census offers it (tail-26-s000 pair 16, decision 598,
// "options options"). The search is not modelled for a repeated-cost cast
// beside this many sources, so the card is refused by name (the Shattering
// Spree precedent) and the recorded game still shows the divergence with it
// implicated.
func TestMBCChangingLoyaltyReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Changing Loyalty" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != changingLoyaltyRepro {
			t.Fatalf("Changing Loyalty's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, changingLoyaltyRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Changing Loyalty has no dirac/support/refused entry: gorge's prefix search gives up on its replicated cast beside a Mantle-only black source")
}
