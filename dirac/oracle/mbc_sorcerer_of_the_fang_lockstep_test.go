package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const sorcererFangRepro = "sorcerer_of_the_fang_activation_offer"

// Sorcerer of the Fang's {5}{B},{T} (pool tail-51, surface autopay, the
// recorded game; gorge ids 71/72): p1 has one untapped land (Radiant Grove),
// every Swamp tapped, Prismite ("{2}: Add one mana of any color") and a
// Treasonous Ogre ("Pay 3 life: Add {R}"). The only {B} left is Prismite's, and
// its {2} fee, with the five generic, is paid from the Grove and six Ogre
// activations (18 of p1's 20 life), so gorge's prefix search offers the
// activation at decision 233. The kernel's filter search funds a fee from taps
// and census gaps; a sole life-only pain unit is neither, so it offers
// nothing. The card is refused by name; the recorded game still shows the
// divergence with it implicated.
func TestMBCSorcererOfTheFangReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Sorcerer of the Fang" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != sorcererFangRepro {
			t.Fatalf("Sorcerer of the Fang's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, sorcererFangRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Sorcerer of the Fang has no dirac/support/refused entry: its activation is payable only through a Prismite fee funded by a life-only Treasonous Ogre")
}
