package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const radiantSolarRepro = "radiant_solar_henge_fee_offer"

// Radiant Solar's "{W}, Discard this card" is payable from a Henge of Ramos
// ({2},{T}: one mana of any colour) funded by two Mountains (CR 605.3a, CR
// 602.2b). With Mercadian Bazaar beside them holding one storage counter, its
// "{T}, Remove X storage counters" mana ability adds an X branch to every node
// of gorge's prefix search (potentialPrefixScript), which finds no script
// within its 164-node budget (potentialScriptNodes): the activation is
// census_incomplete and never offered, while the kernel prices it and offers it
// (tail-26-c001 pair 45, decision 179). Matching the search's node accounting
// over that X branch is far more than the kernel's script mirror holds, so the
// card is refused by name as Feedback is, and the recorded game still shows the
// divergence with it implicated.
func TestMBCRadiantSolarReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Radiant Solar" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != radiantSolarRepro {
			t.Fatalf("Radiant Solar's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, radiantSolarRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Radiant Solar has no dirac/support/refused entry: gorge's prefix search finds no script over Henge of Ramos's fee line beside a one-counter Mercadian Bazaar and the kernel offers the activation")
}
