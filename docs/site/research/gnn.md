# A GNN against the MLP

*The overnight results of 2026-10-24, and what would make a GNN pay*

Every number is a self-paired play gate on the pool17k eval decks, seed 78 unless noted, `az:sims=100` on both sides unless noted. ± is the 95% interval (4,000 games ±0.0155, 2,000 games ±0.022) unless a row says otherwise.

!!! abstract "Bottom line"
    1. **The GNN plays better at equal simulations, reproducibly.** G_M (2.1M parameters, d128, 2 global layers) against the same-data 602,370-parameter MLP scored **0.5302** and **0.5282** on two seeds (pooled 0.529 over 8,000 games, ~5σ). G_MX, the same shape trained on 2.4× the data, scored **0.5266** against the production reference g26.
    2. **It loses at equal time on CPU, every time it was tried.** The cheapest GNN costs 7.5× per searched move and the best costs 18×, while MLP search keeps paying out to at least 1600 sims. G_M@100 vs MLP@400: **0.4855**; G_S@100 vs MLP@800: **0.464**.
    3. **No cheap way to use it worked.** GNN at the root only: 0.50. GNN as leaf value only: 0.507. GNN as priors only: 0.515. Three distillation variants into the MLP: 0.496 / 0.500 / 0.497.
    4. **What does pay:** **search** (+0.038 per 4× sims up to 1600, flat 1600 → 3200), and **the v2 entity block** (+0.019, now 2.4σ). The highest-value work is making simulations cheaper (lazy priors, batching, the 41% spent stepping), not a bigger network.

<ul class="gg-facts">
  <li><strong>0.5302</strong> G_M vs the MLP at equal sims</li>
  <li><strong>0.4855</strong> G_M at 100 sims vs the MLP at 400, equal time</li>
  <li><strong>18×</strong> G_M's cost per searched move</li>
</ul>

## 1. The GNN

| Question | Result |
|---|---|
| Full evaluator, equal sims | G_S (0.93M) **0.5139**; G_M (2.1M) **0.5302**, seed 79 **0.5282**; G_BIG (8.26M) **0.5205** |
| …with 2.4× data (g2–g8, both sides) | G_BIGX **0.5341**; G_MX **0.5210** |
| …vs production g26 | G_MX **0.5266** |
| Cost per searched move (vs the MLP) | G_S 7.5×, G_M 18×, G_BIG 96× (AVX-512 Go forward) |
| Equal time | G_M@100 vs MLP@400 **0.4855** (251 vs 69 ms); G_S@100 vs MLP@800 **0.464** (128 vs 188 ms) |
| Root only (prior + root value) | G_BIG 0.5031, G_M 0.4994, G_S 0.4989 |
| Leaf value only (MLP priors) | 0.5074 (5.4× cost) |
| Priors only (MLP value) | 0.5149 (15× cost) |
| Distil G_BIGX into the MLP | prior under cq 0.4964; pure teacher policy 0.5001; teacher value only 0.4974 |

The equal-sims rows are against the 602,370-parameter MLP trained on the same data: M_CTL, and M_CTLX for the 2.4×-data arms.

**Reading.**

- The edge is about +0.03. The priors carry more of it than the value, neither half alone delivers it, and the 18× cost is mostly the 2–3 prior calls per simulation.
- Size beyond 2.1M does not help (G_BIG < G_M).
- More data helps the 8M net a little (+0.014, ~1.2σ) and the 2M net not at all.
- Distillation improved holdout loss (student policy CE 0.606 vs 0.625) and play not at all: the sixth loss/play inversion in this line.
- **Verdict:** the graph representation is real, but on CPU it is outbid by search. It is worth revisiting only with batched GPU inference (`EvalBatch` exists, and batched leaf evaluation with virtual loss merged that night) at ≥10× lower cost per eval.

## 2. Other questions settled that night

