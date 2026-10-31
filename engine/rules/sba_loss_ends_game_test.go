package rules

import "testing"

// A player's loss by a state-based action ends a two-player game at once
// (CR 104.2a) even when the same pass also carries a legend-rule choice for
// the survivor (CR 704.5j). gorge parked the legend ask and returned before
// checkGameOver, so the finished game posed a decision (dirac lockstep pair
// 1366: Banishing Light leaving with its departed controller returned a
// second Tinybones).
func TestSBALossEndsTheGameBeforeALegendAsk(t *testing.T) {
	const legend = "Name:Lone Legend\nManaCost:1 B\nTypes:Legendary Creature Rat\nPT:1/1\nOracle:x\n"
	e := mayPlayBase(t)
	onBoardGrant(t, e, 1, legend)
	onBoardGrant(t, e, 1, legend)
	e.G.Players[0].Life = 0
	e.checkStateBased()
	if !e.G.Over {
		t.Fatalf("seat 0 at 0 life: the game is not over (pending %+v)", e.Pending())
	}
	if e.legendBatch != nil {
		t.Fatal("a legend batch is parked in a finished game")
	}
	if d := e.Pending(); d != nil {
		t.Fatalf("a %s decision is pending in a finished game", d.Kind)
	}
	if !e.G.Players[0].Lost {
		t.Fatal("seat 0 did not lose")
	}
}
