# Running games, bots and measurements on dirac

dirac, also called the kernel, is the second Magic rules engine in this repository. gorge is the
engine it is checked against, and `dirac/oracle` is the seam between them. This page maps common
jobs to the command or Go entry point that does them. It does not repeat the reference pages:
`dirac/doc.go` is the API authority and `dirac/docs/verification.md` the lockstep authority.

Terms used below:

- **kop**: the card and op table package (`dirac/kop`): card records and instruction opcodes, no
  gorge types.
- **kgen**: the generator that turns gorge's compiled card IR into binary card tables.
- **stall**: the kernel stops a game it cannot continue correctly; the game ends as a draw with a
  named reason.
- **lockstep**: one game played by gorge and the kernel action for action, with every field
  compared. `kfuzz` (`dirac/oracle/cmd/kfuzz`) is the lockstep driver.

## Setup

Anything that reads a card set needs gorge's compiled card corpus and dirac's binary card tables. The
corpus comes from Forge's GPL-3.0 scripts and the tables are translated from it, so neither is
committed. From the repository root:

```sh
make fetch-cards     # once; about 140 MB
make compile-cards
make dirac-tables    # about 10 s
```

Run `make dirac-tables` again after a change to `dirac/kgen`, `dirac/kop`, `dirac/cardset`, a pool
list, or the gorge packages kgen builds against. The tables live under the corpus, keyed by the
corpus and those sources. `-cards` or `GORGE_CARDS` picks the corpus, and a deployed binary can point
`DIRAC_TABLES` at a copy of a `kernel-tables` directory. Missing tables are an error naming the
command, never a fallback.

Without a corpus the module still builds, corpus-dependent tests skip, and the hand-written
`dirac/vanilla` table still plays: `go test -run Example ./dirac` runs a whole game on it.

## Which command?

| I want to … | use | what it does |
|---|---|---|
| play a game from Go | package `dirac` | the `Step`/`Apply` loop, below |
| play both engines and compare them | `dirac/oracle/cmd/kfuzz` | seeded lockstep games on a deck pool or a card list; see `dirac/docs/verification.md` |
| ask whether a card is supported | `dirac/cmd/kgen -supported <pool>` | one line per card in the pool; see `dirac/docs/cards.md` |
| build the card tables | `make dirac-tables`, which runs `dirac/cmd/kgen` | every pool's binary table, under the corpus |
| compare the heuristic bots | `dirac/oracle/cmd/khbot` | gorge's auto-pay heuristic bot against its port on the kernel, per decision kind |
| compare sb-tactical | `dirac/oracle/cmd/ksbt` | gorge's sb-tactical answering through a view built from the kernel position |
| serve dirac as a SpellBench v2 engine | `dirac/cmd/sbv2dirac` | the engine role, with gorge as the lockstep shadow |
| measure speed | `go test -bench` in `./dirac` | the transition benchmarks, below |

## A first lockstep run

```sh
go build -trimpath -o bin/kfuzz ./dirac/oracle/cmd/kfuzz
bin/kfuzz -kernel fdn -first-pair 0 -pairs 2 -seed 1 -keep-going -surface autopay -shrink 0
```

```
fdn: 2 games (0 refused-card, 0 refused-pair, 0 error), 2 clean, 0 divergent, 0 unmodelled [], 0 capacity [], 0 hang (lead gorge, surface autopay)
```

A clone plays the committed slices in `dirac/oracle/testdata/slices`, the first 1500 pairs of the
`fdn` and `blb` pools at seed 1. `kfuzz -synth <pool>` plays a card list instead of a deck list.

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

`dirac.Table` is `kop.Table`. `State.Project(&p)` writes a fixed-size `Projection` for the whole
position (zones, stack, P/T, counters, turn/step, players); `ProjectDelta` and `ProjectSince` write
changes. A copy of `State` is a complete position, which is what a search clones.

Read-only accessors for search and bot code live in `dirac/searchapi.go` (object card, owner,
controller, zone, hidden/visible, hand and library sizes, `Rand`/re-deal helpers) and
`dirac/hbotapi_combat.go` (block pairs, attack target, blocking, planeswalker test, projection flags).
They allocate nothing.

## Bots on dirac

