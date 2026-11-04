# SpellBench v2 score on the Pauper pool

*2026-11-02 → 11-03*

Where does the FDN-trained M_CTLX search bot stand on SpellBench's Pauper board (pauper-bo1, eight decks, 112 distinct cards) against gorge's rule-based bots, and does a Pauper-trained net change that?

!!! abstract "Verdict"
    M_CTLX at 400 sims came first at 1446, but the top five entries are not separated, and it lost its direct match with gorge-bot. At 100 sims the net adds little over the same search with no net (p 0.30): most of its strength on this pool comes from search. This motivated the [Pauper retrain](pauper-retrain.md).

## Setup

Run `sb-score`, 2026-11-02 02:53–07:12: 1,792 games, all rated, `validate: OK`; SpellBench 404c4c0, mtg-kernel 92ec2393, stats seed 20261102; caps 400% CPU, 8G, GOMAXPROCS=2, 4 workers. Elo is anchored at uniform = 1000; intervals are the arena's CI95, a paired bootstrap over seat-swapped pairs. [SpellBench v2 agents](../bots/spellbench.md) describes how gorge's bots sit a SpellBench seat.

## Results

<div class="gg-bars" style="--gg-bar-label: 11rem">
<div class="gg-bar" role="img" title="az-mctlx-400: 1446.0 Elo (1392–1511)" aria-label="az-mctlx-400: 1446.0 Elo (1392–1511)"><span class="gg-bar__label" aria-hidden="true">az-mctlx-400</span><span class="gg-seg gg-seg--ok" style="--w:89.2%"></span></div>
<div class="gg-bar" role="img" title="az-mctlx-100: 1416.5 Elo (1364–1478)" aria-label="az-mctlx-100: 1416.5 Elo (1364–1478)"><span class="gg-bar__label" aria-hidden="true">az-mctlx-100</span><span class="gg-seg gg-seg--ok" style="--w:83.3%"></span></div>
<div class="gg-bar" role="img" title="sb-search-lite-atk: 1408.9 Elo (1352–1474)" aria-label="sb-search-lite-atk: 1408.9 Elo (1352–1474)"><span class="gg-bar__label" aria-hidden="true">sb-search-lite-atk</span><span class="gg-seg gg-seg--unv" style="--w:81.8%"></span></div>
<div class="gg-bar" role="img" title="gorge-bot: 1392.1 Elo (1339–1457)" aria-label="gorge-bot: 1392.1 Elo (1339–1457)"><span class="gg-bar__label" aria-hidden="true">gorge-bot</span><span class="gg-seg gg-seg--unv" style="--w:78.4%"></span></div>
<div class="gg-bar" role="img" title="sb-tactical: 1392.1 Elo (1338–1455)" aria-label="sb-tactical: 1392.1 Elo (1338–1455)"><span class="gg-bar__label" aria-hidden="true">sb-tactical</span><span class="gg-seg gg-seg--unv" style="--w:78.4%"></span></div>
<div class="gg-bar" role="img" title="az-nonet-100: 1368.0 Elo (1312–1430)" aria-label="az-nonet-100: 1368.0 Elo (1312–1430)"><span class="gg-bar__label" aria-hidden="true">az-nonet-100</span><span class="gg-seg gg-seg--ok" style="--w:73.6%"></span></div>
<div class="gg-bar" role="img" title="heuristic: 1201.4 Elo (1146–1263)" aria-label="heuristic: 1201.4 Elo (1146–1263)"><span class="gg-bar__label" aria-hidden="true">heuristic</span><span class="gg-seg gg-seg--unv" style="--w:40.3%"></span></div>
<div class="gg-bar" role="img" title="uniform: 1000 Elo (the anchor)" aria-label="uniform: 1000 Elo (the anchor)"><span class="gg-bar__label" aria-hidden="true">uniform</span><span class="gg-seg gg-seg--unv" style="--w:0%"></span></div>
</div>
<ul class="gg-legend gg-bars__key"><li><i class="gg-seg gg-seg--ok"></i>az search</li><li><i class="gg-seg gg-seg--unv"></i>rule-based or baseline</li><li>bar length is Elo above the uniform anchor, 1000 to 1500</li></ul>

| Bot | Elo | CI95 |
|---|---|---|
| az-mctlx-400 (M_CTLX, 400 sims) | 1446.0 | 1392–1511 |
| az-mctlx-100 | 1416.5 | 1364–1478 |
| sb-search-lite-atk | 1408.9 | 1352–1474 |
| gorge-bot | 1392.1 | 1339–1457 |
| sb-tactical | 1392.1 | 1338–1455 |
| az-nonet-100 | 1368.0 | 1312–1430 |
| heuristic | 1201.4 | 1146–1263 |
| uniform | 1000 | — |

az-mctlx-400 went 295-153 over 448 games with a 3.53% fallback rate. Head to head (64 games each):

| Match | Result | Score | p |
|---|---|---|---|
| mctlx-100 vs nonet-100 | 37-27 | 0.578 (0.469–0.688) | 0.30 |
| mctlx-400 vs nonet-100 | 45-19 | 0.703 | 0.002 |
| mctlx-400 vs gorge-bot | 30-34 | 0.469 | — |

Only 6 of the pool's 112 distinct cards appear in the FDN deck pool.

The Pauper-trained net's boards (sb-g6, sb-fresh) are on the [Pauper retrain](pauper-retrain.md#spellbench-boards) page: pg6 led sb-g6 at 1572.8 [1489.8, 1673.8], then was level with gorge-bot (1418.5 against 1414.2) on the fresh, latency-capped board.
