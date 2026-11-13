// Command mtgsim plays headless games between heuristic.Bot instances over the
// repo's 12 Legacy decks and verifies that each one replays to the same
// event chain: a way to run many real games with no client, no network
// and no wall clock in the loop.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/adams-shaun/gorge/bots"
	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/rules"
	engineseat "github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/matchrecord"
	sbenv "github.com/adams-shaun/gorge/server/spellbench/env"
	sbseat "github.com/adams-shaun/gorge/server/spellbench/seat"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// maxIntents bounds a single game the same way engine/rules/acceptance_test.go's
// own acceptance gate does: a game that has not produced a decision inside
// this many intents is not "slow", it is not terminating, and mtgsim reports
// that as a failure rather than spinning forever.
const maxIntents = 400000

// sparePool recycles finished games' storage (rules.Spare) between the games
// -games runs back to back; reuse never changes a game
// (TestSpareReuseIsInvisible), so a run's output is byte-identical with or
// without it.
var sparePool bench.SparePool

func main() {
	decksFlag := flag.String("decks", "", "comma-separated deck names (default: the first -seats of the repo decks)")
	seats := flag.Int("seats", 4, "number of seats")
	seed := flag.Uint64("seed", 0, "base seed; game i uses seed+i for both the engine and its bot")
	games := flag.Int("games", 1, "number of games to play")
	verify := flag.Bool("verify", false, "replay each game and compare its event chain against the live run")
	verbose := flag.Bool("v", false, "print the deck assigned to each seat before every game")
	dir := flag.String("dir", ".cards", "corpus directory (holds ir.gob.gz / cardsfolder)")
	policyA := flag.String("a", "", "bot policy name for seat 0 (default: heuristic)")
	policyB := flag.String("b", "", "bot policy name for seat 1 (default: heuristic)")
	format := flag.String("format", "constructed", "game format: constructed or commander (40 life, commanders in the command zone)")
	mulligans := flag.Int("mulligans", 0, "London mulligans each seat may take before turn 1 (default 0, none)")
	shape := flag.String("shape", "", "write a JSON game-shape census to this file (game rows, then one aggregate)")
	cpuProf := flag.String("cpuprofile", "", "write a CPU profile here")
	memProf := flag.String("memprofile", "", "write an allocation profile here and print the run's allocation totals")
	var sbAgents []string
	flag.Func("sb-agent", "register a SpellBench v2 stdio agent: name=/path/to/agent (repeatable)", func(arg string) error {
		sbAgents = append(sbAgents, arg)
		return nil
	})
	flag.Parse()

	o := simOptions{
		Decks: *decksFlag, Seats: *seats, Seed: *seed, Games: *games,
		Verify: *verify, Verbose: *verbose, Dir: *dir,
		PolicyA: *policyA, PolicyB: *policyB, SBAgents: sbAgents, Format: *format, Mulligans: *mulligans, Shape: *shape,
	}
	stop := startProfile(*cpuProf, *memProf)
	err := runSim(o, os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mtgsim:", err)
		os.Exit(1)
	}
}

// run is main's testable body. It returns a non-nil error -- and therefore a
// non-zero exit code -- if any game fails to terminate or, with -verify, to
// replay. Everything runs on the caller's own goroutine, one match at a
// time: no goroutines, so two runs with the same arguments print the same
// lines in the same order every time.
//
// out is io.Writer, not *os.File: main passes os.Stdout, but main_test.go
// passes a *bytes.Buffer so this body is exercised by a test without writing
// to a real file.
func run(decksFlag string, seats int, baseSeed uint64, games int, verify, verbose bool, dir string, out io.Writer) error {
	return runAgents(decksFlag, seats, baseSeed, games, verify, verbose, dir, "", "", nil, out)
}

// simOptions is one mtgsim run's settings, the flags main parses.
type simOptions struct {
	Decks           string
	Seats           int
	Seed            uint64
	Games           int
	Verify, Verbose bool
	Dir             string
	PolicyA         string
	PolicyB         string
	SBAgents        []string
	// Format is "constructed" or "commander".
	Format string
	// Mulligans is the London mulligans each seat may take before turn 1.
	Mulligans int
	// Shape, when non-empty, is the path of the JSON game-shape census.
	Shape string
}

func runAgents(decksFlag string, seats int, baseSeed uint64, games int, verify, verbose bool, dir, policyA, policyB string, sbAgents []string, out io.Writer) error {
	return runSim(simOptions{Decks: decksFlag, Seats: seats, Seed: baseSeed, Games: games, Verify: verify, Verbose: verbose, Dir: dir, PolicyA: policyA, PolicyB: policyB, SBAgents: sbAgents, Format: "constructed"}, out)
}

