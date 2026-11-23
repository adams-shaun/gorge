package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const kickedFeeFilterRepro = "kicked_cast_funded_by_repeat_fee_filter"

// A kicked cast whose only red source is Signpost Scarecrow's "{2}: Add one
// mana of any colour", a repeatable fee-only mana ability (CR 605.3a, 602.2b):
// Kavu Aggressor's {2}{R} plus kicker {4} beside four Plains, a Blightstep
// Pathway and a Blighted Gorge (pool tail-45 pair 7, seed 761966, decision
// 224: gorge's options alone carry cast(#11{Kavu Aggressor})). Gorge's
// potential bound lifts the ability to unbounded and offers the kicked cast;
// the kernel's census prices a fee-only mana ability only in the trigger pay
// gate (mana_repeatpotential.go). It is the class Shaman of the Great Hunt is
// refused for, so Kavu Aggressor is refused by name and its recorded game
// still diverges with it implicated.
func TestMBCKavuAggressorReprosLockstep(t *testing.T) {
	t.Parallel()
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Kavu Aggressor" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != kickedFeeFilterRepro {
			t.Fatalf("Kavu Aggressor's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, kickedFeeFilterRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Kavu Aggressor has no dirac/support/refused entry: gorge offers its kicked cast through a repeatable fee-only mana ability the kernel never prices")
	}
}
