package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const tramwayStationRepro = "sac_other_mana_source"

// gorge and dirac part at an `options options` priority decision: only the
// kernel offers activate(Tramway Station, ability 1), the {2}{B}{R}, {T},
// sacrifice-self draw. It is affordable (CR 601.2g, 605.3a): Diamond
// Kaleidoscope's "Sacrifice a Prism token: Add one mana of any color" needs no
// tap, so the two Prism tokens fund the {B} and {R} pips while two untapped
// basics pay the {2}. gorge's payment planner tiers a mana ability whose cost
// sacrifices ANOTHER permanent as TierDeferred (engine/rules/pay/planshape.go,
// the self-sacrifice-only last-resort branch), builds no witness and withholds
// the activation. Carrying a sacrifice victim in the plan witness is a
// versioned wire change of 250-400 lines, so the card is refused by name and
// the recorded game still shows the divergence with it implicated.
func TestMBCTramwayStationReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Tramway Station" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != tramwayStationRepro {
			t.Fatalf("Tramway Station's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, tramwayStationRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Tramway Station has no dirac/support/refused entry: gorge's planner withholds a draw funded by a sacrifice-another mana ability and the kernel offers it")
}