// runSim plays o.Games games and, with o.Verify, replays each one. A
// commander run seats Commander decks only, checks each against the
// Commander deck rules before any game, and plays at 40 life with every
// seat's commander starting in the command zone.
func runSim(o simOptions, out io.Writer) error {
	decksFlag, seats, baseSeed, games := o.Decks, o.Seats, o.Seed, o.Games
	verify, verbose, dir, policyA, policyB, sbAgents := o.Verify, o.Verbose, o.Dir, o.PolicyA, o.PolicyB, o.SBAgents
	var format rules.Format
	commander := false
	switch o.Format {
	case "constructed":
	case "commander":
		commander = true
		format = rules.FormatCommander
		if seats < 2 {
			return fmt.Errorf("-format commander needs at least 2 seats")
		}
	default:
		return fmt.Errorf("-format %q: want constructed or commander", o.Format)
	}
	if seats < 1 {
		return fmt.Errorf("-seats must be at least 1, got %d", seats)
	}
	if games < 1 {
		return fmt.Errorf("-games must be at least 1, got %d", games)
	}
	if seats < 2 && (policyA != "" || policyB != "") {
		return fmt.Errorf("-a/-b name seats 0 and 1, but -seats is %d", seats)
	}
	if err := registerSBAgents(sbAgents); err != nil {
		return err
	}

	names, err := deckNames(decksFlag, seats, commander)
	if err != nil {
		return err
	}
	var cmds [][]int
	var files []deck.File
	if commander {
		cmds = make([][]int, len(names))
		files = make([]deck.File, len(names))
		for i, name := range names {
			f, err := testutil.LoadRepoDeckFile(name)
			if err != nil {
				return err
			}
			if len(f.CommanderNames()) == 0 {
				return fmt.Errorf("deck %q names no commander (-format commander)", name)
			}
			files[i], cmds[i] = f, f.CommanderIndices()
		}
	}

	reg, err := testutil.OpenCorpusRegistry(dir)
	if err != nil {
		return fmt.Errorf("opening corpus at %s: %w (run `make fetch-cards compile-cards` first)", dir, err)
	}
	sbenv.SetCorpusRef(matchrecord.ForgeRef(dir))

	decks := make([][]*cards.Card, seats)
	for i, name := range names {
		d, err := testutil.LoadRepoDeck(reg, name)
		if err != nil {
			return err
		}
		decks[i] = d
	}
	for i, f := range files {
		if err := f.ValidateCommander(reg); err != nil {
			return fmt.Errorf("deck %q: %w", names[i], err)
		}
	}

	if verbose {
		for i, name := range names {
			fmt.Fprintf(out, "seat %d: %s\n", i, name)
		}
	}

	// shapes, when non-nil, is the run's collector; a nil one adds no
	// per-decision work and leaves stdout byte-identical.
	var shapeFile *os.File
	var shapes *shapeRun
	if o.Shape != "" {
		f, err := os.Create(o.Shape)
		if err != nil {
			return fmt.Errorf("creating -shape file %s: %w", o.Shape, err)
		}
		defer f.Close()
		shapeFile = f
		shapes = &shapeRun{kind: fmt.Sprintf("%s/%d", o.Format, seats)}
	}

	failures := 0
	for g := 0; g < games; g++ {
		gameSeed := baseSeed + uint64(g)
		var sg *shapeGame
		if shapes != nil {
			sg = &shapeGame{}
		}
		if !playOne(out, gameSeed, names, decks, reg, verify, policyA, policyB, format, cmds, o.Mulligans, sg) {
			failures++
		}
		if sg != nil {
			if err := shapes.writeGame(shapeFile, sg); err != nil {
				return fmt.Errorf("writing -shape file %s: %w", o.Shape, err)
			}
			shapes.games = append(shapes.games, sg)
		}
	}
	if shapes != nil {
		if err := shapes.writeAggregate(shapeFile); err != nil {
			return fmt.Errorf("writing -shape aggregate to %s: %w", o.Shape, err)
		}
	}
	if failures > 0 {
		return fmt.Errorf("%d of %d games failed to terminate or verify", failures, games)
	}
	return nil
}

