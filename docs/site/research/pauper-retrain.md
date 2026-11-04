# Pauper retrain

*An MLP trained from scratch on the SpellBench Pauper pool, 2026-11-02 → 11-04*

Does a net trained from scratch on the eight SpellBench pauper-bo1 decks beat M_CTLX, the FDN-trained MLP that scored 1446 [1392, 1511] on the [SpellBench board](spellbench-score.md) at 400 sims?

!!! abstract "Verdict"
    **Yes, adopted as the Pauper champion.** Each of g1–g6 beat its predecessor by a CI that clears 0.5, and g6 leads M_CTLX on the sb-g6 board (1572.8 vs 1459.3, overlapping intervals; head to head p 0.1153). On the latency-capped sb-fresh board it is level with gorge-bot. Run 2, on the post-fix engine, moved the title once more to g8 (0.5292 vs g6). It was stopped during g9 self-play at 2026-11-04T00:12Z; g8 is the champion.

    Not done at the time of writing: gorge-bot was not added to run 2's gate set, and no SpellBench board has been run at the run-2 build.

<figure markdown="span">
--8<-- "research/figs/pauper.svg"
</figure>

## Setup

=== "Pool"

    Eight 60-card decks with 112 distinct card names: Wildfire, Rally, Affinity, Elves, Spy, Burn, CawGates, Faeries. They come from SpellBench 404c4c0's pauper kernel deck list. Each deck's card multiset was checked against the materialised mainboard in mtg-kernel's runtime deck file (sha256 68e7602f…), the file the benchmark engine deals from. Every deck is in the `train` split. The search's belief pool is the pool itself.

    **dirac support** at gorge ae85be31d: 112 of 112 lowered, 0 refused, 24 supported with evidence, 88 unverified. Every deck holds an unverified card, so the run allows unverified cards.

=== "Recipe"

    M_CTLX's recipe, changed only where the pool required it:

    - Self-play: `kplay`, 200 sims, `:explore`, one hour per generation, seed 9800+n, 16 workers (12 in run 2). Two mulligans with the constructed mulligan policy.
    - g1 has no net: uniform prior and the heuristic leaf.
    - Pack in format 1. Train the MLP, 602,370 parameters, config `{"d":64,"ff":128,"layers":1,"d_emb":32,"card_d":32,"head_hidden":64,"policy_layers":2,"value_layers":1}`, lr 3e-4, 3,000 steps, the last 3 packs, cq target with c-scale 0.03, value blend 0.5. g1 trains from scratch; later generations start from the previous best.
    - Gate: gN against the champion, 1,000 pairs (2,000 games) at 100 sims, seed 78. The title moves when the 95% lower bound clears 0.5. References: sb-tactical and the no-net search, 250 pairs (500 games) each.
    - Stop on request, at 60 generations, after 72 hours, or after 8 generations without a title. Then a final pooled retrain from scratch, 60,000 steps on the last packs.
    - Play spec: `az:sims=N:autopay:oppnodes:oppfull:morekinds:closedlist:topk=6:topk=6:cpuct=0.5:gonet=<path>.dzgw`.

=== "Builds"

    The loop ran as a capped systemd user unit (MemoryMax 48G, CPUQuota 1600%, Nice 10), with each build's revisions recorded beside its binaries.

    - g1 on gorge c92b6f401.
    - g2–g6 and gF on gorge e177d8565, which fixed a kernel autopay panic seen in ~0.16% of g1 games, always with Spy.
    - Run 2 (g7 on) on gorge cf0487049, with one private-module build throughout.

## Results

Champion 0 is M_CTLX. Its references: vs sb-tactical 0.5807 [0.5590, 0.6022] over 2,000 games; vs no-net 0.6130 [0.5696, 0.6547] over 500. sb-tactical vs no-net scores 0.5160 [0.4722, 0.5595] over 500.

