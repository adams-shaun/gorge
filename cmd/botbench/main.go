// Command botbench plays N matches between two named bot policies and
// reports the result: wins each way, draws, policy A's win rate with a 95%
// confidence interval, and the mean number of turns per game.
//
// The whole run is reproducible from (base seed, policies, N) alone:
//
//   - game i is seeded from base+i, so no game in a run is a repeat of
//     another (and a run of 500 at a base is a strict superset of a run of
//     100 at the same base);
//   - seat assignment alternates every game, so a seating advantage (the
//     first turn, the deck list each seat holds) is spread equally over the
//     run instead of masquerading as a policy advantage;
//   - nothing outside those inputs reaches the output -- no wall clock, no
//     map-range order, no global rand.
//
// Wins are attributed by seat, not by policy name: each win is credited to
// whichever side held the winning seat that game. That matters most for the
// baseline run `-a bot -b bot`, where both sides run the same policy and
// every non-draw winner carries the same name -- the seat is the only thing
// that can split the run. The summary therefore always reports the wins per
// seat (for the two-seat case, with a rate and confidence interval) beside
// the A/B split, and marks the A/B line when both sides are the same policy:
// that split is ~50% by construction, and the seat split is the number that
// measures a play/draw advantage.
//
// `-a bot -b bot` is the same-policy baseline that measures seating bias:
// seat.NewBot is the production policy, and pitting it against itself
// shows how big a seat/play-order artifact is before any real comparison
// is read (the split is ~50% by construction, and it moves with the seed
// and game count, so no single figure is quoted here). The head-to-head
// that credits a policy: -a bot -b legacy, where legacy is the pre-B2
// fuzz-driver combat frozen in botpolicy.LegacyDecide. Registering a
// third policy is one entry in the policies map. Same names on both sides
// is a valid and expected run.
//
// -pairs switches the bench to a deck-pair matrix. The default run (no
// -pairs) plays one deck list per seat and is unchanged; the defect it
// fixes is that with the repo's 12 decks it always and only played the
// first two sorted names (death-n-taxes vs dimir-tempo). -pairs all plays
// every unordered deck pair (66 for 12 decks at 2 seats) and -pairs
// a:b,c:d plays named pairs, so which deck list a seat holds is varied
// across the run instead of pinned forever to one matchup. Three
// properties make a matrix the same measurement the single-pair run is:
//
//   - -games means games PER PAIR (the header and pooled line both say
//     that), so the historical N=4000 on death-n-taxes:dimir-tempo is the
//     matrix's first row and reproduces it exactly;
//   - seats still trade policies within each pair every game (aPlaysSeat),
//     so a deck list can no more masquerade as a policy advantage in a
//     matrix than it can in the single-pair run; each deck pair is instead
//     played at a distinct seed block (pair p uses base+p*games ..
//     base+(p+1)*games), so the whole matrix is a pure function of (base
//     seed, games per pair, pair list) and pairs are independent and
//     playable in parallel;
//   - pair order is the sorted i<j order over RepoDeckNames(), never map
//     iteration, so a matrix is byte-identical run to run and allows an
//     exact -out json diff between two builds.
//
// The report is a per-pair table (pair, A/B wins, A win rate with its 95%
// CI, the seat split with its CI, mean turns) plus a pooled line over all
// pairs with its own CI and the count of pairs whose A-win interval
// excludes 50% in each direction -- the honest summary a single pooled
// percentage hides.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"text/tabwriter"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/view"
)

