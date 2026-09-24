# pn14: exploration, per-card entity encoding, outcome-trained PPO (2026-09-24)

Brief: `.superpowers/briefs/policynet-exit/14-explore-entity-outcome.md`.
Raw results (every round's eval/collect/stats JSON, benches, timings, the
summariser): `.ds4/policynet-exit/14/` in the pn14 worktree
(`results.md` is the full arm × round table).

## What was built

- **Stochastic collection.** `seat.PolicyNetBot.SetSampling(T, seed)`:
  priority and target sample from softmax(score/T) over the scored action
  space; attackers and blockers sample a per-option Bernoulli of
  σ(score/T), then go through the same legality repair as the vote. The
  seat's own PCG stream is seeded from the per-game seat seed. The recorder
  logs `Sampled`, `Temperature` and the behaviour log-probability of the
  answer actually played (`policynet.TemperedLogProb`).
  `botbench -policynet-temperature T` turns it on; the default 0 is the
  existing greedy seat, byte for byte.
- **Opponent mix.** `botbench -opp-mix explore:0.25`: per game, as a pure
  function of the seed, side B plays another built-in policy. The new
  `explore` policy is `seat.NewExploreBot`.
- **Corpus and PPO.** The on-policy records gain optional `temperature` and
  `logp_behaviour` fields, plus entity `cards` and `ent_a`/`ent_b`. All are
  omitted when unset. When a record carries `logp_behaviour`, the PPO ratio
  is π_new/π_behaviour; the KL anchor still reads π_old. The corpus hash now
  follows the checkpoint's feature set. Before this change an mz corpus was
  stamped with the v1 hash. `policytrain` refuses a corpus whose feature set
  differs from `-init`'s.
- **`entity` feature set.** It adds one vector per visible card on top of mz:
  - 68 raw scalars per card: types, current P/T, 31 keywords + other,
    tapped, sick, can-attack/can-block, attacking/blocking/blocked,
    counters, attached/attached-to-mine/hosts, controller, zone, castable,
    mana value, instant speed, face down;
  - hashed name/API rows in the shared table.
  - A learned per-card encoder h_c = relu(W·[raw ‖ E[name]] + b) (K=32)
    feeds a sum+max pool per group (my battlefield / opponent battlefield /
    my hand / stack), which is projected into the state trunk. The value
    head sees it too.
  - Each option's hidden input gains h(own card) ‖ h(related card): the
    blocked attacker, or the attacked planeswalker or battle.
  - The backward pass is hand-derived and pinned by a finite-difference
    test.
  - Checkpoint schema 4 is written for entity models only; v1/mz models are
    still written as schema 3.
- **Warm start.** `policytrain -upgrade-entity K` turns an mz checkpoint into
  an entity one with the pooled projection and the new option columns at
  zero. It scores exactly as the mz source: 200 greedy games were
  identical. `policytrain -set-residual w` provides the residual-prior
  ablation.
- **Loop.** `exitloop -mode ppo -collect-temp T [-collect-temp-final Tf]
  -opp-mix …`: a linear temperature anneal over the collection rounds. Eval
  is always greedy.