// deckNames resolves the -decks flag against the embedded repo decks: an
// explicit comma-separated list, or (when empty) the first `seats` names
// from testutil.LegacyDeckNames(), which is already sorted -- so the default
// assignment is the same on every machine and every run, never whatever
// order a directory listing happened to produce. The default is the 12
// pinned Legacy constructed decks, not RepoDeckNames(): the directory also
// holds the Commander decks, which -format commander plays and which must
// not silently change what the default -verify pool is. With commander set
// the default is the first `seats` Commander decks instead.
func deckNames(flagVal string, seats int, commander bool) ([]string, error) {
	if flagVal == "" {
		all := testutil.LegacyDeckNames()
		kind := "repo"
		if commander {
			var err error
			if all, err = commanderDeckNames(); err != nil {
				return nil, err
			}
			kind = "Commander repo"
		}
		if seats > len(all) {
			return nil, fmt.Errorf("-seats %d exceeds the %d %s decks available; pass -decks explicitly to repeat one", seats, len(all), kind)
		}
		return append([]string(nil), all[:seats]...), nil
	}
	names := strings.Split(flagVal, ",")
	for i, n := range names {
		names[i] = strings.TrimSpace(n)
	}
	if len(names) != seats {
		return nil, fmt.Errorf("-decks names %d decks, but -seats is %d", len(names), seats)
	}
	return names, nil
}

// commanderDeckNames lists the repo decks whose deck file names a
// commander, in RepoDeckNames' sorted order.
func commanderDeckNames() ([]string, error) {
	var out []string
	for _, name := range testutil.RepoDeckNames() {
		f, err := testutil.LoadRepoDeckFile(name)
		if err != nil {
			return nil, err
		}
		if len(f.CommanderNames()) > 0 {
			out = append(out, name)
		}
	}
	return out, nil
}

// gameConfig is one game's rules.Config. A commander game starts at 40
// life with cmds[i] (deck indices) naming seat i's commanders, which
// genesis puts in the command zone.
func gameConfig(seed uint64, names []string, decks [][]*cards.Card, reg *cards.Registry, format rules.Format, cmds [][]int, mulligans int) rules.Config {
	cfg := rules.Config{
		Seed: seed, Names: append([]string(nil), names...), Decks: decks,
		Tokens: reg.Tokens, NameUniverse: reg.Universe(), Mulligans: mulligans,
	}
	if format == rules.FormatCommander {
		cfg.Format = rules.FormatCommander
		cfg.StartingLife = 40
		cfg.Commanders = cmds
	}
	return cfg
}

// playOne plays a single game to completion (or to maxIntents) and prints
// its one-line summary, then -- if verify is set -- a second line reporting
// the replay outcome. It returns false for anything counted as
// failure: non-termination, a replay error or a chain divergence. shape,
// when non-nil, observes every decision and records the game's final shape.
func playOne(out io.Writer, seed uint64, names []string, decks [][]*cards.Card, reg *cards.Registry, verify bool, policyA, policyB string, format rules.Format, cmds [][]int, mulligans int, shape *shapeGame) (ok bool) {
	cfg := gameConfig(seed, names, decks, reg, format, cmds, mulligans)
	// The default object cap, the same volume bound the shared drive
	// loop arms: a degenerate token storm here ends as a LIVELOCK failure
	// line under a bounded arena instead of growing the process until the
	// box OOMs.
	cfg.LoopGuard = rules.WithDefaultObjectCap(cfg.LoopGuard)
	spare := sparePool.Get()
	cfg.Spare = spare
	e := rules.NewStartingPlayerChoice(cfg)
	var observe func(*rules.Engine, *decision.Decision)
	if shape != nil {
		shape.seed = seed
		shape.decks = append([]string(nil), names...)
		observe = shape.observe
	}
	finish := func(n int, status string) {
		if shape != nil {
			shape.finish(e, n, status)
		}
	}
	defer func() {
		// Only a clean finish recycles: a game that failed or did not
		// terminate has no trustworthy log shape to hand to the next game.
		if ok {
			sparePool.Put(spare, e)
		}
	}()
	// CR 103.1's winner-chooses ask: pose it before the seats attach so the
	// table's own policy answers it, and the seat's game_start sees the
	// choice pending. The drive loop answers the pending decision; no Advance
	// runs the no-host default underneath it.
	e.AskStartingPlayer()

	seats, err := matchSeats(e, reg, seed, names, policyA, policyB, matchFormat(format), mulligans, out)
	if err != nil {
		finish(0, "failed")
		fmt.Fprintf(out, "seed %d: %d seats -- FAILED: %v\n", seed, len(names), err)
		return false
	}
	defer func() {
		result := bots.MatchResult{Classification: "halted", Reason: "not_finished"}
		if e.G.Over {
			// A finished game has no halt reason: leaving "not_finished"
			// here made every natural game_over say it did not finish.
			result.Classification, result.Reason = "natural", ""
			result.Outcome = "draw"
			if !e.G.Draw {
				result.Outcome = fmt.Sprintf("p%d_win", e.G.Winner)
				result.Winner = fmt.Sprintf("p%d", e.G.Winner)
			}
		}
		for _, s := range seats {
			bots.EndMatch(s, result)
		}
	}()

	n, err := driveMatch(e, seats, observe)
	if err != nil {
		status := "failed"
		if _, ok := err.(*rules.LivelockError); ok {
			status = "livelock"
		}
		finish(n, status)
		reportDriveFailure(out, seed, len(names), n, err)
		return false
	}

	if !e.G.Over {
		finish(n, "did_not_terminate")
		fmt.Fprintf(out, "seed %d: %d seats, %6d intents, %6d events, %3d turns -- DID NOT TERMINATE\n",
			seed, len(names), n, len(e.L.Events), e.G.Turn)
		return false
	}

	// Draw before Winner -- Winner's zero value is a real seat
	// (0), so reading it unconditionally would misreport a drawn game.
	result := "draw"
	if !e.G.Draw {
		result = e.G.Players[e.G.Winner].Name
	}
	fmt.Fprintf(out, "seed %d: %d seats, %6d intents, %6d events, %3d turns, winner=%s, chain=%s\n",
		seed, len(names), n, len(e.L.Events), e.G.Turn, result, e.L.Head())
	finish(n, "over")

	if !verify {
		return true
	}
	re, err := replay.Replay(e.L, cfg)
	head := ""
	if re != nil {
		head = re.L.Head()
	}
	return printReplayOutcome(out, err, head)
}

