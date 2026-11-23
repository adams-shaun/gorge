package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const underworldDreamsRepro = "prefix_search_lethal_tap_trigger_budget"

// CR 605.3b, 603.3, 602.5d: with Corrupted Roots on one of four Forests and
// one life left, Hall of Tagsin's "{1},{T}: any color" funded by another Forest
// makes the third {B} of Underworld Dreams, so the kernel offers the cast. Tapping
// the enchanted Forest stacks a lethal trigger the sorcery-speed cast must wait
// out; gorge's prefix search branched every tap under that trigger before it
// resolved it, spent its 160 nodes there and left the cast census_incomplete
// (decision 724 of tail-39-c000 pair 6). Mirroring the kernel's settle-first
// walk re-priced the payment of survivable waiting triggers' lines the kernel
// branches instead (Kiri-Onna beside a Scald, Cobblebrute's Scald), so the card
// is refused by name, as Ashnod's Altar is, and the recorded game still shows
// the divergence with it implicated.
func TestMBCUnderworldDreamsReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Underworld Dreams" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != underworldDreamsRepro {
			t.Fatalf("Underworld Dreams's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, underworldDreamsRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Underworld Dreams has no dirac/support/refused entry: gorge's prefix search spends its budget branching the taps beneath a lethal waiting trigger and is census_incomplete")
}
