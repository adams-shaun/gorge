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
// (252/248 at N=500) shows how big a seat/play-order artifact is before
// any real comparison is read. The head-to-head that credits a policy:
// -a bot -b legacy, where legacy is the pre-B2 fuzz-driver combat frozen
// in botpolicy.LegacyDecide. Registering a third policy is one entry in
// the policies map. Same names on both sides is a valid and expected run.
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
//     exact -format json diff between two builds.
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

// maxIntents bounds a single game the same way rules/acceptance_test.go and
// cmd/mtgsim do: a game that has not produced a decision inside this many
// intents is not "slow", it is not terminating, and the bench reports that
// as an error rather than spinning forever. The same budget the fuzz gate
// and the sim run under, so a policy that stops making progress is caught
// here exactly where it would have broken those.
const maxIntents = 400000

// gameSeed returns the seed game i of a run at base seed b plays. The
// engine's log and rng and every seat bot's PCG all derive from this one
// number, seeded base+i -- so the run as a whole is reproducible from
// (base, policies, N) and the first k games of any run are exactly the run
// at the same base with N=k. The "same game N times" failure mode the bench
// exists to rule out is literally this function returning a constant.
func gameSeed(base uint64, game int) uint64 { return base + uint64(game) }

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
// are the two numbers every later bot task reads: an intent count near
// maxIntents is a policy that stopped terminating, and mean turns is the
// metric the report averages. winnerSeat is only meaningful when winner is
// non-empty.
type gameOutcome struct {
	winner     string // policy name of the winning seat; "" for a draw
	winnerSeat int    // the seat the winner sat in (valid when winner != "")
	turns      int32
	intents    int
}

