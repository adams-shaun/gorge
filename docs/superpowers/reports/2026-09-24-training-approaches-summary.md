# Training approaches tried for gorge's bot — summary (2026-09-24)

One page covering every attempt, 2026-09-06 to 2026-09-24, to make the bot
better through fitting, search or learning. Each section says what was tried,
what it measured, and why it stopped. Full detail lives in the linked plan and
report docs and in the commit messages.

Every win rate here is **gorge-internal, from dev seed blocks, against the
default `bot`**, unless marked *held-out* (seed 1,000,000, 400 games per pair,
seats traded).

## TL;DR

| # | Approach | Best measured result | Status |
|---|---|---|---|
| 1 | Hand heuristics (AR7, AR8, BLK, L1c) | AR7 +1.65pp held-out; the rest inside noise | AR7 promoted; ladder exhausted |
| 2 | Linear cast-weight profile, SPSA fit (L1–L4) | 50.9% held-out, CI [49.4, 52.5] | Failed the +3pp gate |
| 3 | PIMC search teacher (sampled worlds) | +5.8pp ± 1.25 paired (3,000 dev games) | Works; the only source of edge |
| 4 | Search teacher as a playable seat (L10) | **53.7% held-out vs 50.4% control (+3.3pp)** | Merged opt-in; slow (664 ms mean per decision) |
| 5 | Distil teacher into per-option net (L9, pn01–pn06) | Holdout matches the bot; in play 46–47% vs 50.1% control | Net copies the bot or does worse |
| 6 | Oracle (clairvoyant) teacher → distil loop | Oracle seat +11–19pp; distilled net flat (14.1–14.6% vs 14.3%) | Ceiling only; the student absorbs nothing |
| 7 | Value head, value leaf, policy prior (pn08–pn10) | Value head predicts outcomes (log loss 0.24 vs 0.46 base rate); no win-rate gain | Merged; inert on its own |
| 8 | Priority-context encoder v2 (pn03) | Top-1 = bot; 0/52 overrides right | Branch, not merged |
| 9 | Expert iteration loop (pn11, `cmd/exitloop`) | 3 gens: eval 45.7 / 44.9 / 45.4% vs 48.7% control | Merged; does not compound |
| 10 | Prior art: mtgbld self-distillation PPO (XMage) | 44.7% → 54.5% vs CP7 over 6 gated rounds | The one recipe that compounded; not yet ported |
| — | MageZero reference run (2 vCPU) | Gen 0: 44% vs minimax pool (baseline 34.5%) | Throughput reference |