// policies registers every bot policy the bench can pit against another,
// keyed by the -a/-b name. Each entry constructs a fresh Seat for one seat
// of one game from that seat's per-game seed, so a policy's decisions are a
// pure function of (base seed, game index, seat) and nothing else. The map
// is read by name only, never ranged over in the reporting path, so its
// iteration order can never reach the output.
var policies = map[string]func(seed uint64) seat.Seat{
	// seat.NewBot's *Bot result is wrapped because Go has no return-type
	// covariance: a func returning *Bot is not assignable to one returning
	// seat.Seat, and the wrapper keeps a future policy free to return any
	// Seat implementation.
	"bot": func(seed uint64) seat.Seat { return seat.NewBot(seed) },
	// legacy is the pre-B2 policy, frozen in botpolicy.LegacyDecide: attack
	// with everything that can, block half the legal pairs on a coin. It is
	// not a production policy -- nothing but the bench drives it -- it is
	// the head-to-head baseline that measures whether the heuristic bot is
	// any better than the fuzz driver it replaced (-a bot -b legacy).
	"legacy": func(seed uint64) seat.Seat {
		return &legacySeat{r: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
	},
}

// legacySeat is the bench seat for the old policy: it reads the same view
// any seat receives but hands the board's IsMain fact only, which is all
// the pre-B2 policy ever read (botpolicy.LegacyDecide), and it seeds its
// rng exactly like seat.NewBot so the legacy policy's consumption is the
// seed-deterministic one.
type legacySeat struct {
	r *rand.Rand
}

func (s *legacySeat) Decide(_ context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	return botpolicy.LegacyDecide(botpolicy.Board{IsMain: v.Phase == "main1" || v.Phase == "main2"}, &d, s.r), nil
}

// gameSeed returns the seed game i of a run at base seed b plays. The
// engine's log and rng and every seat bot's PCG all derive from this one
// number, seeded base+i -- so the run as a whole is reproducible from
// (base, policies, N) and the first k games of any run are exactly the run
// at the same base with N=k. The "same game N times" failure mode the bench
// exists to rule out is literally this function returning a constant.
func gameSeed(base uint64, game int) uint64 { return base + uint64(game) }

// decisionStatsEnabled is switched on by the -decision-stats flag when main
// starts a run. It lives at package scope (not as a run/runMatrix parameter)
// so the dozens of test call sites that exercise the normal path keep their
// current signature and behaviour: when disabled, no collector is created,
// nothing is recorded and nothing is appended, so the normal report is
// byte-identical to a pre-flag build. main sets it once; it is read-only
// thereafter.
var decisionStatsEnabled bool

// aPlaysSeat reports whether policy A (the -a side) holds seat s in game i.
// A holds a seat when (game+seat) is even: with two seats the assignment
// flips every game, and for any seat count a seat sees A in exactly half
// the games of an even run -- the property TestSeatAssignmentAlternates
// pins. Every (policy, seat) pair therefore plays the same number of games,
// so a seating advantage cannot masquerade as a policy advantage regardless
// of how many seats the run uses. This function is the single source of
// truth for both the seat assignment AND the win attribution: the tally
// credits each win to the side aPlaysSeat says held the winning seat, so
// the two cannot drift apart.
func aPlaysSeat(game, seat int) bool { return (game+seat)%2 == 0 }

// resolvePolicy looks a -a/-b name up in the policies table. The error
// message lists the known names sorted, so it is deterministic like every
// other output the bench prints.
func resolvePolicy(name string) (func(seed uint64) seat.Seat, error) {
	p, ok := policies[name]
	if !ok {
		known := make([]string, 0, len(policies))
		for k := range policies {
			known = append(known, k)
		}
		sort.Strings(known)
		return nil, fmt.Errorf("unknown policy %q (known: %s)", name, strings.Join(known, ", "))
	}
	return p, nil
}

// gameOutcome is one bench game's result: the policy that won and the seat
// it won from, how many turns the game lasted (state.Game.Turn at the
// end), and how many intents the seats answered. The seat is what the tally
// attributes by: each win is credited to whichever side held the winning
// seat that game (aPlaysSeat), so the split is correct whether or not the
// two policy names collide. With aName == bName the seat is the only thing
// that can tell the sides apart -- with both names "bot", the A/B split
// would read as "both sides won everything" without it. Intents and turns
// are the two numbers every later bot task reads: an intent count reaching
// the -max-intents cap is a policy that stopped terminating, and mean turns
// is the metric the report averages. winnerSeat is only meaningful when
// winner is non-empty.
type gameOutcome struct {
	winner     string // policy name of the winning seat; "" for a draw
	winnerSeat int    // the seat the winner sat in (valid when winner != "")
	turns      int32
	intents    int
	// stallOn names the watchdog cap that ended the game before it could
	// finish, distinguishing the two failure modes a reader must tell apart:
	// "turns" (the turn watchdog fired at -max-turns -- the game ran long)
	// and "intents" (the intent watchdog fired at -max-intents -- the turn
	// number stopped advancing while intents kept coming). "" means the game
	// is not stalled. A stalled game is a distinct outcome -- NOT a win and
	// NOT a draw -- excluded from the win-rate denominator, so a runner
	// cannot mistake the rate of whatever happened to terminate for an
	// honest win rate.
	stallOn string
}

// isStalled reports whether the game was ended by either watchdog cap.
func (o gameOutcome) isStalled() bool { return o.stallOn != "" }

// playMatch plays one game between the given per-seat seats to completion, or
// ends it as a stall at whichever watchdog cap fires first. pols is the
// policy name sitting at each seat, used only to map the winner's seat back
// to a policy. The seats each own their RNG (seeded by the caller), the
// engine replays from its own Config.Seed, and nothing reads the wall clock,
// so the outcome is a pure function of the inputs.
//
// The two caps catch two different pathologies and are both harneess props,
// not engine rules:
//   - maxTurns (the -max-turns flag): a game that runs long in turn count.
//     0 means no cap. It cannot catch a frozen-turn loop, because the turn
//     number never advances to it.
//   - maxIntents (the -max-intents flag): a game whose turn number stops
//     advancing but that keeps submitting intents forever (the re-arming a
//     {0} Equip onto its own target loop). 0 means no cap.
//
// Reaching either cap is a stalled outcome, never an error: one hung game
// records a stall and the rest of the run keeps going instead of aborting
// the whole matrix.
func playMatch(cfg rules.Config, pols []string, seats []seat.Seat, maxTurns, maxIntents int, collect *decisionStats) (gameOutcome, error) {
	if collect != nil {
		collect.game()
	}
	e := rules.New(cfg)
	e.Advance()
	n := 0
	for !e.G.Over && e.Pending() != nil && (maxIntents <= 0 || n < maxIntents) {
		// The turn watchdog: a game whose turn count reaches the cap is
		// stalled -- neither a win nor a draw (and so excluded from the
		// win-rate denominator) -- and maxTurns==0 means no cap. It reads
		// the turn number the engine already reports (state.Game.Turn) and
		// caps nothing under rules/.
		if maxTurns > 0 && e.G.Turn >= int32(maxTurns) {
			return gameOutcome{stallOn: "turns", turns: e.G.Turn, intents: n}, nil
		}
		d := e.Pending()
		v := view.Project(e.G, e, d.Player, d)
		in, err := seats[d.Player].Decide(context.Background(), v, *d)
		if err != nil {
			return gameOutcome{}, fmt.Errorf("seed %d, intent %d, seat %d: %w", cfg.Seed, n, d.Player, err)
		}
		if collect != nil {
			collect.record(d, in)
		}
		if err := e.Submit(in); err != nil {
			return gameOutcome{}, fmt.Errorf("seed %d, intent %d: %w", cfg.Seed, n, err)
		}
		n++
	}
	if !e.G.Over {
		// The loop exited with the game still live: the intent cap was the
		// limiter (maxIntents <= 0 with a live game would mean a nil pending
		// decision mid-game, itself a non-terminating stall). This is a
		// stalled outcome -- NOT an error -- so the run records it and steps
		// over the pair instead of killing the whole matrix.
		return gameOutcome{stallOn: "intents", turns: e.G.Turn, intents: n}, nil
	}
	return outcomeFrom(e, pols, n), nil
}

// outcomeFrom reads a finished game's result. Ruling P14: Draw must be read
// before Winner -- Winner's zero value is seat 0, a real seat, so reading
// it unconditionally would misreport a drawn game as its first seat's
// policy winning.
func outcomeFrom(e *rules.Engine, pols []string, intents int) gameOutcome {
	var o gameOutcome
	o.turns = e.G.Turn
	o.intents = intents
	if !e.G.Draw {
		o.winner = pols[e.G.Winner]
		o.winnerSeat = int(e.G.Winner)
	}
	return o
}

// stallNotice is the loud, unmistakable summary line a run with any stalled
// games prints beside its win rates. It exists so a run that threw games
// away to either watchdog cap cannot be mistaken for a clean run -- the very
// mistake the win-rate-over-non-stalled-games change exists to prevent. It
// names the two caps separately because they mean different things: a game
// that hit -max-turns ran long, while one that hit -max-intents froze its
// turn count -- a reader who cannot tell them apart cannot act on either. A
// run with no stalls prints nothing, so the constructed default report is
// byte-identical to a run that never saw a watchdog.
func stallNotice(turnStalls, intentStalls, eff int) string {
	if turnStalls == 0 && intentStalls == 0 {
		return ""
	}
	return fmt.Sprintf("\n@@ STALLED: %d game(s) hit the -max-turns cap, %d hit the -max-intents cap; win rates are over %d non-stalled game(s) @@\n", turnStalls, intentStalls, eff)
}

// ci95 returns the 95% confidence interval on a success rate using the
// normal approximation to the binomial (the Wald interval):
//
//	p̂ ± z·√(p̂(1−p̂)/n)   with z = 1.96
//
// clamped to [0, 1]. It is explicitly an approximation: accurate in the
// large-sample centre of the binomial, it degrades at the edges -- p̂ of 0
// or 1 collapses the interval to a point, a known Wald pathology the report
// defends against by always printing the game count beside the interval,
// which is what actually makes a thin sample look thin. This is the
// simplest interval that widens honestly as n shrinks, and the width is
// exactly what TestTheIntervalWidensOnASmallSample pins.
func ci95(successes, trials int) (lo, hi float64) {
	if trials < 1 {
		return 0, 0
	}
	p := float64(successes) / float64(trials)
	se := math.Sqrt(p * (1 - p) / float64(trials))
	lo = p - 1.96*se
	hi = p + 1.96*se
	if lo < 0 {
		lo = 0
	}
	if hi > 1 {
		hi = 1
	}
	return lo, hi
}

// seatLabels renders the per-game seat map ("A@0 B@1") in seat order. It is
// built from the per-seat slice -- the seat-to-policy assignment lives in
// that slice only, never in a map that could range in some other order.
func seatLabels(pols []string) string {
	parts := make([]string, len(pols))
	for i, p := range pols {
		parts[i] = fmt.Sprintf("%s@%d", p, i)
	}
	return strings.Join(parts, " ")
}

// winnerLabel renders the per-game winner: "policy@seat" for a real win,
// "draw" for CR 104.4a's no-surviving-seats ending. The seat is why the
// per-game lines can show a same-policy run's seat bias at all -- with both
// policies named "bot" the raw name alone is identical on both sides.
func winnerLabel(o gameOutcome) string {
	if o.isStalled() {
		return "stalled"
	}
	if o.winner == "" {
		return "draw"
	}
	return fmt.Sprintf("%s@%d", o.winner, o.winnerSeat)
}

// polsFor returns the policy name sitting at each seat of game g: B sits
// everywhere and A takes the seats aPlaysSeat gives it, so seats trade
// policies every game. It is the single builder both the single-pair bench
// and the matrix pair worker use, so the two can never disagree about which
// policy a seat holds in a given game.
func polsFor(g, seats int, aName, bName string) []string {
	pols := make([]string, seats)
	for seat := 0; seat < seats; seat++ {
		pols[seat] = bName
		if aPlaysSeat(g, seat) {
			pols[seat] = aName
		}
	}
	return pols
}

// matchPlayer plays one game of a bench run: given a game's seed and the
// policy name sitting at each seat, it returns that game's outcome. run
// wires the real engine into this shape (playMatch with per-seat bots);
// tests inject synthetic players to pin the attribution and the report
// without paying an engine game each.
type matchPlayer func(seed uint64, pols []string) (gameOutcome, error)

// outcomeKind is the shared classification of one finished bench game,
// produced by classifyOutcome and applied by both the single-pair fold and
// the matrix pair fold, so the two tallies can never drift apart. A real win
// credits whichever side held the winning seat in that game (aPlaysSeat),
// which is the one source of truth for both the seat assignment and the win
// attribution.
type outcomeKind struct {
	stall      bool
	stallOn    string
	draw       bool
	winnerSeat int
	aWin       bool
	bWin       bool
}

// classifyOutcome splits one gameOutcome into the classification the singles
// and matrix folds share. The game index g is needed only because attribution
// reads it; seats bounds the winner-seat check. It returns a BARE
// winner-seat-out-of-range error that the callers wrap with their own
// "game N:" / "pair P, game N:" context, so the folded error messages stay
// exactly as they were before this helper existed.
func classifyOutcome(g int, oc gameOutcome, seats int) (outcomeKind, error) {
	var k outcomeKind
	switch {
	case oc.isStalled():
		k.stall = true
		k.stallOn = oc.stallOn
	case oc.winner == "":
		k.draw = true
	default:
		if oc.winnerSeat < 0 || oc.winnerSeat >= seats {
			return outcomeKind{}, fmt.Errorf("winner seat %d out of range [0,%d)", oc.winnerSeat, seats)
		}
		k.winnerSeat = oc.winnerSeat
		if aPlaysSeat(g, oc.winnerSeat) {
			k.aWin = true
		} else {
			k.bWin = true
		}
	}
	return k, nil
}

// bench is the testable entry of the bench: it validates `games`/`seats`,
// plays `games` matches using the default worker budget (all cores), and
// writes the per-game lines and the summary to out. Every match is seeded
// from base+game (gameSeed) and the seat assignment alternates (aPlaysSeat),
// so the output is byte-identical across runs with the same
// (base, policies, games). run() resolves the corpus and decks, wires the
// real engine in, and passes a caller-sized -workers budget through
// benchWithPool; the tests drive bench directly with a synthetic
// matchPlayer, exactly like the tests of cmd/mtgsim drive its run. The
// parallel body and the full semantics of the fold live on benchWithPool.
//
// Attribution is by seat, not by name: with distinct policies the two agree
// (pols[seat] is the policy at that seat), and with aName == bName the seat
// is the only thing that can split the run. aPlaysSeat decides both the
// assignment and the attribution, so they cannot drift apart.
//
// bench never touches a *state.Game or a *rules.Engine field: it drives
// engines the way hosts and sims do -- view.Project, Seat.Decide,
// Engine.Submit -- so a bench run exercises the same seat-facing path a
// real match does, and the engine and seat packages stay untouched by this
// command.
func bench(baseSeed uint64, games, seats int, aName, bName string, play matchPlayer, out io.Writer) error {
	pool := newGamePool(runtime.NumCPU())
	defer pool.close()
	return benchWithPool(baseSeed, games, seats, aName, bName, play, out, pool)
}

// benchWithPool is bench's parallel body. It plays every game into a
// per-game slot across `pool` (games are seeded base+i, so each is
// independent exactly as the matrix's are), waits for all of them, then
// folds the slots in ascending game order. The fold is the only place that
// mutates the running tally or writes a per-game line, so the output and the
// first error are identical to the old sequential loop regardless of
// completion order: the per-game lines stay in ascending game order, and the
// first error returned is the LOWEST failing game index, not whichever
// goroutine happened to finish first.
func benchWithPool(baseSeed uint64, games, seats int, aName, bName string, play matchPlayer, out io.Writer, pool *gamePool) error {
	if games < 1 {
		return fmt.Errorf("-games must be at least 1, got %d", games)
	}
	if seats < 2 {
		return fmt.Errorf("-seats must be at least 2 (a bench pits two policies), got %d", seats)
	}

	slots := make([]gameResult, games)
	var done sync.WaitGroup
	done.Add(games)
	for g := 0; g < games; g++ {
		g := g
		s := gameSeed(baseSeed, g)
		pols := polsFor(g, seats, aName, bName)
		pool.submit(func() {
			defer done.Done()
			slots[g].outcome, slots[g].err = play(s, pols)
		})
	}
	done.Wait()

	var aWins, bWins, draws, stallTurns, stallIntents int
	seatWins := make([]int, seats)
	var totalTurns int64
	for g, result := range slots {
		if result.err != nil {
			// All games have completed, so the first error picked from the
			// ordered slots is the sequential loop's lowest failing game
			// index, not whichever game finished first.
			fmt.Fprintf(out, "game %d: %v\n", g, result.err)
			return result.err
		}
		oc := result.outcome
		s := gameSeed(baseSeed, g)
		pols := polsFor(g, seats, aName, bName)
		// The per-game line carries the seed next to the result, so a run
		// that ever played the same game twice would show it -- the seed
		// column stops grinding forward. That is the symptom the bench
		// exists to make impossible, and it is visible in the report if it
		// ever regresses.
		fmt.Fprintf(out, "game %d: seed %d, %s, %6d intents, %3d turns, winner=%s\n",
			g, s, seatLabels(pols), oc.intents, oc.turns, winnerLabel(oc))
		kind, err := classifyOutcome(g, oc, seats)
		if err != nil {
			return fmt.Errorf("game %d: %w", g, err)
		}
		switch {
		case kind.stall:
			// A stalled game is neither a win nor a draw: it credits no seat
			// and is excluded from the win-rate denominator below. The two
			// caps are tallied apart so the stall notice can say which
			// pathology ended the game.
			if kind.stallOn == "intents" {
				stallIntents++
			} else {
				stallTurns++
			}
		case kind.draw:
			draws++
		default:
			// A real win: credit the side that held the winning seat this
			// game. classifyOutcome resolved attribution via aPlaysSeat, the
			// same predicate that assigned the seats, so the tally agrees
			// with the assignment by construction.
			seatWins[kind.winnerSeat]++
			if kind.aWin {
				aWins++
			} else {
				bWins++
			}
		}
		totalTurns += int64(oc.turns)
	}

	// The win-rate denominator excludes stalled games: a rate over "whatever
	// happened to terminate" is a biased sample, which is the exact defect
	// the turn watchdog exists to expose. With no stalls this is `games`, so
	// the constructed default report stays byte-identical to today.
	eff := games - (stallTurns + stallIntents)
	lo, hi := ci95(aWins, eff)
	fmt.Fprintf(out, "\ngames played: %d\n", games)
	ab := fmt.Sprintf("A wins: %d  B wins: %d  draws: %d", aWins, bWins, draws)
	if aName == bName {
		// Same policy on both sides: every non-draw winner carries the same
		// name, so the A/B split is whichever side happened to hold the
		// winning seat -- ~50% by construction, not a comparison. Say so
		// openly; the seat split below is the informative number.
		ab += fmt.Sprintf("  (same policy %q on both sides: the split is ~50%% by construction; read the seat split below)", aName)
	}
	fmt.Fprintln(out, ab)
	fmt.Fprintf(out, "A win rate: %s\n", rateFmt(aWins, eff, lo, hi, "(normal approximation to the binomial)"))
	if seats == 2 {
		// The two-seat baseline: seat 0's rate and interval (over the
		// non-stalled games, so it shares the win rate's denominator and a
		// stalled game is not silently credited to a seat). Drawn games win
		// no seat, so the per-seat rates sum to less than 100% when draws
		// occur. This is the number the -a -b baseline run exists to
		// produce.
		sLo, sHi := ci95(seatWins[0], eff)
		fmt.Fprintf(out, "seat 0 wins: %d  seat 1 wins: %d  seat 0 win rate: %s\n",
			seatWins[0], seatWins[1], rateFmt(seatWins[0], eff, sLo, sHi, "(normal approximation to the binomial)"))
	} else {
		// More than two seats: counts per seat, no rate -- the two-seat
		// case is the one the bench's consumers compare policies through,
		// and a bare rate per seat across many seats would be noise without
		// the paired policy result.
		parts := make([]string, seats)
		for s := 0; s < seats; s++ {
			parts[s] = fmt.Sprintf("seat %d wins: %d", s, seatWins[s])
		}
		fmt.Fprintln(out, strings.Join(parts, "  "))
	}
	fmt.Fprintf(out, "mean turns per game: %.1f\n", float64(totalTurns)/float64(games))
	if notice := stallNotice(stallTurns, stallIntents, eff); notice != "" {
		// A run with any stalls must say so loudly -- a line that cannot be
		// mistaken for a clean run. Nothing is printed when there are no
		// stalls, so the constructed default report is unchanged.
		fmt.Fprint(out, notice)
	}
	return nil
}

// pairDef is one deck pair in a matrix run: the repo decks that sit in seat
// 0 (a) and seat 1 (b) for every game of that pair. The pair is ordered so
// the historical default assignment -- seat 0 = death-n-taxes, seat 1 =
// dimir-tempo -- is the matrix's first row and reproduces the single-pair
// run byte-for-byte (same decks, same seed base, same seat assignment).
type pairDef struct {
	a, b string
}

func (p pairDef) String() string { return p.a + ":" + p.b }

// fullPairs returns every unordered pair of names in sorted i<j order, so
// the order is a pure function of the sorted deck list and never depends on
// map iteration -- the invariant TestMatrixReportOrderIsSorted pins. For the
// 12 repo decks that is 66 pairs, and the first pair -- (death-n-taxes,
// dimir-tempo), because death-n-taxes sorts first -- is exactly the pair the
// default single-pair run has always played, so a matrix row reproduces it.
func fullPairs(names []string) []pairDef {
	ps := make([]pairDef, 0, len(names)*(len(names)-1)/2)
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			ps = append(ps, pairDef{names[i], names[j]})
		}
	}
	return ps
}

