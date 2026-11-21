package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const reasonRepro = "split_card_castface_offer"

// CR 106.6, 605.3b: a Powerstone's {C} can't be spent on a nonartifact spell
// but pays a mana ability's activation cost, so Powerstone -> Hall of Tagsin
// ({1},{T}: add one mana of any colour) beside three Islands and Accursed
// Duneyard pays Believe's {4}{G} from the graveyard (aftermath, Reason //
// Believe; decision 424 of tail-39-c000 pair 3). Gorge offers the cast; the
// kernel's census never counts the act-only restricted source toward the
// filter's fee when the payment is a spell, so it offers only the pass and
// the other plays. It is the gap Feast of Worms is refused for (mana_filter.go
// faceReachSet holds restricted units out of a filter's fee), past a bounded
// change, so the card is refused by name and the recorded game still shows
// the divergence with it implicated.
func TestMBCReasonReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Reason" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != reasonRepro {
			t.Fatalf("Reason's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, reasonRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Reason has no dirac/support/refused entry: gorge offers Believe's aftermath cast, the kernel does not")
}