| package | entry points | what it gives a bot |
|---|---|---|
| `dirac/search/hbot` | `NewFacts(*kop.Table)`, `New(*Facts)`, `(*Bot).EnableHoldInstant()` | the native port of gorge's auto-pay heuristic bot |
| `dirac/search/mull` | `KeepLands`, `KeepConstructed`, `KeepNever`, `CoinMulls` | gorge's London-mulligan policies, as pure functions over a hand |
| `dirac/feat` | `NewPool(tbl, specs)`, `NewEncoder(pool)`, `EncodeState`, `EncodeOptions` | the policy-net input encoding: the same bytes `bots/policynet` produces from gorge's redacted view |

`oracle.HBotFacts` and `oracle.FeatPool` build hbot facts and the feat pool from a gorge registry.
In kernel-only code, `hbot.NewFacts` and `feat.NewPool` take only the `kop.Table` plus the
string-derived specs.

## SpellBench v2 on dirac

SpellBench v2 is newline-delimited JSON over stdio: a **host** starts an **engine** child and two
**agent** children, and relays an engine `decision` to the acting seat's agent as a `choose`, then
the agent's `choice` back as a `step`. `dirac/cmd/sbv2dirac` is the engine role with the kernel as the
authority:

- It answers `hello`, `reset`, `step` and `validate_deck`. One game runs in lockstep
  (`oracle.LockstepLed`): the kernel leads, a gorge shadow (`oracle.GorgeSide`) follows action by
  action, and gorge's SpellBench mapping and observe layers build the v2 observation and candidate
  list from the shadow's pending decision.
- The agent's pick is translated to the canonical action (`GorgeSide.CanonOfIntent`), applied to the
  kernel and mirrored into the shadow. A disagreement ends the game with
  `engine_contract_failure:divergence`; the report goes to stderr and `Server.Divergences`.
- Decks are the engine catalog plus `-decks DIR` (one gorge deck JSON per file, catalog id = file
  stem). A deck containing a card the kernel's set does not support is refused by
  `validate_deck`/`reset`, naming the cards.

A handshake smoke, the same line a host sends first:

```sh
go build -trimpath -o bin/sbv2dirac ./dirac/cmd/sbv2dirac
printf '%s\n' '{"request_type":"hello","protocol":"spellbench/v2","protocol_minor":0,"request_id":"r-0"}' \
  | bin/sbv2dirac -corpus .cards -decks dirac/testdata/bench/fdn
```

It answers one `hello_ok` line whose `engine.name` is `dirac` and whose `catalog` lists the decks
passed with `-decks`. The wire reference is [`docs/site/api/spellbench-v2.md`](../../docs/site/api/spellbench-v2.md).

## Performance

Measure, do not assume. The transition benchmarks play real decks one core at a time, and report no
allocations:

```sh
GOMAXPROCS=1 go test -run XXX -bench '^BenchmarkTransition(FDN|BLB|FDNPool)$' -benchmem ./dirac
```

Time per operation moves with the machine and with code layout. Instructions per operation is the
steadier number to compare two commits: run the test binary under `perf stat -e instructions:u` at two
`-test.benchtime` counts, and divide the difference in instructions by the difference in counts.

The rules that keep the kernel fast are structural, not tuning:

- `State` holds a fixed layout of plain values, cloned with `CopyFrom`. `go vet` rejects an
  assignment. See `dirac/doc.go` for the value and projection rules and `dirac/README.md` for the rules
  a kernel change must keep.
- The transition loop allocates nothing.
- No map iteration order may reach an action, option list, record or output file. Same seed, same
  game, byte for byte.
- The oracle compares projections and decisions against gorge, not timing.

When a change is in `dirac/` or `dirac/oracle/`, the cheap check is a `kfuzz` lockstep slice, described
in `dirac/docs/verification.md`, rather than a full seeded sweep.

## Where to read next

- `dirac/doc.go`: the value, decision and projection API and the rules the kernel follows.
- `dirac/docs/DEVIATIONS.md`, `dirac/docs/GORGE-FINDINGS.md`: the differences from gorge.
- `dirac/docs/README.md`: the documentation index.
- `dirac/docs/cards.md`: kgen, card pools and tables, "is card X supported?".
- `dirac/docs/verification.md`: lockstep, `kfuzz`, `khbot` and `ksbt`, and following a gorge change.
- `dirac/README.md`: the package's change and registration rules.
