# How it is measured

What the [engine comparison](index.md) measures, where engines aren't comparable, and how to measure again on your machine. For anyone
checking the numbers or adding an engine. The harness is `scripts/xengine-bench.sh`; it only writes under `$OUT`, never into these pages.
The pages' tables are transcribed from its summaries (and from `make enginebench-pair`) into [`data/perf.json`](https://github.com/adams-shaun/gorge/blob/main/docs/site/performance/data/perf.json).

## What one run is

A **run** is one process playing whole two-player games between two fixed decks, single-threaded, pinned to one CPU core, for a fixed
amount of CPU time. A **row** is one engine, one policy and one deck pair. Each row runs three times at once, on three different physical
cores, and the tables report the median and the spread. Rows run one after another, never together.

A run is repeated on three cores at once; each run prints games, turns, decisions, CPU seconds, peak RSS, load and core contention to
`runs.jsonl`, and the summary table takes medians. JVM engines play warm-up games first, discarded.

| Metric | Definition |
|---|---|
| games per core-second | measured games ÷ CPU seconds of the measured window |
| ms per game | the inverse |
| µs per turn | CPU µs ÷ turns played; each engine's turn counter counts both players' turns |
| µs per decision | CPU µs ÷ decisions answered; decisions are counted differently per engine, below |
| peak RSS | the process's maximum resident set from `/usr/bin/time`, corpus load included |

**CPU time** is user plus system, so it includes the garbage collector (and the JIT for a JVM). Forge's driver reports wall time per game,
which on a pinned core is the core-time used. **Peak RSS is a distribution:** a Go process's peak depends on when the collector runs during
corpus load, so one row's reps can fall in two clusters; the summary keeps the median and lists every rep.

| Engine | A decision is |
|---|---|
| gorge | every `decision.Decision` posed: priority, each mana tap under manual policies, each target, attack and block declarations, mulligans, choices |
| dirac | every decision a policy is asked, after single options are answered in place |
| XMage | each `priority`, `selectAttackers` and `selectBlockers` call on the game thread |
| mtg-kernel | every `advance_until_decision` call on the raw engine API |
| Forge | not counted: the driver has no per-decision hook |

<figure markdown="span">
--8<-- "performance/figs/decisions-per-turn.svg"
<figcaption>Why µs per decision is compared only within one engine: dirac answers single-option decisions in place and poses about five a turn, while gorge, counting each tap and target, poses about 24-28. Derived from the comparison table as µs per turn ÷ µs per decision.</figcaption>
</figure>

## The workload

