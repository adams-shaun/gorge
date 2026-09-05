// Command mtgsim plays headless games between seat.Bot instances over the
// repo's 12 Legacy decks and verifies that each one replays to the same
// event chain. It is the harness M5's performance work and the AI seat will
// both build on -- a way to run many real games with no client, no network
// and no wall clock in the loop.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/replay"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/view"
)

// maxIntents bounds a single game the same way rules/acceptance_test.go's
// own acceptance gate does: a game that has not produced a decision inside
// this many intents is not "slow", it is not terminating, and mtgsim reports
// that as a failure rather than spinning forever.
const maxIntents = 400000

func main() {
	decksFlag := flag.String("decks", "", "comma-separated deck names (default: the first -seats of the repo decks)")
	seats := flag.Int("seats", 4, "number of seats")
	seed := flag.Uint64("seed", 0, "base seed; game i uses seed+i for both the engine and its bot")
	games := flag.Int("games", 1, "number of games to play")
	verify := flag.Bool("verify", false, "replay each game and compare its event chain against the live run")
	verbose := flag.Bool("v", false, "print the deck assigned to each seat before every game")
	dir := flag.String("dir", ".cards", "corpus directory (holds ir.gob.gz / cardsfolder)")
	flag.Parse()

	if err := run(*decksFlag, *seats, *seed, *games, *verify, *verbose, *dir, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "mtgsim:", err)
		os.Exit(1)
	}
}

// run is main's testable body. It returns a non-nil error -- and therefore a
// non-zero exit code -- if any game fails to terminate or, with -verify, to
// replay, exactly the contract Task 26's supplement (Ruling, mtgsim §5)
// asks for. Everything runs on the caller's own goroutine, one match at a
// time: no goroutines, so two runs with the same arguments print the same
// lines in the same order every time.
//
// out is io.Writer, not *os.File (fix round 1, Minor #3): main passes
// os.Stdout, but main_test.go passes a *bytes.Buffer so this body is
// actually exercised by a test, which the old *os.File signature made
// impossible without writing to a real file.
func run(decksFlag string, seats int, baseSeed uint64, games int, verify, verbose bool, dir string, out io.Writer) error {
	if seats < 1 {
		return fmt.Errorf("-seats must be at least 1, got %d", seats)
	}
	if games < 1 {
		return fmt.Errorf("-games must be at least 1, got %d", games)
	}

	names, err := deckNames(decksFlag, seats)
	if err != nil {
		return err
	}

	reg, err := testutil.OpenCorpusRegistry(dir)
	if err != nil {
		return fmt.Errorf("opening corpus at %s: %w (run `make fetch-cards compile-cards` first)", dir, err)
	}

	decks := make([][]*cards.Card, seats)
	for i, name := range names {
		d, err := testutil.LoadRepoDeck(reg, name)
		if err != nil {
			return err
		}
		decks[i] = d
	}

	if verbose {
		for i, name := range names {
			fmt.Fprintf(out, "seat %d: %s\n", i, name)
		}
	}

	failures := 0
	for g := 0; g < games; g++ {
		gameSeed := baseSeed + uint64(g)
		if !playOne(out, gameSeed, names, decks, verify) {
			failures++
		}
	}
	if failures > 0 {
		return fmt.Errorf("%d of %d games failed to terminate or verify", failures, games)
	}
	return nil
}

// deckNames resolves the -decks flag against the embedded repo decks: an
// explicit comma-separated list, or (when empty) the first `seats` names
// from testutil.RepoDeckNames(), which is already sorted -- so the default
// assignment is the same on every machine and every run, never whatever
// order a directory listing happened to produce.
func deckNames(flagVal string, seats int) ([]string, error) {
	if flagVal == "" {
		all := testutil.RepoDeckNames()
		if seats > len(all) {
			return nil, fmt.Errorf("-seats %d exceeds the %d repo decks available; pass -decks explicitly to repeat one", seats, len(all))
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

// playOne plays a single game to completion (or to maxIntents) and prints
// its one-line summary, then -- if verify is set -- a second line reporting
// the replay outcome. It returns false for anything Ruling §5 counts as
// failure: non-termination, a replay error or a chain divergence.
func playOne(out io.Writer, seed uint64, names []string, decks [][]*cards.Card, verify bool) bool {
	cfg := rules.Config{Seed: seed, Names: append([]string(nil), names...), Decks: decks}
	e := rules.New(cfg)
	b := seat.NewBot(seed)
	e.Advance()

	ctx := context.Background()
	n := 0
	for !e.G.Over && e.Pending() != nil && n < maxIntents {
		d := e.Pending()
		v := view.Project(e.G, e, d.Player, d)
		in, err := b.Decide(ctx, v, *d)
		if err != nil {
			fmt.Fprintf(out, "seed %d: bot error at intent %d: %v\n", seed, n, err)
			return false
		}
		if err := e.Submit(in); err != nil {
			fmt.Fprintf(out, "seed %d: intent %d rejected: %v\n", seed, n, err)
			return false
		}
		n++
	}

	if !e.G.Over {
		fmt.Fprintf(out, "seed %d: %d seats, %6d intents, %6d events, %3d turns -- DID NOT TERMINATE\n",
			seed, len(names), n, len(e.L.Events), e.G.Turn)
		return false
	}

	// Ruling P14: Draw before Winner -- Winner's zero value is a real seat
	// (0), so reading it unconditionally would misreport a drawn game.
	result := "draw"
	if !e.G.Draw {
		result = e.G.Players[e.G.Winner].Name
	}
	fmt.Fprintf(out, "seed %d: %d seats, %6d intents, %6d events, %3d turns, winner=%s, chain=%s\n",
		seed, len(names), n, len(e.L.Events), e.G.Turn, result, e.L.Head())

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

// printReplayOutcome prints the -verify line for one game -- "replay OK"
// with its chain head, or the divergence in the shape supplement §5 asks
// for: Seq, both Kinds, and Missing honoured (no zero events.Event printed
// when the recorded log simply ended first). Factored out of playOne (fix
// round 1, Minor #3) so it can be driven directly from a hand-built
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
