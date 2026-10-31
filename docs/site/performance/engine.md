# gorge's speed

Engine speed matters twice: once for hosting games, and again because a search bot clones and steps the engine thousands of times per decision.
This page is for contributors changing engine code: what gorge's benchmark measures, how to run it, the current numbers and where the time
goes. For gorge against other engines see [Performance](index.md).

## What is measured

| Tool | Measures | Output |
|---|---|---|
| `cmd/enginebench`, one row | `random` and `bot` play, `clone` and `step` per operation, `az` and `sampler` search rows, live heap after the corpus loads | one JSON line per run, with commit, Go version, CPU, `GOGC` and core pin |
| `make enginebench-pair BASE= CAND=` | a paired, interleaved ABBA comparison of two revisions, each built from `git archive` | JSONL and a text table: per-row medians and the median candidate/base ratio |
| `make enginebench-profile [REV=]` | CPU and allocation pprof per row, and the GC share | pprof files and printed tops |
| `make enginebench-verify [REV=]` | a memo-verifying build; fails on errors, stalls, panics or stale memos | pass or fail |
| `scripts/xengine-bench.sh` | the cross-engine rows ([How it is measured](methodology.md)) | `runs.jsonl` and a summary table |
| `make sim` | 20 verified four-seat games | replay OK count |
| `make gc-gate` | GC CPU share of one package's tests (default `./server/host`, budget 0.30) | ratio |