| Question | Result | Verdict |
|---|---|---|
| Does the from-scratch MLP loop pass g26? | g4 vs g26 0.4708; g7 vs g26 0.4923; g7 vs g4 0.5194; g8 vs g7 0.4908 | It catches up with g26 by g7 and flattens. |
| From-scratch MLPs vs g26 | M_CTL 0.4951, M_CTLX 0.5028 | Level with g26. |
| v2 entity block | 8,000 games per arm vs g26: F_V2 0.4594, F_NONE 0.4402: **+0.019 ±0.008**, where ±0.008 is one standard error of the difference, not a 95% interval | Real (~2.4σ). Keep it. |
| `:worlds=8:nodecache=64` (4.4× faster per sim) | equal sims 0.4173; equal time 0.4923 | No. |
| The 82% "replay share" | it counts replayed tree edges; time is stepping 41%, priors 27%, the heuristic bot 16% | Per-node snapshots are impossible (one world per sim); lazy priors is an exact −10.5%. |

## 3. Search scaling (M_CTL against itself)

| Sims A vs B | Score | ms per searched move, A vs B |
|---|---|---|
| 150 vs 100 | 0.5126 ±0.0155 | 8.4 vs 5.2 |
| 400 vs 100 | 0.5384 ±0.0154 | 70 vs 14 |
| 800 vs 400 | 0.5255 ±0.022 | 121 vs 54 |
| 1600 vs 400 | 0.5375 ±0.022 | 344 vs 70 |
| 3200 vs 1600 | 0.499 ±0.022 | 1488 vs 672 |

The earlier note "flattens past ~400" (g26@400 vs @100 = 0.535) does not hold for this net: 1600 still beats 400 by +0.038. It does flatten **between 1600 and 3200** (0.499). The useful range is 100 to 1600 sims (~+0.076 over 16×), so a search 2–4× cheaper per sim converts straight into strength at today's 100–200 sim budgets.

## 4. Engineering done that night

