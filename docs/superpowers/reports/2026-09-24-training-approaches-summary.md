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
| 9 | Expert iteration loop (pn11) | *running* | In progress |
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

## 8. Expert iteration (pn11, in progress)

`cmd/exitloop` runs three generations on uw-tempo. Generation k's teacher
uses checkpoint k-1 as its candidate prior and leaf value, and every
checkpoint is evaluated on one fixed seed block. The measurement is required;
a gain is not. Results will be appended when it lands.

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
   - Whether ExIt (pn11) with the teacher in the loop compounds at all.
   - Why the attackers net overrides 34% in play against about 1% on holdout.