// commanderDeckNames returns the repo decks whose deck.File names a
// commander, in RepoDeckNames' sorted order (filtering preserves it). A
// Commander bench only seats these: a Commander game needs commander decks,
// so an -pairs all or -seats run over the full list would otherwise deal a
// 100-card Commander list as a constructed pile -- exactly the defect part A
// fixes. There are five (the foundations-* precon lists).
func commanderDeckNames() ([]string, error) {
	var cmd []string
	for _, n := range testutil.RepoDeckNames() {
		f, err := testutil.LoadRepoDeckFile(n)
		if err != nil {
			return nil, err
		}
		if f.Commander != "" {
			cmd = append(cmd, n)
		}
	}
	return cmd, nil
}

// parsePairsForMode parses a -pairs spec against the deck pool the run's
// format selects: "all" expands over the pool, named pairs are validated
// against it. In commander mode an explicitly named deck that is a repo deck
// but names no commander is a clear flag error -- silently turning it into a
// constructed game would misrepresent the run. Every named deck in the pool
// is validated as a commander deck (with its command-zone index and the
// deck.ValidateCommander CR 903.4/903.5 gate) when the games are set up.
func parsePairsForMode(spec string, pool []string, commander bool) ([]pairDef, error) {
	if spec == "all" {
		return parsePairs("all", pool)
	}
	if commander {
		inPool := make(map[string]bool, len(pool))
		for _, n := range pool {
			inPool[n] = true
		}
		repo := testutil.RepoDeckNames()
		inRepo := make(map[string]bool, len(repo))
		for _, n := range repo {
			inRepo[n] = true
		}
		for _, tok := range strings.Split(spec, ",") {
			tok = strings.TrimSpace(tok)
			if tok == "" {
				continue
			}
			parts := strings.SplitN(tok, ":", 2)
			if len(parts) != 2 {
				continue // parsePairs reports the shape error
			}
			for _, name := range []string{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])} {
				if inRepo[name] && !inPool[name] {
					return nil, fmt.Errorf("-format commander: deck %q is a repo deck but names no commander -- pick a Commander deck (foundations-*)", name)
				}
			}
		}
	}
	return parsePairs(spec, pool)
}