| Gen | Build | vs champion (2,000 games, 95% CI) | Title | vs sb-tactical (500) | vs no-net (500) |
|---|---|---|---|---|---|
| g1 | c92b6f401 | vs M_CTLX 0.5630 [0.5412, 0.5846] | <span class="gg-dot gg-dot--full" title="moves"></span><span class="gg-sr">moves</span> moves | 0.6040 | 0.6780 |
| g2 | e177d8565 | vs g1 0.5530 [0.5311, 0.5747] | <span class="gg-dot gg-dot--full" title="moves"></span><span class="gg-sr">moves</span> moves | 0.6640 | 0.7120 |
| g3 | e177d8565 | vs g2 0.5415 [0.5196, 0.5632] | <span class="gg-dot gg-dot--full" title="moves"></span><span class="gg-sr">moves</span> moves | 0.6960 | 0.7480 |
| g4 | e177d8565 | vs g3 0.5517 [0.5299, 0.5734] | <span class="gg-dot gg-dot--full" title="moves"></span><span class="gg-sr">moves</span> moves | 0.7340 | 0.7740 |
| g5 | e177d8565 | vs g4 0.5467 [0.5249, 0.5685] | <span class="gg-dot gg-dot--full" title="moves"></span><span class="gg-sr">moves</span> moves | 0.7620 | 0.7880 |
| g6 | e177d8565 | vs g5 0.5363 [0.5143, 0.5580] | <span class="gg-dot gg-dot--full" title="moves"></span><span class="gg-sr">moves</span> moves | 0.8020 [0.7648, 0.8346] | 0.8200 [0.7839, 0.8512] |
| gF (pooled, 60k steps) | e177d8565 | vs g6 0.4840 [0.4621, 0.5059] | <span class="gg-dot gg-dot--none" title="stays"></span><span class="gg-sr">stays</span> stays with g6 | 0.7800 | 0.8000 |
| g7 | cf0487049 | vs g6 0.5208 [0.4988, 0.5426] | <span class="gg-dot gg-dot--none" title="stays"></span><span class="gg-sr">stays</span> stays with g6 | 0.8080 | 0.8020 |
| g8 | cf0487049 | vs g6 0.5292 [0.5073, 0.5510] | <span class="gg-dot gg-dot--full" title="moves"></span><span class="gg-sr">moves</span> moves | 0.8060 [0.7691, 0.8383] | 0.8180 [0.7818, 0.8494] |

Self-play per generation in run 1: g1 37,235 games and 1,003,648 records (95 stalls from the autopay panic); g2 22,521 / 931,141; g3 25,012 / 897,363; g4 23,196 / 902,868; g5 19,860 / 723,729. Run 2: g7 16,928 games and 612,514 records with 0 stalls; g8 15,467 / 542,555. The g7 buffer still held two packs from the old engine (g5, g6).

Every generation took the title for six in a row, and the score against sb-tactical rose from 0.6040 to 0.8020 over g1–g6. The pooled retrain did not beat warm-started g6. The g6–g8 rows are dated 2026-11-03T05:21Z–23:38Z; run 2 reprinted the g1–g6 reference rows on resume with identical numbers.

### SpellBench boards

=== "sb-g6, 2026-11-02"

    640 games, 64 per matchup:

    | Bot | Elo | 95% CI |
    |---|---|---|
    | az-pg6-400 (Pauper g6, 400 sims) | 1572.8 | [1489.8, 1673.8] |
    | az-mctlx-400 | 1459.3 | [1386.3, 1551.0] |
    | az-nonet-100 | 1395.2 | |
    | heuristic | 1149.6 | |
    | uniform | 1000 | |

    Head to head, az-mctlx-400 vs az-pg6-400 went 24-40: 0.3750 [0.2500, 0.5156], sign test p 0.1153.

=== "sb-fresh, 2026-11-03"

    960 games, with gorge-bot at 5b979d0b3 and the latency cap `-az-decision-ms 1200`, which makes the search budget depend on wall time and so is not deterministic:

    | Bot | Elo | 95% CI |
    |---|---|---|
    | az-pg6-400 | 1418.5 | [1359.2, 1488.4] |
    | gorge-bot | 1414.2 | [1350.8, 1486.1] |
    | az-nonet-100 | 1378.2 | |
    | az-mctlx-400 | 1359.4 | |
    | heuristic | 1198.1 | |

    pg6 vs mctlx 0.5625 [0.4531, 0.6719]; pg6 vs gorge-bot 0.5000 [0.3750, 0.6250].

The sb-g6 run's own notes describe it but do not record its result; the figures above are from the board's leaderboard.
