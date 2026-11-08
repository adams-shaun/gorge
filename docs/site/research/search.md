# Search: kill tests, the benchmark, and scaling

*2026-10-01 → 10-24*

How much does tree search buy, which knobs matter, and does it keep paying as simulations grow? Three bodies of evidence answer it: the 2026-10-01 kill tests on the first az-redeal search, a replication of DraftZero's search benchmark (agreement with strong human players), and the win-rate scaling curves measured later on the dirac kernel.

!!! abstract "Verdict"
    - Search strength grows with simulations up to about 1600 and is flat from 1600 to 3200 (M_CTL); about +0.038 per 4× in the useful range.
    - The benchmark of agreement with human decisions is flat in simulations and does not measure playing strength.
    - Adopted play-time settings: cpuct 0.5 with root LCB, and sims 400 or more where the time budget allows. Making them the defaults was left as a separate decision.
    - Because more sims keep paying, an exact speed-up per simulation converts directly into strength. That is why the GNN, at 7.5–96× the cost per move, lost at equal time ([architecture](architecture.md#3-the-gnn)).

## 1. Kill tests, 2026-10-01 → 10-03

!!! warning "Records lost"
    The primary results files no longer exist and are not in the archive. What survives is the summary row gorge kept on its [Bot speed](../performance/bots.md) page before the research bots moved out (the page as of the parent of 55ce18b20).

| Bot | Result | Cost |
|---|---|---|
| `az-redeal` (MCTS, heuristic leaf) | 25 sims: 64.7% vs `bot`; 100 sims: 70.5%; loses to `sb-tactical` (32.2%); 400 vs 100 sims +4.4 points; exploration constant has no effect | 65.8 ms per decision at 25 sims |
| `sb-search-lite-atk(-fl)` | 64.1% vs `sb-tactical`; fitted leaf +3.71 points held-out (n=1024, 2026-10-03) | 0.16–0.22 s per decision |
| `search` (PIMC teacher seat) | 53.7% vs 50.4% control (+3.3 points), 2026-09-22; the edge is early-game only | 664 ms mean / 2.2 s p95 per decision |

The 400-vs-100 gain was measured "on a binary that predates the perf merge", at 4.5× the wall time. The fitted leaf is gorge 1de6b2bf2, "fitted logistic leaf (LeafFitted)".

The per-arm game counts and intervals of the cpuct sweep and the fitted leaf's dev/held-out split exist only in a maintainer's note, not in a surviving file; they are listed under [records missing](index.md#records-missing).

**Verdict.** cpuct did not matter for that search (at that time); the fitted leaf paid on held-out seeds and stayed in `bots/sbsearch`; more sims paid (+4.4 points for 4×).

## 2. The search benchmark replication, 2026-09-30 → 10-01

DraftZero experiment #3 (upstream `b1e0ba68`, MIT) asked how much search helps, which kind is best, and whether search must see hidden cards, scored by agreement with top 17lands players' decisions. The maintainers replayed it on gorge with gorge's search.

=== "Setup"

    - 1,000 test decisions from 558 games and 300 dev decisions from 172 games, with quotas spell 375/110, hold 125/40, attack 300/90, block 200/60.
    - Scores: A_set (macro agreement) and the balanced score (mean balanced accuracy of cast/hold, attack/not, block/not), which the report calls "the score to trust". CIs are a 95% game-cluster bootstrap, 1,000 resamples.
    - The search: the tree holds only the searching seat's searched decisions, botpolicy plays the opponent, PUCT c = 0.5 on [0,1], uniform priors, the heuristic leaf (`searchprobe.LeafValue`), no network arm, final choice by most visits.
    - Arms: no search, PIMC-1 and PIMC-4, IS-MCTS and a clairvoyant ceiling, each over a grid of simulation counts, with discount 0.99 per ply and a leak check at 3,000 sims over 16 seeds.

=== "Caveat"

    These numbers came from an unmerged branch (built at 707711116). On the merged engine, a 200-item rerun changed 4–6% of rows for clairvoyant and PIMC and 25% for IS-MCTS.

Balanced score, ours (upstream offline), by simulations; each cell's CI is about ±0.033:

| Method | 100 | 300 | 1,000 | 3,000 | 10,000 |
|---|---|---|---|---|---|
| clairvoyant MCTS | 0.609 (0.567) | 0.607 (0.591) | 0.602 (0.613) | 0.602 (0.627) | — |
| PIMC, 1 world | 0.630 (0.578) | 0.614 (0.589) | 0.617 (0.597) | 0.631 (0.620) | 0.633 (0.631) |
| PIMC, 4 worlds | 0.645 (0.576) | 0.630 (0.602) | 0.629 (0.603) | 0.615 (0.622) | — |
| IS-MCTS | 0.618 (0.591) | 0.621 (0.585) | 0.616 (0.614) | 0.622 (0.623) | 0.637 (0.636) |

No search: gorge bot 0.607 [0.577, 0.637]; upstream's rule heuristic 0.557.

| Upstream finding | Here |
|---|---|
| Fair search ≥ clairvoyant | <span class="gg-dot gg-dot--full" title="replicated"></span><span class="gg-sr">replicated</span> PIMC-1 − clairvoyant at 3k: +4.0 (+0.6, +7.4) A_set points |
| More search helps | <span class="gg-dot gg-dot--none" title="not replicated"></span><span class="gg-sr">not replicated</span> flat from 100 to 10,000 for every method and score |
| Method matters little | <span class="gg-dot gg-dot--full" title="replicated"></span><span class="gg-sr">replicated</span> IS-MCTS costs 2.8–3.6× PIMC-1 |
| Searches are over-active | <span class="gg-dot gg-dot--full" title="replicated"></span><span class="gg-sr">replicated</span> attack 62% vs humans 44%, block 60% vs 30% |
| Disagreements are near-ties | <span class="gg-dot gg-dot--none" title="not replicated"></span><span class="gg-sr">not replicated</span> median Q gap 0.150 (n = 408) vs upstream 0.043 |

The leak test (3,000 sims, 16 seeds): clairvoyant search fails 5 of 6 pairs (for example counterspell −0.372 [−0.476, −0.268]); PIMC-1, PIMC-4 and IS-MCTS show ΔQ +0.000 on every pair in 16 of 16 seeds, so the fair searches do not leak.

Source: the benchmark replication report, removed from gorge's tree in 01caa6d37.

**Verdict.** Search beats no search (PIMC-1 − no search at 3k: +9.2 (+5.3, +13.2) A_set points), but agreement with humans does not grow with simulations. The report's untested explanations are the heuristic leaf and the scripted opponent inside the path. The later win-rate curves below, which use a trained leaf and opponent nodes, do grow.

## 3. Win rate against simulations

All rows below are self-paired play gates with seats swapped. They differ in net, engine and opponent, so compare within a table, not across.

<figure markdown="span">
--8<-- "research/figs/search-scaling.svg"
</figure>

**g7 vs sb-tactical on the dirac kernel** (the az-c6 g7 net, the kernel's stand-in fallback bot of 2026-10-18; ±0.017 at 3,000 games):

| Sims | Score | Games | µs/sim |
|---|---|---|---|
| 25 | 0.564 ±0.018 | 3000 | 45.4 |
| 100 | 0.618 ±0.017 | 3000 | — |
| 400 | 0.681 ±0.017 | 3000 | 63.0 |
| 1600 | 0.702 ±0.028 | 1000 | 64.4 |

With gorge's heuristic bot ported to the kernel, the same curve on the xp scale: sims 25 0.658, 100 0.708, 400 0.754, 1600 0.793 (3,000 games per point).

=== "g7 against itself"

    on the dirac kernel, 1,500 pairs = 3,000 games:

    | A vs B | Score | Wall |
    |---|---|---|
    | 400 vs 100 | 1682-1318 = 0.5607 | 1409 s |
    | 800 vs 100 | 1766-1234 = 0.5887 | 2627 s |
    | 2000 vs 100 (1,200 games) | 0.623 ±0.027 | — |

    "Twenty times the search is worth about 12 points."

=== "M_CTL against itself"

    The f84x line; 4,000 games ±0.0155 or 2,000 games ±0.022:

    | Sims A vs B | Score | ms per searched move, A vs B |
    |---|---|---|
    | 150 vs 100 | 0.5126 ±0.0155 | 8.4 vs 5.2 |
    | 400 vs 100 | 0.5384 ±0.0154 | 70 vs 14 |
    | 800 vs 400 | 0.5255 ±0.022 | 121 vs 54 |
    | 1600 vs 400 | 0.5375 ±0.022 | 344 vs 70 |
    | 3200 vs 1600 | 0.499 ±0.022 | 1488 vs 672 |

    Source: [GNN](gnn.md#3-search-scaling-m_ctl-against-itself).

=== "Search knobs"

    The xp series, paired vs sb-tactical, 3,000 games per arm, points ±95%:

    | Knob | Gain |
    |---|---|
    | cpuct 0.5 + root LCB at sims 100 | +5.0 ±1.4 |
    | cpuct 0.5 at sims 100 | +3.0 ±1.3 |
    | cpuct 0.5 at sims 400 | +2.7 ±1.3 |
    | cached worlds K=2 at equal wall | +3.2 ±1.3 |
    | sims 400 → 1600 | +3.8 ±1.3 |
    | sims 1600 → 6400 | +1.5 ±2.0 |

!!! note "Two cpuct findings, two searches"
    cpuct did nothing for the 2026-10-01 search (heuristic leaf, no opponent nodes) and paid +3 points for the 2026-10-19 search (net prior and leaf, opponent nodes). The two findings are about different searches.
