package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const bloodlineCullingRepro = "bloodline_culling_school_fee_census"

// Bloodline Culling {1}{B}{B} (pool tail-42, surface full, the recorded game;
// decision 310): player 0's only payment is Forest and Forest funding School
// of the Unseen's {2},{T} any-colour ability, with Akoum Refuge making B and
// Karplusan Forest the generic, so the cast is payable under CR 605.3a and
// 601.2g-h. gorge's prefix search finds no script for it, prices the cast
// census_incomplete and withholds it; the kernel prices it. The card is
// refused by name; the recorded game still shows the divergence with it
// implicated.
func TestMBCBloodlineCullingReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Bloodline Culling" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != bloodlineCullingRepro {
			t.Fatalf("Bloodline Culling's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, bloodlineCullingRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Bloodline Culling has no dirac/support/refused entry: gorge's prefix search withholds its cast funded through School of the Unseen's {2},{T} fee")
}
