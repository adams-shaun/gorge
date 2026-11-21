package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const warpSpeedRepro = "filter_five_unit_fee_script_budget_exhausted"

// CR 605.3b, 602.2b: Warp Speed's activation (reduced to {2}{U}{U} plus
// sacrificing itself) is payable only through Starfleet Academy's
// "{5}, {T}: Add X mana of one colour" filter, fed by five other lands.
// Gorge's prefix search walks the filter's five-unit fee through every land's
// colour ask and exhausts its 160-node budget (plain and phased) with no
// script, so it offers pass alone at decision 681 (std-trk pair 128, seed
// 697578); the kernel's filter search funds the same line in 9 nodes and
// offers both activations. Matching gorge's exhaustion node for node is far
// past bounded dirac code, so Warp Speed is refused by name and its recorded
// game still diverges with it implicated.
func TestMBCWarpSpeedReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Warp Speed" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != warpSpeedRepro {
			t.Fatalf("Warp Speed's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, warpSpeedRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Warp Speed has no dirac/support/refused entry: its activation is priced through a five-unit filter fee gorge's prefix search cannot reach")
	}
}
