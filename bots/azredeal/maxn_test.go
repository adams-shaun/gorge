package azredeal

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/bots/azmcts"
	"github.com/adams-shaun/gorge/engine/replay"
)

// TestAZRedealMaxNFourSeatCommanderGame plays the pod game of
// TestAZRedealFourSeatCommanderGame with the max^n config (HostedMaxN) at
// seat 0: the same pod seed, at the fewest simulations that still reach an
// opponent point, so the registry variant is exercised, not re-measured.
// The game finishes and replays to the same head, the opponents were in the
// tree (an opponent point was searched), and no search had to fall back to
// paranoid reading — four seats are exactly maxVecSeats.
func TestAZRedealMaxNFourSeatCommanderGame(t *testing.T) {
	if testing.Short() {
		t.Skip("plays a 4-seat Commander game")
	}
	run := playPod(t, podSeed, 0, func(c *azmcts.SeatConfig) {
		*c = HostedMaxN()
		c.Search.Sims = maxnPodSims
	})
	if run.out.IsStalled() {
		t.Fatalf("game stalled on %q at turn %d (%d intents)", run.out.StallOn, run.out.Turns, run.out.Intents)
	}
	if !run.e.G.Over {
		t.Fatal("the game did not finish")
	}
	if got := len(run.e.G.Players); got != 4 {
		t.Fatalf("%d players, want 4", got)
	}

	re, err := replay.Replay(run.e.L, run.cfg)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if re.L.Head() != run.e.L.Head() {
		t.Fatalf("replay head %s, game head %s", re.L.Head(), run.e.L.Head())
	}

	// Precondition for the two counts below: the pod seed reaches a point
	// where the max^n config answers from the search at all. A config whose
	// feed stopped would play the whole game as the wrapped bot and leave
	// both counts at zero.
	searched := 0
	for _, d := range run.diags {
		if d.Searched {
			searched++
		}
	}
	if searched == 0 && os.Getenv("AZREDEAL_POD_SIMS") == "" {
		t.Fatal("the seat never answered from the search; the max^n counts below are vacuous")
	}

	fallbacks, oppPoints := 0, 0
	for _, d := range run.diags {
		fallbacks += d.Stats.MaxNFallbacks
		oppPoints += d.Stats.OppPoints
		if d.Refused != "" || d.Stats.RootRefused > 0 {
			t.Errorf("seq %d: the honest root was refused: %q", d.Seq, d.Refused)
		}
		if d.Stats.FeedStopped > 0 {
			t.Errorf("seq %d: the seat's feed stopped (%d decisions routed around the search)", d.Seq, d.Stats.FeedStopped)
		}
	}
	if fallbacks != 0 {
		t.Errorf("%d searches fell back to paranoid reading in a 4-seat pod; MaxN did not take effect", fallbacks)
	}
	if oppPoints == 0 {
		t.Error("no opponent point was searched in the whole game; OpponentNodes did not take effect")
	}
	t.Logf("max^n pod game: %d sims, %d turns, %d intents, winner seat %d, %.0f ms wall; %d searched, %d opponent points",
		maxnPodSims, run.out.Turns, run.out.Intents, run.out.WinnerSeat, run.wallMS, searched, oppPoints)
	logPodCost(t, run)
}

// maxnPodSims is the simulations per searched decision the max^n pod test
// runs: the fewest of the measured tries (8, 6, 5, 4) that still reaches an
// opponent point on podSeed, so the test stays inside its budget. Measured,
// not derived: a different budget plays a different game.
const maxnPodSims = 4