- **Readouts** (policytrain stats, per kind): `sampled`, `off_greedy` (the
  played answer ≠ π_old's greedy answer), `mean_p_beh` and `greedy_flip`
  (share of decisions whose deployed answer the round changed).

Determinism, verified:
- The same flags and seed give a byte-identical sampled corpus, including
  across worker counts 1/3/20, and a byte-identical checkpoint and stats.
- The v1 greedy corpus, bench report and PPO checkpoint are byte-identical
  to the pre-pn14 binaries.
- A supervised mz checkpoint is byte-identical to the pre-pn14 trainer's.

## Setup

- **Seed.** mz supervised on pn11 gen-0 oracle labels (5,430; `-residual-init
  2 -value-weight 1`). The entity seed is that checkpoint upgraded with K=32.
- **Rounds.** Every arm runs 10 ungated rounds. Each round:
  - collect 2,000 games against `bot`, with 25% of games against `explore`;
  - train: PPO `-epochs 4 -lr 0.1 -clip 0 -batch 64 -value-weight 0.5
    -ppo-clip 0.2 -ppo-kl 0.1`;
  - eval: 4,000 greedy games on the fixed block (seed 90,000,000), seats
    traded, `-policynet-admission sign`, `-policynet-kinds
    attackers,priority`.
- **Control.** Bot vs bot on the eval block: **50.95% [49.4, 52.5]**
  (±1.55pp).
- **Step-size calibration** (one round, same corpus). With pn13's defaults
  (`-clip 1 -lr 0.05`) the round's KL(π_old‖π) is 0.0006, and priority's is
  0.00002: the global gradient clip binds on every batch and the policy
  barely moves. `-clip 0 -lr 0.1` gives KL ≈ 0.003–0.05 per round and keeps
  the value head stable. `-lr 0.2 -clip 0` collapsed the value head to the
  base rate.

## Results (arm × round, eval block)

| Arm | Round 0 | Mean r1–10 | Best (round) | Round 10 | Attackers deviation % | Priority deviation % | Attackers greedy-flip %/round | Priority greedy-flip %/round | Value ll median (base) |
|---|---|---|---|---|---|---|---|---|---|
| mz, greedy | 51.42 | 50.23 | 51.42 (r0) | **49.28** | 6–17 | 0–23 | 5.8 | 4.40 | 0.407 (0.683) |
| mz, T=1 | 51.42 | 51.45 | 51.70 (r3) | 51.32 | 26–41 | 22–26 | 7.5 | **0.00** | 0.366 (0.685) |
| mz, T 2→0.5 | 51.42 | 50.95 | 51.65 (r10) | 51.65 | 27–51 | 6–57 | 6.1 | 0.43 | 0.432 (0.695) |
| entity, greedy | 51.42 | 50.81 | 51.70 (r2) | **49.65** | 2–15 | 0–18 | 8.1 | 5.01 | 0.383 (0.681) |
| entity, T=1 | 51.42 | 51.50 | 51.78 (r6) | 51.62 | 25–28 | 23–26 | 3.7 | **0.00** | 0.384 (0.689) |
| entity, T 2→0.5 | 51.42 | 50.98 | 51.48 (r9) | 51.35 | 15–49 | 5–56 | 2.5 | 0.42 | 0.441 (0.692) |
| mz, T=1, residual 0.5 | 49.30 | 50.24 | 51.40 (r8) | 50.92 | 51–58 | 49–60 | 13.9 | 1.70 | 0.418 (0.696) |
| entity, T=1, residual 0.5 | 49.30 | 50.27 | 50.90 (r8) | 50.58 | 33–57 | 49–61 | 10.3 | 1.55 | 0.430 (0.694) |

- **Deviation %** is the share of the collected decisions whose played answer
  differs from the bot's.
- **Greedy-flip** is the share of decisions whose deployed (greedy) answer
  the round's update changed.
- **Value ll** is the holdout log loss of V(s) on the round's own games. The
  base-rate predictor is in parentheses. The init head sits at 0.8–1.06 on
  round 1 because it was fitted to teacher states.

**Held-out bench** (seed block 1,000,000; 1,000 games per pair × 5 pairs =
5,000 games; seats traded; sign admission; the same seeds for every line):

| Seat | Win rate [95% CI] |
|---|---|
| bot vs bot (control) | 50.18 [48.79, 51.57] |
| mz seed (round 0) | 50.24 [48.85, 51.63] |
| **entity, T=1, round 6 (best eval of any arm)** | **50.54 [49.15, 51.93]** |
| entity, T=1, round 10 | 50.44 [49.05, 51.83] |
| mz, greedy PPO, round 10 | 48.50 [47.11, 49.89] |
| mz seed with residual 0.5 (round 0) | 48.30 [46.91, 49.69] |
| mz, T=1, residual 0.5, round 10 | 50.30 [48.91, 51.69] |
| entity, T=1, residual 0.5, round 10 | 50.14 [48.75, 51.53] |

**Throughput at 2 vCPU** (`taskset -c 12,28`, 2 workers, T=1, one round of
500 collect + 1,000 eval games):

| Stage | mz | entity |
|---|---|---|
| Collect | 118k games/h (15.3 s) | 113k games/h (15.9 s) |
| Train | 7.8 s | 13.2 s |
| Eval | 129k games/h (27.8 s) | 123k games/h (29.2 s) |
| **Round total** | **51 s** | **58 s** |

At full scale (2,000 collect + 4,000 eval, six arms sharing 22 cores):
- a round took about 2 min;
- mean train time was 38 s (mz) and 56 s (entity), single-threaded;
- each 10-round arm took 20–23 min wall.

## Verdicts

1. **No exploration: supported as a defect, refuted as the limit.**
   - pn13's corpus was greedy: in the pn13 run, priority deviated in 0 of
     about 10,000 decisions per round.
   - Greedy on-policy PPO with a working step size is actively harmful. Both
     greedy arms drift down (mz 51.4 → 49.3; entity → 49.7). Held out, the
     mz greedy round 10 plays 48.5% against the seed's 50.2%. Once its head
     starts overriding priority (0 → 14–23% deviations from round 6) it has
     no counterfactual signal to tell good overrides from bad.
   - Sampling fixes that. The T=1 arms hold flat at +0.4 to +0.8pp over the
     control every round, with 22–41% of decisions off the bot's pick. With
     the prior loosened (residual 0.5), sampled PPO recovers from 49.3% to
     50.6–50.9% (held out: 48.3 → 50.3%, about +2pp, at the edge of the
     unpaired CI).
   - But **no sampled arm beats the bot**: the best held-out line is +0.36pp
     over the control, inside noise.
   - Also measured: at residual 2 the prior pins the DEPLOYED priority answer
     even while sampling. Priority greedy-flip is 0.00% in every T=1 round:
     sampling explores, but the update never overturns the 2-logit gap.
     Only the residual 0.5 arms change deployed priority play.
2. **No per-card binding: inconclusive, leaning refuted.** Entity and mz are
   indistinguishable in every pairing: greedy 49.65 vs 49.28, T=1 51.62 vs
   51.32, schedule 51.35 vs 51.65, residual 0.5 50.58 vs 50.92, all at
   round 10. The value head does not improve either (median holdout log loss
   0.384 vs 0.366). Caveat: the entity blocks start at zero and PPO moves the
   policy only about 0.003–0.05 KL per round, so 10 rounds may not train the
   encoder much. A supervised or value-only pretraining of the entity encoder
   would be needed to rule it out.
3. **Noisy teacher labels: supported that they are not what is missing, but
   the outcome signal is too weak to replace them.** Training from outcomes
   alone neither degrades a good start (the T=1 arms are stable) nor lifts
   it past the bot. The value head fits outcomes (log loss about 0.37–0.44
   against a base of about 0.69). The policy does not, because the per-round
   advantage signal over about 20,000 decisions from 2,000 games is small
   next to the ±1.5pp eval noise. The one clear positive is recovery from a
   deliberately damaged start (residual 0.5).

## Recommendation

- Stop pushing learned-policy PPO at this data scale. After 80 PPO rounds
  across 8 arms, nothing clears the bot on a held-out block; the best is
  +0.36pp.
- Keep the stochastic collection and behaviour-ratio plumbing: it is what
  keeps on-policy PPO from drifting. If PPO is revisited, keep these rules:
  - collect sampled (T≈1);
  - use a residual ≤ 0.5 so priority can actually change;
  - never run with the pn13 `-clip 1` default (the steps are about 5× too
    small).
- Scale the rounds by 10× (20k games per round is about 10 min at 22 cores)
  before judging entity features, and pre-train the entity encoder on the
  value target first.
- The search seat (+3.3pp held-out) remains the only source of edge.
