package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const handGuideFeeFilterRepro = "activation_funded_by_repeat_fee_filter_and_hand_guide"

// Molten Ravager's {R} activation with a Simian Spirit Guide in hand, an
// untapped Sol Grail (chosen blue) and Signpost Scarecrow's "{2}: Add one mana
// of any colour", a repeatable fee-only mana ability (CR 605.3a, 602.2b; pool
// tail-45 pair 25, seed 787572, decision 365: gorge's options alone carry
// activate(#50{Molten Ravager})). Gorge's potential bound lifts the Scarecrow
// to unbounded and prices the activation, then lowers it through the guide's
// {R}; the kernel's activation search funds no hand guide (actScriptPays) and
// prices a fee-only mana ability only in the trigger pay gate
// (mana_repeatpotential.go). It is the class Kavu Aggressor and Shaman of the
// Great Hunt are refused for, so Molten Ravager is refused by name and its
// recorded game still diverges with it implicated.
func TestMBCMoltenRavagerReprosLockstep(t *testing.T) {
	t.Parallel()
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Molten Ravager" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != handGuideFeeFilterRepro {
			t.Fatalf("Molten Ravager's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, handGuideFeeFilterRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Molten Ravager has no dirac/support/refused entry: gorge offers its {R} activation through a repeatable fee-only mana ability and a hand guide the kernel never prices")
	}
}
