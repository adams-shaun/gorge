package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const mazeOfIthRepro = "maze_of_ith_prevention_beside_shield_ask_options"

// CR 616.1: Maze of Ith's Effect prevents all combat damage dealt to and by
// the untapped attacker, and each activation is its own replacement effect
// whose source is the Maze. A Dapper Shieldmate's shield counter (CR 122.1c)
// is another match for damage dealt to it, so gorge asks the order and offers
// the shield beside the Maze's prevention (pair 3 of the 60-deck mz pool,
// decision 321 of 321). The kernel once held the grant as a bare keyword bit
// and offered a different menu, so the card was refused; it now asks the
// CR 616.1 order for a depleting shield beside a granted prevention
// (dirac/combat_prevention.go, 7d5dfe3830), and the recorded game replays
// clean on a raw table of its own decks' cards, its whole script served.
func TestMBCMazeOfIthReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", mazeOfIthRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions}
	res := replayDecodedScript(t, synthReg(t), cardset.Raw(printedDelveSet(t, r.Cards)), r, pol)
	if pol.next != len(r.Actions) || pol.Skipped != 0 {
		t.Fatalf("recording served %d of %d actions (%d skipped): its game path has drifted", pol.next, len(r.Actions), pol.Skipped)
	}
	if !res.Clean() {
		t.Fatalf("Maze of Ith's recorded game no longer replays clean: div %+v, stall %q", res.Div, res.Stall)
	}
}
