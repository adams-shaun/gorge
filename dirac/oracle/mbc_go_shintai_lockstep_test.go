package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const goShintaiRepro = "go_shintai_filter_fee_script_budget_exhausted"

// CR 605.3, 601.2g-h: Go-Shintai of Life's Origin's {W}{U}{B}{R}{G},{T}
// activation is payable beside Pillarverge Pathway (W), a Swamp, Akoum Refuge,
// two Forests, School of the Unseen ({2},{T}: any colour) and a summoning-sick
// Eldrazi Scion: School makes the {U} and its {2} fee is paid by a Forest and
// the Scion's sacrifice. Gorge's planner prices no filter, so the activation is
// found only by its prefix search, and School's fee gap beside five coloured
// pips and the Scion's sacrifice-only source makes both the plain and the
// phased search spend their whole 160-node budget with no script (measured:
// plain and phased both nodes=160, limited, funding 2, one gap); gorge offers
// no activation at decision 621 (tail-42 pair 37, seed 731977) while the
// kernel offers it. Matching gorge's exhaustion node for node is far past
// bounded dirac code, so Go-Shintai of Life's Origin is refused by name and its
// recorded game still diverges with it implicated.
func TestMBCGoShintaiOfLifeSOriginReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Go-Shintai of Life's Origin" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != goShintaiRepro {
			t.Fatalf("Go-Shintai of Life's Origin's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, goShintaiRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Go-Shintai of Life's Origin has no dirac/support/refused entry: its activation is priced by a prefix search that School of the Unseen's fee gap exhausts")
	}
}
