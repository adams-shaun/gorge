# Performance

Part of [Engines](../engines/index.md).

How fast gorge and dirac play Magic, and how they compare with XMage, Forge and mtg-kernel on one machine, the same decks and one CPU
core each. [How it is measured](methodology.md) has the method and commands, [gorge's speed](engine.md) its benchmark rows and profile,
and [Bot speed](bots.md) the cost per decision and per simulation.

**Where the numbers come from.** Every figure on these pages is drawn by `scripts/docs_perf_figs.py` from
[`data/perf.json`](https://github.com/adams-shaun/gorge/blob/main/docs/site/performance/data/perf.json), which transcribes the tables
(`scripts/docs_perf_figs_test.py` checks the committed SVGs match it). Each table names its commit and date; the commands are in
[Measuring again](methodology.md#measuring-again). These are measurements at those commits, not live numbers.

!!! abstract "The short version: one core, FDN limited decks"
    Measured 2026-11-25 at `b6db453064` for gorge, dirac, XMage and Forge, all in one run on one box; mtg-kernel's rows are from 2026-10-20.

    - **gorge plays about 59 random-policy games per core-second**, against 1.8 for XMage.
    - **With each engine's own AI on both seats, a gorge game costs 12-14 ms**, against 0.8 s for Forge and 1.2 s for XMage's
      default AI. The AIs aren't the same kind of player, so that is the price of a game with the bot each engine ships.
    - **dirac plays about 1,100 random-policy games per core-second** and plays
      only its [supported cards](../architecture/dirac/index.md#what-it-supports).
    - **Four-player Commander:** a random-policy pod of the Foundations Commander precons costs gorge about 5 ms a turn and dirac,
      on the same pods with the cards it doesn't play replaced, 0.4 ms; Forge's AI pod costs 0.36 s a turn
      ([below](#four-player-commander-pods)).
    - **mtg-kernel**, a public Rust engine with a few hundred cards, plays a little more games per core-second than gorge on its
      home Burn mirror and is 4-9× slower per turn on the FDN pair it plays.
    - **Memory:** a game-playing process peaks at about 120-125 MB for gorge with the whole corpus loaded,
      about 135-140 MB for dirac, 0.6 GB for Forge, 0.9-1.1 GB
      for XMage with a 2 GB heap cap and 11-16 MB for mtg-kernel, with its cards compiled in.

## The numbers

Each row is the median of six runs (`scripts/xengine-bench.sh`: FDN pairs A and B, three reps each); the spread is the slowest and
fastest run. Every row was measured directly in one run on 2026-11-25, on cores 26, 28 and 30 of metal2 at a 1-minute load of 8-40
(see [What the comparison doesn't show](#what-the-comparison-doesnt-show)). Turns count both players' turns. Compare µs per decision only within one engine, because each engine splits a game into decisions differently.

<figure markdown="span">
--8<-- "performance/figs/us-per-turn.svg"
<figcaption>Read across a row of bars, not down: every tick is ten times the last. dirac sits more than a decade under gorge, and gorge about a decade and a half under XMage's random row and nearly two under the AIs that search (XMage mad, Forge). Whiskers are the slowest and fastest of six runs.</figcaption>
</figure>

<figure markdown="span">
--8<-- "performance/figs/games-per-core-s.svg"
<figcaption>The same rows as whole games. A cheap turn doesn't always mean a cheap game: gorge's bot ends games in 18 turns, so its games cost no more than random ones, and XMage's simple AI plays 68-turn games.</figcaption>
</figure>

<figure markdown="span">
--8<-- "performance/figs/peak-rss.svg"
<figcaption>Peak resident memory of a process playing games, corpus loaded. gorge's and XMage's floor is their card database; mtg-kernel compiles its few hundred cards into the binary, so its figure is not comparable.</figcaption>
</figure>

??? info "The exact numbers, every row"

    | Engine · policy | games / core-s | ms / game | µs / turn | µs / decision | turns / game | peak RSS (MB) | measured |
    |---|--:|--:|--:|--:|--:|--:|---|
    | dirac · random | 1,120 <small>(953-1,300)</small> | 0.908 | 37.1 <small>(33.9-40.5)</small> | 7.32 | 24 | 137 | 2026-11-25 |
    | gorge · random (auto-pay) | 59.1 <small>(55.3-64.1)</small> | 17.0 | 689 <small>(685-700)</small> | 29.1 | 25 | 124 | 2026-11-25 |
    | gorge · random (manual) | 68.9 <small>(67.2-72.8)</small> | 14.5 | 352 <small>(346-357)</small> | 12.9 | 41 | 124 | 2026-11-25 |
    | XMage · random | 1.82 <small>(1.69-2.02)</small> | 550 | 17,300 <small>(15,200-18,900)</small> | 651 | 32 | 947 | 2026-11-25 |
    | dirac · heuristic bot | 1,250 <small>(1,210-1,290)</small> | 0.801 | 45.1 <small>(43.5-47.0)</small> | 8.77 | 18 | 138 | 2026-11-25 |
    | gorge · bot (auto-pay) | 70.3 <small>(67.3-72.4)</small> | 14.2 | 796 <small>(781-821)</small> | 32.2 | 18 | 123 | 2026-11-25 |
    | gorge · bot (manual) | 85.6 <small>(83.4-89.5)</small> | 11.7 | 648 <small>(623-671)</small> | 23.3 | 18 | 123 | 2026-11-25 |
    | XMage · simple AI | 16.0 <small>(13.6-18.4)</small> | 63.6 | 934 <small>(796-1,080)</small> | 59.2 | 68 | 933 | 2026-11-25 |
    | XMage · mad AI | 0.82 <small>(0.552-0.975)</small> | 1,220 | 73,000 <small>(63,600-99,600)</small> | 3,400 | 17 | 1,100 | 2026-11-25 |
    | Forge · Forge AI | 1.29 <small>(1.10-1.47)</small> | 773 | 42,500 <small>(37,700-49,200)</small> | n/a | 18 | 606 | 2026-11-25 |


**Engine cost.** The random rows use a policy that costs almost nothing, so they are as close to pure engine cost as each engine
allows. Per turn, dirac is about 19 times cheaper than gorge with auto-pay,
which is about 25 times cheaper than XMage.
gorge's manual-tap row is cheaper per turn but plays nearly twice as many turns, because a random player tapping lands one at a time
wastes mana. Forge ships no random player.

**AI cost.** The AI rows show what the built-in bot adds. gorge's heuristic bot ends games in fewer turns, so a bot game
costs about the same as a random one even though each turn costs more. XMage's base `ComputerPlayer` is its cheapest policy per
turn, but its games drift to about 68 turns. XMage's default `mad` AI searches the game tree at every priority, and Forge's AI is a
large rule-based player. Each costs tens of milliseconds a turn.

## Transitions and copies

A search copies a position and steps it many times per decision, so these two numbers decide what it can afford. dirac's transition
benchmark plays real decks with a random policy and counts one `Step` and `Apply` as one operation. Its whole-pool row loads a table of the
full FDN pool, as a search with a card belief does, so work that scales with the table shows there.

<figure markdown="span">
--8<-- "performance/figs/copy-vs-step.svg"
<figcaption>One copy and one transition, the two operations a search repeats. dirac copies a position in about 76 ns and steps it in about 3 µs, with no allocation; gorge's clone allocates 195 KB and its garbage collector doubles the cost.</figcaption>
</figure>

<figure markdown="span">
--8<-- "performance/figs/dirac-ops.svg"
<figcaption>Instructions per operation, the number that holds still under machine load. Loading the whole FDN pool's table, as a search with a card belief does, costs about half again as much per transition.</figcaption>
</figure>


These ran on one core (`GOMAXPROCS=1`, capped at 4 CPUs and 4 GB, not pinned) of a Ryzen 9 9950X at a 1-minute load average of about 6. Time per operation moves with the load, the machine and
code layout. Instructions per operation is the steadier number, and
[How it is measured](methodology.md#dirac) shows how to get it.

??? info "The exact numbers"

    | dirac benchmark, 2026-11-11 | ns per op, median of 5 | instructions per op | allocations |
    |---|--:|--:|--:|
    | Transition, Foundations (FDN) | 3,125 | 50,310 | 0 |
    | Transition, Bloomburrow (BLB) | 3,175 | 54,075 | 0 |
    | Transition, FDN whole-pool table | 4,764 | 77,332 | 0 |
    | `CopyFrom`, mid-game FDN position | 76.3 | 215 | 0 |

    Source: `BenchmarkTransitionFDN`, `BenchmarkTransitionBLB`, `BenchmarkTransitionFDNPool` and `BenchmarkCloneFDN` in `dirac/`,
    run as in [methodology](methodology.md#dirac); in [`data/perf.json`](https://github.com/adams-shaun/gorge/blob/main/docs/site/performance/data/perf.json) under `dirac_ops`.

    | gorge row, 2026-11-05 | µs per op, no GC | µs per op, with GC | bytes per op | mallocs per op |
    |---|--:|--:|--:|--:|
    | `clone`: `Engine.Clone` of a mid-game position | 12.9 | 27.2 | 195 KB | 50 |
    | `step`: clone, then submit one candidate | 43.5 | 71.2 | 369 KB | 140 |

    Source: `cmd/enginebench -row clone` and `-row step`, pair A roots from turns 5-8 (`perf.json` `gorge_ops`).


## A second workload: decks mtg-kernel plays

[mtg-kernel](https://github.com/jackmaiorino/mtg-kernel) is a public Rust rules kernel (MIT) built for reinforcement learning on a fixed card
pool. It can't play FDN pair B, so it isn't in the table above. It plays two workloads: **FDN pair A**, which is exactly its two FDN
fixture decks, and the **Burn mirror**, the mono-red Pauper deck from its own pool. It has no standalone bot, so it has random rows only. No
dirac pool holds the Burn deck.

<figure markdown="span">
--8<-- "performance/figs/kernel-workloads.svg"
<figcaption>On its home Burn mirror mtg-kernel costs about what gorge does per turn; on FDN pair A it is 4-9 times slower than gorge. XMage stays around 12-19 ms a turn on both.</figcaption>
</figure>

??? info "The exact numbers"

    | Workload · engine · policy | games / core-s | µs / turn | turns / game | peak RSS (MB) | measured |
    |---|--:|--:|--:|--:|---|
    | Pair A · dirac · random | 1,290 | 34.0 | 23 | 141 | 2026-11-25 |
    | Pair A · gorge · random (auto-pay) | 62.7 | 694 | 23 | 124 | 2026-11-25 |
    | Pair A · gorge · random (manual) | 67.7 | 353 | 42 | 124 | 2026-11-25 |
    | Pair A · mtg-kernel · random | 18.6 | 3,075 | 17 | 15.7 | 2026-10-20 |
    | Pair A · XMage · random | 1.73 | 18,700 | 30 | 945 | 2026-11-25 |
    | Burn · gorge · random (auto-pay) | 109 | 369 | 25 | 117 | 2026-11-25 |
    | Burn · gorge · random (manual) | 132 | 218 | 35 | 116 | 2026-11-25 |
    | Burn · mtg-kernel · random | 174 | 367 | 16 | 10.9 | 2026-10-20 |
    | Burn · XMage · random | 2.88 | 11,800 | 29 | 952 | 2026-11-25 |


Its random player asks every priority window and every mana ability separately, so gorge's manual row is the closest match. Its FDN cards
are a build option, and that build plays Burn several times slower than the default build, so pair A uses the FDN build and Burn the default
one. Its memory isn't comparable, since it compiles its few hundred cards into the binary rather than loading a card database.

## Four-player Commander pods

Commander is played four to a pod. These rows seat four of the five official Foundations Commander (FDC) precons
(`testkit/testutil/decks/foundations-*`) at 40 life with their commanders in the command zone; game `g` leaves out deck `(g/4) mod 5`
and rotates the other four seats by `g mod 4`, so every 20 games play every pod in every seat order. `fdc` is the precons as printed.
dirac's kernel doesn't play 26 of their 500 card copies (cards it doesn't lower, and cards in interactions it refuses), so `fdc-sub`
replaces each with its deck's basic land ([`dirac/cmd/dpod`](https://github.com/adams-shaun/gorge/tree/main/dirac/cmd/dpod) writes the decks
and the list), and gorge and Forge play the same substituted pods for a like-for-like row. XMage's bench driver seats two players, so it has no pod row.

<figure markdown="span">
--8<-- "performance/figs/commander-pods.svg"
<figcaption>A turn of a four-seat pod costs gorge 4-7 times a two-seat FDN turn: a 100-card deck, four boards, and commander rules. On the same substituted pods dirac's random player is about ten times cheaper per turn than gorge's auto-pay random player. Forge's AI pod costs 0.36-0.48 s a turn.</figcaption>
</figure>

??? info "The exact numbers"

    | Pods · engine · policy | games / core-s | µs / turn | µs / decision | turns / game | peak RSS (MB) |
    |---|--:|--:|--:|--:|--:|
    | fdc · gorge · random (auto-pay) | 2.79 | 5,110 | 100 | 70 | 198 |
    | fdc · gorge · random (manual) | 3.54 | 2,150 | 43.0 | 131 | 179 |
    | fdc · gorge · bot (auto-pay) | 8.79 | 3,140 | 70.6 | 36 | 204 |
    | fdc · gorge · bot (manual) | 8.99 | 3,010 | 61.0 | 37 | 204 |
    | fdc · Forge · Forge AI | 0.0714 | 360,000 | n/a | 39 | 722 |
    | fdc-sub · dirac · random | 47.2 | 387 | 54.9 | 55 | 184 |
    | fdc-sub · gorge · random (auto-pay) | 3.83 | 3,720 | 72.8 | 71 | 200 |
    | fdc-sub · gorge · random (manual) | 4.48 | 1,800 | 36.0 | 121 | 184 |
    | fdc-sub · gorge · bot (auto-pay) | 8.94 | 2,990 | 66.9 | 37 | 204 |
    | fdc-sub · gorge · bot (manual) | 9.67 | 2,750 | 55.2 | 38 | 172 |
    | fdc-sub · Forge · Forge AI | 0.0536 | 480,000 | n/a | 39 | 713 |

    Median of three runs, 2026-11-25 at `b6db453064` (plus this page's pod modes), 20 CPU-s per gorge and dirac run, 40 s warm-up and
    120 s per Forge run (6-9 games each). dirac's row stalled 9 of 2,829 games at a declared capacity (board and free-object tables); no
    gorge or Forge game stalled or failed. Forge's driver is `forge.oracle.GameReplay` at the fork's `2323286a00`, the first with Commander pods.

## What the comparison doesn't show

- **Card coverage differs.** gorge's is on [Card support](../support/sets.md) (generated), dirac's on its [supported cards](../architecture/dirac/index.md#what-it-supports),
  and mtg-kernel plays a few hundred cards. A faster engine that plays fewer cards isn't doing the same job.
- **Dates differ.** The mtg-kernel rows are from 2026-10-20 (commit in Provenance); every other row is from one run on 2026-11-25.
- **Load is real.** The box is shared: its own background work was stopped for the run, but CI-gate test runs kept going beside it, at
  a 1-minute load average of 8-40, sometimes on the other hardware thread of a bench core (which the harness's per-core contention figure
  doesn't see). Every engine ran under it: gorge's rows are 8-16% and dirac's about 36% under their quieter 2026-11-05 values.
  Compare rows within this run; a paired A/B separates the code's share from the load's ([gorge's speed](engine.md#comparing-two-revisions)).
- **Policies differ.** XMage's random row answers non-priority asks with its base AI, and the AIs differ in kind. Each row is spelled out in
  [the policies](methodology.md#the-player-policies).
- **JVM numbers are steady state** after 40 s of warm-up games. Start-up isn't counted.

## Provenance

??? info "Commits, toolchains and errors per engine"

    | | |
    |---|---|
    | gorge | `b6db453064` (2026-11-25), go1.26.3, `cmd/enginebench`, on metal2 (Ryzen 9 9950X), cores 26, 28 and 30 |
    | dirac | `b6db453064` (2026-11-25) for the game rows, 2026-11-11 for the transition rows, `dirac/cmd/dplay` (two seats), `dirac/cmd/dpod` (pods) and `go test -bench`, with `-trimpath` |
    | XMage | gorge's fork `adams-shaun/mage` at `6e1ecefc46` (1.4.63, gorge-compliance's `XMAGE_REF`), Temurin JDK 17.0.20.1, driver `XEngineBench` (gorge-compliance), 2026-11-25 |
    | Forge | gorge's Forge fork at `0fe8c90e54` (two seats) and `2323286a00` (Commander pods), JDK 17.0.20.1, driver `forge.oracle.GameReplay`, 2026-11-25 |
    | mtg-kernel | `jackmaiorino/mtg-kernel` `058d075e3`, Rust 1.94.1, release build of `examples/bench_kernel` with gorge's `--xengine-json` mode, 2026-10-20 |
    | Measured time | 20 CPU-s per gorge and dirac run, 15 per gorge Burn run; 40 s warm-up and 60 CPU-s per XMage and Forge run (120 per Forge pod run); 30 CPU-s per mtg-kernel run |
    | Errors | no stalled, unfinished or failed game in any gorge, Forge or mtg-kernel row. dirac's random row counts 4 stalled games of 134,724, all on pair A at a declared capacity (the pending-effect table), and its bot row none; XMage's random row has one game of 664 with no winner. They stay in the rows' counts. XMage's base AI on the Burn mirror never casts its burn spells and hit the 100-turn cap every game, so that row is left out |
