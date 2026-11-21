package azredeal

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/azmcts"
)

// The hosted early stop keeps the game: the 4-seat Commander pod of
// TestAZRedealFourSeatCommanderGame, played at 32 simulations with Hosted()'s
// GapStop and without it, is one game (the same event-chain head), and the
// stop fired on some searched decision.
func TestGapStopKeepsTheGame(t *testing.T) {
	if testing.Short() {
		t.Skip("plays two 4-seat Commander games")
	}
	if _, ok := Hosted().Search.StopEarly.(*azmcts.GapStop); !ok {
		t.Fatalf("Hosted().Search.StopEarly = %#v, want an *azmcts.GapStop", Hosted().Search.StopEarly)
	}
	const sims = 32
	with := playPod(t, podSeed, 0, func(c *azmcts.SeatConfig) { c.Search.Sims = sims })
	without := playPod(t, podSeed, 0, func(c *azmcts.SeatConfig) { c.Search.Sims, c.Search.StopEarly = sims, nil })
	if !with.e.G.Over || !without.e.G.Over {
		t.Fatalf("a game did not finish: with the stop %v, without %v", with.e.G.Over, without.e.G.Over)
	}
	stops, searched, sumWith, sumWithout := 0, 0, 0, 0
	for _, d := range with.diags {
		if d.Searched {
			searched++
			sumWith += d.Stats.Simulations
		}
		stops += d.Stats.EarlyStops
	}
	for _, d := range without.diags {
		if d.Stats.EarlyStops != 0 {
			t.Fatalf("seq %d: an early stop with StopEarly nil", d.Seq)
		}
		sumWithout += d.Stats.Simulations
	}
	if searched == 0 {
		t.Fatal("the az seat searched no decision")
	}
	if stops == 0 {
		t.Fatalf("GapStop fired on none of %d searched decisions", searched)
	}
	if with.e.L.Head() != without.e.L.Head() {
		t.Fatalf("GapStop game head %s, full game head %s", with.e.L.Head(), without.e.L.Head())
	}
	t.Logf("%d searched decisions, %d stopped early; %d simulations with the stop, %d without", searched, stops, sumWith, sumWithout)
}
