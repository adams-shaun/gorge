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
// `-a bot -b bot` is the meaningful run today: seat.NewBot is the only
// registered policy, so pitting it against itself measures seat bias and
// gives every later policy a baseline to beat. Registering a second policy
// is one line in the policies map. Same names on both sides is a valid and
// expected run.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/cards"
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
// of how many seats the run uses.
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
// end), and how many intents the seats answered. The seat is part of the
// result, not decoration: when both sides run the same policy (the
// baseline `-a bot -b bot`), the A/B split is degenerate by construction
// and the per-game winner seat is the only number that exposes a seat
// advantage (first turn, deck list) -- which is what the baseline run
// exists to measure. Intents and turns are the two numbers every later bot
// task reads: an intent count near maxIntents is a policy that stopped
// terminating, and mean turns is the metric the report averages.
//
// With aName == bName every non-draw winner maps to both names, so the
// seat is what distinguishes the games; with distinct names it is context
// on top of the policy split. winnerSeat is only meaningful when winner
// is non-empty.
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
// baseline same-policy run can show a seat advantage at all -- with both
// policies named "bot" the raw name alone would attribute every non-draw
// game to both sides and reveal nothing.
func winnerLabel(o gameOutcome) string {
	if o.winner == "" {
		return "draw"
	}
	return fmt.Sprintf("%s@%d", o.winner, o.winnerSeat)
}

// run is main's testable body: it plays `games` matches between the named
// policies and writes the per-game lines and the summary to out. Every
// match is seeded from base+i (gameSeed) and the seat assignment alternates
// (aPlaysSeat), so the output is byte-identical across runs with the same
// (base, policies, games). Tests drive this body directly through a
// *bytes.Buffer exactly like cmd/mtgsim's run.
//
// run never touches a *state.Game or a *rules.Engine field: it drives
// engines the way hosts and sims do -- view.Project, Seat.Decide,
// Engine.Submit -- so a bench run exercises the same seat-facing path a
// real match does, and the engine and seat packages stay untouched by this
// command.
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

	var aWins, bWins, draws int
	var totalTurns int64
	for g := 0; g < games; g++ {
		s := gameSeed(baseSeed, g)
		pols := make([]string, seats)
		botSeats := make([]seat.Seat, seats)
		for seat := 0; seat < seats; seat++ {
			pols[seat] = bName
			if aPlaysSeat(g, seat) {
				pols[seat] = aName
			}
			// One bot per seat, each seeded from the game's seed the same
			// way host/defaultSeats does (seed ^ seat+1) so a policy's RNG
			// is distinct from the engine's and from every other seat's.
			botSeats[seat] = policies[pols[seat]](s ^ uint64(seat+1))
		}
		cfg := rules.Config{Seed: s, Names: names[:seats], Decks: decks, Tokens: reg.Tokens}
		oc, err := playMatch(cfg, pols, botSeats)
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
		switch oc.winner {
		case aName:
			aWins++
		case bName:
			bWins++
		default:
			draws++
		}
		totalTurns += int64(oc.turns)
	}

	lo, hi := ci95(aWins, games)
	rate := float64(aWins) / float64(games)
	fmt.Fprintf(out, "\ngames played: %d\n", games)
	fmt.Fprintf(out, "A wins: %d  B wins: %d  draws: %d\n", aWins, bWins, draws)
	fmt.Fprintf(out, "A win rate: %.1f%%  95%% CI [%.1f%%, %.1f%%] (normal approximation to the binomial)\n",
		rate*100, lo*100, hi*100)
	fmt.Fprintf(out, "mean turns per game: %.1f\n", float64(totalTurns)/float64(games))
	return nil
}

func main() {
	a := flag.String("a", "bot", "policy name on side A")
	b := flag.String("b", "bot", "policy name on side B (same as -a is a valid, expected run)")
	games := flag.Int("games", 100, "number of games to play")
	seed := flag.Uint64("seed", 0, "base seed; game i plays at seed+i")
	seats := flag.Int("seats", 2, "number of seats")
	dir := flag.String("dir", ".cards", "corpus directory (holds ir.gob.gz / cardsfolder)")
	flag.Parse()

	if err := run(*seed, *games, *seats, *a, *b, *dir, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "botbench:", err)
		os.Exit(1)
	}
}
