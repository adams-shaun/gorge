package azredeal

import (
	"os"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azmcts"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// podBelief is a belief of the Commander acceptance decks.
func podBelief(t *testing.T) *searchprobe.Belief {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	_, decks, _, err := testutil.CommanderAcceptanceDecks(reg)
	if err != nil {
		t.Fatal(err)
	}
	b, err := searchprobe.NewBelief(decks)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestClosedMaxNEnables: EnableClosedMaxN registers a closed-list, max^n,
// 4-seat Commander-and-constructed search whose seats close their opponents'
// lists under the belief, and a second call re-arms it instead of
// registering twice.
func TestClosedMaxNEnables(t *testing.T) {
	b := podBelief(t)
	if err := EnableClosedMaxN(nil, 8); err == nil {
		t.Error("a nil belief was accepted")
	}
	if err := EnableClosedMaxN(b, 0); err == nil {
		t.Error("zero simulations were accepted")
	}
	for range 2 {
		if err := EnableClosedMaxN(b, 8); err != nil {
			t.Fatal(err)
		}
	}
	e, ok := bots.Lookup(PolicyClosedMaxN)
	if !ok {
		t.Fatal("not registered")
	}
	if !e.Env || !e.Search || !e.ClosedLists || e.ReadsDecklists() || e.MaxSeats != 4 ||
		!slices.Contains(e.Formats, "commander") || !slices.Contains(e.Formats, "constructed") {
		t.Errorf("entry %+v", e.Info)
	}
	s, err := bots.New(PolicyClosedMaxN, bots.Options{Seed: 3, AutoPayMana: true})
	if err != nil {
		t.Fatal(err)
	}
	hs := s.(*hostedSeat)
	if cl, ok := s.(searchseat.ClosedListSeat); !ok || cl.SearchBelief() != b {
		t.Error("the seat does not close its opponents' lists under the belief")
	}
	if c := hs.cfg; c.World != azmcts.WorldRedealClosed || !c.Search.OpponentNodes || !c.Search.MaxN || c.Search.Sims != 8 {
		t.Errorf("config: world %v, opponent nodes %v, max^n %v, sims %d", c.World, c.Search.OpponentNodes, c.Search.MaxN, c.Search.Sims)
	}
}

// TestAZClosedMaxNFourSeatCommanderGame plays the pod game with the closed
// max^n seat (ClosedMaxNFactory over the acceptance decks) at seat 0: it
// finishes and replays, the honest closed root is never refused, and no
// search falls back to the paranoid read. AZREDEAL_POD_SIMS runs it at the
// hosted budget to measure the per-decision cost (logPodCost).
func TestAZClosedMaxNFourSeatCommanderGame(t *testing.T) {
	if testing.Short() {
		t.Skip("plays a 4-seat Commander game")
	}
	t.Parallel()
	b := podBelief(t)
	sims := maxnPodSims
	if os.Getenv("AZREDEAL_POD_SIMS") != "" {
		sims = podSims(t)
	}
	run := playPod(t, podSeed, 0, func(c *azmcts.SeatConfig) {
		*c = closedConfig(bots.Options{}, b, sims)
		c.Search.OpponentNodes, c.Search.MaxN = true, true
	})
	if run.out.IsStalled() {
		t.Fatalf("game stalled on %q at turn %d (%d intents)", run.out.StallOn, run.out.Turns, run.out.Intents)
	}
	if !run.e.G.Over {
		t.Fatal("the game did not finish")
	}
	re, err := replay.Replay(run.e.L, run.cfg)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if re.L.Head() != run.e.L.Head() {
		t.Fatalf("replay head %s, game head %s", re.L.Head(), run.e.L.Head())
	}
	searched, fallbacks, oppPoints := 0, 0, 0
	for _, d := range run.diags {
		if d.Searched {
			searched++
		}
		fallbacks += d.Stats.MaxNFallbacks
		oppPoints += d.Stats.OppPoints
		if d.Refused != "" || d.Stats.RootRefused > 0 {
			t.Errorf("seq %d: the honest root was refused: %q", d.Seq, d.Refused)
		}
	}
	if searched == 0 {
		t.Fatal("the seat never answered from the search")
	}
	if fallbacks != 0 {
		t.Errorf("%d searches fell back to the paranoid read in a 4-seat pod", fallbacks)
	}
	t.Logf("closed max^n pod game: %d sims, %d turns, %d intents, winner seat %d, %.0f ms wall; %d searched, %d opponent points",
		sims, run.out.Turns, run.out.Intents, run.out.WinnerSeat, run.wallMS, searched, oppPoints)
	logPodCost(t, run)
}