// reportDriveFailure prints playOne's failure line for a game the drive
// loop ended early: the livelock diagnostic when the engine's watcher fired
// (the LIVELOCK marker makes the failure greppable and unmistakable next to
// the DID NOT TERMINATE / winner lines), or the plain error otherwise.
// Returns false, playOne's failure signal.
func reportDriveFailure(out io.Writer, seed uint64, seats, n int, err error) bool {
	if lle, ok := err.(*rules.LivelockError); ok {
		fmt.Fprintf(out, "seed %d: %d seats, %6d intents -- LIVELOCK: %v\n", seed, seats, n, lle)
	} else {
		fmt.Fprintf(out, "seed %d: %d seats, %6d intents -- FAILED: %v\n", seed, seats, n, err)
	}
	return false
}

// driveGame submits bot intents until the game ends, a seat errors, an
// intent is rejected -- or the engine's livelock watcher (engine/rules/livelock.go)
// aborts the game from inside Advance/Submit. That last shape cannot be a
// returned error from Submit: the loop is stuck INSIDE one Submit call, so
// the watcher panics with a *rules.LivelockError and this recovers it,
// converts it to a plain return, and leaves every other panic -- a real bug
// either way -- crashing loudly. The engine is abandoned once it has
// livelocked; nothing may drive it again.
func driveGame(e *rules.Engine, s engineseat.Seat) (int, error) {
	return driveMatch(e, []engineseat.Seat{s}, nil)
}

// driveMatch submits intents for a table until the game ends. observe, when
// non-nil, is called with every decision before the seat is asked; it only
// reads the engine.
func driveMatch(e *rules.Engine, seats []engineseat.Seat, observe func(*rules.Engine, *decision.Decision)) (n int, err error) {
	defer func() {
		if r := recover(); r != nil {
			if lle, ok := r.(*rules.LivelockError); ok {
				err = lle
				return
			}
			panic(r)
		}
	}()
	ctx := context.Background()
	for !e.G.Over && e.Pending() != nil && n < maxIntents {
		d := e.Pending()
		if int(d.Player) >= len(seats) {
			return n, fmt.Errorf("decision for seat %d, but only %d seats", d.Player, len(seats))
		}
		if observe != nil {
			observe(e, d)
		}
		// A consumer seat (the SpellBench v2 agent) sees the planner's cast
		// candidates only through Decision.PaymentActions; build them before
		// the projection copies the decision.
		if consumer, ok := seats[d.Player].(engineseat.PaymentPlanConsumer); ok && consumer.WantsPaymentActions() {
			e.EnsurePaymentActions()
		}
		v := view.Project(e.G, e, d.Player, d)
		v.Round = view.RoundOf(e.G, e.L.Events)
		in, err := seats[d.Player].Decide(ctx, v, *d)
		if err != nil {
			return n, fmt.Errorf("bot error at intent %d: %w", n, err)
		}
		if err := e.Submit(in); err != nil {
			return n, fmt.Errorf("intent %d rejected: %w", n, err)
		}
		n++
	}
	return n, nil
}

