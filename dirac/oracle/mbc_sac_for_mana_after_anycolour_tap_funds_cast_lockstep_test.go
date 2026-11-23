package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const pucaSCovenantRepro = "sac_for_mana_after_anycolour_tap_funds_cast"

// CR 605.3b, 601.2g: with The World Tree (nine lands, six or more) granting the
// untapped Sunlit Marsh "{T}: Add one mana of any color" and Rain of Filth
// giving every land "Sacrifice this land: Add {B}", tapping the Marsh for {G}
// and sacrificing two lands pays Puca's Covenant's {2}{B} alongside
// Snarespinner's {1}{B}, so priority offers both casts (decision 546, tail-08
// pair 13). gorge's prefix search (potentialPrefixScript) spends its 160-node
// budget walking the sacrifice answers and finds no script over those grants,
// so it offers neither cast while the kernel offers both. Matching the search's
// node accounting over a dynamic grant list is far more than the kernel's
// script mirror holds, so the card is refused by name as Kor Haven and Iron
// Fist, Hero for Hire are, and the recorded game still shows the divergence
// with it implicated. Snarespinner itself is not refused: it is evidenced in
// std-fdn/fdn and the recorded divergence passes with Puca's Covenant alone.
func TestMBCPucaSCovenantReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Puca's Covenant" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != pucaSCovenantRepro {
			t.Fatalf("Puca's Covenant's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, pucaSCovenantRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Puca's Covenant has no dirac/support/refused entry: gorge's prefix search finds no script over the granted sacrifices and the granted any-colour tap and the kernel offers the cast")
}