// parsePairs parses a -pairs spec: "all" for every unordered pair of the
// given (sorted) deck names, or a comma-separated list of colon-separated
// deck pairs "a:b,c:d". Deck names must exist in names. The lookup map is
// only probed by key -- never ranged over in the report path -- so iteration
// order cannot reach the output. names is the pool for the run's format
// (commander mode passes the commander-only list).
func parsePairs(spec string, names []string) ([]pairDef, error) {
	if spec == "all" {
		return fullPairs(names), nil
	}
	known := make(map[string]bool, len(names))
	for _, n := range names {
		known[n] = true
	}
	var ps []pairDef
	for _, tok := range strings.Split(spec, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			return nil, fmt.Errorf("-pairs: empty pair in %q", spec)
		}
		parts := strings.SplitN(tok, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("-pairs: %q is not a \"a:b\" deck pair", tok)
		}
		a, b := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if !known[a] {
			return nil, fmt.Errorf("-pairs: unknown deck %q (not one of the %d decks in this format's pool)", a, len(names))
		}
		if !known[b] {
			return nil, fmt.Errorf("-pairs: unknown deck %q (not one of the %d decks in this format's pool)", b, len(names))
		}
		ps = append(ps, pairDef{a, b})
	}
	if len(ps) == 0 {
		return nil, fmt.Errorf("-pairs: no deck pairs given")
	}
	return ps, nil
}

// pairPlayer plays one game of a matrix pair. pos indexes into the pairs
// slice (so the closure can resolve that pair's two decks); seed is the
// game's seed and pols is the policy name sitting at each seat, built by
// playOnePair exactly the way bench builds it, so a pair is the single-pair
// run with a different deck list. It is matrix-mode's analogue of run's
// `play` closure.
type pairPlayer func(pos int, seed uint64, pols []string) (gameOutcome, error)

// pairResult tallies one deck pair: the raw A/B and seat counts plus mean
// turns, exactly the numbers a single-pair run reports, so a matrix row is
// directly comparable to the historical single-pair number. stalls counts
// the games either watchdog ended -- they are neither A/B wins nor draws,
// credit no seat, and are excluded from the pair's win-rate denominator --
// split into stallTurns (the -max-turns cap: the game ran long) and
// stallIntents (the -max-intents cap: the turn count froze), so the pooled
// notice can say which pathology each dropped game hit.
type pairResult struct {
	pd           pairDef
	games        int
	aWins        int
	bWins        int
	draws        int
	stalls       int
	stallTurns   int
	stallIntents int
	seatWins     [2]int
	totalTurns   int64
}

// commanderIndex returns the command-zone index a deck names -- its File's
// CommanderIndex -- validating the deck against the Commander deck rules
// first (deck.ValidateCommander, the m35 CR 903.4/903.5 gate cmd/gorged
// applies up front) so a commander bench never half-starts on an illegal
// deck. A deck with no commander is an error, the same gate the deck-pool
// selection enforces: a Commander game must seat commander decks.
func commanderIndex(reg *cards.Registry, name string) (int, error) {
	f, err := testutil.LoadRepoDeckFile(name)
	if err != nil {
		return 0, err
	}
	if f.Commander == "" {
		return 0, fmt.Errorf("-format commander: deck %q is a repo deck but names no commander", name)
	}
	if verr := f.ValidateCommander(reg); verr != nil {
		return 0, fmt.Errorf("commander deck %q: %w", name, verr)
	}
	return f.CommanderIndex(), nil
}

// parseGameFormat turns the -format flag's string into the commander flag
// that threads through run/runMatrix. "constructed" is the zero value.
func parseGameFormat(s string) (bool, error) {
	switch s {
	case "constructed", "":
		return false, nil
	case "commander":
		return true, nil
	}
	return false, fmt.Errorf("-format must be \"constructed\" or \"commander\", got %q", s)
}

// gameSeedPair returns the seed game g of pair number pos in a matrix run at
// base baseSeed, where every pair plays exactly `games` games. Game i of a
// run plays at base+i, so a pair's games are distinct from every other
// pair's because the offset pos*games shifts the whole block; and pair 0 is
// base+0..base+games-1, byte-identical to the single-pair run of the same
// len games at the same base -- the property that lets a matrix row
// reproduce today's number (and that cmd/botbench's own liveness probe
// would catch if it ever stopped holding).
func gameSeedPair(baseSeed uint64, pos, games, g int) uint64 {
	return baseSeed + uint64(pos*games+g)
}

