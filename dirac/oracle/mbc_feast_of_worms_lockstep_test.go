package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const feastOfWormsRepro = "feast_of_worms_mana_options"

// CR 106.6, 605.3b: a Powerstone's {C} can't be spent on a nonartifact spell
// but pays a mana ability's activation cost, so Powerstone -> Ferrous Lake
// ({1},{T}: add {U}{R}) beside Forest {G}, Swamp {B} and Wirewood Channeler's
// {G} pays Feast of Worms' {3}{G}{G}. Gorge offers the cast (decision 527 of
// tail-11-c000 pair 6); the kernel's prefix search never taps the act-only
// restricted source to fund the filter's fee, so it offers only the pass. The
// fix reaches the script search's fee funding (mana_filterfirst.go filtDo,
// the restricted gap walk), past a bounded change, so the card is refused by
// name and the recorded game still shows the divergence with it implicated.
func TestMBCFeastOfWormsReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Feast of Worms" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != feastOfWormsRepro {
			t.Fatalf("Feast of Worms's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, feastOfWormsRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Feast of Worms has no dirac/support/refused entry: gorge offers its cast, the kernel does not")
}
