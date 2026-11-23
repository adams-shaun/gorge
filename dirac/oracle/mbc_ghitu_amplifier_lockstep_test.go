package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const ghituKickedRepro = "ghitu_amplifier_kicked_counter_filter_budget"

// Ghitu Amplifier's kicked cast {1}{R} + {2}{U} (pool tail-01, surface autopay,
// the recorded game; decision 462): player 1's untapped sources are three
// Plains-or-Mountain basics beside Tendo Ice Bridge (a charge-counter
// any-colour ability, the only blue), Fetid Heath and Sunscorched Divide, so
// the cost is payable under CR 601.2h. gorge's prefix search walks its 160-node
// budget (potentialScriptNodes) across the three gap sources, dies before a
// script for the kicked cast is found and withholds it as census_incomplete,
// offering only the plain cast; the kernel's search prices the kicked cast.
// The card is refused by name; the recorded game still shows the divergence
// with it implicated.
func TestMBCGhituAmplifierReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Ghitu Amplifier" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != ghituKickedRepro {
			t.Fatalf("Ghitu Amplifier's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, ghituKickedRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Ghitu Amplifier has no dirac/support/refused entry: gorge's prefix search exhausts its node budget on the kicked cast beside Tendo Ice Bridge, Fetid Heath and Sunscorched Divide")
}
