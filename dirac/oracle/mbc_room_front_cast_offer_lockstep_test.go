package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const spikedCorridorRepro = "room_front_cast_offer"

// Spiked Corridor (Enchantment Room) in hand beside a Manabarbs, four life and
// three Treasures (decision 770 of tail-34-c000 pair 10, seed
// 11730875627245604381). CR 309.4 makes both doors castable, and gorge's
// planner defers every land under the unvetted TapsForMana watcher, so the
// front-door cast ({3}{R}) and the other door's room_alt cast are priced only
// by a scripted prefix (CR 119.4: the four land taps alone are lethal). The
// autopay surface leaves the plain scripted cast out, where the kernel's walk
// lists it (options options), and once that is lifted the kernel's castface
// payment taps lands to 0 life where gorge's script, through a Treasure,
// keeps 1. The card is refused by name and the recorded game still shows the
// divergence with it implicated.
func TestMBCSpikedCorridorReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Spiked Corridor" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != spikedCorridorRepro {
			t.Fatalf("Spiked Corridor's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, spikedCorridorRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Spiked Corridor has no dirac/support/refused entry: its Manabarbs-board casts part from gorge's planner")
}
