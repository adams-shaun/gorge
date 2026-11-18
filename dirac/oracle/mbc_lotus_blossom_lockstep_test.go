package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const lotusBlossomAnyColourSacManaRepro = "lotus_blossom_any_colour_counter_scaled_sacrifice_mana_unmodelled"

// CR 605.1a with CR 106.4: Lotus Blossom's "{T}, Sacrifice this artifact: Add X
// mana of any one color, where X is the number of petal counters on it" is a
// counter-scaled sacrifice source, which gorge's prefix search activates to
// price a cast from the floating mana. The kernel models that shape for one
// fixed colour only (mana_sacctr.go's sacCtrManaAb), so with one petal and a
// Mountain it never offers Lotus Blossom or Novellamental ({1}{U}, the Blossom's
// colour chosen) that gorge offers (tail-74-c000 pair 5, decision 86). The
// any-colour choice, and gorge's pick of it for the prefix, are no bounded
// kernel change, so the card is refused by name and the recorded game still
// shows the option difference with it implicated.
func TestMBCLotusBlossomReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Lotus Blossom" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != lotusBlossomAnyColourSacManaRepro {
			t.Fatalf("Lotus Blossom's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, lotusBlossomAnyColourSacManaRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Lotus Blossom has no dirac/support/refused entry: its any-colour counter-scaled sacrifice mana is unmodelled")
}
