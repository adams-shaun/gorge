package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const dryadIlysianGroveRepro = "dryad_ilysian_grove_grant_over_a_storage_counter_land_unmodelled"

// CR 305.6: Dryad of the Ilysian Grove makes every land of its controller every
// basic land type, and a land with a basic land type has the intrinsic mana
// abilities beside its own. Dwarven Hold's own ability ({T}, remove X storage
// counters: Add {R} for each) is no plain one-mana tap, so gorge offers the
// intrinsic productions beside it and the kernel's one-ability pcard holds a
// single production (static_landtype.go blgPlain: stall landgrant, tail-08
// pair 11). The grant card is refused by name as Blanket of Night is, and the
// recorded game still shows the stall with it implicated.
// dryadIlysianGroveRefused is a step of TestMBCTail08ReprosLockstep, the pool's
// one test.
func dryadIlysianGroveRefused(t *testing.T) {
	t.Helper()
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Dryad of the Ilysian Grove" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != dryadIlysianGroveRepro {
			t.Fatalf("Dryad of the Ilysian Grove's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, dryadIlysianGroveRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Dryad of the Ilysian Grove has no dirac/support/refused entry: its grant over Dwarven Hold stalls landgrant")
}