The workloads are four 17lands Foundations (FDN) decks and an mtg-kernel Burn deck in
[`cmd/enginebench/testdata/decks`](https://github.com/adams-shaun/gorge/blob/main/cmd/enginebench/testdata/decks/README.md). Pair A is UG vs WG,
and pair B is WR vs UR. Seats alternate every game and game `g` uses seed `base + g`, so a row is reproducible.

| Row | What it runs |
|---|---|
| `random` | whole games, uniform over every option, each mana tap asked |
| `random -autopay` | whole games, uniform over auto-pay candidates (payment plans) |
| `bot` | whole games, `heuristic.NewBot`, manual taps |
| `bot -autopay` | the same with `EnableAutoPayMana` |
| `clone` | `Engine.Clone` of a mid-game position |
| `step` | clone, then submit one candidate |
| `az -sims N` | `azmcts` with an honest re-deal; needs build tag `enginebench_az` ([Bot speed](bots.md)) |
| `sampler` | the older rollout sampler |

## How to run

From a checkout with the corpus fetched (`make fetch-cards`):

```sh
go run -trimpath ./cmd/enginebench -row random -pair A -secs 20    # one row, one JSON line
make enginebench-profile REV=.
make enginebench-verify REV=.
```

Knobs for the `make` targets are `BUDGET` (wall seconds for the whole measured phase, default 60), `ROWS`, `REPS`, `SECS`, `PIN`, `GOGC`,
`BENCH_DIR` and `HEAVY_LOCK`. `scripts/enginebench-lib.sh` documents them.

## Current numbers

Measured 2026-10-31 at `17767dd48`, Go 1.26.3, `GOMAXPROCS=1`, by the commands in [How it is measured](methodology.md#gorge): three reps of a row at
once on cores 24, 26 and 30, 20 CPU-seconds a run (15 for Burn). The medians are below, with min-max in brackets. The machine was shared, at a 1-minute load average of 13-19.

| Row | Pair A | Pair B | Burn |
|---|---|---|---|
| `random -autopay`, games per core-second | 69.9 (69.6-72.3) | 67.6 (67.3-68.0) | 140 (140-141) |
| `random -autopay`, µs per turn / per decision | 620 / 25.9 | 566 / 24.0 | 287 / 13.1 |
| `random`, games per core-second | 71.7 (71.6-73.8) | 76.6 (75.7-78.4) | 166 (163-166) |
| `random`, µs per turn / per decision | 330 / 12.1 | 326 / 11.8 | 173 / 7.90 |
| `bot -autopay`, games per core-second | 74.4 (73.8-74.4) | 84.4 (82.0-84.5) | 111 (111-112) |
| `bot -autopay`, µs per turn / per decision | 746 / 29.7 | 673 / 27.7 | 460 / 19.7 |
| `bot`, games per core-second | 105 (100-105) | 110 (110-111) | 132 (129-132) |
| `bot`, µs per turn / per decision | 534 / 19.1 | 507 / 18.4 | 391 / 15.5 |
| Turns per game: `random -autopay` / `random` / bots | 22.7 / 41.9 / 18.0 | 26.1 / 40.0 / 17.8 | 24.9 / 35.0 / 19.5 |
| Peak RSS, corpus loaded | 353-365 MB | 347-367 MB | 348-365 MB |

| One operation, pair A roots (turns 5-8) | No GC | With GC | Per operation |
|---|--:|--:|---|
| `clone` | 22.7 µs | 45.3 µs | 200 KB, 51 mallocs |
| `step` | 74.9 µs | 114 µs | 389 KB, 153 mallocs |

Live heap after the corpus loads is 41 MB.

## Comparing two revisions

Absolute rows on a shared machine mostly measure the machine: pinned rows repeat to within a few per cent inside one session, and unpinned
rows on a loaded machine move by 15-30%. To judge a change, compare it with its base, interleaved in one session:

```sh
make enginebench-pair BASE=main CAND=.                         # BUDGET=60s for the whole run
make enginebench-pair BASE=main CAND=. PIN=24 REPS=3           # pinned to one core, GOMAXPROCS=1
make enginebench-pair BASE=main CAND=. BUILD_ARGS="-tags enginebench_az" ROWS="-row az -sims 100"
```

The table it prints ends each row with the ratio. A value above 1 means the candidate is faster, for a higher-is-better unit:

```text
row                   unit             base med    cand med  cand/base  per-rep ratios
clone A               ops/cpu-s           27705       28020      1.015  1.064 0.967
```

That line came from two reps of the same code, with `BUDGET=30 PIN=26 REPS=2 ROWS='-row clone'`, so it also shows the size of the noise.

## Where the time goes

CPU profiles of one 20-second pair-A run each, 2026-10-31 at `17767dd48`, pinned to core 26, `GOMAXPROCS=1`. Flat time is grouped by package, and
`engine/rules` includes its sub-packages.

| Package | `random -autopay` | `bot -autopay` |
|---|--:|--:|
| `engine/rules` (legal-action walk, payment planning, resolution, triggers) | 61.2% | 52.1% |
| Go runtime (GC, allocation, maps) | 13.8% | 14.9% |
| `engine/state` | 7.8% | 7.4% |
| `engine/cards` | 6.3% | 5.4% |
| `engine/effects` | 5.3% | 4.6% |
| Go standard library | 3.7% | 4.0% |
| `engine/events` | 1.5% | 1.8% |
| `bots/botpolicy` + `bots/heuristic` | - | 7.2% + 1.1% |

By call tree (cumulative), in the random auto-pay run: `Engine.Submit` 75% and `Engine.Advance` 40% (they nest), the legal-action walk over the hand
(`legalWalk.handWalk`) 24%, payment planning (`paymentActions`) 22%, and emitting events through `events.Apply` (`Engine.emit`) 17%. In the bot run, the
bot's own work is about 15% cumulative: building its board (`botpolicy.BoardFromGameInto`) 11% and deciding (`heuristic.(*Bot).DecideBoard`) 4.4%. In both, the
engine rather than the policy is most of the cost. Listing what a player may do, with payments planned, is the hot path.

To reproduce:

```sh
go run -trimpath ./cmd/enginebench -row random -autopay -pair A -secs 20 -cpuprofile ra.cpu
go tool pprof -top -cum ra.cpu
```
