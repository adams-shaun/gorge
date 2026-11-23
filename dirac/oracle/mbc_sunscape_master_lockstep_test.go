package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const candleguideFilterPrefixBudgetRepro = "candleguide_fee_filter_prefix_budget"

// CR 106.1, 605.3b, 601.2g: Crossroads Candleguide's "{2}: Add one mana of any
// color" is a repeatable mana ability with no {T}, so two activations paid from
// the five other untapped sources make {U}{U}, and Sunscape Master's
// "{U}{U}, {T}: Return target creature to its owner's hand" is payable. Gorge's
// potentialPrefixScript spends its 160-clone budget (plain and phased run both
// limited) on that filter's fee branches before it funds the blue, so the
// activation is priced census_incomplete and not offered, while it does fund
// the same board's "{G}{G}, {T}" activation and the kernel's mirrored search
// offers both (tail-26 pair 49, decision 598, "options options"). Aligning
// that search node for node is beyond a bounded fix, so the card is refused by
// name, as Smiting Helix is for the same reason, and the recorded game still
// shows the divergence with it implicated.
func TestMBCSunscapeMasterReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Sunscape Master" {
			continue
		}
		found = true
		if e.Repro.Pool != "custom" || e.Repro.File != candleguideFilterPrefixBudgetRepro {
			t.Fatalf("Sunscape Master's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, candleguideFilterPrefixBudgetRepro)
		}
		checkRefusedRepro(t, e)
	}
	if !found {
		t.Error("Sunscape Master has no dirac/support/refused entry: gorge's prefix search spends its node budget on Crossroads Candleguide's filter before funding {U}{U} and the kernel offers the activation")
	}
}
