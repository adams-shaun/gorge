package rules

import (
	"runtime"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// BenchmarkFormatCost is a measurement, not an optimisation: it answers, with
// a number, how much of the "Commander tables run 2x slower per step" report
// is the demo's pacing knob (decisions-per-step x pace) and how much is the
// engine's own cost per decision. It drives the engine directly -- never
// through gorged, whose -pace sleep is exactly the variable it exists to
// remove, and never through mtgsim/botbench, which have no format flag.
//
// It builds one rules.Config per format exactly the way host/match.go does
// (FormatCommander: 40 starting life, per-seat Commanders indices from the
// loaded decks; constructed: the zero format/life/commander Config), then
// simulates the same number of full turns of each at the same seed, counting
// decisions and measuring wall time and allocations. What it does NOT hold
// equal -- deck size (100 vs 60 cards) and starting life (40 vs 20) -- is
// inherent to the format: that is a finding, not a flaw (see the report).
func BenchmarkFormatCost(b *testing.B) {
	reg := testutil.CorpusRegistry(b)

	b.Run("commander", func(b *testing.B) {
		runFormatBench(b, commanderBenchConfig(b, reg))
	})
	b.Run("constructed", func(b *testing.B) {
		runFormatBench(b, constructedBenchConfig(b, reg, 4))
	})
	// Reproduces the user's actual constructed table: the report says the
	// Commander table had 4 seats "where constructed demo tables have fewer".
	// Holding seats equal isolates the format mechanics above; this case
	// re-introduces seat count so the decomposition can say how much of the
	// user's 2x is the *number* of decisions (pace x decisions/turn) rather
	// than format. The seat count is the only thing that differs from the
	// 4-seat constructed case: same decks, seed, turns.
	b.Run("constructed-2seat", func(b *testing.B) {
		runFormatBench(b, constructedBenchConfig(b, reg, 2))
	})
	// Round 1: bucket the same 4-seat games' decisions by STEP (not turn), for
	// both formats, then compute the wall time of the two step windows the user
	// actually watched (main1->main2 on the Commander table, begin-combat->end on
	// the constructed table) at 1.5s per decision, so the report can say whether
	// step selection -- not format -- is what produced the ~2x the user saw.
	// Both rows here are 4 seats: the constructed-2seat case above IS the round-0
	// artifact, and this comparison deliberately excludes it.
	b.Run("commander-perstep", func(b *testing.B) {
		logStepWindows(b, "commander(4seats)", perTurnPerStep(b, commanderBenchConfig(b, reg)))
	})
	b.Run("constructed-perstep", func(b *testing.B) {
		logStepWindows(b, "constructed(4seats)", perTurnPerStep(b, constructedBenchConfig(b, reg, 4)))
	})
}

// benchSeed is the one seed both formats play, reported in the benchmark
// output so the two runs are byte-identical to reproduce. A fixed seed here
// is the whole determinism contract: same seed, same decks, same seat
// count, same turn count on both sides of the comparison.
const benchSeed = 0x55b00f

// benchTurns is how many full turns each simulated game plays. 7 is the turn
// the user was watching when they saw the 2x. Both formats survive seven
// full turns at benchSeed (a series ending before turn 7 would Fail the
// completion assertion below, which is how this number was chosen).
const benchTurns = 7

// benchCmdDecks are four of the five interim 100-card Commander repo decks
// (foundations-*). Each carries a commander, so each seat's Config.Commanders
// index resolves.
var benchCmdDecks = [...]string{
	"foundations-calling-all-angels",
	"foundations-keen-engineering",
	"foundations-tramplesaurus-rex",
	"foundations-wretched-ranks",
}

// benchConDecks are four of the twelve 60-card constructed Legacy repo
// decks. A commander table's seats get a command zone; a constructed
// table's seats get none (host/match.go's FormatCommander gate).
var benchConDecks = [...]string{
	"mono-red-goblins",
	"uw-control",
	"tron",
	"death-n-taxes",
}

// commanderBenchConfig builds the FormatCommander rules.Config: 40 life, a
// per-seat Commanders index (deck.File.CommanderIndex holds the one shared
// resolution, so the benchmark and gorged cannot disagree on where a
// commander sits), London mulligan and the corpus token table.
func commanderBenchConfig(b *testing.B, reg *cards.Registry) Config {
	names := make([]string, len(benchCmdDecks))
	decks := make([][]*cards.Card, len(benchCmdDecks))
	cmds := make([][]int, len(benchCmdDecks))
	for i, n := range benchCmdDecks {
		names[i] = n
		decks[i] = testutil.RepoDeck(b, reg, n)
		cmds[i] = []int{testutil.RepoDeckFile(b, n).CommanderIndex()}
	}
	return Config{
		Seed:         benchSeed,
		Names:        names,
		Decks:        decks,
		Tokens:       reg.Tokens,
		Format:       FormatCommander,
		StartingLife: 40,
		Commanders:   cmds,
		Mulligans:    1,
	}
}

// constructedBenchConfig builds the zero-format rules.Config: no Format, no
// StartingLife (the 20 default), no Commanders -- exactly the constructed
// side of host/match.go's switch default. Mulligans is set to 1 to hold the
// London mulligan round equal to the commander side (a held-equal variable,
// not a format distinction).
func constructedBenchConfig(b *testing.B, reg *cards.Registry, seats int) Config {
	names := make([]string, seats)
	decks := make([][]*cards.Card, seats)
	for i := range seats {
		decks[i] = testutil.RepoDeck(b, reg, benchConDecks[i%len(benchConDecks)])
		names[i] = benchConDecks[i%len(benchConDecks)]
	}
	return Config{
		Seed:      benchSeed,
		Names:     names,
		Decks:     decks,
		Tokens:    reg.Tokens,
		Mulligans: 1,
	}
}

// runFormatBench simulates benchTurns full turns of cfg per iteration,
// measuring wall time, decision count and allocations, then reports the
// per-decision and per-turn numbers. Allocations are read from
// runtime.MemStats around the timed window (wall time is measured
// separately, so the MemStats reads do not distort the per-decision clock).
func runFormatBench(b *testing.B, cfg Config) {
	var decisions, ns int64
	var mallocs, allocBytes uint64
	for i := 0; i < b.N; i++ {
		e := New(cfg)
		bot := newTestBot(cfg.Seed) // recreated per block: same rng stream each time
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)
		d, el := playFormatTurns(b, e, bot, benchTurns)
		runtime.ReadMemStats(&m2)
		ns += int64(el)
		decisions += d
		allocBytes += m2.TotalAlloc - m1.TotalAlloc
		mallocs += m2.Mallocs - m1.Mallocs
	}
	if decisions == 0 {
		b.Fatalf("no decisions simulated")
	}
	blocks := int64(b.N)
	turns := float64(blocks * benchTurns)
	decTot := float64(decisions)
	nsTot := float64(ns)
	b.ReportMetric(decTot/turns, "decisions/turn")
	b.ReportMetric(nsTot/decTot, "ns/decision")
	b.ReportMetric(nsTot/turns, "ns/turn")
	b.ReportMetric(float64(mallocs)/decTot, "allocs/decision")
	b.ReportMetric(float64(allocBytes)/decTot, "B/decision")
}

