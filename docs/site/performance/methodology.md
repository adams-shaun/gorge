# How the engine comparison is measured

This page says exactly what the [cross-engine comparison](index.md) measures,
where the engines are and are not comparable, and how to rerun it on your own
machine. It is for anyone who wants to check the numbers, rerun them after a
change, or add another engine. Terms such as *decision* and *seat* are defined
in the [glossary](../contributing/glossary.md).

## What one run is

A **run** is one operating-system process that plays whole two-player games
between two fixed decks, single-threaded, pinned to one CPU core, for a fixed
amount of CPU time. A **row** is one engine, one player policy and one deck
pair; every row is run three times (three **reps**) and the tables report the
median with the min-max spread of the three.

```mermaid
flowchart LR
  S[xengine-bench.sh] -->|"taskset -c CORE"| P1[rep 0: core 24]
  S --> P2[rep 1: core 26]
  S --> P3[rep 2: core 28]
  P1 --> W{JVM?}
  W -->|yes| WU[warm-up: whole games<br/>for WARM s, discarded]
  W -->|no: Go| M
  WU --> M[measure: whole games<br/>for SECS CPU-seconds]
  M --> R[runs.jsonl: games, turns,<br/>decisions, CPU s, peak RSS,<br/>load, core contention]
  R --> T[summary table:<br/>median and min-max]
```

## Metrics

| Metric | Definition | Why it is reported |
|---|---|---|
| games per core-second | measured games ÷ CPU seconds of the measured window | the headline: how many self-play games one core produces |
| ms per game | the inverse, in milliseconds | easier to read for the slow engines |
| µs per turn | CPU µs ÷ turns played | normalises game length; *turn* is each engine's own turn counter, which counts both players' turns in all four engines |
| µs per decision | CPU µs ÷ decisions answered | normalises game length per choice, but each engine splits a game into decisions differently (below), so compare it within one engine, not across |
| peak RSS | the process's maximum resident set size (`/usr/bin/time %M`) | memory cost of one game-playing process, card database included |

**CPU time** is the process's user + system time over the measured window, so
it includes every thread of the process: for the Go engines the garbage
collector, for the JVM engines the garbage collector, the JIT compiler and
XMage's AI search threads. With the process pinned to one core, CPU time and
wall time agree to within a few per cent. Forge's driver reports wall time per
game; on a pinned core that is the core-time the game used.

**Decisions** differ by engine:

| Engine | A decision is | Counted from |
|---|---|---|
| gorge | every `decision.Decision` the engine poses: priority, each mana tap (manual policies), each target, attack and block declarations, mulligans, choices | `enginebench` `detail.Decisions` |
| dirty-dirac | every intent the kernel asks a policy for | `kplay` `intents` per game |
| XMage | each `priority`, `selectAttackers` and `selectBlockers` call on the game thread (targets and other asks are not counted) | the driver's counter |
| Forge | not counted: the driver has no per-decision hook | - |

## The workload

Two FDN (Foundations) limited deck pairs from
[`cmd/enginebench/testdata/decks`](https://github.com/adams-shaun/gorge/blob/main/cmd/enginebench/testdata/decks/README.md),
the same files every engine reads:

| Pair | Seat A | Seat B |
|---|---|---|
| A | `FDN_top_04956_UG` | `FDN_top_20626_WG` |
| B | `FDN_top_07961_WR` | `FDN_top_02581_UR` |

They are 40-card decks of 17lands-sourced (CC BY 4.0) Premier Draft lists. FDN
is used because it is the one card pool every engine here plays in full:
dirty-dirac is generated for a fixed set of cards (FDN among them), while
gorge, XMage and Forge have every card. The Burn mirror deck that
`enginebench` also carries is not used here, because dirty-dirac does not play
it. Seats alternate every game, 20 life, 7-card hands, no mulligans except
where an engine's own AI decides one (XMage and Forge AIs may mulligan; the
gorge and dirty-dirac runs use their default of no mulligan round). Every game
has a turn cap (300 turns in `enginebench`, 100 in the other drivers); in the
published run no game reached it, stalled or failed.

## The player policies

The policy is the biggest difference between rows, so every row says which
one it uses. There are two classes.

**Random legal action** measures the engine almost alone: the policy is a few
lines, so the cost is listing the legal actions and applying the chosen one.

