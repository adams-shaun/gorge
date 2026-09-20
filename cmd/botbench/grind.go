package main

import (
	"fmt"
	"io"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/seat"
)

// Grind mode: the throughput/profiling shape the pair matrix is not.
//
// The matrix (-pairs) interleaves many games across a worker pool sized by
// -workers, which is right for a win-rate measurement but wrong for
// profiling a deck's hotspots: pool scheduling moves games between cores and
// the profile mixes every deck's work into one sample stream. Grind mode
// deliberately does neither: ONE repo deck is pinned to ONE goroutine (its
// two seats play the same deck against each other, both policies "bot"), and
// that goroutine plays as many games as the budget allows, so a CPU profile
// of the run is that deck's engine work and nothing else.
//
// The budget is wall-clock by default (-grind-seconds; -grind-iters sets an
// iteration cap instead, or on top of the wall clock). Wall clock is the
// right default limiter here because the thing a grind run measures is
// throughput and hotspot stability: a fixed iteration count does not bound
// the profile's sample mass (a deck whose games run 5x longer gets 5x the
// samples under an iteration cap, exactly the confound a hotspot comparison
// across decks must not carry), while a wall-clock budget gives every deck
// the same measurement window and answers "how many games per second does
// this deck sustain". The deadline is checked BETWEEN games, so a run always
// plays at least one game no matter how small the budget.
//
// Determinism, scoped honestly: every game's outcome is still a pure
// function of its seed (grindSeed below makes iteration i of deck d play at
// base + d*stride + i, so a run's first k iterations of a deck are exactly
// a longer run's first k), and the report's intent/turn/stall columns are
// pure functions of the seeds. Only the iteration COUNT and the
// iterations/sec column read the wall clock -- they are the measurement, not
// engine outputs. No wall clock reaches an event or an engine path.
//
// -cpuprofile/-memprofile wrap grind exactly as they wrap every other mode
// (profiler.start/finish run in mainExit around the whole run), so a
// profiled grind needs no new code -- and profiling cannot change the game
// results, which are seed-pure.

// grindSeedStride separates decks' seed blocks: iteration i of deck index d
// plays at base + d*stride + i, so no two (deck, iteration) pairs in a run
// share a seed and no deck's stream can run into another's.
const grindSeedStride = uint64(1) << 40

// grindSeed returns the seed iteration i of deck index d plays.
func grindSeed(base uint64, d int, i int) uint64 {
	return base + uint64(d)*grindSeedStride + uint64(i)
}

// grindDefaultSeconds is the budget a -grind run with both budgets unset
// gets. Documented in the flag help and here so the two cannot drift.
const grindDefaultSeconds = 30.0

// grindDeck is one deck's tally.
type grindDeck struct {
	name   string
	iters  int
	stalls int
	// livelocks is the subset of stalls the engine's livelock watcher
	// aborted (an engine bug, not a slow game); firstLivelock records the
	// first one's diagnostic so the report can name it concretely.
	livelocks     int
	firstLivelock string
	wins          [2]int // seat 0 / seat 1 (same deck, same policy: the split is the seat, not the deck)
	draws         int
	intents       int64
	turns         int64
	elapsed       time.Duration
	outcomes      []gameOutcome // kept for tests (TestGrindIterationIsAPrefixOfALongerRun)
}