// gamePool is the one shared execution budget for a matrix run. Pair workers
// submit individual games to it instead of creating a worker pool per pair;
// therefore -workers bounds live games even when many pairs are active.
type gamePool struct {
	jobs chan func()
	wg   sync.WaitGroup
}

func newGamePool(workers int) *gamePool {
	if workers < 1 {
		workers = 1
	}
	p := &gamePool{jobs: make(chan func())}
	p.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer p.wg.Done()
			for job := range p.jobs {
				job()
			}
		}()
	}
	return p
}

func (p *gamePool) submit(job func()) {
	p.jobs <- job
}

func (p *gamePool) close() {
	close(p.jobs)
	p.wg.Wait()
}

type gameResult struct {
	outcome gameOutcome
	err     error
}

// playOnePair plays `games` matches of one deck pair and tallies them into a
// pairResult. Seats trade policies every game (aPlaysSeat), exactly as the
// single-pair bench does, so the deck a seat holds is fixed for the pair but
// which policy plays it alternates -- a deck list can no more masquerade as
// a policy advantage here than it can in the single-pair run. Every game is
// seeded from gameSeedPair so the whole matrix is a pure function of (base
// seed, per-pair game count, pair list) and nothing else.
func playOnePair(baseSeed uint64, pos, games int, aName, bName string, pd pairDef, play pairPlayer, prog *progressWriter) (pairResult, error) {
	pool := newGamePool(runtime.NumCPU())
	defer pool.close()
	return playOnePairWithPool(baseSeed, pos, games, aName, bName, pd, play, prog, pool)
}

// playOnePairWithPool runs all games concurrently, but folds their slots in
// ascending game order. The fold is the only place that mutates pairResult,
// so tallying and the first error remain identical to the old sequential loop
// regardless of completion order.
func playOnePairWithPool(baseSeed uint64, pos, games int, aName, bName string, pd pairDef, play pairPlayer, prog *progressWriter, pool *gamePool) (pairResult, error) {
	var r pairResult
	r.pd = pd
	r.games = games
	step := games
	if step > 1000 {
		step = 1000
	}

	slots := make([]gameResult, games)
	var done sync.WaitGroup
	done.Add(games)
	for g := 0; g < games; g++ {
		g := g
		s := gameSeedPair(baseSeed, pos, games, g)
		pols := polsFor(g, 2, aName, bName)
		pool.submit(func() {
			defer done.Done()
			slots[g].outcome, slots[g].err = play(pos, s, pols)
		})
	}
	done.Wait()

	for g, result := range slots {
		if result.err != nil {
			// All games have completed, so selecting the first error from the
			// ordered slots preserves the sequential loop's lowest failing
			// game, not whichever worker happened to finish first.
			return pairResult{}, fmt.Errorf("pair %s: %w", pd, result.err)
		}
		oc := result.outcome
		kind, err := classifyOutcome(g, oc, 2)
		if err != nil {
			return pairResult{}, fmt.Errorf("pair %s, game %d: %w", pd, g, err)
		}
		switch {
		case kind.stall:
			// A stalled game is a distinct outcome -- not a win, not a draw,
			// credited to no seat -- and is excluded from the win-rate
			// denominator when the pair is reported. The cause is tallied
			// apart so the pooled notice can name the cap that ended the game.
			r.stalls++
			if kind.stallOn == "intents" {
				r.stallIntents++
			} else {
				r.stallTurns++
			}
		case kind.draw:
			r.draws++
		default:
			r.seatWins[kind.winnerSeat]++
			if kind.aWin {
				r.aWins++
			} else {
				r.bWins++
			}
		}
		r.totalTurns += int64(oc.turns)
		if g > 0 && g%step == 0 {
			prog.line("pair %d (%s): %d/%d games", pos+1, pd, g, games)
		}
	}
	return r, nil
}

// progressWriter serialises live progress lines from parallel workers onto a
// single writer. It is deliberately NOT the report writer: a matrix run's
// report is printed once, serially and in pair order, after every pair
// finishes, so the report is byte-identical across runs regardless of how
// the workers interleaved. Progress is only so an hour's run is not a black
// box; it never reaches the deterministic report.
type progressWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (p *progressWriter) line(format string, a ...any) {
	if p == nil || p.w == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.w, format+"\n", a...)
}

// runPairs plays every pair in `pairs` over `games` games each and returns
// the per-pair tallies, in pair order. Pairs are independent games -- each is
// a distinct seed block -- so they can be played in parallel (workers) and
// the result is byte-deterministic regardless of how many workers run: each
// result is written to results[pos] by disjunct position and read back in
// slice order, never through a map. A worker that hits an error (an intent
// cap, a policy failure) records it, closes stop, and the whole run returns
// that error -- the same abort-on-error behaviour as the single-pair bench.
func runPairs(baseSeed uint64, games int, aName, bName string, pairs []pairDef, play pairPlayer, workers int, prog *progressWriter) ([]pairResult, error) {
	total := len(pairs)
	if total == 0 {
		return nil, fmt.Errorf("no deck pairs to run")
	}
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	// TWO budgets, deliberately not the same number.
	//
	// `workers` is the total live-game budget and is NOT clamped by the pair
	// count: a one-pair run must still be able to play its games across every
	// core, which is the whole point of the inner pool. Clamping it here is
	// what made a single-pair matrix run sequential again -- measured, before
	// this fix: 1 pair x 40 games took 0.46s at 158% CPU both with and without
	// the inner pool, because the pool had been sized to 1.
	//
	// `coords` is how many pair COORDINATORS to spawn, and there is no use for
	// more of those than there are pairs.
	coords := workers
	if coords > total {
		coords = total
	}
	results := make([]pairResult, total)
	if prog == nil {
		prog = &progressWriter{}
	}
	// Every pair coordinator submits into this one pool, so the number of
	// games actually in flight is `workers` no matter how many pairs are
	// active -- a matrix cannot multiply pair workers by game workers.
	pool := newGamePool(workers)
	defer pool.close()

	var (
		next int32
		mu   sync.Mutex
		fail error
		wg   sync.WaitGroup
	)
	stop := make(chan struct{})
	record := func(e error) {
		mu.Lock()
		if fail == nil {
			fail = e
			close(stop)
		}
		mu.Unlock()
	}

	wg.Add(coords)
	for w := 0; w < coords; w++ {
		go func() {
			defer wg.Done()
			for {
				pos := int(atomic.AddInt32(&next, 1)) - 1
				if pos >= total {
					return
				}
				select {
				case <-stop:
					return
				default:
				}
				prog.line("pair %d/%d (%s): playing %d games", pos+1, total, pairs[pos], games)
				r, err := playOnePairWithPool(baseSeed, pos, games, aName, bName, pairs[pos], play, prog, pool)
				if err != nil {
					record(err)
					return
				}
				results[pos] = r
				prog.line("pair %d/%d (%s): done -- policy A %d/%d (%.1f%%)",
					pos+1, total, pairs[pos], r.aWins, r.games, float64(r.aWins)/float64(r.games)*100)
			}
		}()
	}
	wg.Wait()
	if fail != nil {
		return nil, fail
	}
	return results, nil
}

// mergedResult pools every pair's counts into the single set of numbers the
// pooled line and the JSON summary report. Pooling is over COUNTS, not rates:
// the pooled win rate is (sum of A wins)/(sum of all games), which is what a
// single run of all the games would have produced, and its CI is the Wald
// interval on that pooled count. Averaging the per-pair rates instead would
// over-weight the small pairs -- the mutation TestPooledCIPoolsCountsNotRates
// exists to catch.
type mergedResult struct {
	pairs        int
	games        int
	aWins        int
	bWins        int
	draws        int
	stalls       int
	stallTurns   int
	stallIntents int
	seatWins     [2]int
	totalTurns   int64
}

func mergeResults(results []pairResult) mergedResult {
	var m mergedResult
	m.pairs = len(results)
	for _, r := range results {
		m.games += r.games
		m.aWins += r.aWins
		m.bWins += r.bWins
		m.draws += r.draws
		m.stalls += r.stalls
		m.stallTurns += r.stallTurns
		m.stallIntents += r.stallIntents
		m.seatWins[0] += r.seatWins[0]
		m.seatWins[1] += r.seatWins[1]
		m.totalTurns += r.totalTurns
	}
	return m
}

