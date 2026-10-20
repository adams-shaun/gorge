// Command duel plays seeded two-seat games between two registered policies
// on real repo decks, seats traded every game, and prints the score. It is
// the in-process path: the engine, both seats and the loop in one process,
// no network and no clock in any decision.
//
//	go run ./cmd/duel -cards ../../../.cards -a allin -b bot -games 20
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/adams-shaun/gorge/bots"
	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	_ "github.com/adams-shaun/gorge/examples/bots/go/allin"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func main() {
	dir := flag.String("cards", ".cards", "card corpus directory (make fetch-cards compile-cards)")
	a := flag.String("a", "allin", "policy A")
	b := flag.String("b", "bot", "policy B")
	deckA := flag.String("deck-a", "mono-green-stompy", "deck for policy A")
	deckB := flag.String("deck-b", "mono-red-goblins", "deck for policy B")
	games := flag.Int("games", 20, "games to play")
	seed := flag.Uint64("seed", 1, "base seed; game i uses seed+i")
	flag.Parse()

	reg, err := testutil.OpenCorpusRegistry(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "duel:", err)
		os.Exit(1)
	}
	decks := map[string][]*cards.Card{}
	for _, name := range []string{*deckA, *deckB} {
		if decks[name], err = testutil.LoadRepoDeck(reg, name); err != nil {
			fmt.Fprintln(os.Stderr, "duel:", err)
			os.Exit(1)
		}
	}

	var winsA, winsB, draws, stalls, decisions int
	start := time.Now() // reporting only; no decision reads the clock
	for g := 0; g < *games; g++ {
		gameSeed := *seed + uint64(g)
		// Seat 0 alternates between A and B so neither policy always plays first.
		pol, dk := []string{*a, *b}, []string{*deckA, *deckB}
		if g%2 == 1 {
			pol, dk = []string{*b, *a}, []string{*deckB, *deckA}
		}
		seats := make([]seat.Seat, 2)
		for i := range seats {
			s, err := bots.New(pol[i], bots.Options{Seed: gameSeed ^ uint64(i+1), AutoPayMana: true, Deps: bots.Deps{Cards: reg}})
			if err != nil {
				fmt.Fprintln(os.Stderr, "duel:", err)
				os.Exit(1)
			}
			seats[i] = s
		}
		cfg := rules.Config{Seed: gameSeed, Names: dk, Decks: [][]*cards.Card{decks[dk[0]], decks[dk[1]]},
			Tokens: reg.Tokens, NameUniverse: reg.Universe()}
		out, _, err := bench.PlayGame(cfg, seats, 200, 200000, bench.Hooks{})
		if err != nil {
			fmt.Fprintln(os.Stderr, "duel:", err)
			os.Exit(1)
		}
		decisions += out.Intents
		switch {
		case out.IsStalled():
			stalls++
		case out.Draw:
			draws++
		case (out.WinnerSeat == 0) == (g%2 == 0): // A sits at seat 0 on even games
			winsA++
		default:
			winsB++
		}
	}
	el := time.Since(start)
	fmt.Printf("%s (%s) %d - %d %s (%s), %d draws, %d stalls\n", *a, *deckA, winsA, winsB, *b, *deckB, draws, stalls)
	fmt.Printf("%d games, %d decisions in %.2fs: %.1f µs per decision, both seats, engine included\n",
		*games, decisions, el.Seconds(), float64(el.Microseconds())/float64(decisions))
}