- In gorge, on an unmerged branch: the GraphNet Go forward, with PyTorch parity to 1e-6; an AVX-512 GEMM, 35 → 1.66 ms per G_BIG eval; and a bit-exact multi-state `EvalBatch`. A GraphNet Go forward has since landed in gorge as [`bots/gonet/gnn.go`](https://github.com/adams-shaun/gorge/blob/b14b8a543b1ae0d401af3bf6c3e2d5ebde2c52af/bots/gonet/gnn.go).
- Also done: the net evaluator loads gnn `.dzgw` files; batched leaf evaluation with virtual loss; lazy priors (the exact −10.5% µs/sim).

**Recommended next:** profile stepping (41% of search CPU), since search scaling says every exact µs is strength; spend the extra speed as sims in self-play and gates (the loop runs at 200 sims, and 400+ is worth +0.03 or more); park the GNN line with this page as the record, and reopen it only for a GPU-batched evaluator; keep the v2 entity block. It landed in gorge on 2026-10-25 as a26426691 and a2df098cc, and the kernel encoder went to width 100.

## What would make a GNN pay

*A note written the same day, 2026-10-24.* What has to change *structurally* for a graph or entity net to beat the 602,370-parameter MLP in play, rather than in holdout loss? This combines the maintainers' measurements with the published record. Claims marked *(inference)* are ours, not a source's.

### What was measured

| Fact | Number | Source |
|---|---|---|
| G_BIG GraphNet (d256 h8 gl4 ff1024), 8.26M params, one uncached Eval | **35 ms**, 1 core, 0 allocs | `BenchmarkGNNEval`, on the unmerged Go-forward branch |
| d=128 GraphNet | 6.4 ms | same |
| MLP d=64, one uncached Eval | ~0.37 ms; in search far less (parts cache, 51% eval-cache hits) | same; a generation's summary |
| G_BIG time split | global layers 58% (FFN 47%), local pass 34%, option heads 5%; 81% in scalar `dot4` | pprof |
| G_BIG in play | ~2 min/game vs ~19 games/s for the MLP: **~1000× per game** | smoke gates |
| Holdout, same packs (sploop g5–g7), same training budget and targets | G_BIG polCE **0.5849** / vLL 0.4262; M_CTL MLP 0.6453 / 0.4346 | training logs |
| Prior GNN (10-19), parameter-matched | −0.66% polCE, play gate 0.507 | the 10-19 study ([net width](net-width.md#the-gnn-question-was-already-answered)) |
| Search time (CPU profile, sims=200, g26) | stepping 41%, network priors 27%, heuristic bot 16%, new world 4%, leaf value 3%. (The 0.82 replay share counts replayed tree edges, not time) | the lazy-priors specification |

The G_BIG loss gap (−9.4% polCE) is 14× the 10-19 effect. Five loss-vs-play inversions in this line mean that is still not evidence; a 400-game gate of G_BIG against M_CTL at equal sims was the first play number. Because the GNN costs ~1000× per game, an equal-*time* comparison was hopeless as built.

### Why the GraphNet is the wrong shape for this search

1. **Nothing is reusable across tree nodes.** The MLP is cheap in search because the evaluator caches each card's vector by version and sums parts, so a sibling node pays only for the cards that changed. That is NNUE's idea. GraphNet's global attention makes every token depend on every other one, so one changed card re-runs the whole net, and the evaluator must use the materialised (uncached) path for it.
2. **The graph is a fixed pooling tree, not relations.** The edges are root → player → zone → card → leaf. Cards relate to each other only through generic global attention. The relations that matter in Magic (what a stack item targets, who blocks whom, what is attached to what, controller ≠ owner) are neither edges nor features: the relational gap. The graph has nothing to reason about that the MLP's pooled bag lacks *(inference: this is why the graph effect was 0.66% when parameter-matched)*.
3. **Most leaves aren't entities.** ~150–190 sparse-bag rows each become a leaf with a full K/V projection under the root. That is about half the local-pass cost, spent on features the MLP handles with one bag sum.
4. **Counting goes through softmax attention.** "How many untapped creatures" and "power vs life" are weighted *means* under attention. Mean and attention pooling cannot count multiplicities (GIN, Xu et al. 2019); sum pooling and explicit scalars can.
5. **The option head already gets most of a pointer head's benefit.** Options are scored action-as-input with the embeddings of their entities A/B (DouZero-style). The graph's main published advantage, attending over entities to pick actions, is mostly present already *(inference)*.

### What the literature says

- **Where entity attention won, three things held:**
    - actions defined over entities (pointer or query-key heads: AlphaStar, the DORA decoder, Lc0/Chessformer from-to heads);
    - relational or positional bias injected into attention. In Lc0 that was worth "+50% effective size" for a 10% throughput loss: relations beat depth;
    - data in the 10⁷–10⁹ range with batched GPU inference.

    DORA (Diplomacy) is the cleanest ablation inside a search agent: a transformer beat a 46.8M GraphConv with 9.3M params (Elo 1041 vs 898).
- **The strongest imperfect-information card agents don't use entity transformers.** ReBeL (MLP over beliefs), DouZero (action-as-input MLP + LSTM), DeepNash (conv), ByteDance Hearthstone/LOCM (LSTM + masked autoregressive head). The Hearthstone gains were all training-side.
- **Loss is a known bad strength metric for AlphaZero nets** (AlphaZero scaling laws, App. F). The five inversions found here are expected, not an anomaly.
- **Latency trades directly against simulations** (Jones 2021). A net 24× slower per eval must be worth ~24× the sims. No source shows an entity net clearing that bar without batching.
- **Mixed-net search works.** MPV-MCTS (NoGo) runs a small net for most simulations and a big net on the most-visited leaves. At equal compute that scored 527 Elo, vs 472 for the big net alone and 323 for the small one.
- **Grow, don't restart** (KataGo: 6×96 → 20×256, each larger net trained alongside on the same data and promoted only when it wins).
- Cheaper levers with measured multipliers (KataGo): auxiliary targets ~1.65×; playout-cap randomisation ~1.37×; global pooling ~1.6×; Gumbel root selection and its improved-policy target, for low simulation counts.

### The structural changes, ranked

Ranked by expected play payoff per unit of work.

1. **Cut redundant search work.** Per-node snapshots cannot be exact: every simulation redeals a fresh world, so a snapshot is used exactly once. Two levers instead:
    - lazy priors: skip the network prior at replayed points whose priors are never read. Exact (byte-identical games), −10.5% µs/sim, forward passes 2.89 → 1.86 per sim;
    - `:worlds=8:nodecache=64`: PIMC over 8 fixed deals with an exact node-state cache, 4.4× faster per sim, but it changes the search (and lost its gates; see §2).
2. **Batched leaf evaluation with virtual loss.** That turns GEMV into GEMM, and a GPU inference path becomes possible. Without it, nothing above ~1M params competes on time.
3. **Put the big net only where it's evaluated rarely.** A GNN prior at the root and the MLP everywhere in the tree; then MPV-style, the GNN also evaluates the top-N most-visited leaves. The cost is one 35 ms eval per move instead of one per simulation, so the GNN can be gated at a fair cost.
4. **Distil the GNN into the MLP.** Train the GNN offline on pooled multi-generation packs, where a big net has enough data, and add its policy and value as soft targets for the MLP. The search keeps its cheap evaluator and inherits the representation.
5. **Redesign the net to be cache-friendly ("pointer-lite")**, if the graph earns a place in play:
    - an expensive **per-card encoder**, context-free, cached by card version exactly as the evaluator caches MLP card vectors (NNUE-style);
    - **one cheap global step**: sum/mean/max pool → broadcast (O(n·d²), not attention's O(n²·d)), KataGo's global pooling;
    - **real relational inputs** as features or attention bias: targets, blockers, attachments, controller. Lc0's finding says relations beat depth;
    - a **query-key option head** over card keys;
    - **sum-pooled count channels** and explicit scalars for counts;
    - drop sparse-bag rows as leaves and feed the bag as one summed token. Pre-norm on leaves also blocks precomputing their K/V as table lookups, so make leaves norm-free;
    - small d (32–64) with rich relations, not d=256 with 8 heads.
6. **Training-side levers** (independent of architecture, and the ones the card-game literature actually credits): Gumbel root + improved-policy target at 100–200 sims; auxiliary targets (final life differential, turns to end, the opponent's next action); playout-cap randomisation.
7. **Measure on play at equal wall time.** Loss is recorded, never used to choose.

What would have changed the plan: G_BIG vs M_CTL ≥ 0.55 at equal sims would have said the representation is worth having (go to #3 and #4 first, and #5 only if those cash); ≈ 0.5 would have made the 9% loss edge another inversion (drop GNN-as-evaluator; spend on #1, #2 and #6, with relational features added to the MLP). The 4,000-game gates in §1 put G_BIG at 0.5205 and G_M at 0.5302.

### Sources

DORA <https://arxiv.org/abs/2110.02924> · Lc0 transformers <https://lczero.org/blog/2024/02/transformer-progress/> · Chessformer <https://arxiv.org/abs/2605.19091> · AlphaStar <https://www.nature.com/articles/s41586-019-1724-z> · Relational RL <https://arxiv.org/abs/1806.01830> · OpenAI Five <https://arxiv.org/abs/1912.06680> · ReBeL <https://arxiv.org/abs/2007.13544> · DouZero <https://arxiv.org/abs/2106.06135> · DeepNash <https://arxiv.org/abs/2206.15378> · Hearthstone/LOCM <https://arxiv.org/abs/2303.05197> · MPV-MCTS <https://arxiv.org/abs/1905.13521> · KataGo <https://arxiv.org/abs/1902.10565> · AlphaZero scaling laws <https://arxiv.org/abs/2210.00849> · Jones, scaling with board games <https://arxiv.org/abs/2104.03113> · Gumbel MuZero <https://openreview.net/forum?id=bERaNdoegnO> · searchless chess <https://arxiv.org/abs/2402.04494> · GIN <https://arxiv.org/abs/1810.00826> · counting in transformers <https://arxiv.org/abs/2407.15160> · NNUE <https://www.chessprogramming.org/NNUE>

Not independently verified: AlphaStar layer sizes (secondary sources) and the Chessformer numbers (fetched by a survey pass, not re-read).