**The one durable finding:** the search teacher beats the bot. Every attempt
to compress it into a cheap learned policy has so far reproduced the bot and
nothing more. The likely cause is that the teacher's better choices depend on
hidden information (the opponent's hand and the future draws) that a redacted
view cannot express. The labels are also thin: about 1–3% of labelled
decisions are overrides of the bot.

## 1. Hand heuristics (pre-L-series and in parallel)

- **What:** hand-written decision rules layered onto `botpolicy`, each an
  opt-in `botbench` policy gated before promotion.
- **Results:**
  - AR7 `lethal-pressure`: +1.65pp held-out, CI above 50%. Promoted into the
    default `bot` (9be52252).
  - AR8 combined-attacker lethal: 51.5% [49.3, 53.7]. Noise.
  - BLK whole-assignment blocker: 49.45% against a 49.5% same-seed control.
    Inert.
  - L1c mana-efficiency term: 0–3 games of 2,000. Inert.
- **Verdict:** "the hand-heuristic ladder is exhausted on the mono suite"
  (`docs/superpowers/plans/2026-09-19-learned-cast-profile.md`).

## 2. Fitted linear cast profile — the L-series (L0–L4)

- **Why:** R1 (`docs/superpowers/specs/2026-09-06-gorge-learned-policy-research.md`)
  measured spell selection at priority as the biggest untouched surface.
  Randomising it costs 16.8pp; blocking costs 3.8pp and targeting 4.3pp.
- **What:**
  - `CastWeights` (L1) and the `cast-profile` policy (L2).
  - `cmd/policytune` (L3): SPSA with common random numbers. A fix at de72cf91
    moved the iterate to float with per-weight scaling.
  - L4: 300 iterations over 11 weights, then the held-out gate.
- **Result:** 50.92%, CI [49.38, 52.47]. **Fails +3pp.** In-fit benches
  walked 46.3–52.0% with no trend.
- **Early probes:** the ranking surface is live, since reversing the weights
  costs 8.4pp and holding instants costs 8pp. Small nudges flip almost no picks.
- **Verdict:** the current casting rule sits near the top of this feature
  family. The harness stays; the network phases L5/L6 stay closed on this
  track.
- **Rerun on uw-tempo (2026-09-24, scratch `uwtune`):** a deck-local SPSA fit
  of the same 11 weights, 77 games/s on 2 vCPU. The uw-tempo seat's win rate
  vs `bot` went from 14.3% to 17.2% and then plateaued. It was aborted at
  iteration 280 because it isn't comparable to a learned policy.

## 3. PIMC search teacher (L7, L8 and the spike)

- **What:**
  - `internal/searchprobe` and `cmd/searchteacher`.
  - An ensemble determinization: sample K opponent worlds consistent with the
    observed history, roll each candidate out with the default bot, and pick
    the best mean.
  - L7 fixed sampler coverage (acceptance 9.9% → 17.4%).
  - L8 emits a label corpus.
  - Report: `docs/superpowers/reports/2026-09-19-search-teacher-spike.md`.
- **Results:**
  - The spike: attackers +2.80pp ± 1.10, cast +2.70pp ± 1.86.
  - Combined, on 3,000 dev games: **+5.80pp ± 1.25**. That was the first
    significant margin in the programme.
  - Coverage is 44.9% of asked decisions, falling off late (14.5% at turn
    13+). Cost is 2.28 s per labelled decision.
- **Kinds added 2026-09-23/24:**

  | Kind | Ticket | Paired Δ over attackers-only, oracle teacher |
  |---|---|---|
  | blockers | pn04 | +5.3pp |
  | single-choice target | pn05 | +6.93pp (all 104 overrides loss→win); sampled coverage 23% |

- **Label schema 2 (pn07)** records the game outcome, for value training.
- **Cheaper search (pn09):** a 2-turn horizon with a leaf evaluator cuts the
  cost about 7× (56 → 8 ms per labelled decision). It gives up about 2pp of
  paired delta, which is within the CIs.

## 4. The teacher as a seat (L10)

- **What:** `internal/searchseat.Choose`, the teacher's decision function,
  shared by the generator and a playable `botbench -a search` seat. Perf
  passes:
  - `-decision-workers` parallelism: 496 → 171 ms latency.
  - Skipping potential actions in replay: Sample −28%.
  - Engine trigger gates: TeacherChoice −15%.
- **Held-out gate (795a52c1):** **53.7% [52.2, 55.2] vs self-control 50.4%,
  +3.3pp.** Worst pair 48.2%, zero stalls.
- **Limits:**
  - The edge is early-game only. Coverage by turn is 64.5% (t1–6), 13.0%
    (t7–12) and 4.8% (t13+).
  - Mean 664 ms, p95 2.2 s per asked decision.
  - Promotion to default is an operator decision, not taken.
- **Spikes (not merged):**
  - Carry-forward worlds die across the opponent's turn (2.4% survive).
  - Redeal PIMC helps only on uncovered late decisions, so it's the right
    *fallback*.

## 5. Distilling the teacher into a network (L9, pn01–pn06)

- **Model:** `internal/policynet`, a pure-Go per-option scorer (hashed state
  features dotted with per-option features, pointer-style over the legal set).
  Trained by `cmd/policytrain` and played by `seat.PolicyNetBot`.
- **What was tried, in order:**
  1. **Value regression plus margin ranking (L9).** Collapsed to constant
     scores. Within-decision candidate values differ by a median of 1/16, one
     sampled world, so "predict the mean" is the optimum.
  2. **Pure argmax cross-entropy, optionally with a residual bot-prior head
     (L9 fix2).**
     - Attackers are learnable: holdout 0.968 vs bot 0.887.
     - Priority is not. It stays flat at 0.36 across three orders of
       magnitude of learning rate; with the residual it merely recovers the
       bot's 0.669.
  3. **Attack admission fix (L9d).** The first checkpoints lost 1000/1000:
     every CE logit was above 0, so the seat attacked with everything.
  4. **Feature-family experiment (00ca45b6):** instant-speed, counter and
     available-mana features. Priority stayed below the bot (0.628 vs 0.733).
  5. **Honest holdout (pn02):** split by game and scored on the override
     subset. It showed the net picks the bot's move 100% of the time and gets
     0 overrides right.
  6. **Gated priority scoring (pn01).** Score only the shape the head was
     trained on. Gated, it overrides 0 of about 5,150 decisions.
  7. **Blockers and target in the seat (pn06, opt-in).**
     - Target: never overrides.
     - Blockers: 22% overrides, almost all adding a block. It adds about
       0.4pp of loss.
  8. **Encoder v2 (pn03):** opponent open mana, stack-top context and hand
     counters. Top-1 is identical to the bot and 0/52 overrides are right.
     Not merged. It carries a real fix: the decision-kind "other" slot
     collided with the Land bit.
- **In-play result (pn06 bench, 1,000 games per arm):** control 50.1% [47.0,
  53.2]; the nets 46.6–47.0%. In play, the attackers path overrides the bot
  on 34% of decisions, mostly attacking less, against about 1% on holdout. It
  is the main loss, and that distribution shift is the open follow-up.

## 6. Oracle teacher and the distillation loop (2026-09-24, uw-tempo)

- **What:** `searchteacher -oracle` clones the real engine. It sees the
  opponent's hand **and future draws**, so it is an upper bound on what any
  search can gain.
- **Loop:** six generations of 500 games each (`exit/loop.sh`, scratch):
  label with the oracle, train the net, eval on a fixed block.
- **Results (uw-tempo seat vs `bot` on the five mono decks):**

  | Seat | Win rate |
  |---|---|
  | `bot` baseline | 14.3% |
  | Sampled-PIMC seat | 16.7% |
  | Oracle, attackers | 25.6% |
  | Oracle, attackers + cast | 33.6% |
  | Distilled net, all 6 generations | 14.1–14.6% (flat) |

- **Why it's flat:** holdout agreement with the teacher was 96.2%, below the
  bot's own 98.9%, and the net overrode only 1.4% of decisions.
- **Throughput on 2 vCPU:**

  | Stage | Rate |
  |---|---|
  | bot vs bot | 57 games/s |
  | Oracle teacher, attackers | ~18k games/h |
  | Oracle teacher, attackers + cast | ~7.8k games/h |
  | Sampled PIMC | ~0.5 games/s |
  | Full distil generation | 3.5–4 min |

## 7. Value head, value leaf, policy prior (pn08–pn10)

- **Value head (pn08, checkpoint v3):** a win-probability head on the shared
  trunk.
  - Holdout log loss 0.244 vs 0.460 for the base rate; Brier 0.075 vs 0.142.
  - It did not move the policy heads.
  - The effective sample is about 150 games, since examples share their
    game's outcome.
- **Value leaf (pn09):** the value head scores horizon leaves.
  - +11.8pp vs +12.2pp for the heuristic leaf at horizon 2. No difference.
  - It costs nothing extra.
- **Policy prior (pn10):** the net ranks which candidates get rollouts.
  - +15.6pp, the same as unguided.
  - Top-2 widening: +14.6pp at 79 ms, against 119 ms for the unguided list.
  - A plain 3-candidate cap is as good and as cheap (+15.2pp, 68 ms).
  - The prior only helps once the net knows something the bot doesn't.

## 8. Expert iteration (pn11, `cmd/exitloop`)

- **What:** `cmd/exitloop` runs searchteacher, then policytrain, then
  botbench per generation.
  - Gen 0's teacher is the unguided oracle, playing out to game end.
  - Gen k's teacher uses checkpoint k-1 twice: as the candidate prior
    (top-5 of 16) and as the value head scoring leaves at horizon 2.
  - The training corpus is cumulative.
  - Every checkpoint is evaluated against `bot` on one fixed 1,000-game seed
    block.
- **Run:** 3 generations × 500 teacher seeds on uw-tempo vs the five mono
  decks. It took 5m46s on 22 workers.

| Gen | Teacher overrides | Paired Δ | Net vs `bot` (control 48.7% [45.6, 51.8]) | Value log loss (base rate) |
|---|---|---|---|---|
| 0 | 1.4% | +14.8pp ± 3.1 | 45.7% [42.6, 48.8] | 0.248 (0.467) |
| 1 | 35.4% | +9.2pp ± 3.5 | 44.9% [41.8, 48.0] | 0.379 (0.540) |
| 2 | 29.9% | +10.0pp ± 3.4 | 45.4% [42.3, 48.5] | 0.455 (0.619) |

- **Reading:**
  - Eval is flat, and below the control, in every generation.
  - Priority scoring never changes a pick (0% right on overrides; the bot's
    answer 100% of the time), so the attackers+priority arm is byte-identical
    to attackers-only.
  - The guided teacher's labels get **noisier, not sharper**: 20× the
    overrides for less paired gain.
  - This is confounded. Gens 1+ switch from game-end rollouts to a horizon-2
    value leaf, which pn09 measured at about 2pp weaker with 28–33% overrides
    on its own.
  - A clean rerun would keep game-end rollouts and use the prior only.
- **Throughput:**
  - 22 workers, per stage: teacher 69k seeds/h (gen 0), 200–240k seeds/h
    (guided); eval about 1M games/h.
  - Training is single-threaded and grows with the corpus: 47 → 92 → 140 s.
    It is the bottleneck.
  - At 2 vCPU (`taskset -c 12,28`, 2 workers): teacher + train for a 100-seed
    generation takes 52 s (gen 0; about 16.5k teacher games/h counting the bot
    twin) or 29 s (guided; about 65k games/h).

## 9. MageZero reference (external, for throughput and curve shape)

- **What:** MageZero, AlphaZero-style MCTS plus a transformer on an XMage
  fork, run locally on UW Tempo at 2 vCPU (one core's hyperthread pair).
  Settings: 300 sims per decision, minimax opponent pool (MTGA mono decks).
- **Throughput:** about 36–50 games/hour; generation 0 took 83 min for 50
  games. Minimax vs minimax runs at 6.5 games/min.
- **Win rates:**
  - Our minimax baseline: 34.5% over 200 games. The README quotes 16%.
  - Generation 0 (offline MCTS, no network): 44% (22/50).
  - Their published curve: 37% → about 64% by generation 17, at about 1,000
    games per generation.
- **Comparison:** gorge's simulator is about 1,000× faster per game, but none
  of gorge's learned policies has yet shown a rising curve. MageZero's does,
  though it trains with `see_opponent_hand: true` (clairvoyant).

## 10. Prior art: the mtgbld/XMage policy work (May–June 2026)

Before gorge, the same programme ran on XMage in mtgbld, against XMage's
built-in minimax AI (CP7). Those numbers do not transfer: different engine,
bot and decks. The lessons do. Sources are the mtgbld project memories
(`project_ai_bc_ceiling`, `project_self_distillation_iteration`,
`project_ppo_v2_iteration_attempts`, `project_ppo_stabilization_2026-06-02`,
`project_mcts_rollout_inference_no_help`, `project_universal_archetype_policy`,
`project_legacy_perdeck_bc_underperforms`) and the handoff,
`.superpowers/ds4/RESUME-distillation.md`.

**What failed there, the same way it fails here:**

| Attempt | Result |
|---|---|
| Behaviour cloning of the bot | In control of decisions, the cloned policy was about 15pp worse than CP7 (55% → 40%). It learns CP7's average behaviour and loses the per-state context CP7's own search sees. |
| REINFORCE on the corpus | Nothing to learn from: CP7 plays the same way whether it wins or loses, so outcomes don't separate good moves from bad. |
| Decision-time MCTS rollouts over a weak prior | 30–50× slower and worse (22% vs 37.5%). A weak prior makes search repeat the prior's mistakes. Revisit only once the policy alone beats the baseline. |
| Naive multi-round PPO | Every variant fell (42.5–49.6% vs 51.8%). The training games came from a sampled policy much weaker than the deployed argmax one: an off-policy corpus. |
| Small per-deck corpora (600 games) | Below CP7 on every Legacy deck. Even a gated loop gained only about +2pp and stayed about 9pp under. |

**What worked:**

| Change | Result |
|---|---|
| Richer action outputs (joint card+target "slots") | +5.9pp. The "BC ceiling" was partly output coarseness. |
| Single-pass PPO (clip + KL anchor) from the BC policy | +1.2pp mirror, +2.8pp cross-deck. Stable where REINFORCE collapsed. |
| **Self-distillation PPO** | 6 gated rounds took mirror 44.7% → 54.5% and cross-deck 34.6% → 40.4%. It was the first run to beat CP7, and the only multi-round recipe that compounded. About 10 min per round. |
| Matchup conditioning (archetype and matchup tokens) | Mono-green +27pp (24.3% → 50.5%); tempo/control +2 to +9pp; aggro −4 to −5pp. |

**How self-distillation PPO worked.** The corpus is the *deployed*
argmax policy's own games against the fixed opponent, not sampled play and
not a teacher's. Each row records π_old = the probability the policy gave
the action it took. The update is PPO clip + KL anchor with a per-state
value baseline and normalised advantage. It used a small learning rate and
gated each round.

**What that means for gorge:**

1. **pn11 is the off-policy pattern that failed there.** It trains the net
   on the teacher's labels and never on its own play. The loop that
   compounded was on-policy: the net plays the bot, outcomes weight its own
   choices, and updates stay small and gated.
2. **Their order was policy first, search second.** pn10's inert prior
   matches their MCTS finding: search amplifies a prior only once the prior
   is better than the bot.
3. **Matchup conditioning was their biggest single lever**, and gorge's
   encoder has no deck or matchup signal. Deployable only if inferred from
   public information (cards seen so far), not the opponent's decklist.
4. **n ≥ 500 per arm or it is noise.** It still holds.

## What we know now

1. **Search wins; distillation hasn't.** PIMC gives +3.3pp held-out as a seat
   and +5.8pp paired as a teacher. Oracle search is +11–19pp on uw-tempo.
   No distilled network has beaten the bot in play.
2. **Why distillation stalls:**
   - Overrides are 1–3% of labels.
   - The override often depends on hidden information the net can't see.
   - The residual bot prior makes "copy the bot" the easy optimum.
   - Holdout top-1 hides all three. Read only override-subset metrics
     (pn02) and in-play benches.
3. **Levers with evidence behind them:**
   - Make search cheaper (horizon + leaf is 7× cheaper for about 2pp).
   - Cover the late game (redeal fallback).
   - Ship the search seat rather than a student.
   - The literature agrees: LOCM's search beat its own distilled net by
     +24.6pp.
4. **Open:**
   - Whether ExIt compounds with game-end rollouts (pn11's run changed the
     leaf and the prior at once, and was flat).
   - Why the attackers net overrides 34% in play against about 1% on holdout.