// runGrind is grind mode's entry: it opens the corpus, resolves the deck
// pool (-format selects commander decks or the repo decks), spawns one
// goroutine per deck, and prints the per-deck table plus the combined line.
// collectors (-decision-stats / -action-coverage) apply here as everywhere.
func runGrind(baseSeed uint64, deck string, seconds float64, iters int, dir, format string, maxTurns, maxIntents int, out, prog io.Writer) error {
	commander, err := parseGameFormat(format)
	if err != nil {
		return err
	}
	if seconds < 0 || iters < 0 {
		return fmt.Errorf("-grind-seconds and -grind-iters must be >= 0, got %g/%d", seconds, iters)
	}
	if seconds == 0 && iters == 0 {
		seconds = grindDefaultSeconds
	}
	deadline := time.Time{}
	if seconds > 0 {
		deadline = time.Now().Add(time.Duration(seconds * float64(time.Second)))
	}
	var pool []string
	if commander {
		pool, err = commanderDeckNames()
		if err != nil {
			return err
		}
	} else {
		pool = testutil.RepoDeckNames()
	}
	var names []string
	if deck == "all" {
		names = pool
	} else {
		known := false
		for _, n := range pool {
			if n == deck {
				known = true
				break
			}
		}
		if !known {
			return fmt.Errorf("-grind: unknown deck %q (not one of the %d %s decks; use \"all\" or one of: %s)",
				deck, len(pool), deckKind(commander), pool)
		}
		names = []string{deck}
	}

	reg, err := testutil.OpenCorpusRegistry(dir)
	if err != nil {
		return fmt.Errorf("opening corpus at %s: %w (run `make fetch-cards compile-cards` first)", dir, err)
	}
	decks := make([][]*cards.Card, len(names))
	commanders := make([][]int, len(names))
	for i, name := range names {
		d, err := testutil.LoadRepoDeck(reg, name)
		if err != nil {
			return err
		}
		decks[i] = d
		if commander {
			cis, err := commanderIndices(reg, name)
			if err != nil {
				return err
			}
			commanders[i] = cis
		}
	}

	// collect/cov are shared across the goroutines; both are mutex-guarded
	// and pure observation, exactly as in the matrix.
	var collect *decisionStats
	if decisionStatsEnabled {
		collect = newDecisionStats()
	}
	var cov *actionCoverage
	if actionCoverageEnabled {
		cov = newActionCoverage()
	}

	results := make([]grindDeck, len(names))
	errs := make([]error, len(names))
	var wg sync.WaitGroup
	wg.Add(len(names))
	for d, name := range names {
		d, name := d, name
		go func() {
			defer wg.Done()
			results[d], errs[d] = grindOne(baseSeed, d, name, decks[d], commanders[d], commander, deadline, iters, maxTurns, maxIntents, collect, cov, prog)
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return writeGrindReport(out, baseSeed, seconds, iters, commander, results, collect, cov)
}

// grindSeats builds one grind iteration's two bot seats. A package var so
// the livelock tests can substitute a seat that aborts the game (the same
// injection point bench has through matchPlayer): the production value is
// the historical inline construction.
var grindSeats = func(seed uint64) []seat.Seat {
	return []seat.Seat{seat.NewBot(seed ^ 1), seat.NewBot(seed ^ 2)}
}

// grindOne plays one deck against itself on THIS goroutine until the budget
// runs out (checked between games, so at least one game always plays) or the
// iteration cap is hit. The tally fields are written only by this
// goroutine; the shared collectors guard themselves. A zero deadline means
// no wall-clock budget.
func grindOne(baseSeed uint64, d int, name string, deck []*cards.Card, commanders []int, commander bool, deadline time.Time, iters, maxTurns, maxIntents int, collect *decisionStats, cov *actionCoverage, prog io.Writer) (grindDeck, error) {
	g := grindDeck{name: name}
	start := time.Now()
	for i := 0; iters <= 0 || i < iters; i++ {
		if i > 0 && !deadline.IsZero() && time.Now().After(deadline) {
			break
		}
		seed := grindSeed(baseSeed, d, i)
		botSeats := grindSeats(seed)
		pols := []string{"bot", "bot"}
		cfg := buildGameConfig(seed, []string{name, name}, [][]*cards.Card{deck, deck}, [][]int{commanders, commanders}, commander)
		o, err := playMatch(cfg, pols, botSeats, maxTurns, maxIntents, collect, cov)
		if err != nil {
			// A failed game aborts the deck's grind (the matrix aborts the
			// whole run on an error; a grind run is per-deck, so the other
			// decks keep going and the first error surfaces after the wait).
			if prog != nil {
				fmt.Fprintf(prog, "grind %s, iteration %d: %v\n", name, i, err)
			}
			g.elapsed = time.Since(start)
			return g, fmt.Errorf("grind %s, iteration %d: %w", name, i, err)
		}
		g.iters++
		g.intents += int64(o.intents)
		g.turns += int64(o.turns)
		if o.isStalled() {
			g.stalls++
			if o.stallOn == "livelock" {
				// A livelocked game is recorded against this deck and seed and
				// the grind continues -- a single stuck game must not hang or
				// abort the batch -- but the tally feeds the end-of-run error:
				// a livelock is an engine bug, not a slow game.
				g.livelocks++
				if g.firstLivelock == "" {
					g.firstLivelock = fmt.Sprintf("iteration %d (seed %d): %s", i, seed, o.livelock)
				}
				if prog != nil {
					fmt.Fprintf(prog, "grind %s, iteration %d: LIVELOCK: %s\n", name, i, o.livelock)
				}
			}
		} else if o.winner == "" {
			g.draws++
		} else {
			g.wins[o.winnerSeat]++
		}
		g.outcomes = append(g.outcomes, o)
	}
	g.elapsed = time.Since(start)
	return g, nil
}

// writeGrindReport prints the per-deck table and the combined line. The
// iters/sec column is the wall-clock measurement; every other column is a
// pure function of the seeds, so two runs at the same base and iteration cap
// report identical tallies even when their rates differ.
func writeGrindReport(out io.Writer, baseSeed uint64, seconds float64, iters int, commander bool, results []grindDeck, collect *decisionStats, cov *actionCoverage) error {
	budget := "unbounded"
	if iters > 0 {
		budget = fmt.Sprintf("%d iteration(s)/deck", iters)
	}
	if seconds > 0 {
		if iters > 0 {
			budget += fmt.Sprintf(" or %.0fs wall clock, whichever first", seconds)
		} else {
			budget = fmt.Sprintf("%.0fs wall clock", seconds)
		}
	}
	hdr := fmt.Sprintf("grind: base seed %d, budget %s, %d deck(s), one goroutine per deck", baseSeed, budget, len(results))
	if commander {
		hdr += " (commander format)"
	}
	fmt.Fprintln(out, hdr)
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "deck\titers\titers/sec\tmean intents\tmean turns\tdraws\tstalls\tlivelocks")
	var totalIters int
	var totalSecs float64
	for _, r := range results {
		rate := 0.0
		if r.elapsed > 0 {
			rate = float64(r.iters) / r.elapsed.Seconds()
		}
		meanIntents, meanTurns := 0.0, 0.0
		if r.iters > 0 {
			meanIntents = float64(r.intents) / float64(r.iters)
			meanTurns = float64(r.turns) / float64(r.iters)
		}
		fmt.Fprintf(tw, "%s\t%d\t%.2f\t%.1f\t%.1f\t%d\t%d\t%d\n",
			r.name, r.iters, rate, meanIntents, meanTurns, r.draws, r.stalls, r.livelocks)
		totalIters += r.iters
		totalSecs += r.elapsed.Seconds()
	}
	// The combined rate is total iterations over the SUM of the decks' own
	// elapsed times -- the per-goroutine busy time, not the run's wall clock
	// (which the decks' goroutines overlap), so the combined figure is the
	// throughput the harness actually delivered across goroutines rather
	// than a per-core figure. The deck-level rates are what a hotspot
	// comparison reads; this line is the run's aggregate.
	combined := 0.0
	if totalSecs > 0 {
		combined = float64(totalIters) / totalSecs
	}
	fmt.Fprintf(tw, "combined\t%d\t%.2f\t\t\t\t\t\t\n", totalIters, combined)
	tw.Flush()
	for _, r := range results {
		if r.firstLivelock != "" {
			fmt.Fprintf(out, "grind %s: LIVELOCK: %s\n", r.name, r.firstLivelock)
		}
	}
	if collect != nil {
		collect.write(out)
	}
	if cov != nil {
		cov.write(out)
	}
	for _, r := range results {
		if r.livelocks > 0 {
			// After the full report: a livelocked game is an engine bug, and
			// a zero exit would read as a clean pass to the automated caller.
			return fmt.Errorf("grind %s: %d game(s) aborted with a livelock (see the LIVELOCK diagnostics above)", r.name, r.livelocks)
		}
	}
	return nil
}
