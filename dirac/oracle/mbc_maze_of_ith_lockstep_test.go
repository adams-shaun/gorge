package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const mazeOfIthRepro = "maze_of_ith_prevention_beside_shield_ask_options"

// CR 616.1: Maze of Ith's Effect prevents all combat damage dealt to and by
// the untapped attacker, and each activation is its own replacement effect
// whose source is the Maze. A Dapper Shieldmate's shield counter (CR 122.1c)
// is another match for damage dealt to it, so gorge asks the order and offers
// the shield (the Shieldmate) beside one option for every Maze that
// activated on it. The kernel holds the grant as a keyword bit on the
// recipient: it names the recipient for the prevention, not the Maze, and
// counts one candidate however many Mazes activated, so the two menus
// differ (pair 3 of the 60-deck mz pool, decision 321 of 321). Modelling it
// needs the granting Maze recorded per recipient and per grant, so the card is
// refused by name, as Blanket of Night is, and the recorded game still shows
// the divergence with it implicated.
func TestMBCMazeOfIthReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Maze of Ith" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != mazeOfIthRepro {
			t.Fatalf("Maze of Ith's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, mazeOfIthRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Maze of Ith has no dirac/support/refused entry: its Effect prevention beside a shield counter offers a different replacement-order menu")
}
