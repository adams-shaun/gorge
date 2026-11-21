package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const lochmereSerpentFilterSacRepro = "lochmere_serpent_sac_cost_beside_filter_mana_unmodelled"

// Lochmere Serpent's "{B}, Sacrifice a Swamp: You gain 1 life and draw a card"
// is the cause. With only a tapped Xander's Lounge as the Swamp, p1 pays the {B}
// through a chain: Azorius Chancery's {W}{U} pays Cabal Coffers' {2}, and
// Coffers' {B} per Swamp (the Lounge, still on the battlefield) pays the
// activation, which then sacrifices the Lounge (CR 601.2g-h, 605.3b). gorge's
// planner offers it; dirac's filter-chain pricing (script_gen.go actScriptPays,
// plainPricedCost) leaves every sacrifice cost out, so only the {U}, sacrifice an
// Island activation is offered (decision 821 of tail-41-c000). Pricing the chain
// beside a sacrifice part would need the sacrifice candidates' asks modelled in
// the filter search, so the card is refused and its recorded game is pinned.
func TestMBCLochmereSerpentReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Lochmere Serpent" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != lochmereSerpentFilterSacRepro {
			t.Fatalf("Lochmere Serpent's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, lochmereSerpentFilterSacRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Lochmere Serpent has no dirac/support/refused entry: its sacrifice activation beside a filter chain diverges")
	}
}
