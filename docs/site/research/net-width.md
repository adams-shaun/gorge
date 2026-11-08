# Net width and the encoding

*The f84x investigation, 2026-10-22 → 10-23*

Find the configuration in which the az net actually gets **stronger**, measured only by self-paired gates: never by holdout loss, which this line shows to be anti-correlated with strength.

!!! abstract "Verdict"
    - **Width does not help.** The 2,287,618-parameter W128 MLP beats the 601,858-parameter W64 on every holdout metric and loses about 2 points to it in play (0.4514 against 0.4723 vs g26).
    - **From scratch on pooled packs cannot reproduce g26.** Every from-scratch arm lands at 0.45–0.49 against a root the self-play loop built by iteration.
    - **The v2 entity block** (colour, pips, category) was written, found dead twice, fixed, and kept.
    - **Holdout loss anti-predicts strength**, five times in this line.

## Standing decisions

- **The benchmark is our own past performance.** Two axes, both self-paired:
    - `vs prev`: the new net against the net it was trained from (g26 root). On the hard pool this calibrates to exactly 0.5000, so it is unbiased.
    - `vs g7`: against the f84 g7 anchor net, for comparability across arms.
- **Holdout policy CE, value logloss and AUC are not progress metrics.** The arm with the best holdout numbers (B) was the worst player.
- **`sbt` is retired as a gate axis.** It saturates (~0.79) and cannot resolve a 0.01 effect. `bot` is stronger and 2.4× faster (measured below).

## 1. What was built and measured, 2026-10-22 → 10-23

### Deck pools

All verified with the supported-pool check, 0 dropped.

| Pool | Decks | Train / eval | Note |
|---|---|---|---|
| **pool17k** | 17,305 distinct FDN mainboards | 15,574 / 1,731 | from the 17lands replay; every win-rate bucket 0.48–0.92; 4,610 not in the old set |
| **hard70** | 2,080 | 1,872 / 208 | bucket ≥ 0.70 |
| **hard66** | 6,561 | 5,904 / 657 | bucket ≥ 0.66 |
| 31k | 31,010 (deduped) | — | the pre-existing set |

### The harness

Each arm runs self-play → pack → train → 3 gates → census, with the variables set per arm: the targets, the game budget, the replay window, gate size and opponent, search sims, the self-play and gate pools (a plain heuristic-bot opponent as a control) and the net architecture. A **fixed game count per generation** is recommended, because it makes generations comparable regardless of search cost. The tooling is not in this repository.

### Gate measurements

**Fallback bots** (g26, 100 sims, pool17k eval, seed 78):

| Fallback | Score | ±95% | n | Turns p50 | Wall |
|---|---|---|---|---|---|
| sbt | 0.7885 | 0.0179 | 2000 | 18 | 156 s |
| hbot | 0.8650 | 0.0193 | 1200 | 16 | 66 s |
| **bot** | **0.9279** | 0.0146 | 1200 | 16 | 65 s |

