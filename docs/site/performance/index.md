# Performance

Part of [Engines](../engines/index.md).

How fast gorge and dirac play Magic, and how they compare with XMage, Forge and mtg-kernel on one machine, the same
decks and one CPU core each. This page is for anyone choosing an engine to host games or generate self-play data.
[How it is measured](methodology.md) has the method and the commands, [gorge's speed](engine.md) has its benchmark rows
and profile, and [Bot speed](bots.md) has the cost per decision and per simulation.

!!! abstract "The short version: one core, FDN limited decks"
    Measured 2026-11-02 at `b78e4c8b1` for gorge and dirac, and 2026-10-20 for XMage, Forge and mtg-kernel, which weren't re-run.

    - **gorge plays about 70 random-policy games per core-second**, against 2.4 for XMage.
    - **With each engine's own AI on both seats, a gorge game costs 9-12 ms**, against about 1 s for Forge and for XMage's
      default AI. The AIs aren't the same kind of player, so that is the price of a game with the bot each engine ships.
    - **dirac plays about 1,800 random-policy games per core-second**, with
      2.7 µs a transition, and plays
      only its [supported cards](../architecture/dirac/index.md#what-it-supports), for two players.
    - **mtg-kernel**, a public Rust engine with a few hundred cards, plays about as many games per core-second as gorge on its
      home Burn mirror and is 5-11× slower per turn on the FDN pair it plays.
    - **Memory:** a game-playing process peaks at about 350-400 MB for gorge with the whole corpus loaded,
      about 100 MB for dirac, 0.9 GB for Forge, 1.2-1.3 GB
      for XMage with a 2 GB heap cap and 11-16 MB for mtg-kernel, with its cards compiled in.

## The numbers

Each row is the median of six runs: FDN pairs A and B, three reps each. The spread is the slowest and fastest run. Turns count
both players' turns. Compare µs per decision only within one engine, because each engine splits a game into decisions differently.

<figure markdown>
<svg viewBox="0 0 760 330" width="100%" role="img" aria-label="Bar chart, log scale: CPU microseconds per turn for each engine and policy">
<text class="dg-text-bold" x="0" y="16">CPU microseconds per turn (log scale; lower is cheaper)</text>
<rect class="dg-box-solid" x="460" y="6" width="12" height="12" rx="2"/><text class="dg-text-muted" x="476" y="16">random legal action</text>
<rect class="dg-box-accent" x="610" y="6" width="12" height="12" rx="2"/><text class="dg-text-muted" x="626" y="16">engine's own AI</text>
<line class="dg-line-dashed" x1="210.0" y1="28" x2="210.0" y2="308"/>
<text class="dg-text-muted" x="210.0" y="322" text-anchor="middle">10</text>
<line class="dg-line-dashed" x1="306.0" y1="28" x2="306.0" y2="308"/>
<text class="dg-text-muted" x="306.0" y="322" text-anchor="middle">100</text>
<line class="dg-line-dashed" x1="402.0" y1="28" x2="402.0" y2="308"/>
<text class="dg-text-muted" x="402.0" y="322" text-anchor="middle">1,000</text>
<line class="dg-line-dashed" x1="498.0" y1="28" x2="498.0" y2="308"/>
<text class="dg-text-muted" x="498.0" y="322" text-anchor="middle">10,000</text>
<line class="dg-line-dashed" x1="594.0" y1="28" x2="594.0" y2="308"/>
<text class="dg-text-muted" x="594.0" y="322" text-anchor="middle">100,000</text>
<line class="dg-line-dashed" x1="690.0" y1="28" x2="690.0" y2="308"/>
<text class="dg-text-muted" x="690.0" y="322" text-anchor="middle">1,000,000</text>
<g><title>dirac · random: 23.1 µs/turn (min 21.6, max 24.3)</title>
<text class="dg-text" x="202" y="48" text-anchor="end">dirac · random</text>
<rect class="dg-box-solid" x="210" y="34" width="34.9" height="20" rx="3"/>
<line class="dg-line" x1="242.1" y1="44" x2="246.9" y2="44"/>
<text class="dg-text" x="252.9" y="48">23.1</text></g>
<g><title>gorge · random (auto-pay): 577 µs/turn (min 567, max 614)</title>
<text class="dg-text" x="202" y="76" text-anchor="end">gorge · random (auto-pay)</text>
<rect class="dg-box-solid" x="210" y="62" width="169.0" height="20" rx="3"/>
<line class="dg-line" x1="378.4" y1="72" x2="381.6" y2="72"/>
<text class="dg-text" x="387.6" y="76">577</text></g>
<g><title>gorge · random (manual): 293 µs/turn (min 279, max 311)</title>
<text class="dg-text" x="202" y="104" text-anchor="end">gorge · random (manual)</text>
<rect class="dg-box-solid" x="210" y="90" width="140.8" height="20" rx="3"/>
<line class="dg-line" x1="348.7" y1="100" x2="353.3" y2="100"/>
<text class="dg-text" x="359.3" y="104">293</text></g>
<g><title>XMage · random: 13,193 µs/turn (min 12,774, max 13,751)</title>
<text class="dg-text" x="202" y="132" text-anchor="end">XMage · random</text>
<rect class="dg-box-solid" x="210" y="118" width="299.6" height="20" rx="3"/>
<line class="dg-line" x1="508.2" y1="128" x2="511.3" y2="128"/>
<text class="dg-text" x="517.3" y="132">13,193</text></g>
<g><title>dirac · heuristic bot: 29.1 µs/turn (min 25.0, max 33.8)</title>
<text class="dg-text" x="202" y="160" text-anchor="end">dirac · heuristic bot</text>
<rect class="dg-box-accent" x="210" y="146" width="44.6" height="20" rx="3"/>
<line class="dg-line" x1="248.2" y1="156" x2="260.8" y2="156"/>
<text class="dg-text" x="266.8" y="160">29.1</text></g>
<g><title>gorge · bot (auto-pay): 658 µs/turn (min 621, max 702)</title>
<text class="dg-text" x="202" y="188" text-anchor="end">gorge · bot (auto-pay)</text>
<rect class="dg-box-accent" x="210" y="174" width="174.6" height="20" rx="3"/>
<line class="dg-line" x1="382.1" y1="184" x2="387.2" y2="184"/>
<text class="dg-text" x="393.2" y="188">658</text></g>
<g><title>gorge · bot (manual): 520 µs/turn (min 483, max 543)</title>
<text class="dg-text" x="202" y="216" text-anchor="end">gorge · bot (manual)</text>
<rect class="dg-box-accent" x="210" y="202" width="164.7" height="20" rx="3"/>
<line class="dg-line" x1="371.6" y1="212" x2="376.6" y2="212"/>
<text class="dg-text" x="382.6" y="216">520</text></g>
<g><title>XMage · simple AI: 761 µs/turn (min 743, max 791)</title>
<text class="dg-text" x="202" y="244" text-anchor="end">XMage · simple AI</text>
<rect class="dg-box-accent" x="210" y="230" width="180.6" height="20" rx="3"/>
<line class="dg-line" x1="389.6" y1="240" x2="392.2" y2="240"/>
<text class="dg-text" x="398.2" y="244">761</text></g>
<g><title>XMage · mad AI: 62,343 µs/turn (min 49,069, max 101,733)</title>
<text class="dg-text" x="202" y="272" text-anchor="end">XMage · mad AI</text>
<rect class="dg-box-accent" x="210" y="258" width="364.3" height="20" rx="3"/>
<line class="dg-line" x1="564.3" y1="268" x2="594.7" y2="268"/>
<text class="dg-text" x="600.7" y="272">62,343</text></g>
<g><title>Forge · Forge AI: 54,513 µs/turn (min 45,904, max 71,387)</title>
<text class="dg-text" x="202" y="300" text-anchor="end">Forge · Forge AI</text>
<rect class="dg-box-accent" x="210" y="286" width="358.7" height="20" rx="3"/>
<line class="dg-line" x1="561.5" y1="296" x2="579.9" y2="296"/>
<text class="dg-text" x="585.9" y="300">54,513</text></g>
<line class="dg-line" x1="210" y1="28" x2="210" y2="308"/>
</svg>
<figcaption>CPU µs per turn, log scale, FDN pairs A and B pooled, median of 6 runs, thin line = min-max. gorge and dirac 2026-11-02, XMage and Forge 2026-10-20. Filled bars: random legal action. Outlined bars: the engine's own AI.</figcaption>
</figure>

| Engine · policy | games / core-s | ms / game | µs / turn | µs / decision | turns / game | peak RSS (MB) | measured |
|---|--:|--:|--:|--:|--:|--:|---|
| dirac · random | 1,800 <small>(1,660-1,920)</small> | 0.557 | 23.1 <small>(21.6-24.3)</small> | 4.54 | 24 | 100 | 2026-11-02 |
| gorge · random (auto-pay) | 69.8 <small>(64.0-77.1)</small> | 14.3 | 577 <small>(567-614)</small> | 24.2 | 24 | 375 | 2026-11-02 |
| gorge · random (manual) | 83.7 <small>(80.9-85.6)</small> | 11.9 | 293 <small>(279-311)</small> | 10.7 | 41 | 356 | 2026-11-02 |
| XMage · random | 2.38 <small>(2.16-2.58)</small> | 420 | 13,193 <small>(12,774-13,751)</small> | 499 | 32 | 1,228 | 2026-10-20 |
| dirac · heuristic bot | 1,960 <small>(1,650-2,280)</small> | 0.511 | 29.1 <small>(25.0-33.8)</small> | 5.65 | 18 | 97 | 2026-11-02 |
| gorge · bot (auto-pay) | 84.6 <small>(79.7-90.0)</small> | 11.8 | 658 <small>(621-702)</small> | 26.6 | 18 | 353 | 2026-11-02 |
| gorge · bot (manual) | 107 <small>(103-115)</small> | 9.36 | 520 <small>(483-543)</small> | 18.6 | 18 | 396 | 2026-11-02 |
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

| dirac benchmark, 2026-11-02 | ns per op, median of 5 | instructions per op | allocations |
|---|--:|--:|--:|
| Transition, Foundations (FDN) | 2,748 | 45,604 | 0 |
| Transition, Bloomburrow (BLB) | 2,909 | 51,990 | 0 |
| Transition, FDN whole-pool table | 3,601 | 60,463 | 0 |
| `CopyFrom`, mid-game FDN position | 40.2 | 187 | 0 |

These ran on one pinned core at a 1-minute load average of 13 on the box. Time per operation moves with the load, the machine and
code layout. Instructions per operation is the steadier number, and
[How it is measured](methodology.md#dirac) shows how to get it.

| gorge row, 2026-10-31 | µs per op, no GC | µs per op, with GC | bytes per op | mallocs per op |
|---|--:|--:|--:|--:|
| `clone`: `Engine.Clone` of a mid-game position | 22.7 | 45.3 | 200 KB | 51 |
| `step`: clone, then submit one candidate | 74.9 | 114 | 389 KB | 153 |

## A second workload: decks mtg-kernel plays

[mtg-kernel](https://github.com/jackmaiorino/mtg-kernel) is a public Rust rules kernel (MIT) built for reinforcement learning on a fixed card
pool. It can't play FDN pair B, so it isn't in the table above. It plays two workloads: **FDN pair A**, which is exactly its two FDN
fixture decks, and the **Burn mirror**, the mono-red Pauper deck from its own pool. It has no standalone bot, so it has random rows only. No
dirac pool holds the Burn deck.

| Workload · engine · policy | games / core-s | µs / turn | turns / game | peak RSS (MB) | measured |
|---|--:|--:|--:|--:|---|
| Pair A · dirac · random | 1,860 | 23.6 | 23 | 104 | 2026-11-02 |
| Pair A · gorge · random (auto-pay) | 74.8 | 580 | 23 | 397 | 2026-11-02 |
| Pair A · gorge · random (manual) | 85.0 | 279 | 42 | 356 | 2026-11-02 |
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
- **Load differs.** The box is shared. gorge's and dirac's rows ran at a 1-minute load average of 12-14, while XMage and
  Forge ran at 4-15. Cache and memory contention move these numbers, and a paired A/B separates the code's share from the load's
  ([gorge's speed](engine.md#comparing-two-revisions)).
- **Policies differ.** XMage's random row answers non-priority asks with its base AI, and the AIs differ in kind. Each row is spelled out in
  [the policies](methodology.md#the-player-policies).
- **JVM numbers are steady state** after 40 s of warm-up games. Start-up isn't counted.

## Provenance

| | |
|---|---|
| Machine | AMD Ryzen 9 9950X (16 cores, 32 threads), 120 GB, Ubuntu 26.04 |
| gorge | `b78e4c8b1` (2026-11-02), go1.26.3, `cmd/enginebench` |
| dirac | `b78e4c8b1`, `dirac/cmd/dplay` and `go test -bench`, with `-trimpath` |
| XMage | `magefree/mage` `6b602a1c85` (1.4.61), Temurin JDK 17.0.20.1, driver `XEngineBench` (gorge-compliance), 2026-10-20 |
| Forge | gorge's Forge fork at `da1c712c31`, JDK 17.0.20.1, driver `forge.oracle.GameReplay`, 2026-10-20 |
| mtg-kernel | `jackmaiorino/mtg-kernel` `058d075e3`, Rust 1.94.1, release build of `examples/bench_kernel` with gorge's `--xengine-json` mode, 2026-10-20 |
| Measured time | 20 CPU-s per gorge and dirac run, 15 per gorge Burn run; 40 s warm-up and 60 CPU-s per XMage and Forge run; 30 CPU-s per mtg-kernel run |
| Errors | no stalled, unfinished or failed game in any gorge, XMage, Forge or mtg-kernel row. dirac's random row counts 385 stalled games of 215,042 and its bot row 39 of 234,124, all on pair A and all at a declared capacity (the stack and pending-effect tables); they stay in the rows' counts. XMage's base AI on the Burn mirror never casts its burn spells and hit the 100-turn cap every game, so that row is left out |