Two FDN (Foundations) limited deck pairs from
[`cmd/enginebench/testdata/decks`](https://github.com/adams-shaun/gorge/blob/main/cmd/enginebench/testdata/decks/README.md), 40-card
Premier Draft lists sourced from 17lands (CC BY 4.0):

| Pair | Seat A | Seat B |
|---|---|---|
| A | `FDN_top_04956_UG` | `FDN_top_20626_WG` |
| B | `FDN_top_07961_WR` | `FDN_top_02581_UR` |

The second workload, the **Burn mirror** (`burn.json`), is the 60-card mono-red Pauper list from mtg-kernel's pool (MIT). gorge, XMage, Forge and
mtg-kernel play it; no dirac pool holds it. mtg-kernel lacks four cards of pair B, so it plays pair A and Burn.

Seats alternate every game, at 20 life with 7-card hands. gorge and dirac runs have no mulligan round; XMage's and Forge's AIs may mulligan.
Every game has a turn cap: 300 in `enginebench`, 100 in the other drivers.

## The player policies

**Random legal action** measures the engine almost alone.

| Engine | Row | The policy |
|---|---|---|
| gorge | `random-autopay` | uniform over the priority options with mana paid automatically, uniform over every other decision. The closest match to the other engines' random players |
| gorge | `random` | uniform over every option including each mana tap, so it wastes mana and plays about twice as many turns |
| dirac | `random` | uniform over the options at every decision |
| XMage | `random` | at each priority a random playable ability or pass, random attackers and blockers; targets, modes and other asks go to its base `ComputerPlayer` |
| mtg-kernel | `random` | its own benchmark's policy, through the raw engine API: every priority window and mana ability asked separately |

**Each engine's own AI** measures what a game costs with the bot it ships. These differ in algorithm, so read them as prices, not as engine speed.

| Engine | Row | The AI |
|---|---|---|
| gorge | `bot`, `bot-autopay` | the default heuristic bot (`heuristic.NewBot`), with manual taps or `EnableAutoPayMana` |
| dirac | `hbot` | a native port of gorge's auto-pay heuristic bot; compare it with `bot-autopay` |
| XMage | `simple` | the base `ComputerPlayer`, with no look-ahead |
| XMage | `mad` | `ComputerPlayer7`, the client's default AI, at skill 1 (search depth 4, at most 3 s per decision) |
| Forge | `ai` | `PlayerControllerAi` with the default profile on both seats |

mtg-kernel has no AI row: its searchers are wired to its training contracts and have no standalone game runner.

## Pinning and noise

- Every run is pinned with `taskset -c CORE`. Go runs with `GOMAXPROCS=1`; a JVM pinned to one core picks the serial collector, heap capped by `-Xmx2g`.
- The harness records the 1-minute load before and after each run and **core contention** (the core's busy time beyond the run's own CPU
  time); a rep with more than a few per cent of other work on its core is rerun.
- Reps share L3 and memory bandwidth, so load still moves the numbers. The spread shows repeatability at that load, not the idle-machine value.
- **JVM warm-up.** Each run plays `WARM` seconds of games and discards them. Start-up and warm-up are excluded from per-game numbers but included in peak RSS.
- **The card cache is private.** Every compile prunes other fingerprints' IR caches in its directory (`cards.PruneCaches`), so a timed run on
  a shared `.cards` could pay a whole-corpus recompile. The harness hardlinks a private corpus under `$OUT/work`, warms each engine's cache with
  one untimed run, and records in `run.txt` any batch whose reps changed it.

## Measuring again

The harness is [`scripts/xengine-bench.sh`](https://github.com/adams-shaun/gorge/blob/main/scripts/xengine-bench.sh), with the stdlib-only
[`scripts/xengine-bench-summarize.py`](https://github.com/adams-shaun/gorge/blob/main/scripts/xengine-bench-summarize.py). Choose `CORES` as
idle cores on different physical cores (`lscpu -e` shows which share one). The script's header lists every knob.

### gorge

About three minutes at `SECS=20`, which is how the published rows were made:

```sh
git clone https://github.com/adams-shaun/gorge && cd gorge
make fetch-cards
OUT=/tmp/xengine CORES="24 26 30" SECS=20 scripts/xengine-bench.sh gorge
OUT=/tmp/xengine-burn CORES="24 26 30" SECS=15 PAIRS=burn scripts/xengine-bench.sh gorge
```

Output is one summary row per engine, policy and pair (columns: games/core-s, ms/game, µs/turn, µs/decision, turns/game, peak RSS, load1, other
busy, RSS reps). `$OUT/runs.jsonl` holds one line per run and `$OUT/summary.jsonl` the table as JSON; `python3 scripts/xengine-bench-summarize.py $OUT/runs.jsonl` reprints it.

### XMage

[gorge-compliance](https://github.com/adams-shaun/gorge-compliance) builds XMage out of tree for its compliance oracle, and the benchmark driver,
`XEngineBench.java`, sits beside the oracle driver. These rows were last run on 2026-10-20.

```sh
git clone https://github.com/adams-shaun/gorge-compliance && cd gorge-compliance
make xmage-oracle-setup XMAGE_ORACLE_DIR=$HOME/xmage
D=$HOME/xmage
(cd $D/mage && $D/maven/bin/mvn -q -B -pl Mage.Tests dependency:build-classpath \
   -Dmdep.outputFile=$D/tests.cp -Dmdep.includeScope=test)
cd ../gorge
JAVA=$D/jdk/bin/java XMAGE_TESTS=$D/mage/Mage.Tests XMAGE_CP=$D/tests.cp \
XMAGE_BENCH=../gorge-compliance/oracle/xmageoracle/bench/org/mage/test/bench/XEngineBench.java \
OUT=/tmp/xengine CORES="2 4 6" SECS=60 WARM=40 scripts/xengine-bench.sh xmage
```

### Forge

gorge-compliance's `make forge-oracle-setup` builds forge-core, forge-game and forge-ai with their classpath in `cp.txt`. The driver,
`forge.oracle.GameReplay`, is in the `forge-oracle` module on branch
[`oracle-harness`](https://github.com/adams-shaun/forge/tree/oracle-harness) of gorge's Forge fork. These rows were last run on 2026-10-20.

```sh
cd gorge-compliance && make forge-oracle-setup FORGE_ORACLE_DIR=$HOME/forgeoracle
F=$HOME/forgeoracle
cd ../gorge
JAVA=$HOME/xmage/jdk/bin/java FORGE_CP=$F/cp.txt FORGE_RES=$F/forge/forge-gui/res \
FORGE_SRC=<forge checkout>/forge-oracle/src/main/java \
OUT=/tmp/xengine CORES="2 4 6" SECS=60 WARM=40 scripts/xengine-bench.sh forge
```

### mtg-kernel

Measured at `058d075e30532b15a6417cafd7086e5b98fd88da` on 2026-10-20. gorge adds a small `--xengine-json` mode to mtg-kernel's benchmark, which plays any two
deck lists for a fixed CPU time with that benchmark's own policy
([`cmd/enginebench/scripts/xengine/mtgkernel/`](https://github.com/adams-shaun/gorge/tree/main/cmd/enginebench/scripts/xengine/mtgkernel)); nothing from mtg-kernel is committed to gorge. The driver refuses a deck with a card mtg-kernel doesn't fully support.

```sh
git clone https://github.com/jackmaiorino/mtg-kernel && cd mtg-kernel
git checkout 058d075e30532b15a6417cafd7086e5b98fd88da
(cd mtg-kernel && python3 ../../gorge/cmd/enginebench/scripts/xengine/mtgkernel/apply.py \
   ../../gorge/cmd/enginebench/scripts/xengine/mtgkernel/xengine.rs)
CARGO_TARGET_DIR=$PWD/target     cargo build --release -p mtg-kernel --example bench_kernel
CARGO_TARGET_DIR=$PWD/target-fdn cargo build --release -p mtg-kernel --example bench_kernel \
   --features limited-fdn-fixtures
cd ../gorge
MTGKERNEL_BENCH=../mtg-kernel/target-fdn/release/examples/bench_kernel MK_PAIRS=A \
  OUT=/tmp/xengine CORES="2 4 6" SECS=30 scripts/xengine-bench.sh mtg-kernel
MTGKERNEL_BENCH=../mtg-kernel/target/release/examples/bench_kernel MK_PAIRS=burn \
  OUT=/tmp/xengine CORES="2 4 6" SECS=30 scripts/xengine-bench.sh mtg-kernel
```

### dirac

The game rows use `dirac/cmd/dplay`, built from the checkout; it needs the corpus and dirac's tables (`make fetch-cards compile-cards dirac-tables`):

```sh
OUT=/tmp/xengine CORES="24 26 30" SECS=20 scripts/xengine-bench.sh dirac
```

Both rows use `dplay`'s default `-config lockstep-autopay`, the configuration lockstep verifies against gorge's auto-pay seat, on a table holding only the pair's two decks.

The transition rows are Go benchmarks, one core, reporting no allocations:

```sh
GOMAXPROCS=1 taskset -c 26 go test -trimpath -run XXX \
  -bench '^Benchmark(Transition(FDN|BLB|FDNPool)|CloneFDN)$' -benchmem -count 5 -benchtime 2s ./dirac
```

**Instructions per operation** don't depend on load the way nanoseconds do, so compare them between commits. Run the test binary under
`perf stat -x, -e instructions:u` (build it with `go test -trimpath -c -o dirac.test ./dirac`; run `./dirac.test -test.run XXX -test.bench '^BenchmarkTransitionFDN$'`) at two fixed `-test.benchtime Nx` counts (500000 and 1500000, pinned with `taskset`, `GOMAXPROCS=1`) and
difference them, which cancels setup: (instructions at 1500000 − at 500000) / 1000000.

## Adding an engine

An engine needs a driver that plays whole games between two `.dck` lists on one thread and reports games, turns, decisions and CPU time for a
measured window after any warm-up. Add a `<engine>_run` and `<engine>_all` function to the script and a `KIND` to `--record` in the summarizer.
