package azredeal

import (
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azmcts"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The 4-seat Commander pod: az-redeal at one seat, the production bot at the
// other three, the five-precon acceptance seating of the Commander head.

const (
	// podTestSims is the simulations per searched decision the pod tests run:
	// far under the hosted 100, to fit a test budget. AZREDEAL_POD_SIMS
	// overrides it for the cost measurement.
	podTestSims = 8
	// podSeed is the game seed, chosen by measurement: seat 0 sees an
	// attack to block and a target to choose in it, so all four searched
	// kinds occur. Every seat's seed derives from it.
	podSeed = 3
	// podLostSeed is the seed of the elimination game, chosen by measurement:
	// an opponent goes out first, then seat 0, and the game plays on.
	podLostSeed = 10
	// podMaxTurns and podMaxIntents cap the game so a stall fails fast.
	podMaxTurns, podMaxIntents = 200, 100000
)

// podSims is the simulations per searched decision for this run.
func podSims(t *testing.T) int {
	t.Helper()
	if s := os.Getenv("AZREDEAL_POD_SIMS"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 {
			t.Fatalf("AZREDEAL_POD_SIMS=%q: want a positive integer", s)
		}
		return n
	}
	return podTestSims
}

// podRun is one finished pod game and what the az seat did in it.
type podRun struct {
	cfg     rules.Config
	out     bench.Outcome
	e       *rules.Engine
	diags   []azmcts.Diag
	wallMS  float64
	askedAt []podAsk
}

// podAsk is one decision some seat was asked, with that seat's state then.
type podAsk struct {
	seat int
	kind decision.Kind
	lost bool  // the asked seat had already lost
	out  int   // seats out of the game at the ask
	self bool  // the searching seat (seat 0) was out at the ask
	turn int32 // the game turn at the ask
}

// playPod plays the pod game with az-redeal (Hosted() plus tune) at azSeat.
func playPod(t *testing.T, gameSeed uint64, azSeat int, tune func(*azmcts.SeatConfig)) podRun {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	names, decks, commanders, err := testutil.CommanderAcceptanceDecks(reg)
	if err != nil {
		t.Fatal(err)
	}
	cfg := rules.CommanderAcceptanceConfig(reg, names, decks, commanders)
	cfg.Seed = gameSeed

	acfg := Hosted()
	acfg.Search.Sims = podSims(t)
	if tune != nil {
		tune(&acfg)
	}
	seats := make([]seat.Seat, len(names))
	for i := range seats {
		seed := cfg.Seed ^ uint64(i+1)
		if i == azSeat {
			s, err := New(bots.Options{Seed: seed}, acfg)
			if err != nil {
				t.Fatal(err)
			}
			seats[i] = s
			continue
		}
		seats[i] = heuristic.NewBot(seed)
	}

	var run podRun
	prevWatch, prevMillis := azmcts.Watch, azmcts.Millis
	t0 := time.Now()
	azmcts.Watch = func(d azmcts.Diag) { run.diags = append(run.diags, d) }
	azmcts.Millis = func() float64 { return float64(time.Since(t0)) / float64(time.Millisecond) }
	t.Cleanup(func() { azmcts.Watch, azmcts.Millis = prevWatch, prevMillis })

	var live *rules.Engine
	run.cfg = cfg
	out, e, err := bench.PlayGame(cfg, seats, podMaxTurns, podMaxIntents, bench.Hooks{
		EnvSeats: true, HonestRoot: true,
		Setup: func(e *rules.Engine) { live = e },
		Decision: func(i int, d *decision.Decision, _ decision.Intent, _ *botpolicy.Board) error {
			out := 0
			for p := range live.G.Players {
				if live.G.Players[p].Lost {
					out++
				}
			}
			run.askedAt = append(run.askedAt, podAsk{seat: i, kind: d.Kind, lost: live.G.Players[i].Lost, out: out, self: live.G.Players[0].Lost, turn: live.G.Turn})
			return nil
		},
	})
	run.wallMS = float64(time.Since(t0)) / float64(time.Millisecond)
	if err != nil {
		t.Fatalf("pod game: %v", err)
	}
	run.out, run.e = out, e
	return run
}

// searched counts the searched decisions of each kind the seat answered.
func (r podRun) searched() map[string]int {
	m := map[string]int{}
	for _, d := range r.diags {
		if d.Searched {
			m[d.Kind]++
		}
	}
	return m
}

// TestAZRedealFourSeatCommanderGame plays a whole 4-seat Commander game with
// az-redeal at seat 0 and the production bot elsewhere: it finishes, replays
// to the same chain head, the seat searched a decision of each kind it
// searches, no honest root was refused, and a second run is the same game.
func TestAZRedealFourSeatCommanderGame(t *testing.T) {
	if testing.Short() {
		t.Skip("plays a 4-seat Commander game")
	}
	run := playPod(t, podSeed, 0, nil)
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

	// Precondition for the kind counts: the seat's decisions reached the
	// search at all (a feed that stopped at turn one would play the whole game
	// as the wrapped bot and leave every count below at zero).
	searched := run.searched()
	for _, kind := range []string{"priority", "attackers", "blockers", "target"} {
		// A different simulation budget plays a different game: the kinds a
		// measurement run meets are reported, not required.
		if searched[kind] == 0 && os.Getenv("AZREDEAL_POD_SIMS") == "" {
			t.Errorf("the seat never searched a %s decision (searched: %v)", kind, searched)
		}
	}
	for _, d := range run.diags {
		if d.Stats.RootRefused > 0 || d.Refused != "" {
			t.Errorf("seq %d: the honest root was refused: %q", d.Seq, d.Refused)
		}
		if d.Stats.FeedStopped > 0 {
			t.Errorf("seq %d: the seat's feed stopped (%d decisions routed around the search)", d.Seq, d.Stats.FeedStopped)
		}
	}

	asked := map[decision.Kind]int{}
	for _, a := range run.askedAt {
		if a.seat == 0 {
			asked[a.kind]++
		}
	}
	t.Logf("seat 0 was asked %v", asked)

	again := playPod(t, podSeed, 0, nil)
	if again.e.L.Head() != run.e.L.Head() {
		t.Fatalf("the same seed gave two games: head %s, then %s", run.e.L.Head(), again.e.L.Head())
	}
	t.Logf("pod game: %d sims, %d turns, %d intents, winner seat %d, %.0f ms wall; searched %v",
		podSims(t), run.out.Turns, run.out.Intents, run.out.WinnerSeat, run.wallMS, searched)
	logPodCost(t, run)
}

// logPodCost reports the searched decisions' wall ms (mean, p95).
func logPodCost(t *testing.T, r podRun) {
	var ms []float64
	for _, d := range r.diags {
		if d.Searched {
			ms = append(ms, d.MS)
		}
	}
	if len(ms) == 0 {
		return
	}
	slices.Sort(ms)
	sum := 0.0
	for _, m := range ms {
		sum += m
	}
	t.Logf("searched decisions %d: mean %.0f ms, p95 %.0f ms, max %.0f ms",
		len(ms), sum/float64(len(ms)), ms[(len(ms)*95)/100], ms[len(ms)-1])
}

// TestAZRedealSkipsLostSeats: with the opponents in the tree (the paranoid
// reading of the other three seats), a seat that has lost is never asked a
// decision, and the searching seat keeps searching after an opponent is out.
// The opponent points of a lost seat could only come from a decision posed to
// it; none is.
func TestAZRedealSkipsLostSeats(t *testing.T) {
	if testing.Short() {
		t.Skip("plays a 4-seat Commander game")
	}
	run := playPod(t, podLostSeed, 0, func(c *azmcts.SeatConfig) {
		c.Search.OpponentNodes = true
		c.Search.Sims = min(c.Search.Sims, 4)
	})
	if run.out.IsStalled() {
		t.Fatalf("game stalled on %q at turn %d", run.out.StallOn, run.out.Turns)
	}

	// Preconditions: the game went on past an opponent's elimination with the
	// searching seat still in, and past the searching seat's own, or the
	// checks below ask nothing of a lost seat.
	firstOpp, afterSelf := int32(-1), 0
	for _, a := range run.askedAt {
		if a.lost {
			t.Errorf("seat %d was asked a decision at turn %d after it had lost", a.seat, a.turn)
		}
		if a.out > 0 && !a.self && firstOpp < 0 {
			firstOpp = a.turn
		}
		if a.self {
			afterSelf++
		}
	}
	if firstOpp < 0 {
		t.Fatal("no opponent went out while the searching seat was in; the elimination checks are vacuous")
	}
	if afterSelf == 0 {
		t.Fatal("the game ended with the searching seat's own loss; there is nothing after it to ask")
	}

	// The searching seat searched once an opponent was out, with the
	// opponents' points in its trees, and its roots were never refused.
	afterLoss, oppPoints := 0, 0
	for _, d := range run.diags {
		oppPoints += d.Stats.OppPoints
		if d.Searched && d.Turn >= firstOpp {
			afterLoss++
		}
		if d.Refused != "" || d.Stats.RootRefused > 0 {
			t.Errorf("seq %d: the honest root was refused: %q", d.Seq, d.Refused)
		}
	}
	if afterLoss == 0 {
		t.Errorf("the seat searched nothing from turn %d, when the first opponent went out", firstOpp)
	}
	// Precondition: the opponents were in the tree, so "no point for a lost
	// seat" is about points that exist.
	if oppPoints == 0 {
		t.Error("no opponent point was selected in the whole game; OpponentNodes did not take effect")
	}
}
