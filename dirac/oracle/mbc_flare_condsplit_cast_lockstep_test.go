package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const flareCondsplitCastRepro = "flare_condsplit_cast"

// Mana Flare beside a lucky Gemstone Caverns (pool dr-mbc-abbb, surface full,
// the recorded game; decision 703): Mana Flare has resolved onto the
// battlefield, p1's only untapped mana source is Gemstone Caverns with one
// luck counter, and Goblin Wardriver {R}{R} is in hand. gorge prices the cast:
// Gemstone's Condition-split arm (add one mana of any colour, the counter's
// gate) plus Mana Flare's on-battlefield TapsForMana extra make two red
// (CR 605.1a, 605.3b, 107.4), and offers it by a script whose terminal
// pricing rides the trigger the activation itself sets off. The kernel
// withholds the cast: the watcher's extra beside the deferred arm's
// activation is not in its pricing. gorge is right per the rules; the card is
// refused by name and the recorded game still shows the divergence with it
// implicated.
func TestMBCGoblinWardriverReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Mana Flare" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != flareCondsplitCastRepro {
			t.Fatalf("Mana Flare's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, flareCondsplitCastRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Mana Flare has no dirac/support/refused entry: the kernel cannot price the TapsForMana extra beside a Condition-split arm's activation, so it withholds the Goblin Wardriver cast gorge rightly offers")
}
