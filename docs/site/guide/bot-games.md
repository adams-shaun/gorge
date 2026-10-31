# Bot-vs-bot games

This page is for people who want to pit bot strategies against each other: to check that a change
to a policy helps, to compare two policies, or to watch them play. It covers the tools gorge ships
for that, the registered policies, how to run a fair matchup, how many games a difference needs, and
replay verification. For a bot in another language, see [Bots](../bots/index.md); for limited events,
[Draft events](draft.md).

The commands and output are from 2026-10-31, on 4 CPUs of a shared Linux box. Output depends on the
gorge commit and the corpus pin, so yours may differ.

## Which tool?

| Tool | Seats | Who plays | Fair seating | Use it for |
|---|---|---|---|---|
| `make sim`, [`mtgsim`](https://github.com/adams-shaun/gorge/blob/main/cmd/mtgsim/main.go) | 1-8 | `-a` on seat 0, `-b` on seat 1, the built-in heuristic elsewhere; built-in decks; one core | no: swap by hand | smoke tests, replay checks, quick looks, multiplayer |
| your program on [`testkit/bench`](https://github.com/adams-shaun/gorge/blob/main/testkit/bench/bench.go) | 2 | any two policies, any decks, parallel and deterministic | yes, every game | real comparisons |
| `gorged` startup tables | 1-8 | one policy for every bot seat, `-decks` directory | n/a | [watching](#watching-bots-play) |
| [`draftsim`](draft.md#run-events-headless-with-draftsim) | 4 or 8 | a policy per seat, on decks they drafted | round robin or Swiss | limited events |
| `cmd/enginebench` | | | | engine speed, not bot strength ([gorge's speed](../performance/engine.md)) |

## The registered policies

A **policy** is a named bot strategy registered in the
[`bots`](https://github.com/adams-shaun/gorge/blob/main/bots/registry.go) registry. A program links
them with a blank import of `bots/all` and builds one with `bots.New(name, bots.Options{...})`. To list
what a build has:

```sh
go run ./cmd/spellbench-gorge-agent -list-policies
```

```text
az-redeal	az-redeal (AlphaZero search)	experimental
bot	Bot	production
forge-heuristic	Forge heuristic	experimental
forge-heuristic-hold	Forge heuristic + hold	experimental
lethal-pressure	Lethal Pressure	experimental
sb-tactical	sb-tactical (SpellBench tactical heuristic)	experimental
```

[Bots](../bots/index.md#the-bots-gorge-ships) describes each one, with its formats and seats.

`sb-tactical` and `az-redeal` need a driver that supplies a rebuilt game to search or score: the
matchup program below does, and `mtgsim` doesn't.

A policy may declare options (`bots.Info.Options`). `gorged` sets them with `-bot-opt`, `-bot-config` or the
environment ([Bots at the table](play.md#bots-at-the-table)); your own program passes
`bots.Options{Config: map[string]string{...}}`.

## A quick look with `mtgsim`

`mtgsim` plays games one after another on one goroutine. Game *i* uses seed `-seed + i` for the engine and the
bots, so a run is a pure function of its flags. `-a` and `-b` put registered policies on seats 0 and 1:

```sh
go run ./cmd/mtgsim -seats 2 -games 3 -seed 1 -a forge-heuristic -b bot -decks uw-tempo,mono-red-goblins
```

```text
seed 1: 2 seats,    323 intents,   1802 events,  13 turns, winner=mono-red-goblins, chain=584a6ddb06889c79
seed 2: 2 seats,    215 intents,   1225 events,  10 turns, winner=mono-red-goblins, chain=084fd9760febc4c7
seed 3: 2 seats,    284 intents,   1608 events,  12 turns, winner=mono-red-goblins, chain=b98877a35b958904
```

Each line names the **winning deck**, not the winning policy. Seat 0 always holds the first deck, so a deck
advantage and a policy advantage are mixed up in one run. Swap `-a` and `-b`, run again over the same seeds, and
add up per policy.

`mtgsim`'s limits:

- **The winner is a deck name.** In a mirror (`-decks uw-tempo,uw-tempo`) the line can't tell you who won.
- **Built-in decks only.** `-decks` takes the names of the repo decks in `testkit/testutil/decks`, not file paths.
- **Environment policies don't work.** `-a sb-tactical` fails with `Deps.Cards is nil`. `-a az-redeal` runs but never
  searches, because `mtgsim` calls the plain `Decide` and the policy falls back to `bot`.
- **Seats without `-a` or `-b`** play the plain heuristic with manual mana tapping and its coin mulligan, which is
  neither `bot` nor any other registered policy.
- **Commander decks play as 20-life constructed games.** A Commander comparison needs a program that sets
  `rules.Config`'s format and commander fields.

For three to eight seats, `-seats N` deals the first N pinned decks, or `-decks` with one name per seat.
To balance seats, rotate the deck list across runs:

```sh
go run ./cmd/mtgsim -seats 8 -games 2 -verify -a forge-heuristic
```

```text
seed 0: 8 seats,   3543 intents,  16451 events,  48 turns, winner=mono-green-stompy, chain=63760184ea25d100
  replay OK (chain 63760184ea25d100)
seed 1: 8 seats,   3938 intents,  18059 events,  53 turns, winner=mono-red-goblins, chain=e2e6f50d2eb697fa
  replay OK (chain e2e6f50d2eb697fa)
```

## A fair matchup with `testkit/bench`

For a comparison you can trust, play both policies on both sides of every deck pair. `bench.RunPairs` plays *N* games
per deck pair, puts policy A in seat 0 in even games and in seat 1 in odd games, gives every seat its own seed (the game
seed XOR seat+1), runs games on a worker pool, and folds the results in game order so the tally doesn't depend on the
number of workers. `bench.PlayGame` is the drive loop, with the watchdogs and the driver support environment policies
need.

The program below is a complete matchup tool on top of it. Put it in a directory next to your gorge checkout:

```sh
mkdir matchup && cd matchup
go mod init example.com/matchup
go mod edit -require github.com/adams-shaun/gorge@v0.0.0 -replace github.com/adams-shaun/gorge=../gorge
# save the program below as main.go
go build -o matchup .
```

??? example "`main.go` (91 lines)"

    ```go
    // Command matchup plays two registered bot policies against each other over
    // deck pairs, trading seats every game, and prints policy A's win rate.
    package main

    import (
    	"flag"
    	"fmt"
    	"log"
    	"math"
    	"os"

    	"github.com/adams-shaun/gorge/bots"
    	_ "github.com/adams-shaun/gorge/bots/all" // link every built-in policy
    	"github.com/adams-shaun/gorge/engine/cards"
    	"github.com/adams-shaun/gorge/engine/replay"
    	"github.com/adams-shaun/gorge/engine/rules"
    	"github.com/adams-shaun/gorge/engine/seat"
    	"github.com/adams-shaun/gorge/testkit/bench"
    	"github.com/adams-shaun/gorge/testkit/testutil"
    )

    func main() {
    	a := flag.String("a", "forge-heuristic", "policy A")
    	b := flag.String("b", "bot", "policy B")
    	pairs := flag.String("pairs", "uw-tempo:mono-red-goblins", `deck pairs "x:y,z:w", or "all"`)
    	games := flag.Int("games", 20, "games per deck pair (even, so seats balance)")
    	seed := flag.Uint64("seed", 1, "base seed")
    	workers := flag.Int("workers", 4, "games in flight")
    	corpus := flag.String("cards", ".cards", "corpus directory")
    	verify := flag.Bool("verify", false, "replay every game and check its chain head")
    	flag.Parse()

    	reg, err := testutil.OpenCorpusRegistry(*corpus)
    	if err != nil {
    		log.Fatal(err)
    	}
    	pds, err := bench.ParsePairs(*pairs, testutil.LegacyDeckNames())
    	if err != nil {
    		log.Fatal(err)
    	}
    	// A seat constructor per side: bench hands it the per-seat seed.
    	ctor := func(policy string) bench.SeatCtor {
    		return func(s uint64) seat.Seat {
    			st, err := bots.New(policy, bots.Options{Seed: s, AutoPayMana: true, Deps: bots.Deps{Cards: reg}})
    			if err != nil {
    				log.Fatal(err)
    			}
    			return st
    		}
    	}
    	// Seat 0 always holds the pair's first deck; bench decides which
    	// policy sits there (A in even games, B in odd ones).
    	play := func(pos int, gameSeed uint64, g int, seats [2]seat.Seat) (bench.Outcome, error) {
    		pd := pds[pos]
    		decks := make([][]*cards.Card, 2)
    		for i, name := range []string{pd.A, pd.B} {
    			if decks[i], err = testutil.LoadRepoDeck(reg, name); err != nil {
    				return bench.Outcome{}, err
    			}
    		}
    		cfg := rules.Config{Seed: gameSeed, Names: []string{pd.A, pd.B}, Decks: decks,
    			Tokens: reg.Tokens, NameUniverse: reg.Universe()}
    		cfg.LoopGuard = rules.WithDefaultObjectCap(cfg.LoopGuard)
    		hooks := bench.Hooks{EnvSeats: true, HonestRoot: true} // drive search/env seats as the server does
    		o, e, err := bench.PlayGame(cfg, seats[:], 0, 400000, hooks)
    		if err == nil && *verify {
    			if re, rerr := replay.Replay(e.L, cfg); rerr != nil || re.L.Head() != e.L.Head() {
    				err = fmt.Errorf("seed %d: replay diverged: %v", gameSeed, rerr)
    			}
    		}
    		return o, err
    	}
    	res, err := bench.RunPairs(*seed, *games, pds, ctor(*a), ctor(*b), play, *workers, nil)
    	if err != nil {
    		log.Fatal(err)
    	}
    	var wins, decided int
    	for _, r := range res {
    		fmt.Printf("%-40s A %3d  B %3d  draws %d  stalls %d  seat0 wins %d\n",
    			r.PD, r.AWins, r.BWins, r.Draws, r.Stalls, r.SeatWins[0])
    		wins += r.AWins
    		decided += r.AWins + r.BWins
    	}
    	if decided == 0 {
    		os.Exit(1)
    	}
    	p := float64(wins) / float64(decided)
    	half := 1.96 * math.Sqrt(p*(1-p)/float64(decided))
    	fmt.Printf("%s vs %s: A won %d of %d decided games = %.1f%% (95%% CI +/- %.1f pp)\n",
    		*a, *b, wins, decided, 100*p, 100*half)
    }
    ```

Run it from the gorge checkout, so `.cards` resolves. The control plays the baseline against itself; `-pairs all` plays
every unordered pair of the 12 pinned constructed decks (66 pairs):

```sh
cd ../gorge
../matchup/matchup -a bot -b bot -pairs all -games 20      # the control
```

```text
bot vs bot: A won 631 of 1320 decided games = 47.8% (95% CI +/- 2.7 pp)
```

Those 1,320 games took about 9 seconds on 4 cores. `bot` against itself is 50% by symmetry, so 47.8% is what noise and the
seat order look like at this size. To compare policies, run `-a yours -b bot` with the same pairs, games and seed: its
result means something only when it sits outside the control's interval. `-verify` replays every game and checks its chain
head.

**A single pair** prints one line per pair as well as the total:

```text
uw-tempo:mono-red-goblins                A  10  B  10  draws 0  stalls 0  seat0 wins 2
```

Here `seat0 wins 2` says the deck in seat 1 won 18 of 20. The deck pairing dominates a single pair, which is why the seats
trade and why you want many pairs.

**Your own decks.** `testutil.LoadRepoDeck` reads only the repo decks. To play deck files of your own (for example from
[`deckimport`](play.md#decks)), load them with `_, cards, err := deck.Load(reg, path)` from `engine/deck`, and build the pair
list yourself instead of calling `bench.ParsePairs`.

**Your own policy.** Write a type that implements
[`seat.Seat`](https://github.com/adams-shaun/gorge/blob/main/engine/seat/seat.go), register it in an `init` with
`bots.Register(bots.Entry{Info: ..., New: ...})`, and blank-import its package in `main.go`; `-a yourpolicy` then works.
[Writing a native Go bot](../bots/native-go.md) covers the seat contract.

## Watching bots play

To see a policy in action rather than count its wins, run bot-only tables and open the lobby (this needs the
[browser client](install.md#2-build-the-browser-client-if-you-want-one)):

```sh
go run ./cmd/gorged -addr 127.0.0.1:8090 -dir /tmp/gorge-watch -feedback /tmp/gorge-play-feedback \
  -tables 4 -seats 2 -format constructed,commander -commander-seats 4 \
  -bot-policy forge-heuristic -pace 1s
```

Every bot seat on these tables plays `-bot-policy`; a server can't put two policies at one table. `-pace` slows the game for
humans, and each finished match has a replay at `/t/<table>/m/<k>`.

## Reading the result

**Count decided games.** Draws and stalls (a game a watchdog stopped) are neither a win nor a loss; leave them out of the rate
and look at why they happened. A livelock or a panic stall is an engine bug.

**How many games?** A win rate from *n* independent decided games has a 95% interval of about ±1.96·√(p(1−p)/n), widest at p =
50%. The table gives the games needed at 50%:

| Difference from 50% | Games for a 95% interval this narrow | Games to detect it 80% of the time |
|---|---|---|
| 10 points (60%) | 97 | 197 |
| 5 points (55%) | 385 | 785 |
| 3 points (53%) | 1,068 | 2,181 |
| 2 points | 2,401 | 4,906 |
| 1 point | 9,604 | 19,623 |

So a 20-game run (±22 points) tells you almost nothing, and a change worth 2 points needs about five thousand games. The normal
approximation is also poor below about 30 games.

**Run a control.** Play the baseline against itself with the same pairs, games and seeds. If the control isn't within its interval
of 50%, something in the setup favours a side.

**Look at the structure.** `seat0 wins` and the per-pair lines show how much the deck pairing, rather than the policy, decides games.
A policy that only wins one archetype's games is a different finding from one that is a little better everywhere.

## Replays and determinism

Every game is a pure function of its configuration (decks, seed) and the answers the seats gave, which the event log records.
`-verify` replays each finished game from that log and compares the rebuilt event chain head with the live one. A
`replay diverged at event N` line is an engine bug; please report it with the command that produced it.

- **Same build, same corpus, same flags, same output.** That holds across `-workers` too.
- **The seed is the game.** `mtgsim` game *i* uses `-seed + i`. `RunPairs` gives pair *p*, game *g* the seed `base + p·games + g`, so the
  first 20 games of a 40-game run on one pair are the 20-game run. On a server, table *i* has seed `-seed + i − 1` and match *k* derives its
  own from that; a play-vs-bot game draws a fresh random seed and returns it from `POST /api/games`.
- **A different engine or corpus is a different game.** Record the gorge commit and `.cards/cards.lock` with any result you keep.

## Speed

- Parallelism comes from `-workers`, the games in flight. Set `GOMAXPROCS` to the cores you want to use; more workers than cores buys
  nothing.
- Memory is mostly the loaded corpus plus one engine per game in flight; on a shared machine cap it, for example
  `systemd-run --user --scope -p MemoryMax=4G -p CPUQuota=400% env GOMAXPROCS=4 ../matchup/matchup ...`.
- Search policies cost far more per decision than the heuristics. On a server, `-bot-search-slots` bounds how many searched decisions
  run at once.
- [gorge's speed](../performance/engine.md) measures engine throughput.
