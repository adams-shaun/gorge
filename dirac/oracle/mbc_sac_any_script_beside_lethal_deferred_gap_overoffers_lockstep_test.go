package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const sacAnyScriptRepro = "sac_any_script_beside_lethal_deferred_gap_overoffers"

// Ashnod's Altar's "Sacrifice a creature: Add {C}{C}" beside a Crumbling
// Necropolis deferred by p1's Burning Earth (pool tail-71, surface full, the
// recorded shrunk game; gorge ids 3/1): gorge's one prefix search over the two
// census gaps walks the deferred land first, its tap's lethal trigger waits on
// the stack while the repeatable sac-any asks branch per creature, and the
// potentialScriptNodes budget dies before the Altar-only line is reached — so gorge offers
// neither Drana, Kalastria Bloodchief's cast nor Spiked Ripsaw's equip at
// decision 614. The kernel's sac-any tapX search walks its own 24-path budget
// with the defer gap last and prices both plays. The Altar is refused by name;
// the recorded game still shows the divergence with it implicated.
func TestMBCDranaKalastriaBloodchiefReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Ashnod's Altar" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != sacAnyScriptRepro {
			t.Fatalf("Ashnod's Altar's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, sacAnyScriptRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Ashnod's Altar has no dirac/support/refused entry: its repeatable sac-any expansions explode gorge's prefix-search budget beside a Burning-Earth-deferred land")
}