// playFormatTurns drives e from engine creation through benchTurns full
// turns: it first runs the pre-game London mulligan round untimed and
// uncounted (a mulligan is not a turn), then times and counts every decision
// from the start of turn 1 through the end of turn targetTurn. It returns
// the decision count and the wall-clock time over the timed window.
//
// targetTurn is exercised over the whole game (e.G.Turn == 1 on the first
// turn; see rules/turn.go beginTurn). The loop stops when turn targetTurn+1
// begins (e.G.Turn > targetTurn) or the game ends. A game that ends before
// completing targetTurn full turns would silently shrink the denominator, so
// the completion assertion below turns that into a hard failure instead --
// the same honesty a number that moves between runs demands.
func playFormatTurns(b *testing.B, e *Engine, bot *testBot, targetTurn int32) (int64, time.Duration) {
	e.Advance()
	// Pre-game mulligan round (Mulligans=1): not a turn, not timed, not
	// counted. It ends when beginTurn sets e.G.Turn = 1.
	for e.Pending() != nil && !e.G.Over && e.G.Turn <= 0 {
		e.Submit(bot.answer(e, e.Pending()))
	}
	start := time.Now()
	var decisions int64
	for e.Pending() != nil && !e.G.Over && e.G.Turn <= targetTurn {
		e.Submit(bot.answer(e, e.Pending()))
		decisions++
	}
	if e.G.Over {
		b.Fatalf("game ended during turn %d, before completing turn %d", e.G.Turn, targetTurn)
	}
	if e.G.Turn != targetTurn+1 {
		b.Fatalf("stopped at turn %d, want completed turn %d (next turn %d)", e.G.Turn, targetTurn, targetTurn+1)
	}
	return decisions, time.Since(start)
}