// excludeCounts classifies the per-pair intervals against 50%: a pair whose
// whole A-win interval sits below 50% is a pair where policy A loses; above
// is a pair where A wins; the rest are undecided. The interval is over the
// pair's NON-stalled games (a stalled game is not a win for either side), so
// the honest summary a single pooled percentage hides stays honest when the
// watchdog dropped games -- one that reads "beats B on 41 of 66, loses on 3,
// undecided on 22" says more than "53% overall".
func excludeCounts(results []pairResult) (below, above, undecided int) {
	for _, r := range results {
		lo, hi := ci95(r.aWins, r.games-r.stalls)
		if hi < 0.5 {
			below++
		} else if lo > 0.5 {
			above++
		} else {
			undecided++
		}
	}
	return below, above, undecided
}

// writeMatrixText prints the per-pair table and the pooled line. The header
// states plainly that -games is PER PAIR, because a reader who mistakes the
// total for the per-pair N draws wrong conclusions from every row -- that
// the number is per-pair is the whole point of the flag.
func writeMatrixText(out io.Writer, aName, bName string, baseSeed uint64, games int, results []pairResult, commander bool) error {
	m := mergeResults(results)
	// staleHeader is the header's per-row arithmetic denominator: a pair's
	// A win rate and seat 0 rate are over ITS non-stalled games, not over
	// the byte N, so a stalled game is never distorted into a win or a loss
	// for either side. mEff is the pooled analogue.
	hdr := fmt.Sprintf("bot bench matrix: base seed %d, %s vs %s, %d deck pairs, %d games PER PAIR (total %d games)",
		baseSeed, aName, bName, len(results), games, m.games)
	if commander {
		// The header must say which format the run used, so a Commander
		// number cannot be mistaken for a constructed baseline in a
		// scrollback.
		hdr += " (commander format)"
	}
	fmt.Fprintln(out, hdr)
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "deck pair\tA wins\tB wins\tdraws\tstalls\tA win rate (95% CI)\tseat 0 rate (95% CI)\tmean turns")
	for _, r := range results {
		eff := r.games - r.stalls
		lo, hi := ci95(r.aWins, eff)
		sLo, sHi := ci95(r.seatWins[0], eff)
		// rateCell renders "no rate" for an all-stalled pair, which a bare
		// "0.0%" would otherwise mis-represent as a real rate over no games.
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%s\t%s\t%.1f\n",
			r.pd, r.aWins, r.bWins, r.draws, r.stalls,
			rateCell(r.aWins, eff, lo, hi),
			rateCell(r.seatWins[0], eff, sLo, sHi),
			float64(r.totalTurns)/float64(r.games))
	}
	tw.Flush()

	mEff := m.games - m.stalls
	blo, bhi := ci95(m.aWins, mEff)
	sLo, sHi := ci95(m.seatWins[0], mEff)
	below, above, undecided := excludeCounts(results)
	fmt.Fprintf(out, "\ngames per pair: %d  (total %d games across %d pairs)\n", games, m.games, m.pairs)
	fmt.Fprintf(out, "pooled across %d pairs: %s wins %d, B wins %d, draws %d", m.pairs, aName, m.aWins, m.bWins, m.draws)
	if m.stalls > 0 {
		// The pooled line names the thrown-away games so a reader sees how
		// much of the run the stalls cost.
		fmt.Fprintf(out, ", %d stalled", m.stalls)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "pooled %s win rate: %s\n", aName,
		rateFmt(m.aWins, mEff, blo, bhi, "(normal approximation to the binomial, pooled over counts)"))
	fmt.Fprintf(out, "pooled seat 0 win rate: %s\n",
		rateFmt(m.seatWins[0], mEff, sLo, sHi, "(normal approximation to the binomial)"))
	fmt.Fprintf(out, "mean turns per game (pooled): %.1f\n", float64(m.totalTurns)/float64(m.games))
	fmt.Fprintf(out, "pairs whose A-win interval excludes 50%%: A loses on %d, A wins on %d, undecided on %d\n", below, above, undecided)
	if notice := stallNotice(m.stallTurns, m.stallIntents, mEff); notice != "" {
		// A matrix with any stalls says so loudly, like the single-pair run.
		fmt.Fprint(out, notice)
	}
	return nil
}

// rateFmt renders a rate with its CI and a trailing suffix for the long-form
// report lines ("A win rate:", "pooled ... win rate:"). Over zero
// non-stalled games a rate is not 0%, it is undefined -- a "0.0%" over no
// games is a number that gets quoted as if it meant something -- so it prints
// a phrase that cannot be parsed as a win rate.
func rateFmt(wins, eff int, lo, hi float64, suffix string) string {
	if eff <= 0 {
		return "no rate (all games stalled)"
	}
	s := fmt.Sprintf("%.1f%%  95%% CI [%.1f%%, %.1f%%]", float64(wins)/float64(eff)*100, lo*100, hi*100)
	if suffix != "" {
		s += " " + suffix
	}
	return s
}

// rateCell is the tabwriter column variant of rateFmt: the terse
// "12.3% [1.2%, 23.4%]" shape a matrix row's width needs, with the same
// all-stalled guard.
func rateCell(wins, eff int, lo, hi float64) string {
	if eff <= 0 {
		return "no rate"
	}
	return fmt.Sprintf("%.1f%% [%.1f%%, %.1f%%]", float64(wins)/float64(eff)*100, lo*100, hi*100)
}

// jsonPair and jsonPooled are the machine-readable report, one element per
// pair plus the pooled summary. Rates are fractions (0..1) so a future task
// can diff two runs numerically without re-parsing % strings. Field order is
// struct order, so the JSON is byte-deterministic for identical inputs.
type jsonPair struct {
	Pair       string    `json:"pair"`
	AWins      int       `json:"a_wins"`
	BWins      int       `json:"b_wins"`
	Draws      int       `json:"draws"`
	Stalls     int       `json:"stalls"`
	AWinRate   float64   `json:"a_win_rate"`
	AWinRateCI []float64 `json:"a_win_rate_ci"`
	Seat0Wins  int       `json:"seat0_wins"`
	Seat1Wins  int       `json:"seat1_wins"`
	Seat0Rate  float64   `json:"seat0_rate"`
	Seat0CI    []float64 `json:"seat0_ci"`
	MeanTurns  float64   `json:"mean_turns"`
}

type jsonPooled struct {
	Pairs        int       `json:"pairs"`
	Games        int       `json:"games"`
	GamesPerPair int       `json:"games_per_pair"`
	AWins        int       `json:"a_wins"`
	BWins        int       `json:"b_wins"`
	Draws        int       `json:"draws"`
	Stalls       int       `json:"stalls"`
	AWinRate     float64   `json:"a_win_rate"`
	AWinRateCI   []float64 `json:"a_win_rate_ci"`
	Seat0Rate    float64   `json:"seat0_rate"`
	Seat0CI      []float64 `json:"seat0_ci"`
	MeanTurns    float64   `json:"mean_turns"`
	ExcludeBelow int       `json:"pairs_excluding_50_below"`
	ExcludeAbove int       `json:"pairs_excluding_50_above"`
	Undecided    int       `json:"pairs_undecided"`
}

type jsonDoc struct {
	PolicyA      string     `json:"policy_a"`
	PolicyB      string     `json:"policy_b"`
	BaseSeed     uint64     `json:"base_seed"`
	Format       string     `json:"format"`
	GamesPerPair int        `json:"games_per_pair"`
	Pairs        []jsonPair `json:"pairs"`
	Pooled       jsonPooled `json:"pooled"`
}

