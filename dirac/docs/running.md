# Running games, bots and measurements on dirac

This page maps common jobs to the command or Go entry point that does them. `dirac/doc.go` is the API authority and
[verification.md](verification.md) the lockstep authority. Terms: [glossary](glossary.md).

## Setup

Anything that reads a card set needs gorge's compiled card corpus and dirac's binary card tables. Both derive from
Forge's GPL-3.0 scripts, so neither is committed. From the repository root:

```sh
make fetch-cards     # once
make compile-cards
make dirac-tables
```

Run `make dirac-tables` again after a change to `dirac/kgen`, `dirac/kop`, `dirac/cardset`, a pool list or the gorge
packages kgen builds against. The tables live under the corpus, keyed by the corpus and those sources
([cards.md](cards.md#where-tables-live)). `-cards` or `GORGE_CARDS` picks the corpus; `DIRAC_TABLES` points a deployed
binary at a copy of a `kernel-tables` directory. Missing tables are an error naming the command, never a fallback.

Without a corpus the module still builds, corpus-dependent tests skip, and the hand-written `dirac/vanilla` table
still plays (`dirac/example_test.go` runs a whole game on it).

## Which command?

| I want to … | use | what it does |
|---|---|---|
| play a game from Go | package `dirac`, or `dirac/lab` ([lab page](../../docs/site/engines/lab.md)) | the `Step`/`Apply` loop, below |
| play seeded games and see who wins, or how fast | `dirac/cmd/dplay` | dirac alone, a line per game and a summary |
| play both engines and compare them | `dirac/oracle/cmd/kfuzz` | seeded lockstep games ([verification](verification.md)) |
| ask whether a card is supported | `kgen -supported <pool>` | one line per card ([cards](cards.md)) |
| build the card tables | `make dirac-tables` | every pool's binary table, under the corpus |
| compare the heuristic bots | `dirac/oracle/cmd/khbot` | gorge's auto-pay bot against its dirac port, per decision kind |
| compare sb-tactical | `dirac/oracle/cmd/ksbt` | sb-tactical answering through a view built from the dirac position |
| write the card-by-card audit | `dirac/oracle/cmd/cardaudit` | one row per card joining gorge's claim, dirac's status and its evidence |
| serve games on dirac | `gorged -engine dirac` | [serving](serving.md) |
| serve dirac as a stdio SpellBench v2 engine | `dirac/cmd/sbv2dirac` | dirac as authority, gorge as lockstep shadow |
| maintain the namespace manifest | `dirac/cmd/kmanifest` | `show`, `gen` ([README](../README.md#registering-an-op-stall-or-ext-sub-op-contributors)) |
| measure transition speed | `go test -bench` in `./dirac` | [Performance](#performance) |
| measure whole-game speed against other engines | `scripts/xengine-bench.sh dirac` | `dplay` on one pinned core; see `docs/site/performance/methodology.md` |

## A first lockstep run

```sh
go build -trimpath -o bin/kfuzz ./dirac/oracle/cmd/kfuzz
bin/kfuzz -kernel fdn -first-pair 0 -pairs 2 -seed 1 -keep-going -surface autopay -shrink 0
```

```
fdn: 2 games (0 refused-card, 0 refused-pair, 0 error), 2 clean, 0 divergent, 0 unmodelled [], 0 capacity [], 0 hang (lead gorge, surface autopay)
```

A clone plays the committed slices in `dirac/oracle/testdata/slices` (the first 1500 pairs of the `fdn` and `blb` pools
at seed 1), and `kfuzz -synth <pool>` deals synthetic decks from a card list. The full deck pools are not published.
`make dirac-slices` plays both slices on both surfaces.

## Playing games with dplay

`dirac/cmd/dplay` plays dirac alone. Every game runs under `dirac.LockstepConfig` on the table `Set.Matchup` builds for
the pair, the configuration and table `kfuzz` verifies against gorge, so a clean lockstep of the same seed is the same game.

```sh
go run -trimpath ./dirac/cmd/dplay -pairs 20 -a hbot -b random
```

Each game is one JSON line on stdout (`pair`, `decks`, `seed`, `winner`, `turns`, `intents`); the summary goes to stderr.
`winner` is 0 for seat A, 1 for seat B, -1 for no winner. A stalled game adds `stall` (its reason) and `class`:
`capacity` (a declared limit), `unmodelled` (a rule gap) or `cap` (`-max-turns` or `-max-intents`). A game that
panicked adds `error`. The summary counts games, wins by seat, draws, stalls by class and reason, refusals, turns,
decisions and the run's CPU and wall seconds (from after the first pair's table is built). The games file depends only
on the flags, whatever `-workers` is, unless `-wall` ends the run.

| flag | default | meaning |
|---|---|---|
| `-cards` | `$GORGE_CARDS`, else `.cards` in or above the working directory | the corpus |
| `-set` | `fdn` | the card set: a deck pool by name, or a synthetic pool (`std-trk`, `tail-NN`) |
| `-decks` | the set's pool, else its committed slice | deck directory, `<name>.json` per deck |
| `-pool` | `<decks>/../pool.tsv` | the decks pairs are drawn from |
| `-split` | `all` | pool split: `train`, `eval` or `all` |
| `-deck-a` | none | deck JSON file for seat A, with `-deck-b`, instead of a pool |
| `-deck-b` | none | deck JSON file for seat B |
| `-pairs` | 100 | deck pairs, one game each |
| `-first-pair` | 0 | index of the first pair |
| `-seed` | 1 | base seed; pairs and game seeds derive from it as in `kfuzz` |
| `-pairs-file` | none | play these pairs: `deckA<TAB>deckB[<TAB>seed]` per line |
| `-a` | `hbot` | seat A's player, who plays the pair's first deck: `random`, `first`, `hbot` or `sbt` |
| `-b` | `random` | seat B's player |
| `-mulligans` | 0 | London mulligans each player may take; 0 skips the round |
| `-mulligan-policy` | `coin` | how every seat mulligans: `coin`, `lands`, `never`, `constructed` or `forge` (`dirac/search/mull`) |
| `-max-turns` | 100 | turn cap; a capped game is a stall |
| `-max-intents` | 20000 | decision cap per game; a capped game is a stall |
| `-workers` | 1 | games in parallel |
| `-wall` | 0 | start no game after this much wall time; 0 plays every pair |
| `-out` | stdout | games file |
| `-summary` | stderr | summary file |
| `-skip-refused` | off | skip a pair the set refuses and count it, instead of failing |
| `-sample-replay` | 0 | fraction of games also written as `koracle-game-2` records under `<out>.replays`, which `kfuzz -replay` re-executes in gorge; needs `-out` |
| `-config` | `lockstep-autopay` | the verified surface: `lockstep-autopay` or `lockstep-full` |
| `-cpuprofile` | none | write a CPU profile to this file |
| `-memprofile` | none | write an allocation profile to this file and print the run's allocation totals |

`random` picks uniformly from its own stream, seeded from the game's seed. `first` takes the first
option offered. `hbot` is `dirac/search/hbot`, the port of gorge's auto-pay heuristic bot, and `sbt` is
gorge's sb-tactical answering through `dirac/oracle`. `hbot` and `sbt` read gorge's compiled cards, so
they open the corpus; `random` and `first` read only the card tables.

A clone with no deck pool plays the committed slice: `-split all -seed 1` pairs 0 to 1499. Other pairs need
`GORGE_POOLS`.

A deck holding a card the set does not play is an error naming the card and why, before any game. With `-skip-refused`
the pair is skipped instead: the summary's `refused_pairs` counts it and `refused_cards` names the first such card of
each refused deck. Two decks the set plays can still be refused together (the error names the pair; `refused_pair_reasons`
counts it under `-skip-refused`). A card missing from the set altogether is always an error.

## Driving dirac from Go

The value API is in `dirac/doc.go`:

```go
set, _ := cardset.ByName("fdn")
tbl, deckA, deckB, _ := set.Matchup(listA, listB)
s := dirac.NewConfig(tbl, deckA, deckB, seed, dirac.Config{Mulligans: 2})
for {
	d := s.Step() // or s.StepInto(&d)
	if d.Kind == dirac.DkNone {
		break
	}
	s.Apply(d.Options[0]) // any index in d.Options[:d.N]
}
over, winner := s.Terminal()
```

`State.Project(&p)` writes a fixed-size `Projection` of the whole position; `ProjectDelta` and `ProjectSince` write
changes. A copy of `State` is a complete position, which is what a search clones. Read-only accessors for search and bot
code are in `dirac/searchapi.go` (object card, owner, controller, zone, hidden/visible, hand and library sizes,
re-deal helpers) and `dirac/hbotapi_combat.go` (block pairs, attack target, blocking). They allocate nothing.

## Bots on dirac

| package | entry points | what it gives a bot |
|---|---|---|
| `dirac/search` | `DefaultOptions()`, `NewSearcher(opts, eval, bot)`, `(*Searcher).Search`, `(*Searcher).SeatSearch` | gorge's `bots/azmcts` PUCT search on a `dirac.State` |
| `dirac/neteval` | `LoadWeights(path)`, `New(w, pool)` | a network `search.Evaluator` over a `.dzgw` file |
| `dirac/search/hbot` | `NewFacts(*kop.Table)`, `New(*Facts)`, `(*Bot).EnableHoldInstant()` | the native port of gorge's auto-pay heuristic bot |
| `dirac/search/mull` | `KeepLands`, `KeepConstructed`, `KeepNever`, `CoinMulls` | gorge's London-mulligan policies, as pure functions over a hand |
| `dirac/feat` | `NewPool(tbl, specs)`, `NewEncoder(pool)`, `EncodeState`, `EncodeOptions` | the policy-net input encoding: the same bytes `bots/policynet` produces from gorge's redacted view |

`oracle.HBotFacts` and `oracle.FeatPool` build hbot facts and the feat pool from a gorge registry; dirac-only code
passes a `kop.Table` and string-derived specs to `hbot.NewFacts` and `feat.NewPool`. For a Go-side lab on top of these,
see [`dirac/lab`](../lab/doc.go).

## SpellBench v2 on dirac

SpellBench v2 is newline-delimited JSON over stdio: a **host** starts an **engine** child and two **agent** children and
relays decisions between them. `dirac/cmd/sbv2dirac` is the engine role with dirac as the authority:

- It answers `hello`, `reset`, `step` and `validate_deck`. One game runs in lockstep
  (`oracle.LockstepLed`): dirac leads, a gorge shadow (`oracle.GorgeSide`) follows action by
  action, and gorge's SpellBench mapping and observe layers build the v2 observation and candidate
  list from the shadow's pending decision.
- The agent's pick is translated to the canonical action (`GorgeSide.CanonOfIntent`), applied to
  dirac and mirrored into the shadow. A disagreement ends the game with
  `engine_contract_failure:divergence`; the report goes to stderr and `Server.Divergences`.
- Decks are the engine catalog plus `-decks DIR` (one gorge deck JSON per file, catalog id = file
  stem). A deck containing a card dirac's set does not support is refused by
  `validate_deck`/`reset`, naming the cards.

A handshake smoke (the line a host sends first):

```sh
go build -trimpath -o bin/sbv2dirac ./dirac/cmd/sbv2dirac
printf '%s\n' '{"request_type":"hello","protocol":"spellbench/v2","protocol_minor":0,"request_id":"r-0"}' \
  | bin/sbv2dirac -corpus .cards -decks dirac/testdata/bench/fdn
```

It answers one `hello_ok` line whose `engine.name` is `dirac` and whose `catalog` lists the decks
passed with `-decks`. The wire reference is [`docs/site/api/spellbench-v2.md`](../../docs/site/api/spellbench-v2.md).

## Performance

The transition benchmarks play real decks one core at a time and report no allocations. Current figures, with date
and commit, are on [Performance](../../docs/site/performance/index.md#transitions-and-copies).

```sh
GOMAXPROCS=1 go test -run XXX -bench '^BenchmarkTransition(FDN|BLB|FDNPool)$' -benchmem ./dirac
```

Time per operation moves with the machine and code layout. To compare two commits, use instructions per operation: run
the test binary under `perf stat -e instructions:u` at two `-test.benchtime` counts and divide the difference in
instructions by the difference in counts. `scripts/dirac-gate.sh <base>` holds the allocation and instruction steps.

The rules that keep dirac fast are structural ([`dirac/README.md`](../README.md#rules-for-code-that-touches-state)):
a plain-value `State` cloned with `CopyFrom`, no allocation in the transition loop, no map iteration order reaching
output. For a change in `dirac/` or `dirac/oracle/`, the cheap check is a `kfuzz` lockstep slice
([verification](verification.md)).

## Where to read next

[`dirac/doc.go`](../doc.go) (API), [`README.md`](README.md) (index), [`cards.md`](cards.md), [`verification.md`](verification.md),
[`DEVIATIONS.md`](DEVIATIONS.md), [`GORGE-FINDINGS.md`](GORGE-FINDINGS.md), [`dirac/README.md`](../README.md).