// playMatch plays one game between the given per-seat seats to completion
// (or to maxIntents) and returns its outcome. pols is the policy name
// sitting at each seat, used only to map the winner's seat back to a
// policy. The seats each own their RNG (seeded by the caller), the engine
// replays from its own Config.Seed, and nothing reads the wall clock, so
// the outcome is a pure function of the inputs.
func playMatch(cfg rules.Config, pols []string, seats []seat.Seat) (gameOutcome, error) {
	e := rules.New(cfg)
	e.Advance()
	n := 0
	for !e.G.Over && e.Pending() != nil && n < maxIntents {
		d := e.Pending()
		v := view.Project(e.G, e, d.Player, d)
		in, err := seats[d.Player].Decide(context.Background(), v, *d)
		if err != nil {
			return gameOutcome{}, fmt.Errorf("seed %d, intent %d, seat %d: %w", cfg.Seed, n, d.Player, err)
		}
		if err := e.Submit(in); err != nil {
			return gameOutcome{}, fmt.Errorf("seed %d, intent %d: %w", cfg.Seed, n, err)
		}
		n++
	}
	if !e.G.Over {
		return gameOutcome{}, fmt.Errorf("seed %d: did not terminate within %d intents (turn %d)", cfg.Seed, maxIntents, e.G.Turn)
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
	if o.winner == "" {
		return "draw"
	}
	return fmt.Sprintf("%s@%d", o.winner, o.winnerSeat)
}

// matchPlayer plays one game of a bench run: given a game's seed and the
// policy name sitting at each seat, it returns that game's outcome. run
// wires the real engine into this shape (playMatch with per-seat bots);
// tests inject synthetic players to pin the attribution and the report
// without paying an engine game each.
type matchPlayer func(seed uint64, pols []string) (gameOutcome, error)

// bench is the testable body of the bench: it plays `games` matches between
// the named policies, crediting each win to the side that held the winning
// seat, and writes the per-game lines and the summary to out. Every match
// is seeded from base+game (gameSeed) and the seat assignment alternates
// (aPlaysSeat), so the output is byte-identical across runs with the same
// (base, policies, games). run() resolves the corpus and decks and wires
// the real engine in; the tests drive bench directly with a synthetic
// matchPlayer, exactly like the tests of cmd/mtgsim drive its run.
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
	if games < 1 {
		return fmt.Errorf("-games must be at least 1, got %d", games)
	}
	if seats < 2 {
		return fmt.Errorf("-seats must be at least 2 (a bench pits two policies), got %d", seats)
	}

	var aWins, bWins, draws int
	seatWins := make([]int, seats)
	var totalTurns int64
	for g := 0; g < games; g++ {
		s := gameSeed(baseSeed, g)
		pols := make([]string, seats)
		for seat := 0; seat < seats; seat++ {
			pols[seat] = bName
			if aPlaysSeat(g, seat) {
				pols[seat] = aName
			}
		}
		oc, err := play(s, pols)
		if err != nil {
			// playMatch's error already names the seed; the game index is
			// the only context this frame can add.
			fmt.Fprintf(out, "game %d: %v\n", g, err)
			return err
		}
		// The per-game line carries the seed next to the result, so a run
		// that ever played the same game twice would show it -- the seed
		// column stops grinding forward. That is the symptom the bench
		// exists to make impossible, and it is visible in the report if it
		// ever regresses.
		fmt.Fprintf(out, "game %d: seed %d, %s, %6d intents, %3d turns, winner=%s\n",
			g, s, seatLabels(pols), oc.intents, oc.turns, winnerLabel(oc))
		switch {
		case oc.winner == "":
			draws++
		default:
			// A real win: credit the side that held the winning seat this
			// game. aPlaysSeat is the same predicate that assigned the
			// seats, so attribution agrees with the assignment by
			// construction.
			if oc.winnerSeat < 0 || oc.winnerSeat >= seats {
				return fmt.Errorf("game %d: winner seat %d out of range [0,%d)", g, oc.winnerSeat, seats)
			}
			seatWins[oc.winnerSeat]++
			if aPlaysSeat(g, oc.winnerSeat) {
				aWins++
			} else {
				bWins++
			}
		}
		totalTurns += int64(oc.turns)
	}

	lo, hi := ci95(aWins, games)
	rate := float64(aWins) / float64(games)
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
	fmt.Fprintf(out, "A win rate: %.1f%%  95%% CI [%.1f%%, %.1f%%] (normal approximation to the binomial)\n",
		rate*100, lo*100, hi*100)
	if seats == 2 {
		// The two-seat baseline: seat 0's rate and interval over the whole
		// run (drawn games win no seat, so the per-seat rates sum to less
		// than 100% when draws occur; the game count is printed beside the
		// interval as always). This is the number the -a -b baseline run
		// exists to produce.
		sLo, sHi := ci95(seatWins[0], games)
		fmt.Fprintf(out, "seat 0 wins: %d  seat 1 wins: %d  seat 0 win rate: %.1f%%  95%% CI [%.1f%%, %.1f%%] (normal approximation to the binomial)\n",
			seatWins[0], seatWins[1], float64(seatWins[0])/float64(games)*100, sLo*100, sHi*100)
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

// parsePairs parses a -pairs spec: "all" for every unordered pair of the
// given (sorted) deck names, or a comma-separated list of colon-separated
// deck pairs "a:b,c:d". Deck names must exist in names. The lookup map is
// only probed by key -- never ranged over in the report path -- so iteration
// order cannot reach the output.
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
			return nil, fmt.Errorf("-pairs: unknown deck %q (not one of the %d repo decks)", a, len(names))
		}
		if !known[b] {
			return nil, fmt.Errorf("-pairs: unknown deck %q (not one of the %d repo decks)", b, len(names))
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
// directly comparable to the historical single-pair number.
type pairResult struct {
	pd         pairDef
	games      int
	aWins      int
	bWins      int
	draws      int
	seatWins   [2]int
	totalTurns int64
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

// playOnePair plays `games` matches of one deck pair and tallies them into a
// pairResult. Seats trade policies every game (aPlaysSeat), exactly as the
// single-pair bench does, so the deck a seat holds is fixed for the pair but
// which policy plays it alternates -- a deck list can no more masquerade as
// a policy advantage here than it can in the single-pair run. Every game is
// seeded from gameSeedPair so the whole matrix is a pure function of (base
// seed, per-pair game count, pair list) and nothing else.
func playOnePair(baseSeed uint64, pos, games int, aName, bName string, pd pairDef, play pairPlayer, prog *progressWriter) (pairResult, error) {
	var r pairResult
	r.pd = pd
	r.games = games
	step := games
	if step > 1000 {
		step = 1000
	}
	for g := 0; g < games; g++ {
		s := gameSeedPair(baseSeed, pos, games, g)
		pols := make([]string, 2)
		for seat := 0; seat < 2; seat++ {
			pols[seat] = bName
			if aPlaysSeat(g, seat) {
				pols[seat] = aName
			}
		}
		oc, err := play(pos, s, pols)
		if err != nil {
			// playMatch's error already names the seed; the pair is the
			// context this frame adds. An intent-cap overrun therefore
			// surfaces loudly with both the pair and the seed, which is
			// the liveness signal a matrix run is supposed to catch.
			return pairResult{}, fmt.Errorf("pair %s: %w", pd, err)
		}
		switch {
		case oc.winner == "":
			r.draws++
		default:
			if oc.winnerSeat < 0 || oc.winnerSeat > 1 {
				return pairResult{}, fmt.Errorf("pair %s, game %d: winner seat %d out of range [0,2)", pd, g, oc.winnerSeat)
			}
			r.seatWins[oc.winnerSeat]++
			if aPlaysSeat(g, oc.winnerSeat) {
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
	if workers > total {
		workers = total
	}
	results := make([]pairResult, total)
	if prog == nil {
		prog = &progressWriter{}
	}

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

	wg.Add(workers)
	for w := 0; w < workers; w++ {
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
				r, err := playOnePair(baseSeed, pos, games, aName, bName, pairs[pos], play, prog)
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
	pairs      int
	games      int
	aWins      int
	bWins      int
	draws      int
	seatWins   [2]int
	totalTurns int64
}

func mergeResults(results []pairResult) mergedResult {
	var m mergedResult
	m.pairs = len(results)
	for _, r := range results {
		m.games += r.games
		m.aWins += r.aWins
		m.bWins += r.bWins
		m.draws += r.draws
		m.seatWins[0] += r.seatWins[0]
		m.seatWins[1] += r.seatWins[1]
		m.totalTurns += r.totalTurns
	}
	return m
}

// excludeCounts classifies the per-pair intervals against 50%: a pair whose
// whole A-win interval sits below 50% is a pair where policy A loses; above
// is a pair where A wins; the rest are undecided. This is the honest summary
// a single pooled percentage hides -- one that reads "beats B on 41 of 66,
// loses on 3, undecided on 22" says more than "53% overall".
func excludeCounts(results []pairResult) (below, above, undecided int) {
	for _, r := range results {
		lo, hi := ci95(r.aWins, r.games)
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
func writeMatrixText(out io.Writer, aName, bName string, baseSeed uint64, games int, results []pairResult) error {
	m := mergeResults(results)
	fmt.Fprintf(out, "bot bench matrix: base seed %d, %s vs %s, %d deck pairs, %d games PER PAIR (total %d games)\n",
		baseSeed, aName, bName, len(results), games, m.games)
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "deck pair\tA wins\tB wins\tdraws\tA win rate (95% CI)\tseat 0 rate (95% CI)\tmean turns")
	for _, r := range results {
		lo, hi := ci95(r.aWins, r.games)
		sLo, sHi := ci95(r.seatWins[0], r.games)
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%.1f%% [%.1f%%, %.1f%%]\t%.1f%% [%.1f%%, %.1f%%]\t%.1f\n",
			r.pd, r.aWins, r.bWins, r.draws,
			float64(r.aWins)/float64(r.games)*100, lo*100, hi*100,
			float64(r.seatWins[0])/float64(r.games)*100, sLo*100, sHi*100,
			float64(r.totalTurns)/float64(r.games))
	}
	tw.Flush()

	blo, bhi := ci95(m.aWins, m.games)
	sLo, sHi := ci95(m.seatWins[0], m.games)
	below, above, undecided := excludeCounts(results)
	fmt.Fprintf(out, "\ngames per pair: %d  (total %d games across %d pairs)\n", games, m.games, m.pairs)
	fmt.Fprintf(out, "pooled across %d pairs: %s wins %d, B wins %d, draws %d\n", m.pairs, aName, m.aWins, m.bWins, m.draws)
	fmt.Fprintf(out, "pooled %s win rate: %.1f%%  95%% CI [%.1f%%, %.1f%%] (normal approximation to the binomial, pooled over counts)\n",
		aName, float64(m.aWins)/float64(m.games)*100, blo*100, bhi*100)
	fmt.Fprintf(out, "pooled seat 0 win rate: %.1f%%  95%% CI [%.1f%%, %.1f%%] (normal approximation to the binomial)\n",
		float64(m.seatWins[0])/float64(m.games)*100, sLo*100, sHi*100)
	fmt.Fprintf(out, "mean turns per game (pooled): %.1f\n", float64(m.totalTurns)/float64(m.games))
	fmt.Fprintf(out, "pairs whose A-win interval excludes 50%%: A loses on %d, A wins on %d, undecided on %d\n", below, above, undecided)
	return nil
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
	GamesPerPair int        `json:"games_per_pair"`
	Pairs        []jsonPair `json:"pairs"`
	Pooled       jsonPooled `json:"pooled"`
}

func writeMatrixJSON(out io.Writer, aName, bName string, baseSeed uint64, games int, results []pairResult) error {
	m := mergeResults(results)
	doc := jsonDoc{PolicyA: aName, PolicyB: bName, BaseSeed: baseSeed, GamesPerPair: games}
	for _, r := range results {
		lo, hi := ci95(r.aWins, r.games)
		sLo, sHi := ci95(r.seatWins[0], r.games)
		doc.Pairs = append(doc.Pairs, jsonPair{
			Pair:       r.pd.String(),
			AWins:      r.aWins,
			BWins:      r.bWins,
			Draws:      r.draws,
			AWinRate:   float64(r.aWins) / float64(r.games),
			AWinRateCI: []float64{lo, hi},
			Seat0Wins:  r.seatWins[0],
			Seat1Wins:  r.seatWins[1],
			Seat0Rate:  float64(r.seatWins[0]) / float64(r.games),
			Seat0CI:    []float64{sLo, sHi},
			MeanTurns:  float64(r.totalTurns) / float64(r.games),
		})
	}
	blo, bhi := ci95(m.aWins, m.games)
	sLo, sHi := ci95(m.seatWins[0], m.games)
	below, above, undecided := excludeCounts(results)
	doc.Pooled = jsonPooled{
		Pairs:        m.pairs,
		Games:        m.games,
		GamesPerPair: games,
		AWins:        m.aWins,
		BWins:        m.bWins,
		Draws:        m.draws,
		AWinRate:     float64(m.aWins) / float64(m.games),
		AWinRateCI:   []float64{blo, bhi},
		Seat0Rate:    float64(m.seatWins[0]) / float64(m.games),
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

// runMatrix is the matrix mode's entry through the real engine: it opens the
// corpus, resolves every distinct deck the pairs name, plays each pair for
// `games` matches (parallelised across pairs by runPairs), and prints either
// the text table+pooled line or the JSON document. It shares playMatch, the
// policies table and ci95 with run(), so the seat-trades-policies and
// seed-determinism properties are the same two seats a single-pair run has.
func runMatrix(baseSeed uint64, games, seats int, aName, bName, dir, format string, pairs []pairDef, workers int, out, prog io.Writer) error {
	if seats != 2 {
		return fmt.Errorf("-pairs requires -seats 2 (a matrix pits one deck pair against another), got %d", seats)
	}
	if games < 1 {
		return fmt.Errorf("-games must be at least 1 per pair, got %d", games)
	}
	if format != "text" && format != "json" {
		return fmt.Errorf("-format must be \"text\" or \"json\", got %q", format)
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
	// cannot reach the output.
	deckByName := make(map[string][]*cards.Card, len(pairs)*2)
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
		}
	}

	play := func(pos int, seed uint64, pols []string) (gameOutcome, error) {
		pd := pairs[pos]
		botSeats := make([]seat.Seat, 2)
		for seat := 0; seat < 2; seat++ {
			// Same per-seat seed derivation as run(): policy RNG is distinct
			// from the engine's and from every other seat's.
			botSeats[seat] = policies[pols[seat]](seed ^ uint64(seat+1))
		}
		cfg := rules.Config{
			Seed:   seed,
			Names:  []string{pd.a, pd.b},
			Decks:  [][]*cards.Card{deckByName[pd.a], deckByName[pd.b]},
			Tokens: reg.Tokens,
		}
		return playMatch(cfg, pols, botSeats)
	}

	results, err := runPairs(baseSeed, games, aName, bName, pairs, play, workers, &progressWriter{w: prog})
	if err != nil {
		return err
	}
	if format == "json" {
		return writeMatrixJSON(out, aName, bName, baseSeed, games, results)
	}
	return writeMatrixText(out, aName, bName, baseSeed, games, results)
}

// run is main's entry through the real engine: it validates the flags,
// opens the corpus and the per-seat repo decks, and plays `games` matches
// between the named policies via bench. It exists so the whole flag-driven
// path stays open to tests that need it (determinism, end-to-end); the loop
// and the report live in bench, which tests can also drive directly with a
// synthetic matchPlayer.
func run(baseSeed uint64, games, seats int, aName, bName, dir string, out io.Writer) error {
	if games < 1 {
		return fmt.Errorf("-games must be at least 1, got %d", games)
	}
	if seats < 2 {
		return fmt.Errorf("-seats must be at least 2 (a bench pits two policies), got %d", seats)
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
	// first repo deck), and seats trade policies every game, so each policy
	// plays each deck-list the same number of times -- the deck can no more
	// masquerade as a policy advantage than seat order can. RepoDeckNames is
	// sorted, so the assignment is identical on every machine.
	names := testutil.RepoDeckNames()
	if seats > len(names) {
		return fmt.Errorf("-seats %d exceeds the %d repo decks available", seats, len(names))
	}
	decks := make([][]*cards.Card, seats)
	for i := 0; i < seats; i++ {
		d, err := testutil.LoadRepoDeck(reg, names[i])
		if err != nil {
			return err
		}
		decks[i] = d
	}

	fmt.Fprintf(out, "bot bench: base seed %d, %s vs %s, %d games, %d seats, decks %s\n",
		baseSeed, aName, bName, games, seats, strings.Join(names[:seats], ","))

	play := func(s uint64, pols []string) (gameOutcome, error) {
		botSeats := make([]seat.Seat, seats)
		for seat := 0; seat < seats; seat++ {
			// One bot per seat, each seeded from the game's seed the same
			// way host/defaultSeats does (seed ^ seat+1) so a policy's RNG
			// is distinct from the engine's and from every other seat's.
			botSeats[seat] = policies[pols[seat]](s ^ uint64(seat+1))
		}
		cfg := rules.Config{Seed: s, Names: names[:seats], Decks: decks, Tokens: reg.Tokens}
		return playMatch(cfg, pols, botSeats)
	}
	return bench(baseSeed, games, seats, aName, bName, play, out)
}

func main() {
	a := flag.String("a", "bot", "policy name on side A")
	b := flag.String("b", "bot", "policy name on side B (same as -a is a valid, expected run)")
	games := flag.Int("games", 100, "number of games to play (matrix mode: per pair)")
	seed := flag.Uint64("seed", 0, "base seed; game i plays at seed+i")
	seats := flag.Int("seats", 2, "number of seats")
	pairs := flag.String("pairs", "", "deck-pair matrix: \"all\" for every unordered repo-deck pair, or a comma-separated \"a:b,c:d\" list; empty keeps today's single-pair behaviour")
	format := flag.String("format", "text", "matrix output format: text or json (json is machine-readable for diffing runs)")
	workers := flag.Int("workers", 0, "matrix parallelism across pairs; 0 = use all cores (result is deterministic regardless)")
	dir := flag.String("dir", ".cards", "corpus directory (holds ir.gob.gz / cardsfolder)")
	flag.Parse()

	if *pairs != "" {
		ps, err := parsePairs(*pairs, testutil.RepoDeckNames())
		if err != nil {
			fmt.Fprintln(os.Stderr, "botbench:", err)
			os.Exit(1)
		}
		if err := runMatrix(*seed, *games, *seats, *a, *b, *dir, *format, ps, *workers, os.Stdout, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, "botbench:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(*seed, *games, *seats, *a, *b, *dir, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "botbench:", err)
		os.Exit(1)
	}
}
