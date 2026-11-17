package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const shrineTapSacDynamicRepro = "shrine_of_boundless_growth_tap_sac_dynamic_mana_unmodelled"

// CR 605.1a, CR 602.2b: Shrine of Boundless Growth's "{T}, Sacrifice this
// artifact: Add {C} for each charge counter on it" is a mana ability gorge's
// payment planner spends as a last resort, so gorge offers a six-mana
// activation (Joraga Auxiliary's {4}{G}{W}, Quilled Wolf's {5}{G}) that four
// Forests, a Plains and the Shrine pay. The kernel keeps no source for a
// tap-and-sacrifice ability of a counted amount (gen.go varPC lowers only a
// {T}-only dynamic amount, and prep.go manaSac only a fixed one), so it offers
// the pass alone (tail-34 pair 7, autopay, decision 362). The card is refused
// by name and the recorded game still shows the divergence with it implicated.
func TestMBCQuilledWolfReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Shrine of Boundless Growth" {
			continue
		}
		// A card has one refused entry, and land's names the Resurrection Orb
		// recording; this divergence's own recording replays against the same
		// card through a copy of the entry.
		own := *e
		own.Repro.File = shrineTapSacDynamicRepro
		checkRefusedRepro(t, &own)
		return
	}
	t.Fatal("Shrine of Boundless Growth has no dirac/support/refused entry: its tap-and-sacrifice counted mana is outside the kernel's sources")
}