// printReplayOutcome prints the -verify line for one game -- "replay OK"
// with its chain head, or the divergence: Seq, both Kinds, and Missing
// honoured (no zero events.Event printed when the recorded log simply ended
// first). Factored out of playOne so it can be driven directly from a hand-built
// *replay.Divergence in a test, without needing to corrupt a real event log
// to force one out of replay.Replay.
func printReplayOutcome(out io.Writer, err error, chainHead string) bool {
	if err == nil {
		fmt.Fprintf(out, "  replay OK (chain %s)\n", chainHead)
		return true
	}
	var div *replay.Divergence
	if errors.As(err, &div) {
		switch {
		case div.Missing:
			fmt.Fprintf(out, "  replay diverged at event %d: recorded log ends there, replay produced %s\n",
				div.Seq, div.Got.Kind)
		case div.Short:
			// M12 (final whole-branch review): Missing's mirror image -- the
			// replay ran out of recorded Intents before the recorded log
			// itself ended, so there is nothing the replay actually
			// produced at Seq to name (unlike Missing, printing Got here
			// would be a fabricated zero events.Event).
			fmt.Fprintf(out, "  replay diverged at event %d: replay ended there, recorded log continues with %s\n",
				div.Seq, div.Want.Kind)
		default:
			fmt.Fprintf(out, "  replay diverged at event %d: recorded %s, replayed %s\n",
				div.Seq, div.Want.Kind, div.Got.Kind)
		}
		return false
	}
	fmt.Fprintf(out, "  replay error: %v\n", err)
	return false
}

func registerSBAgents(specs []string) error {
	seen := map[string]bool{}
	for _, spec := range specs {
		name, argv, err := sbseat.ParseSpec(spec)
		if err != nil {
			return err
		}
		if seen[name] {
			return fmt.Errorf("-sb-agent: duplicate name %q", name)
		}
		seen[name] = true
		if _, err := exec.LookPath(argv[0]); err != nil {
			return fmt.Errorf("-sb-agent %s: %w", name, err)
		}
		if _, ok := bots.Lookup(name); ok {
			return fmt.Errorf("-sb-agent: %q is already a registered bot policy", name)
		}
		if err := sbseat.Register(name, argv); err != nil {
			return err
		}
	}
	return nil
}

func matchSeats(e *rules.Engine, reg *cards.Registry, seed uint64, names []string, policyA, policyB, format string, mulligans int, logOut io.Writer) ([]engineseat.Seat, error) {
	seatsOut := make([]engineseat.Seat, len(names))
	for i := range names {
		policy := ""
		switch i {
		case 0:
			policy = policyA
		case 1:
			policy = policyB
		}
		if policy == "" {
			// Preserve mtgsim's pre-SB history: every default heuristic
			// seat shares the game seed, so the pinned replay corpus is
			// unchanged. A named policy still gets its own per-seat seed.
			seatsOut[i] = heuristic.NewBot(seed)
			continue
		}
		s, err := bots.New(policy, bots.Options{Seed: seed + uint64(i), AutoPayMana: true, Deps: bots.Deps{Cards: reg}})
		if err != nil {
			return nil, err
		}
		seatsOut[i] = s
	}
	for i, s := range seatsOut {
		mc := &bots.MatchContext{
			Game:         gorgegame.Game{E: e},
			Engine:       e,
			Registry:     reg,
			GameID:       fmt.Sprintf("g-%016x", seed),
			Seat:         i,
			Seats:        len(names),
			Format:       format,
			Seed:         []byte(fmt.Sprintf("%d", seed)),
			OwnDeck:      e.OwnDeck(state.PlayerID(i)),
			Mulligan:     matchMulligan(mulligans),
			StartingSeat: int(e.G.Active),
			DeadlineMS:   0,
			Logf: func(format string, args ...any) {
				fmt.Fprintf(logOut, "  "+format+"\n", args...)
			},
		}
		if err := bots.AttachMatch(s, mc); err != nil {
			for _, attached := range seatsOut[:i] {
				bots.EndMatch(attached, bots.MatchResult{Classification: "halted", Reason: "attach_failed"})
			}
			return nil, fmt.Errorf("seat %d (%s): %w", i, names[i], err)
		}
	}
	return seatsOut, nil
}

// matchFormat is the bots.MatchContext.Format word for a game's format.
func matchFormat(f rules.Format) string {
	if f == rules.FormatCommander {
		return "commander"
	}
	return "constructed"
}

// matchMulligan is the bots.MatchContext.Mulligan word for a Config's
// mulligan count, the same derivation server/host and testkit/bench use: a
// seat is told "london" exactly when the engine will pose the round.
func matchMulligan(n int) string {
	if n > 0 {
		return "london"
	}
	return "none"
}