| Engine | Row | What the policy does |
|---|---|---|
| gorge | `random-autopay` | uniform over the priority options with mana paid automatically (payment plans, no individual taps), uniform over every other decision. The closest match to the other engines' random players. |
| gorge | `random` | uniform over every option including each individual mana tap, so it wastes mana and plays about twice as many turns per game |
| dirty-dirac | `random` | uniform over the kernel's options at every intent |
| XMage | `random` | at each priority, a uniformly random playable ability or pass (the rule of XMage's own MCTS rollout player `SimulatedPlayerMCTS`), random attackers, random blockers. Targets, modes and the other asks are answered by XMage's base `ComputerPlayer` heuristics, so this row carries some AI cost |
| Forge | - | Forge ships no random player; this comparison has no Forge random row |

**Each engine's own AI** measures what a game costs with the engine's built-in
bot on both seats. These AIs are not the same strength or the same algorithm,
so read these rows as "the price of a game with the bot this engine ships",
not as engine speed.

| Engine | Row | The AI |
|---|---|---|
| gorge | `bot` | the default heuristic bot (`heuristic.NewBot`), manual mana taps |
| gorge | `bot-autopay` | the same bot with auto-pay (`EnableAutoPayMana`) |
| dirty-dirac | `hbot` | a native port of gorge's auto-pay heuristic bot, which agrees with it on over 99.9% of priority decisions; compare it with gorge's `bot-autopay` |
| XMage | `simple` | the base `ComputerPlayer`: no look-ahead. Its games run long (it rarely races) |
| XMage | `mad` | `ComputerPlayer7`, "Computer - mad", the XMage client's default AI, at skill 1 (search depth 4, at most 3 s per decision) |
| Forge | `ai` | Forge's `PlayerControllerAi` with the default AI profile on both seats, as Forge plays AI-vs-AI matches |

Separating engine cost from AI cost: within one engine, the random row is the
engine floor and the AI rows add the AI on top. In gorge the default bot plays
games *faster* than the random player, because it ends games sooner (about 18
turns against 23-42) and its decisions are cheap; in XMage, `mad` costs about
an order of magnitude more per turn than `random`, because its search copies
and simulates the game.

## Pinning, concurrency and noise

- Every run is one process pinned with `taskset -c CORE`; Go runs with
  `GOMAXPROCS=1`, and a JVM pinned to one core picks the serial garbage
  collector by itself (`-XX:+PrintFlagsFinal` shows `UseSerialGC` ergonomic).
  JVM heap is capped at `-Xmx2g`.
- The three reps of a row run **at the same time**, on cores 24, 26 and 28
  (three different physical cores of a 16-core / 32-thread Ryzen 9 9950X;
  their SMT siblings are not reserved). Rows and engines run one after
  another, never together. The concurrency cuts the wall time by three; the
  cost is that the reps share the chip's L3 cache and memory bandwidth with
  each other (and with whatever else runs on the machine).
- The 1-minute load average before and after each run is recorded, and so is
  **core contention**: the run's core's busy time (from `/proc/stat`) beyond
  the run's own CPU time. A rep with more than a few per cent of other work on
  its core is rerun; the summary prints the worst rep's contention.
- The box is shared with other work; the load average during the published
  run is printed beside every table. Variation between reps was small (see
  the min-max columns), which is the evidence that the numbers are stable at
  this load, not a guarantee that they hold on an idle machine.

## JVM warm-up

A JVM engine starts interpreted and gets faster as the JIT compiles its hot
code: XMage's random games went from 11 to about 60-75 turns per second over
the first 15 games (about 15 s) on this machine. Each JVM run therefore plays
whole games for `WARM` = 40 CPU-seconds first and discards them, then
measures for `SECS` = 60 CPU-seconds. The reported JVM numbers are **steady
state after warm-up**. Start-up (loading the card database: 8-10 s for XMage,
6 s for Forge) and warm-up are excluded from the per-game numbers but included
in peak RSS. The Go engines need no warm-up; `enginebench` plays a few games
before its timer starts, and its corpus load (and `kplay`'s, about 1-3 s) is
excluded the same way. One asymmetry works against dirty-dirac: `kplay`
writes a JSON line per game (about 1 KB) inside its measured window, while
`enginebench` writes only its summary.

## What is not comparable

!!! warning "Read the comparison with these in mind"
    - **Rules coverage differs.** XMage and Forge implement every card and
      every format; gorge implements most of Forge's card corpus (the coverage
      ratchet is on the [support pages](../support/sets.md)); dirty-dirac is a
      two-player kernel for a fixed card set. A faster engine that plays fewer
      cards is not doing the same job.
    - **Decisions are split differently.** gorge asks for each mana tap and
      each target; XMage's counted decisions are priority, attack and block
      only; Forge's are not counted. Compare µs per decision within an
      engine.
    - **The AIs differ in kind.** gorge's bot and dirty-dirac's port are
      cheap one-ply heuristics; XMage's `mad` is a depth-limited game-tree
      search; Forge's AI is a large rule-based player. Their game lengths
      differ too.
    - **XMage's random row is not fully random**: non-priority asks go to its
      base AI.
    - **One machine, one load.** All numbers come from one shared machine at a
      moderate load. Relative positions are robust to that; absolute numbers
      are not.

## Rerunning it

The harness is [`scripts/xengine-bench.sh`](https://github.com/adams-shaun/gorge/blob/main/scripts/xengine-bench.sh)
(bash, with the stdlib-only
[`scripts/xengine-bench-summarize.py`](https://github.com/adams-shaun/gorge/blob/main/scripts/xengine-bench-summarize.py)
for the deck conversion and the table). It plays every engine that is set up
and prints the summary table; the raw per-run lines stay in
`$OUT/runs.jsonl`.

### 1. gorge alone (about 4 minutes)

```sh
git clone https://github.com/adams-shaun/gorge && cd gorge
make fetch-cards                 # the Forge card corpus, pinned, into .cards/
OUT=/tmp/xengine CORES="2 4 6" SECS=30 scripts/xengine-bench.sh gorge
```

Choose `CORES` on your machine: three idle cores on different physical cores
(`lscpu -e` shows which logical CPUs share a core).

### 2. XMage (one-time build about 2 minutes plus the download)

The XMage build comes from
[gorge-compliance](https://github.com/adams-shaun/gorge-compliance), which
builds XMage out of tree for its compliance oracle. The benchmark driver,
`XEngineBench.java`, lives there beside the oracle driver.

```sh
git clone https://github.com/adams-shaun/gorge-compliance && cd gorge-compliance
make xmage-oracle-setup XMAGE_ORACLE_DIR=$HOME/xmage    # JDK 17 + Maven + XMage at XMAGE_REF, built
D=$HOME/xmage
(cd $D/mage && $D/maven/bin/mvn -q -B -pl Mage.Tests dependency:build-classpath \
   -Dmdep.outputFile=$D/tests.cp -Dmdep.includeScope=test)
cd ../gorge
JAVA=$D/jdk/bin/java XMAGE_TESTS=$D/mage/Mage.Tests XMAGE_CP=$D/tests.cp \
XMAGE_BENCH=../gorge-compliance/oracle/xmageoracle/bench/org/mage/test/bench/XEngineBench.java \
OUT=/tmp/xengine CORES="2 4 6" SECS=60 WARM=40 scripts/xengine-bench.sh xmage
```

### 3. Forge

Forge is built by gorge-compliance's `make forge-oracle-setup` (forge-core,
forge-game and forge-ai at `FORGE_ORACLE_REF` from gorge's Forge fork, with
their classpath in `cp.txt`). The whole-game driver is `forge.oracle.GameReplay`
in the fork's `forge-oracle` module.

```sh
cd gorge-compliance && make forge-oracle-setup FORGE_ORACLE_DIR=$HOME/forgeoracle
F=$HOME/forgeoracle
cd ../gorge
JAVA=$HOME/xmage/jdk/bin/java FORGE_CP=$F/cp.txt FORGE_RES=$F/forge/forge-gui/res \
FORGE_SRC=<forge-oracle checkout with GameReplay>/forge-oracle/src/main/java \
OUT=/tmp/xengine CORES="2 4 6" SECS=60 WARM=40 scripts/xengine-bench.sh forge
```

The Forge driver stops the JVM once `WARM + SECS` seconds of games are
written, and the warm-up games are dropped when the run is recorded.

### 4. dirty-dirac

dirty-dirac is the maintainers' private kernel, so its row cannot be rerun
from the public repositories. It was measured with the same script, which
calls the kernel's game runner `kplay` when `KPLAY` names a binary built from
the private module:

```sh
KPLAY=<kplay binary> OUT=/tmp/xengine CORES="24 26 28" SECS=30 scripts/xengine-bench.sh dirty-dirac
# each run: kplay -cards .cards -deckfile <the pair, as JSON> -pairs-file <200000 games, seats alternating>
#           -pairs 0 -a random|hbot -b random|hbot -wall 30s -out <games.jsonl>
```

### Reading the output

`runs.jsonl` holds one line per run: `engine`, `row`, `pair`, `rep`, `core`,
`load1_before`/`load1_after`, `maxrss_mb`, `games`, `turns`, `decisions`,
`cpu_s`, `games_per_cpu_s`, `ms_per_game`, `us_per_turn`, `us_per_decision`,
`warmup_games`, `errors` (stalls, unfinished or harness-failed games) and
`other_busy` (the core contention above). Rerun the table with
`python3 scripts/xengine-bench-summarize.py $OUT/runs.jsonl`.

## Adding an engine

An engine needs a driver that plays whole games between two `.dck` deck lists
on one thread, pinned, and reports games, turns, decisions and CPU time for a
measured window after any warm-up. Add a `<engine>_run` and `<engine>_all`
function to the script and a `KIND` to `--record` in the summarizer. Other
engines were not added because none is installed on the measuring machine;
Rust engines such as mtg-kernel would also need a Rust toolchain there.