func writeMatrixJSON(out io.Writer, aName, bName string, baseSeed uint64, games int, results []pairResult, commander bool) error {
	m := mergeResults(results)
	format := "constructed"
	if commander {
		format = "commander"
	}
	doc := jsonDoc{PolicyA: aName, PolicyB: bName, BaseSeed: baseSeed, Format: format, GamesPerPair: games}
	for _, r := range results {
		eff := r.games - r.stalls
		lo, hi := ci95(r.aWins, eff)
		sLo, sHi := ci95(r.seatWins[0], eff)
		doc.Pairs = append(doc.Pairs, jsonPair{
			Pair:       r.pd.String(),
			AWins:      r.aWins,
			BWins:      r.bWins,
			Draws:      r.draws,
			Stalls:     r.stalls,
			AWinRate:   winRateFrac(r.aWins, eff),
			AWinRateCI: []float64{lo, hi},
			Seat0Wins:  r.seatWins[0],
			Seat1Wins:  r.seatWins[1],
			Seat0Rate:  winRateFrac(r.seatWins[0], eff),
			Seat0CI:    []float64{sLo, sHi},
			MeanTurns:  float64(r.totalTurns) / float64(r.games),
		})
	}
	mEff := m.games - m.stalls
	blo, bhi := ci95(m.aWins, mEff)
	sLo, sHi := ci95(m.seatWins[0], mEff)
	below, above, undecided := excludeCounts(results)
	doc.Pooled = jsonPooled{
		Pairs:        m.pairs,
		Games:        m.games,
		GamesPerPair: games,
		AWins:        m.aWins,
		BWins:        m.bWins,
		Draws:        m.draws,
		Stalls:       m.stalls,
		AWinRate:     winRateFrac(m.aWins, mEff),
		AWinRateCI:   []float64{blo, bhi},
		Seat0Rate:    winRateFrac(m.seatWins[0], mEff),
		Seat0CI:      []float64{sLo, sHi},
		MeanTurns:    float64(m.totalTurns) / float64(m.games),
		ExcludeBelow: below,
		ExcludeAbove: above,
		Undecided:    undecided,
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

// winRateFrac is a 0..1 rate with a guarded denominator, the machine-readable
// counterpart of rateFmt/rateCell; it stays 0 when eff <= 0 (the JSON
// consumers diff runs numerically, where a 0 for a no-rate pair is an honest
// less-than-any-real-rate rather than prose that might get quoted).
func winRateFrac(wins, eff int) float64 {
	if eff <= 0 {
		return 0
	}
	return float64(wins) / float64(eff)
}

// runMatrix is the matrix mode's entry through the real engine: it opens the
// corpus, resolves every distinct deck the pairs name, plays each pair for
// `games` matches (parallelised across pairs by runPairs), and prints either
// the text table+pooled line or the JSON document. It shares playMatch, the
// policies table and ci95 with run(), so the seat-trades-policies and
// seed-determinism properties are the same two seats a single-pair run has.
func runMatrix(baseSeed uint64, games, seats int, aName, bName, dir, format string, pairs []pairDef, workers, maxTurns, maxIntents int, commander bool, out, prog io.Writer) error {
	if seats != 2 {
		return fmt.Errorf("-pairs requires -seats 2 (a matrix pits one deck pair against another), got %d", seats)
	}
	if games < 1 {
		return fmt.Errorf("-games must be at least 1 per pair, got %d", games)
	}
	if format != "text" && format != "json" {
		return fmt.Errorf("-out must be \"text\" or \"json\", got %q", format)
	}
	if _, err := resolvePolicy(aName); err != nil {
		return err
	}
	if _, err := resolvePolicy(bName); err != nil {
		return err
	}

	reg, err := testutil.OpenCorpusRegistry(dir)
	if err != nil {
		return fmt.Errorf("opening corpus at %s: %w (run `make fetch-cards compile-cards` first)", dir, err)
	}

	// Resolve each distinct deck once; pairs share decks so one deck list
	// maps to many pairs. The map is only looked up by key during play --
	// never ranged over in the reporting path -- so its iteration order
	// cannot reach the output. commander mode also resolves each deck's
	// command-zone index and validates the deck against the Commander deck
	// rules (deck.ValidateCommander) up front, so a commander bench never
	// half-starts on an illegal deck or a deck with no commander (which an
	// explicit pair or the -seats path could otherwise slip past).
	deckByName := make(map[string][]*cards.Card, len(pairs)*2)
	cmdrByName := make(map[string]int, len(pairs)*2) // commander mode only
	for _, pd := range pairs {
		for _, name := range []string{pd.a, pd.b} {
			if _, ok := deckByName[name]; ok {
				continue
			}
			d, err := testutil.LoadRepoDeck(reg, name)
			if err != nil {
				return err
			}
			deckByName[name] = d
			if commander {
				ci, err := commanderIndex(reg, name)
				if err != nil {
					return err
				}
				cmdrByName[name] = ci
			}
		}
	}

	// collect is the -decision-stats histogram for this run; nil when the
	// flag is off, so a default run records nothing and appends nothing.
	var collect *decisionStats
	if decisionStatsEnabled {
		collect = newDecisionStats()
	}

	play := func(pos int, seed uint64, pols []string) (gameOutcome, error) {
		pd := pairs[pos]
		botSeats := make([]seat.Seat, 2)
		for seat := 0; seat < 2; seat++ {
			// Same per-seat seed derivation as run(): policy RNG is distinct
			// from the engine's and from every other seat's.
			botSeats[seat] = policies[pols[seat]](seed ^ uint64(seat+1))
		}
		var commanders [][]int
		if commander {
			commanders = [][]int{{cmdrByName[pd.a]}, {cmdrByName[pd.b]}}
		}
		cfg := buildGameConfig(seed, []string{pd.a, pd.b},
			[][]*cards.Card{deckByName[pd.a], deckByName[pd.b]}, commanders, commander)
		cfg.Tokens = reg.Tokens
		return playMatch(cfg, pols, botSeats, maxTurns, maxIntents, collect)
	}

	results, err := runPairs(baseSeed, games, aName, bName, pairs, play, workers, &progressWriter{w: prog})
	if err != nil {
		return err
	}
	if format == "json" {
		if err := writeMatrixJSON(out, aName, bName, baseSeed, games, results, commander); err != nil {
			return err
		}
		collect.write(out)
		return nil
	}
	if err := writeMatrixText(out, aName, bName, baseSeed, games, results, commander); err != nil {
		return err
	}
	collect.write(out)
	return nil
}

// seatedDeckNames returns the deck-list order a run seats, with seat s
// holding names[(s+rotate)%len(names)] -- rotate=0 is the historical fixed
// assignment (seat 0 = names[0] etc.), and over the rotate=0..seats-1 cycle
// every deck sits at every seat exactly once, which is the property a
// rotate experiment relies on: if a four-seat imbalance follows the seat
// index it cannot be a fixed deck sitting there, and vice versa.
func seatedDeckNames(names []string, rotate int) []string {
	n := len(names)
	seated := make([]string, n)
	for s := 0; s < n; s++ {
		seated[s] = names[(s+rotate)%n]
	}
	return seated
}

// run is main's entry through the real engine: it validates the flags,
// opens the corpus and the per-seat repo decks, and plays `games` matches
// between the named policies via benchWithPool, sized by the -workers budget
// (0 uses all cores). It exists so the whole flag-driven path stays open to
// tests that need it (determinism, end-to-end); the loop and the report live
// in benchWithPool, which tests can also drive directly with a synthetic
// matchPlayer.
//
// rotate shifts which deck-list a seat holds (seatDeckIndex: seat s holds
// the pool's (s+rotate)%seats-th deck), so a four-seat commander run can
// be repeated with each deck at each seat -- the experiment that tells a
// seat-index artifact from a deck-strength difference. rotate must be in
// [0, seats); 0 is today's assignment, so a run without the flag is
// byte-identical to the pre-flag bench.
func run(baseSeed uint64, games, seats, rotate, workers int, aName, bName, dir string, maxTurns, maxIntents int, commander bool, out io.Writer) error {
	if games < 1 {
		return fmt.Errorf("-games must be at least 1, got %d", games)
	}
	if seats < 2 {
		return fmt.Errorf("-seats must be at least 2 (a bench pits two policies), got %d", seats)
	}
	if rotate < 0 || rotate >= seats {
		return fmt.Errorf("-rotate must be in [0,%d) with %d seats, got %d", seats, seats, rotate)
	}
	if _, err := resolvePolicy(aName); err != nil {
		return err
	}
	if _, err := resolvePolicy(bName); err != nil {
		return err
	}

	reg, err := testutil.OpenCorpusRegistry(dir)
	if err != nil {
		return fmt.Errorf("opening corpus at %s: %w (run `make fetch-cards compile-cards` first)", dir, err)
	}

	// Decks are tied to seats for the whole run (seat 0 always holds the
	// first deck of the pool), and seats trade policies every game, so each
	// policy plays each deck-list the same number of times -- the deck can
	// no more masquerade as a policy advantage than seat order can. The pool
	// is the full deck list for a constructed run and the commander-only
	// list for a commander run (a Commander game must seat commander decks;
	// dealing a 100-card Commander list as a constructed pile is exactly the
	// defect this command fixes). Both lists stay sorted, so the assignment
	// is identical on every machine.
	var names []string
	if commander {
		names, err = commanderDeckNames()
		if err != nil {
			return err
		}
	} else {
		names = testutil.RepoDeckNames()
	}
	if seats > len(names) {
		return fmt.Errorf("-seats %d exceeds the %d %s decks available", seats, len(names), deckKind(commander))
	}
	// The deck a seat holds is the pool's (s+rotate)%seats-th entry, so
	// rotate=0 is the historical assignment and the header, the Config and
	// the loaded lists all describe the same seated order. The deck list
	// stays sorted (names is sorted); only which seat holds which entry
	// changes.
	seated := seatedDeckNames(names[:seats], rotate)
	decks := make([][]*cards.Card, seats)
	commanders := make([][]int, seats) // commander mode only
	for s := 0; s < seats; s++ {
		d, err := testutil.LoadRepoDeck(reg, seated[s])
		if err != nil {
			return err
		}
		decks[s] = d
		if commander {
			ci, err := commanderIndex(reg, seated[s])
			if err != nil {
				return err
			}
			commanders[s] = []int{ci}
		}
	}

	hdr := fmt.Sprintf("bot bench: base seed %d, %s vs %s, %d games, %d seats, decks %s",
		baseSeed, aName, bName, games, seats, strings.Join(seated, ","))
	if commander {
		// The header names the format so a Commander number cannot be quoted
		// as a constructed baseline -- the mix-up this task exists to end.
		hdr += " (commander format)"
	}
	fmt.Fprintln(out, hdr)

	// collect is the -decision-stats histogram for this run; nil when the
	// flag is off, so a default run records nothing and appends nothing.
	var collect *decisionStats
	if decisionStatsEnabled {
		collect = newDecisionStats()
	}

	play := func(s uint64, pols []string) (gameOutcome, error) {
		botSeats := make([]seat.Seat, seats)
		for seat := 0; seat < seats; seat++ {
			// One bot per seat, each seeded from the game's seed the same
			// way host/defaultSeats does (seed ^ seat+1) so a policy's RNG
			// is distinct from the engine's and from every other seat's.
			botSeats[seat] = policies[pols[seat]](s ^ uint64(seat+1))
		}
		cfg := buildGameConfig(s, seated, decks, commanders, commander)
		cfg.Tokens = reg.Tokens
		return playMatch(cfg, pols, botSeats, maxTurns, maxIntents, collect)
	}
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	pool := newGamePool(workers)
	defer pool.close()
	if err := benchWithPool(baseSeed, games, seats, aName, bName, play, out, pool); err != nil {
		return err
	}
	collect.write(out)
	return nil
}

// buildGameConfig turns one game's seat assignment into its rules.Config,
// applying commander mode's life and command-zone settings. Extracted from
// run/runMatrix so the commander Config is unit-testable without paying an
// engine game: commander mode sets FormatCommander, exactly 40 starting life
// (CR 903.6) and each seat's command-zone commander index, matching how
// cmd/gorged's deckLoader seats a commander table.
func buildGameConfig(seed uint64, names []string, decks [][]*cards.Card, commanders [][]int, commander bool) rules.Config {
	cfg := rules.Config{Seed: seed, Names: names, Decks: decks}
	if commander {
		cfg.Format = rules.FormatCommander
		cfg.StartingLife = 40
		cfg.Commanders = commanders
	}
	return cfg
}

// deckKind names the deck pool a run selects, for an error message that
// says why a -seats count is too high.
func deckKind(commander bool) string {
	if commander {
		return "commander"
	}
	return "repo"
}

func main() {
	a := flag.String("a", "bot", "policy name on side A")
	b := flag.String("b", "bot", "policy name on side B (same as -a is a valid, expected run)")
	games := flag.Int("games", 100, "number of games to play (matrix mode: per pair)")
	seed := flag.Uint64("seed", 0, "base seed; game i plays at seed+i")
	seats := flag.Int("seats", 2, "number of seats")
	rotate := flag.Int("rotate", 0, "rotate the seat-to-deck assignment by N positions (seat s holds the (s+N)%%seats-th deck); 0 is today's fixed assignment")
	pairs := flag.String("pairs", "", "deck-pair matrix: \"all\" for every unordered repo-deck pair, or a comma-separated \"a:b,c:d\" list; empty keeps today's single-pair behaviour")
	format := flag.String("format", "constructed", "construction format: constructed or commander (commander deals commander decks into the command zone, starts at 40 life, and plays for -max-turns before a game is recorded as a stall)")
	out := flag.String("out", "text", "matrix output format: text or json (json is machine-readable for diffing runs)")
	workers := flag.Int("workers", 0, "parallelism budget for bench games, single-pair and matrix, across pairs and games; 0 = use all cores (result is deterministic regardless)")
	// The two watchdog caps catch different pathologies and must not be
	// conflated: -max-turns ends a game that runs long in turn count (a cap
	// a frozen-turn loop never reaches, because its turn number stops);
	// -max-intents ends a game whose turn count freezes but that keeps
	// submitting intents forever. Both end the game as a stall -- not a win,
	// not a draw, excluded from the win-rate denominator. Healthy constructed
	// games measure 171-1136 intents, so the 20000 default is ~17x the worst
	// of those with room for Commander's longer games.
	maxTurns := flag.Int("max-turns", 200, "maximum turns per game before it ends as a stall (not a win, not a draw); catches a game that runs long in turn count; 0 = no cap")
	maxIntents := flag.Int("max-intents", 20000, "maximum intents per game before it ends as a stall (not a win, not a draw); catches a game whose turn count never advances but that keeps submitting intents; 0 = no cap")
	dir := flag.String("dir", ".cards", "corpus directory (holds ir.gob.gz / cardsfolder)")
	decisionStats := flag.Bool("decision-stats", false, "append a per-decision-kind histogram (count, mean per game, mean option count, singleton share, first-option share) at the end of a run; default off so the normal report is unchanged")
	flag.Parse()
	decisionStatsEnabled = *decisionStats

	commander, err := parseGameFormat(*format)
	if err != nil {
		fmt.Fprintln(os.Stderr, "botbench:", err)
		os.Exit(1)
	}

	// The deck pool is the format's: a Commander run seats only commander
	// decks, so -pairs all and -seats both expand over that pool rather than
	// dealing a 100-card Commander list as a constructed pile.
	var deckPool []string
	if commander {
		deckPool, err = commanderDeckNames()
		if err != nil {
			fmt.Fprintln(os.Stderr, "botbench:", err)
			os.Exit(1)
		}
	} else {
		deckPool = testutil.RepoDeckNames()
	}

	if *pairs != "" {
		if *rotate != 0 {
			fmt.Fprintln(os.Stderr, "botbench: -rotate applies to the single-run bench only, not -pairs (a 2-seat pair already plays both seatings)")
			os.Exit(1)
		}
		ps, err := parsePairsForMode(*pairs, deckPool, commander)
		if err != nil {
			fmt.Fprintln(os.Stderr, "botbench:", err)
			os.Exit(1)
		}
		if err := runMatrix(*seed, *games, *seats, *a, *b, *dir, *out, ps, *workers, *maxTurns, *maxIntents, commander, os.Stdout, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, "botbench:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(*seed, *games, *seats, *rotate, *workers, *a, *b, *dir, *maxTurns, *maxIntents, commander, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "botbench:", err)
		os.Exit(1)
	}
}