// spendPace is the gorged default per-decision sleep the round-0 report named
// as the demonstrator's pacing knob (cmd/gorged/main.go `-pace` default). A
// step's wall time in the served-game model is its decision count x this pace;
// the only thing that varies between the two windows below is which steps the
// user watched, so the pace constant cancels out of their ratio. Kept as a
// named constant here so the report's arithmetic is exactly reproducible.
const spendPace = 1500 * time.Millisecond

// perTurnPerStep plays one deterministic benchTurns-turn game of cfg at the
// benchmark seed and returns the average decision count per full step per
// turn. It reads e.G.Step before each Submit (a struct-field read: no
// allocation, no wall clock, deterministic), so a decision is attributed to
// the step it was asked in. The game is deterministic at a fixed seed, so one
// play is enough; the existing timing/allocation sub-cases keep the repeated
// b.N loop.
func perTurnPerStep(b *testing.B, cfg Config) []float64 {
	e := New(cfg)
	bot := newTestBot(cfg.Seed)
	e.Advance()
	// Pre-game London mulligan round, excluded exactly as playFormatTurns
	// excludes it: a mulligan is not a turn and its steps are not the ones the
	// user watched.
	for e.Pending() != nil && !e.G.Over && e.G.Turn <= 0 {
		e.Submit(bot.answer(e, e.Pending()))
	}
	counts := make([]int64, int(state.StepCleanup)+1)
	for e.Pending() != nil && !e.G.Over && e.G.Turn <= benchTurns {
		s := e.G.Step
		if !s.Valid() {
			b.Fatalf("invalid step %d during turn %d", int(s), e.G.Turn)
		}
		counts[int(s)]++
		e.Submit(bot.answer(e, e.Pending()))
	}
	if e.G.Over {
		b.Fatalf("game ended during turn %d before completing turn %d", e.G.Turn, benchTurns)
	}
	if e.G.Turn != benchTurns+1 {
		b.Fatalf("stopped at turn %d, want completed turn %d", e.G.Turn, benchTurns)
	}
	per := make([]float64, len(counts))
	for s := range counts {
		per[s] = float64(counts[s]) / benchTurns
	}
	return per
}

// sumSteps totals per[s] over the inclusive step range [lo, hi], the window
// arithmetic the report re-states from the per-step table.
func sumSteps(per []float64, lo, hi state.Step) float64 {
	var tot float64
	for s := int(lo); s <= int(hi); s++ {
		tot += per[s]
	}
	return tot
}

// logStepWindows logs one format's per-step decision table and the two windows
// the user watched -- main1->main2 (the main-phase pair where casting decisions
// land) and begin-combat->end (the late, decision-sparse steps) -- with their
// decision counts and, at spendPace, the wall time a served game would show
// between those step markers. This is the "decisions per STEP" alternative the
// round-0 report never considered.
func logStepWindows(b *testing.B, label string, per []float64) {
	mainW := sumSteps(per, state.StepMain1, state.StepMain2)
	cbeW := sumSteps(per, state.StepBeginCombat, state.StepEnd)
	total := sumSteps(per, state.StepUntap, state.StepCleanup)
	// Summary first -- Go truncates verbose benchmark logs, and the window
	// wall-times below are the whole point of this round.
	b.Logf("== %s: window wall-times (avg/turn, %d turns) @ %v pace ==", label, benchTurns, spendPace)
	b.Logf("   window main1->main2   %6.2f dec/turn -> %.1fs/turn @ %v", mainW, mainW*spendPace.Seconds(), spendPace)
	b.Logf("   window begin-combat->end %5.2f dec/turn -> %.1fs/turn @ %v", cbeW, cbeW*spendPace.Seconds(), spendPace)
	b.Logf("   total per turn        %6.2f decisions -> %.1fs @ %v", total, total*spendPace.Seconds(), spendPace)
	// Per-step breakdown, only the steps that see a decision (the rest are 0.00
	// for both formats and would only pad the log).
	for s := range per {
		if per[s] > 0 {
			b.Logf("   step %-16s %5.2f dec/turn", state.Step(s).String(), per[s])
		}
	}
}
