# Performance

Part of [Engines](../engines/index.md).

How fast gorge and dirac play Magic, and how they compare with XMage, Forge and mtg-kernel on one machine, the same
decks and one CPU core each. This page is for anyone choosing an engine to host games or generate self-play data.
[How it is measured](methodology.md) has the method and the commands, [gorge's speed](engine.md) has its benchmark rows
and profile, and [Bot speed](bots.md) has the cost per decision and per simulation.

!!! abstract "The short version: one core, FDN limited decks"
    Measured 2026-11-05 at `b1630d6b89bb` for gorge, 2026-11-05 at `0ec285f67` for dirac, and 2026-10-20 for XMage, Forge and mtg-kernel, which weren't re-run.

    - **gorge plays about 64 random-policy games per core-second**, against 2.4 for XMage.
    - **With each engine's own AI on both seats, a gorge game costs 10-13 ms**, against about 1 s for Forge and for XMage's
      default AI. The AIs aren't the same kind of player, so that is the price of a game with the bot each engine ships.
    - **dirac plays about 1,750 random-policy games per core-second**, with
      2.7 µs a transition, and plays
      only its [supported cards](../architecture/dirac/index.md#what-it-supports), for two players.
    - **mtg-kernel**, a public Rust engine with a few hundred cards, plays about as many games per core-second as gorge on its
      home Burn mirror and is 5-11× slower per turn on the FDN pair it plays.
    - **Memory:** a game-playing process peaks at about 350-400 MB for gorge with the whole corpus loaded,
      about 100 MB for dirac, 0.9 GB for Forge, 1.2-1.3 GB
      for XMage with a 2 GB heap cap and 11-16 MB for mtg-kernel, with its cards compiled in.

## The numbers

Each row is the median of six runs: FDN pairs A and B, three reps each. The spread is the slowest and fastest run, except on the
four gorge rows (below), where it is the 2026-11-02 spread scaled by the same factor. The four gorge rows are the 2026-11-02
`b78e4c8b1` medians scaled by this head's interleaved paired cand/base ratios, so they carry the code's share and not the loaded
box's absolute speed. Turns count both players' turns. Compare µs per decision only within one engine, because each engine splits a game into decisions differently.

<figure markdown="span">
--8<-- "performance/figs/us-per-turn.svg"
<figcaption>Read across a row of bars, not down: every tick is ten times the last. dirac sits a decade and a half under gorge, and gorge another decade and a half under XMage's random row and two under the AIs that search (XMage mad, Forge). Whiskers are the slowest and fastest of six runs.</figcaption>
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
    | dirac · random | 1,759 <small>(1,645-1,870)</small> | 0.569 | 23.5 <small>(23.2-23.6)</small> | 4.61 | 24 | 105 | 2026-11-05 |
    | gorge · random (auto-pay) | 64.0 <small>(58.7-70.7)</small> | 15.6 | 629 <small>(618-669)</small> | 26.4 | 24 | 375 | 2026-11-05 |
    | gorge · random (manual) | 78.8 <small>(76.2-80.6)</small> | 12.6 | 311 <small>(296-330)</small> | 11.4 | 41 | 356 | 2026-11-05 |
    | XMage · random | 2.38 <small>(2.16-2.58)</small> | 420 | 13,193 <small>(12,774-13,751)</small> | 499 | 32 | 1,228 | 2026-10-20 |
    | dirac · heuristic bot | 1,971 <small>(1,763-2,175)</small> | 0.507 | 28.9 <small>(26.2-31.6)</small> | 5.60 | 18 | 103 | 2026-11-05 |
    | gorge · bot (auto-pay) | 78.6 <small>(74.0-83.6)</small> | 12.7 | 709 <small>(669-756)</small> | 28.7 | 18 | 353 | 2026-11-05 |
    | gorge · bot (manual) | 102 <small>(97.8-109)</small> | 9.86 | 548 <small>(509-572)</small> | 19.6 | 18 | 396 | 2026-11-05 |
    | XMage · simple AI | 19.3 <small>(18.6-19.7)</small> | 51.9 | 761 <small>(743-791)</small> | 48.3 | 68 | 1,219 | 2026-10-20 |
    | XMage · mad AI | 0.947 <small>(0.552-1.25)</small> | 1,121 | 62,343 <small>(49,069-101,733)</small> | 2,902 | 17 | 1,288 | 2026-10-20 |
    | Forge · Forge AI | 1.02 <small>(0.723-1.22)</small> | 983 | 54,513 <small>(45,904-71,387)</small> | n/a | 18 | 882 | 2026-10-20 |


**Engine cost.** The random rows use a policy that costs almost nothing, so they are as close to pure engine cost as each engine
allows. Per turn, dirac is about 25 times cheaper than gorge with auto-pay,
which is about 23 times cheaper than XMage's 2026-10-20 row.
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
<figcaption>One copy and one transition, the two operations a search repeats. dirac copies a position in 40 ns and steps it in under 3 µs, with no allocation; gorge's clone allocates 195 KB and its garbage collector doubles the cost.</figcaption>
</figure>

<figure markdown="span">
--8<-- "performance/figs/dirac-ops.svg"
<figcaption>Instructions per operation, the number that holds still under machine load. Loading the whole FDN pool's table, as a search with a card belief does, costs a third more per transition.</figcaption>
</figure>


These ran on one pinned core at a 1-minute load average of 13 on the box. Time per operation moves with the load, the machine and
code layout. Instructions per operation is the steadier number, and
[How it is measured](methodology.md#dirac) shows how to get it.

??? info "The exact numbers"

    | dirac benchmark, 2026-11-02 | ns per op, median of 5 | instructions per op | allocations |
    |---|--:|--:|--:|
    | Transition, Foundations (FDN) | 2,748 | 45,604 | 0 |
    | Transition, Bloomburrow (BLB) | 2,909 | 51,990 | 0 |
    | Transition, FDN whole-pool table | 3,601 | 60,463 | 0 |
    | `CopyFrom`, mid-game FDN position | 40.2 | 187 | 0 |

    | gorge row, 2026-11-05 | µs per op, no GC | µs per op, with GC | bytes per op | mallocs per op |
    |---|--:|--:|--:|--:|
    | `clone`: `Engine.Clone` of a mid-game position | 12.9 | 27.2 | 195 KB | 50 |
    | `step`: clone, then submit one candidate | 43.5 | 71.2 | 369 KB | 140 |


## A second workload: decks mtg-kernel plays

[mtg-kernel](https://github.com/jackmaiorino/mtg-kernel) is a public Rust rules kernel (MIT) built for reinforcement learning on a fixed card
pool. It can't play FDN pair B, so it isn't in the table above. It plays two workloads: **FDN pair A**, which is exactly its two FDN
fixture decks, and the **Burn mirror**, the mono-red Pauper deck from its own pool. It has no standalone bot, so it has random rows only. No
dirac pool holds the Burn deck.

<figure markdown="span">
--8<-- "performance/figs/kernel-workloads.svg"
<figcaption>On its home Burn mirror mtg-kernel costs about what gorge does per turn; on FDN pair A it is 5-11 times slower than gorge. XMage stays around 11-13 ms a turn on both.</figcaption>
</figure>

??? info "The exact numbers"

    | Workload · engine · policy | games / core-s | µs / turn | turns / game | peak RSS (MB) | measured |
    |---|--:|--:|--:|--:|---|
    | Pair A · dirac · random | 1,867 | 23.5 | 23 | 106 | 2026-11-05 |
    | Pair A · gorge · random (auto-pay) | 70.8 | 613 | 23 | 397 | 2026-11-05 |
    | Pair A · gorge · random (manual) | 83.3 | 285 | 42 | 356 | 2026-11-05 |
    | Pair A · mtg-kernel · random | 18.6 | 3,075 | 17 | 15.7 | 2026-10-20 |
    | Pair A · XMage · random | 2.50 | 13,063 | 31 | 1,228 | 2026-10-20 |
    | Burn · gorge · random (auto-pay) | 140 | 287 | 25 | 353 | 2026-10-31 |
    | Burn · gorge · random (manual) | 166 | 173 | 35 | 349 | 2026-10-31 |
    | Burn · mtg-kernel · random | 174 | 367 | 16 | 10.9 | 2026-10-20 |
    | Burn · XMage · random | 3.14 | 11,188 | 29 | 1,262 | 2026-10-20 |


Its random player asks every priority window and every mana ability separately, so gorge's manual row is the closest match. Its FDN cards
are a build option, and that build plays Burn several times slower than the default build, so pair A uses the FDN build and Burn the default
one. Its memory isn't comparable, since it compiles its few hundred cards into the binary rather than loading a card database.

## What the comparison doesn't show

- **Card coverage differs.** XMage and Forge implement every card. gorge plays most of Forge's corpus ([Card support](../support/sets.md)). dirac
  plays two players and its [supported cards](../architecture/dirac/index.md#what-it-supports). mtg-kernel plays a few hundred cards. A faster engine that plays fewer cards isn't doing the
  same job.
- **Dates differ.** The XMage, Forge and mtg-kernel rows are from 2026-10-20 and came from their own measured commits.
  Both gorge and dirac have changed since then, so read those ratios as orders of magnitude, which is what they're large enough to support.
- **Load differs.** The box is shared. gorge's rows ran at a 1-minute load average of 10-40 and dirac's game rows at 4-6, while XMage and
  Forge ran at 4-15. Cache and memory contention move these numbers, and a paired A/B separates the code's share from the load's
  ([gorge's speed](engine.md#comparing-two-revisions)).
- **Policies differ.** XMage's random row answers non-priority asks with its base AI, and the AIs differ in kind. Each row is spelled out in
  [the policies](methodology.md#the-player-policies).
- **JVM numbers are steady state** after 40 s of warm-up games. Start-up isn't counted.

## Provenance

??? info "Commits, toolchains and errors per engine"

    | | |
    |---|---|
    | gorge | `b1630d6b89bb` (2026-11-05), go1.26.3, `cmd/enginebench` |
    | dirac | `0ec285f67` (2026-11-05) for the game rows, 2026-11-02 for the transition rows, `dirac/cmd/dplay` and `go test -bench`, with `-trimpath` |
    | XMage | `magefree/mage` `6b602a1c85` (1.4.61), Temurin JDK 17.0.20.1, driver `XEngineBench` (gorge-compliance), 2026-10-20 |
    | Forge | gorge's Forge fork at `da1c712c31`, JDK 17.0.20.1, driver `forge.oracle.GameReplay`, 2026-10-20 |
    | mtg-kernel | `jackmaiorino/mtg-kernel` `058d075e3`, Rust 1.94.1, release build of `examples/bench_kernel` with gorge's `--xengine-json` mode, 2026-10-20 |
    | Measured time | 20 CPU-s per gorge and dirac run, 15 per gorge Burn run; 40 s warm-up and 60 CPU-s per XMage and Forge run; 30 CPU-s per mtg-kernel run |
    | Errors | no stalled, unfinished or failed game in any gorge, XMage, Forge or mtg-kernel row. dirac's random row counts 385 stalled games of 215,042 and its bot row 39 of 234,124, all on pair A and all at a declared capacity (the stack and pending-effect tables); they stay in the rows' counts. XMage's base AI on the Burn mirror never casts its burn spells and hit the 100-turn cap every game, so that row is left out |
