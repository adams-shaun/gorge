package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const diregrafFlashbackRepro = "diregraf_rebirth_flashback_two_filter_budget"

// Diregraf Rebirth's flashback cast {5}{B}{G} (pool tail-51, surface autopay,
// the recorded shrunk game; decision 1183): the board holds four Swamps,
// Artifact Unknown Shores and Heap Gate ({1},{T}: add one mana of any color)
// beside Turbulent Shore and an Eldrazi Spawn, and the cost is payable under
// CR 601.2h. gorge's prefix search walks its 160-node budget (potentialScriptNodes)
// across the two filter lands, dies before a script for the flashback cast is
// found and withholds the cast as census_incomplete, offering only pass and
// the land play; the kernel's filter search prices the cast. The same cost cast
// from hand finds a script in gorge. The card is refused by name; the recorded
// game still shows the divergence with it implicated.
func TestMBCDiregrafRebirthReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Diregraf Rebirth" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != diregrafFlashbackRepro {
			t.Fatalf("Diregraf Rebirth's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, diregrafFlashbackRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Diregraf Rebirth has no dirac/support/refused entry: gorge's prefix search exhausts its node budget on the flashback cast beside two filter lands")
}