**Search depth** (8,000 games): g26@400 vs g26@100 = **0.5351 ± 0.0109**. Deeper search is stronger, but only +0.035 for 4× compute. The note at the time said it "flattens past ~400"; the later M_CTL curve showed that does not hold ([search](search.md#3-win-rate-against-simulations)).

### Experiments run, and what each proved

| Run | Change | Result | Verdict |
|---|---|---|---|
| **A** (7 gens) | self-play warm start, pool17k | `vs g7` 0.6178 → 0.5994 0.6068 0.5965 0.5895 0.5951 0.6005 0.6011 | a small consistent **loss** vs the root |
| **B** (7 gens) | self-play **vs bot** | `vs g7` 0.6178 → … → **0.5509**; `vs bot` rose to 0.9356; holdout AUC 0.907 → 0.914 | **worst player, best-looking curves**: holdout is anti-correlated |
| **S0** | from scratch, all 26 packs (1.79M records) | `vs g7` **0.5606**, `vs root` **0.4860**, `vs bot` 0.9118 | from scratch is **worse** than the root; the warm start was not the problem |
| **H4** (5 gens) | 400-sim self-play, 2,000 games/gen, **hard70** gate | `vs prev` 0.5000 / 0.5008 / 0.4914 / 0.5110 / 0.4951 | flat at parity; hard decks change nothing |
| **sched/1k** | short training schedule | `vs g26` 0.4968, best_step **74** | parity |
| **sched/lr1e4-6k** | lower learning rate, long schedule | `vs g26` 0.4985, best_step **148** | parity |
| **sched/6k-pat10** | longer early-stopping patience | `vs g26` 0.5010, best_step **74** | parity |
| **sched/lr3e4-6k** | long schedule, baseline learning rate | ~0.500, best_step **74** | parity |
| **R1** | relational slots scaled ×20 | `vs g26` **0.4930** | nothing; those slots are genuinely sparse |

!!! note "The decisive observation"
    Across 40+ runs, **the warm-started trainer peaks at best_step 74–259 of a 6,000-step schedule, and every schedule converges to identical holdout numbers** (polCE ≈ 0.611, vlogl ≈ 0.419, vauc ≈ 0.883). Six independent hypotheses were eliminated: pool, search depth, deck quality, warm start vs scratch, game budget, training schedule. The common factor is the **model and its inputs**, not the loop.

**Feature finding.** Measured on the g26 pack (1.48M entity rows): the 16 relational slots (68–83 of `card_raw`) have **mean |value| 0.0007 vs 0.0874** for the base 68 slots and are non-zero on **0.09%** of rows. Slots 72–77 are never non-zero. That is consistent with R1's null result: they encode genuinely rare events and contribute nothing.

**The width argument.**

- `d: 64` is the width **after all pooling**; `card_d: 32` and `d_emb: 32` serve ~34k cards. The library default is `d:384, ff:1024, layers:3, d_emb:128, card_d:256`: the nets ran at 1/40th of the intended width, chosen early for speed and never revisited.
- Missing from the encoding: **colour** (only inferable from a name hash), **structured mana cost** (one scalar, `mv/5`), **rules text** (only a hashed API presence flag), **card category** (removal/ramp/trick/bomb; the spell-class census never reached the features), set, rarity and tribes.
- So names are opaque hashes and the net **cannot generalise across cards**. That was the most plausible reason 40 runs all converged.

**The wide-net run (R2S0).** `d_emb 128, card_d 128` (2,287,618 parameters, 3.8×) from scratch on all 26 packs, on a short schedule: **best_step = the last step**, the first run in the whole line that had not already peaked. It was not yet a result: that schedule is under two epochs. And `netsimd.Load` refused it (`shape d=128 ...`), so it could not be evaluated yet.

## 2. Why the SIMD net blocked this

`bots/netsimd` refused anything but `d == 64 && hh == 64 && e == 32 && cd == 32 && ff%32 == 0 && layers == 1`. Its AVX-512 kernels are hand-specialised to 64 lanes (`dot64`, `ln64`, `addv64`, 4×zmm blocks, 16-lane `hsum16`).

The chosen approach: keep the fast 64/32 path untouched and add a generic fallback for every other shape. The generic kernels already implemented every operation at arbitrary width, and their tests asserted them bit-identical to the assembly, so a wide net was *correct* on the scalar path. That was enough to **measure** whether width helps before spending days on assembly. The work: make the vector width a load-time field instead of a constant, relax the loader's shape check and route other shapes to the generic kernels (logging once), keep the 64/32 path as is, and optionally support more than one layer. Then re-gate R2S0 on the scalar path, at about 5× the gate cost.

The plan that followed, in order: unblock width, then gate R2S0 vs the root. If it beat 0.50, run a long wide run; if not, enrich the encoding (colour, mana pips, card category, names composed from features so unseen cards generalise), in the trainer first. Port the hot kernels to arbitrary width only once a configuration had won.

### Ideas parked, with the evidence against them

| Idea | Status |
|---|---|
| Deep self-play corpus (2,000 sims) | gated on depth: 400 vs 100 is only +0.035, so the corpus is weak value for 10× the cost |
| 17lands human imitation | the replay CSV has **per-turn aggregates only**, no action traces: no lines to imitate |
| plat+ deck subset | `opp_rank` is empty in every row, and win rate is flat across rank (matchmaking), so rank ≠ skill. The 114k builds are still a usable *deck pool*, not a corpus of skilled lines |
| two-opponent packs | already the case: ~99% of f84 games record both seats (the same net on both sides), so there is nothing extra to extract |
| duplicate-state pruning in search | **not implemented**: the tree is a PUCT/ISMCTS tree with a state *cache*, no transposition table. Safe only within a determinised world; across worlds it would leak information |
| clairvoyant / perfect-information training | needs a separate encoder variant (the entity walk covers my hand but not the opponent's), which breaks byte-parity with `policynet` that the oracle's `FeatCheck` asserts. Diagnostic only; cannot ship |
| Possibility Storm "mate in N" puzzles | a promising correctness oracle and a deterministic perft-style gate; needs a position format. Not scoped |

Lessons that cost time: a warm start cannot change `value_layers`, `layers`, `card_d` or `d_emb`, so any capacity change means from scratch; the play driver must write pack format 1, or every training run fails.

## Addendum, 2026-10-23: width is dead; from scratch is the real variable

Controlled arms, g26's own training recipe, from scratch on all 26 f84 packs, gated on pool17k vs g26:

| Arm | e = cd | Parameters | Holdout polCE | Holdout vlogl | vs g26 |
|---|---|---|---|---|---|
| W64 | 32 | 601,858 | 0.6828 | 0.4452 | **0.4723 ± 0.0155** |
| W128 | 128 | 2,287,618 | 0.6683 | 0.4419 | **0.4514 ± 0.0154** |
| R2S0 (earlier, different recipe) | 128 | 2,287,618 | 0.647 | 0.419 | 0.4696 ± 0.0155 |

1. **Width does not help.** W128 beats W64 on every holdout metric and loses ~2 points to it in play. The sign is the same as every earlier holdout-vs-strength inversion (arm B). One controlled arm, 4,000 games each.
2. **R2S0's loss was never about its recipe.** The control arm with g26's recipe loses the same ~3 points. The suspected training-recipe confound does not exist.
3. **From scratch on the f84 packs cannot reproduce g26 at all.** S0 0.4860, R2S0 0.4696, W64 0.4723, W128 0.4514: all ~0.45–0.49 against a root the self-play loop built. g26 came from *iterated* net-vs-net play with fresh packs each generation; a single schedule over the pooled corpus does not reproduce it. With pool, search depth, deck quality, game budget, training schedule and now width eliminated, the bottleneck moves off architecture: any feature-encoding or GNN work needs a training regime that first reproduces the baseline.
4. **Two latent bugs, found and fixed**, both corrupting any wide net:
    - `SumRows` panicked on widths other than 32 and 64. R2S0's e=128 hit it: 3,991 panics, reported as a 7-second "0.4989" run before the stall count was read.
    - `bagAVX512` silently treated any width other than 64 as 32 in its fallthrough, reading past each row. It is now gated to widths 32 and 64, with the generic kernel otherwise.

## Addendum, 2026-10-23 overnight: the GNN already aborted; features made real

### The GNN question was already answered

An earlier study (2026-10-19) ran the GNN properly: 4 arms × 3 seeds, a paired bootstrap over games, pre-registered thresholds, from scratch on g1–g2 (618,551 records).

| Arm | Policy CE | Value LL |
|---|---|---|
| C0 snapshot MLP | 0.75659 | 0.51835 |
| T1 +history MLP | 0.75502 | 0.51857 |
| C1 snapshot GNN | 0.75159 | 0.51889 |
| T2 +history GNN | 0.75153 | 0.51872 |

Graph effect: −0.66% policy CE (3/3 seeds), a thirtieth of the design's own 1% "worth it" bar, with a small value cost. Verdict: **abort**. Temporal history: −0.21% policy CE, no value. The report also records that an earlier, larger GNN beat the MLP on loss and then played a **flat play gate** (0.507 over 400 games).

Given this line's own four holdout-vs-strength inversions (arm B, W64, W128, R2S0), a 0.66% CE delta that did not move a play gate was not worth a Go forward pass on its own. The report named the real headroom, in order: make the search cash information (the root is prior-locked; the hand is worth 2 points at 100 sims and 5 at 1,600), and give the redeal the opponent's behaviour. Both are search and belief changes with measured payoff curves, and both are cheaper than an architecture. ([GNN](gnn.md) has what happened when the GNN was gated in play anyway.)

### The v2 entity block: written, found dead, fixed

Colour (84..89), pip counts (90..95) and a coarse category (96..99), appended above the relational block so nothing below moves; `EntityRawWidth` 84 → 100, with `EntityRawWidthRelEnd = 84` pinning the relational range. Both encoders carry it: gorge's `bots/policynet` produces the training data, and the kernel's encoder mirrors it.

Two bugs, both silent, both found only by inspecting packed data:

1. **Nothing wrote the block in gorge's policynet.** The offsets went into both encoders, but the value-writing went only into the kernel's. Every pack carried 16 zero columns. Caught by packing a real game and counting non-zero lanes in `card_raw[:, 84:100]`: 0 of 260 rows.
2. **`relCard` wiped it.** It cleared `raw[EntityRawWidthV1:]`, correct when that slice was the whole remainder and fatal once the block was appended. `relSig` had the same over-broad slice in its cache key.

!!! tip "Why the test exists"
    Both bugs left the encoder's own tests green. The block was written correctly and then cleared; the offsets were right and the values were never produced. `TestV2BlockFiresAndMatchesGorge` now asserts, over three real games (43,513 cards), that each group has non-zero lanes and that every one of the 16 slots equals gorge's encoder (0 mismatches). A pack of zero columns trains fine and reports a flattering loss, so nothing weaker would catch it.

**sploop** then started: the self-play loop from random weights, on the v2 binary, 8 generations, gates vs prev (1,000 pairs) and vs bot. It answers the outstanding hypothesis, that from-scratch training can't reproduce g26 because g26 came from *iteration*, and produces v2 packs for the feature A/B at the same time. The A/B (F_V2 vs F_NONE on identical packs, zeroing `card_raw[:, 84:100]`) could not run on the f84 packs, which are 84 wide.

## Addendum, 2026-10-23 evening: the crash, the feature result, the bottleneck

### Seven hours lost to one constant

The first sploop died an hour in. The trainer still expected 84-wide raw rows, so it rejected the 100-wide shards the v2 binary had recorded, and the following export failed on the missing checkpoint. Roughly 7.5 h were lost. The fix: the trainer's raw width went 84 → 100, with the relational block's end named separately; narrower shards are zero-padded, so a 68- or 84-wide pack still reads; and the relational-sparsity scale covers only the relational columns, since applying it to the dense colour one-hots would have squashed them.

It was verified end to end before relaunch: pack → train → export → `netsimd.Load` (`RAW_W=100`, `fast=true`) → the play driver plays it with **0 errors, 0 stalls**. The first attempt's 100-wide pack was not wasted: 99,279 of 100,000 rows carry non-zero v2 values, so it could feed the feature A/B directly. And an empty initial-net setting could not mean "from scratch", because the same empty value was g0's gate net; from scratch became its own switch, applied to generation 1 only.

### The feature A/B: the v2 block earns its place

One pack, two nets, same seed, schedule and config, differing only in whether `card_raw[:, 84:100]` is visible. F_NONE is the same shard with those 16 columns zeroed, so the game data, search and targets are identical.

| Arm | vs g26 (4,000 games) | Holdout polCE | Eval polCE |
|---|---|---|---|
| **F_V2** | **0.4632 ± 0.0155** | 0.6931 | 0.9286 |
| **F_NONE** | **0.4401 ± 0.0154** | 0.6929 | 0.9238 |

**+0.0231 for the v2 block (~1.05σ)**, and note the sign flip: F_NONE is the *better* model on every loss number and the *worse* one in play, the **fifth** independent confirmation in this line that holdout loss anti-predicts strength. One sigma is suggestive, not conclusive. The larger gate that settled it, +0.019 ±0.008 (one standard error) over 8,000 games per arm, is on [GNN](gnn.md#2-other-questions-settled-that-night).

### The bottleneck: replay, not training

Per 1 h generation: self-play 3,600 s, pack ~120 s, **train 27 s**, gates ~200 s. The loop is simulation-bound by ~130×, and the GPUs sit at **0%** except during those 27 seconds.

The g3 generation's summary decomposes its 42.9 µs per simulation:

| Signal | Value |
|---|---|
| µs per sim | 42.9 (930 sims/s aggregate) |
| **replay share of play** | **0.82** |
| net evals per sim | 2.82 |
| net cache hit rate | 0.51 |
| leaf plies, mean | 6.08 |
| unavailable candidates per sim | 20.0 |
| environment steps per sim | 0.77 |

The reading at the time: 82% of every simulation is replaying stored actions to reach the leaf, not searching; second, 2.8 net evals per sim where 1 should do, half of them cache hits; third, ~20 candidate actions per sim rejected as unavailable inline. A later profile corrected the first point: the 0.82 counts replayed tree *edges*, not time, and time goes to stepping (41%), priors (27%) and the heuristic bot (16%) ([GNN](gnn.md#2-other-questions-settled-that-night)).

### sploop at hand-off

Relaunched from scratch with the v2 binaries: 200 sims, 1,000-pair gates, no g7 axis. 2,000 games per gate:

| Gen | vs bot | vs prev |
|---|---|---|
| g0 (reference) | 0.9307 ±0.0111 | 0.5050 ±0.0310 (null) |
| g1 | 0.9230 ±0.0117 | **0.4582 ±0.0218** |
| g2 | 0.9293 ±0.0112 | **0.5115 ±0.0219** |
| g3 | 0.9277 ±0.0113 | **0.5195 ±0.0219** |

g1's dip is the cold-start generation (random weights on reference-net data); from g2 the series sits above 0.5 and rising, the first *rising* self-paired series in the whole line. It did not yet show the loop *exceeds* g26: its absolute level, ~0.93 vs bot, is g26's. It caught up with g26 by g7 and then flattened ([GNN](gnn.md#2-other-questions-settled-that-night)).
