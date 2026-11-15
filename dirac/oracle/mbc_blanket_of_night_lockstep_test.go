package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const blanketOfNightRepro = "blanket_of_night_grant_over_a_counter_cost_land_unmodelled"

// CR 305.6: Blanket of Night makes every land a Swamp, and a land with a basic
// land type has the intrinsic "{T}: Add {B}" beside its own abilities. Remote
// Farm's own ability ({T}, remove a depletion counter: Add {W}{W}) is no plain
// one-mana tap, so gorge offers both productions of the one land and the
// kernel's one-ability pcard holds a single production (static_landtype.go
// blgPlain: stall landgrant, tail-06 pairs 19 and 59). The grant card is
// refused by name as Nylea's Presence is, and the recorded game still shows the
// divergence with it implicated.
func TestMBCTail06ReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Blanket of Night" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != blanketOfNightRepro {
			t.Fatalf("Blanket of Night's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, blanketOfNightRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Blanket of Night has no dirac/support/refused entry: its grant over Remote Farm stalls landgrant")
}
