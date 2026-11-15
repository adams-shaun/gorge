package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const shrineDynamicSacRepro = "shrine_dynamic_sac_mana_unpriced"

// CR 605.1a, 601.2g: Shrine of Boundless Growth's "{T}, Sacrifice it: Add {C}
// for each charge counter" is a mana ability whose sacrifice may be paid while
// paying a cost. gorge prices it as a last-resort sacrifice unit; the kernel's
// dynamic-amount variants (gen.go varPC) hold {T}-only costs, so its autopay
// never counts it and omits the Equip {2} (Oathkeeper, Takeno's Daisho) gorge
// offers when a Sandstone Bridge and a six-counter Shrine are the only untapped
// sources (tail-01 pair 43). The card is refused by name and the recorded game
// still shows the divergence with it implicated.
func TestMBCOathkeeperTakenoSDaishoReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Shrine of Boundless Growth" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != shrineDynamicSacRepro {
			t.Fatalf("Shrine of Boundless Growth's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, shrineDynamicSacRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Shrine of Boundless Growth has no dirac/support/refused entry: its dynamic {T}, sacrifice mana is no kernel autopay source")
}
