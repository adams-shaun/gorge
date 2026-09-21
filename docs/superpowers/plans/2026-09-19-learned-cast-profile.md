# Learned cast profile — plan (L-series)

Date: 2026-09-19. Base: `9c24c8b3`.
Supersedes the ordering (not the content) of the "Next experiments" list in
`docs/superpowers/reports/2026-09-19-hosted-bot-policy-selection.md`.

## Why this, why now

The R1 research (`docs/superpowers/specs/2026-09-06-gorge-learned-policy-research.md`)
set one gate before any network is built: a *fitted* score over **spell
selection at priority** must clear +3pp at N=4000. That surface is the only
measured-sensitive one nobody has worked (random spell choice costs 16.8pp;
blocking 3.8pp, targeting 4.3pp). Since R1:

- Phase 0 landed: botbench has the deck-pair matrix, seat trading, CIs,
  decision stats, and an atomic JSONL decision trace.
- Hosted policy selection landed: a named policy is a first-class, persisted,
  opt-in table setting, so a fitted policy can ship without touching `bot`
  goldens.
- AR7 (`lethal-pressure`) showed the ladder works: +1.65pp held-out, CI above 50%.

`chooseCast` (`botpolicy/cast.go:346`) is still the hand constant
`creature: 30+4*Power, else CMC` plus kicker/flashback/reserve/tax bonuses.
Those constants have never been fitted to anything.

The prior XMage project's binding lessons (mtgbld memory:
`project_ai_bc_ceiling`, `project_beat_cp7_exhausted`,
`project_phase5_iter_regression`, `project_self_distillation_iteration`,
`project_universal_archetype_policy`):

1. Behaviour cloning cannot beat its teacher; the teacher here is weak.
2. Ungated MageZero-style iteration walked in [4%, 47%] — gate every
   promotion at +1.96σ vs current best.
3. Archetype/matchup conditioning was the largest single lift (+27pp on one
   deck) — per-archetype profiles are worth testing.
4. Inference-time search on a weak prior only amplifies its mistakes.

A fitted linear scorer is the smallest learned policy that respects all four:
no teacher needed (black-box fit against win rate), gated, profile-able, no
search at inference.

## The series

Every L-task is opt-in, deterministic, reads only the deciding seat's legal
view (`Board`), ties on option index, and leaves `bot` goldens
(`rules/heads_test.go`) untouched.

### L1 — feature × weight cast scorer, default-equivalent

Refactor `castScore`/`chooseCast` into `features(o) · w`, where `w` is a
`CastWeights` struct and `DefaultCastWeights` reproduces today's arithmetic
exactly (creature base 30, per-power 4, non-creature CMC 1, kicked 6,
flashback 4, reserve scale 5, commander tax scale 5). Add features the current
rule does not read, all with default weight 0 so behavior is unchanged:

- `curveFit` — 1 when the card's cost equals the mana the seat can produce
  this turn (pool + untapped sources), else 0;
- `manaLeft` — pool total after the cast;
- `precombat` — 1 in the first main phase (creature-before-combat question);
- `instantOnOwnTurn` — 1 for an instant-speed card cast in own main phase;
- `oppCreatures`, `ownCreatures` — public battlefield counts;
- `lifeDelta` — own life minus the lowest opponent life;
- `toughness` for creatures (add `Card.Toughness` on both adapter halves,
  parity-tested).

*Done means:* `go test ./botpolicy ./seat -count=1` passes;
`go test ./rules -run TestHeads -count=1` unchanged;
`TestBotAdaptersAgreeOverWholeGame` still green; a new table test proves
`DefaultCastWeights` picks identically to the pre-refactor rule on every
existing cast test case.

### L2 — named `cast-profile` policy with a weight file

`botpolicy.WeightedDecide(b, d, r, w)` and a `host.CastProfilePolicy`
(`"cast-profile"`) that loads weights from an embedded JSON profile
(`botpolicy/profiles/*.json`, `go:embed`, schema-versioned, unknown keys
rejected). botbench gains `-a cast-profile -profile <file>` so a candidate
file can be benched without a rebuild.

*Done means:* hosted-policy tests extended to `cast-profile`
(`TestHostedPoliciesReplayDeterministically` covers it); with the default
profile it is intent-identical to `bot` over a whole game.

### L3 — `cmd/policytune`: SPSA fit with common random numbers

Simultaneous-perturbation stochastic approximation over `w`:
each iteration draws Rademacher Δ, plays `w+cΔ` directly against `w−cΔ`
on the development suite (seed range disjoint from held-out, all ten approved
pairs, seats traded), and steps along the paired win-rate difference.
Head-to-head with shared seeds is the variance reduction; no absolute
baseline run is needed per step. Deterministic given `-seed`; writes a
profile JSON plus a CSV trajectory. Budget: 17.5 ms/game ⇒ 2,000 games/iter
≈ 1 s on 32 threads; 300 iterations ≈ 5 min.

*Done means:* unit test on a synthetic concave objective converges;
`go test ./cmd/policytune -count=1`; a 20-iteration smoke run is
byte-identical across two invocations.

### L4 — fit, then gate

Fit a universal profile. Gate exactly as AR7: held-out seed 1,000,000,
400 games/pair, all ten pairs, candidate vs `bot` **and** vs
`lethal-pressure`; zero new stalls/errors; report per-pair and seat splits.
R1's programme gate: +3pp pooled and no pair below −5pp. If it fails, the
band above the trivial rule is narrow: ship nothing, record the number,
and the network phase stays closed.

### L5 — per-archetype profiles (only if L4 opens)

Fit one profile per deck (self-deck conditioning, per the prior project's
archetype lift). Gate each against the universal profile on its own pairs.
Selected by the deciding seat's own deck identity — public to that seat.

### L6 — network phase (only if L4 opens; R1 §4.3 unchanged)

Per-option scorer over the trace features, self-distillation corpus from the
deployment (argmax) policy, PPO+KL, +1.96σ gating per generation. The L3
profile is the warm start and the baseline to beat, which answers R1's
"gorge has no teacher" risk: the fitted profile *is* a teacher that has
already beaten `bot`.

## Early probes (2026-09-19, after L2 merged, before L3)

Hand-set profiles via `botbench -a cast-profile -b bot -profile <f>`, ten
approved pairs, 100 games/pair, **development seed 0** (not held-out).
Baseline `bot` self-control on this suite was 515-485 (51.5%).

| profile (delta from default) | pooled | 95% CI | note |
|---|---|---|---|
| CurveFit=8 | 51.7% | [48.6, 54.8] | ≈ control; 9/10 pairs within ±2 games |
| CreaturePrecombat=−10 | 51.7% | [48.6, 54.8] | ≈ control |
| NonCreatureOppCreatures=6 | 51.6% | [48.5, 54.7] | ≈ control |
| InstantSpeedOffTurnHold=−40, CastThreshold=0 | **43.5%** | [40.4, 46.6] | holding instants hurts |
| reversed ranking (all base weights negated) | **43.1%** | [40.0, 46.2] | worst-first costs 8.4pp |

A trace of 180 `bot`-v-`bot` games: 6,061 priority decisions offered a
cast; 55% of those offered ≥2 distinct castable cards (up to 9).

Reading:

- The ranking surface is live (reversal −8.4pp) and so is cast/hold
  (−8pp), but small nudges on the new features flip almost no picks: the
  creature base (30 + 4·Power) dominates every non-creature score, so a
  weight of 6–10 only reorders ties inside a class. L3 must scale its SPSA
  perturbation **per weight** (c_i ∝ max(1, |w_i|/4)) or the fit will read
  pure noise on the new dimensions.
- R1's 16.8pp "spell selection" sensitivity randomised among cast/ability/
  **pass**; these probes suggest a large part of it is cast-vs-pass. The
  band above the current rule may be narrow — which is exactly what L4's
  gate decides.
- If L4 is flat, the next candidate is a **within-turn mana-efficiency**
  feature (value of the best affordable remaining set after this cast), since
  greedy best-first casting only differs from any other order when mana
  runs out mid-turn.

## L4 result (2026-09-19) — gate not passed

Fit: `policytune` at `de72cf91` (float iterate, per-weight scale
max(1,|w|/4), A=100, C=4), 300 iterations, 10 approved pairs × 100 games per
head-to-head evaluation, dev seed block from 10,000,000, fitting CreatureBase,
CreaturePower, NonCreatureCMC, CurveFit, ManaLeft, CreaturePrecombat,
CreatureOppCreatures, NonCreatureOppCreatures, CreatureLifeDelta,
InstantSpeedOffTurnHold, ReserveScale. ~14 min wall on 32 threads.

In-fit bench vs `bot` (1,000 dev games each, ±1.6pp SE) every 50 iterations:
50.8, 50.5, 50.4, 46.3, 49.5, 52.0 — a noise walk, no trend. Final profile
moved CreatureBase 30→49, NonCreatureCMC 1→−2, ManaLeft 0→−2, the rest ±1.

**Held-out gate** (seed 1,000,000, 400/pair, seats traded): fitted profile vs
`bot` **2,037–1,963, 50.92%, 95% CI [49.38%, 52.47%]**, zero stalls; every
pair within 193–211 of 400. Fails the +3pp gate. Not promoted, not committed
as a profile.

**Decision per R1 §4.3:** a fitted linear score over spell selection does not
clear +3pp, so the band above the current casting rule is narrow on this
suite; the L5/L6 network phases stay closed. Together with the early probes
(reversal −8.4pp; small nudges inert) the current rule sits near the top of
this feature family. The infrastructure (CastWeights, `cast-profile`,
`policytune`) stays: it is the harness for any future feature family.

## Revision after L4 (2026-09-19): a search teacher, then a scorer

Operator chose all four follow-ups: AR7 promoted into default `bot`
(9be52252, heads 2/6/8 moved); AR8 combined-attacker lethal and BLK block
assignment queued (opt-in); L1c within-turn mana efficiency queued; and a
search-teacher spike.

**Spike result** (`docs/superpowers/reports/2026-09-19-search-teacher-spike.md`,
harness ae3e03f9, dev seeds only): ensemble determinization on the existing
`internal/searchprobe` sampler (K=8 worlds, rollouts to game end with the
default bot) beats the bot, measured as paired Δ vs bot-v-bot on the same seeds:

| teacher on | paired Δ (95%) | coverage | cost/labelled decision |
|---|---|---|---|
| attackers | +2.80pp ± 1.10 | 34% | ~2.5 s |
| priority ≥2 casts | +2.70pp ± 1.86 | 53% | ~2.1 s |

Label quality vs the true state (audit): attackers 84% best-option vs bot
69%; with a ≥0.3 margin, 18 right / 0 wrong. The bottleneck is the sampler
(8–19% acceptance; 61% of attack decisions get no world), dominated by
PolicyCompetition rejections. The spike also found a livelock-detector index
bug (fixed, 2ba585ba).

This answers R1 §6.2 risk 1 ("gorge has no teacher"): the search teacher is
measurably better than the bot on the decisions it covers, so the network
phase reopens as **expert iteration**, not self-play from scratch:

- **L7 — sampler coverage.** Fix the opponent-hand PolicyCompetition
  rejection (a proposed hand in which the bot would have cast something else
  than what was observed) so more decisions get worlds, especially late game.
  Measured by acceptance rate and coverage-by-turn, not win rate.
- **L8 — label corpus.** `searchteacher -labels`: per labelled decision, the
  decision-trace board snapshot (existing redacted schema), every candidate's
  mean value and world count, the bot's answer, and the margin. Positive label
  only above the margin (attackers 0.25, cast 0.3). Offline, deterministic,
  dev seeds only; K=16, 128 attempts, MinESS 2.
- **L9 — per-option scorer.** Pure-Go MLP over hashed state features ·
  per-option features (AlphaStar-style pointer scoring over the legal set,
  R1 §1.4 shape), trained on L8 labels (value regression + margin ranking).
  Warm start and teacher are the search labels, not the bot, which removes the
  BC-ceiling objection. Opt-in hosted policy, and it must pass the standard
  held-out gate (seed 1,000,000, 400/pair) before any promotion.
- **L10 — iterate.** Use the L9 scorer as the rollout/prior policy for the
  next teacher generation (ExIt), gated +1.96σ per generation, which is the mtgbld
  lesson.

**AR8 (merged opt-in as botbench `ar8`)**: dev seed 20,000,000, 200/pair,
vs promoted `bot`: 1,030–970, 51.5% [49.3, 53.7]; a variant counting chump
blocks as absorbing (the brief's literal reading) scored 50.85% [48.7, 53.0].
Both noise: with AR7 in the default, combined-attacker lethal adds little on
the mono suite. Not promoted.

**BLK and L1c (merged opt-in, 2026-09-20)**: dev seed 20,000,000, 200/pair
(2,000 games) vs the promoted `bot`. The `bot`-v-`bot` control on these exact
seeds is 990-1010, so read every row against 49.5%, not 50%:

| policy | pooled | vs control |
|---|---|---|
| `blocks` (whole-assignment defender) | 989-1011 (49.45%) | 1 game |
| `cast-profile` SetValue=4 / 8 / 16 / 32 | 988 / 990 / 989 / 987 of 2000 | 0-3 games |

Both are inert: the whole-assignment blocker reaches the same answer as the
per-blocker rule in nearly every offered shape, and the mana-efficiency term
almost never reorders a cast. Wiring verified (botbench `blocks` builds
seat.NewBlocksBot; the profile path reaches `SetValue`), so this is a real
measurement, not a dead flag. **The hand-heuristic ladder is exhausted on the
mono suite** — AR8, BLK and L1c all land inside the control's noise, and the
fitted profile failed the held-out gate. The search teacher (L7-L10) is the
only track with measured headroom.

## Label corpus + combined teacher (2026-09-20, after L7/L8 merged)

`searchteacher -games 300 -seed 60000000 -worlds 16 -attempts 128 -min-ess 2
-kinds attackers,cast -workers 14 -labels dev2.jsonl` (3,000 dev games, 46 min
wall, 128 MB corpus):

- **paired delta +5.80pp ± 1.25 (95%, n=3,000; 376 games changed outcome)** —
  the two kinds TOGETHER, and the first significant margin in this programme.
  The spike measured them apart (+2.8 attackers, +2.7 cast) and warned they
  might not add; measured, they roughly do.
- Sampler acceptance **17.41%** (was 9.92% before L7), coverage **44.9%** of
  32,496 asked decisions (t01-06 81.4%, t07-12 33.2%, t13+ 14.5% — the late
  game is still the hole).
- Corpus: 14,588 labelled decisions (9,436 priority, 5,152 attackers), 57,060
  scored candidate options, 3,723 teacher overrides, 881 of them above the
  0.25 margin. Cost 2.28 s per labelled decision.
- Corpus lives outside git (scratchpad); regenerate with the command above.

This is the training signal for L9. A scorer that merely reproduces the
teacher's covered decisions, at negligible inference cost, would be worth
~+5.8pp before any iteration — provided it generalises to the 55% of
decisions the sampler cannot cover, which is the open risk.

## L9 first training run (2026-09-20) — the scorer does not learn to rank

Trained the merged L9b trainer on the 14,588-decision corpus (29 s, 8 epochs).
Measured per decision kind on multi-option labelled decisions, against
baselines computed from the same corpus:

| kind | trained model | copy the bot | first labelled | random |
|---|---|---|---|---|
| priority (cast) | 0.422 | 0.419 | **0.422** | 0.246 |
| attackers | 0.802 | 0.802 | **0.802** | 0.906 |

The model's agreement equals the first-labelled baseline exactly, for three
hyperparameter settings and for a checkpoint trained only on overrides: the
learned scores are CONSTANT across options. `-rank-weight` ≥ 10 NaNs.

Two lessons, both now in `bot-l9b-fix-rank-collapse`:

- **The blended top-1 the trainer prints is not a metric.** Random scores
  0.906 on attackers (the teacher's preferred attacking SET covers most
  offered options) and 0.246 on cast. Only per-kind numbers against the
  bot-copy baseline mean anything.
- **74.5% of labelled decisions are "the teacher agreed with the bot".**
  Minimising a value loss over near-identical candidate means is solved by a
  constant, and imitating the bot is the easy optimum. The signal is the
  3,723 overrides (881 above margin 0.25); the fix centres value targets
  within a decision, stabilises the ranking term, and weights overrides.

The teacher itself remains +5.8pp; nothing here disputes that. What is
unproven is that a cheap scorer can absorb it.

## Reference pass 2 (2026-09-20) — why the scorer collapsed, and what the field does

Sources checked this round (V = read at source):

- **LOCM PIMC distillation, arXiv 2609.06816** [V] — the closest published
  analogue: PIMC over sampled opponent worlds, distilled into a network. They
  train the policy head with **plain cross-entropy on the single action the
  teacher chose** (not value regression), and report imitation alone recovers
  most of the teacher's strength. They also keep search at decision time,
  where it adds a further **+24.6pp** over the greedy network.
- **Ranking-distillation collapse, arXiv 2505.21058** [V] — names our exact
  pathology: when the teacher's within-group scores carry too little entropy,
  the objective is trivially satisfied and training collapses.
- **ADPO, arXiv 2510.18913** [V] — pointwise regression on absolute scores
  "smears" probability and is fragile; group-relative (advantage) targets are
  the stable form.
- **Residual policy learning, arXiv 2004.05097** [S] — learn a correction on
  top of a frozen base policy; the standard answer when the base is already
  right most of the time (here: 74.5%).
- **CRR / AWR** [S] — advantage-filtered imitation, i.e. weight the decisions
  where the teacher actually disagreed.
- **MageZero** [V] — updated since our April snapshot (commits through
  2026-09-16; open issues #3 on MCTS/inference hot paths and #4 on hardening
  search semantics). It gates gradients to one policy head per decision type
  over a shared trunk, and documents no legal-action masking or visit-count
  normalisation.
- **MTG-Causal-RL, arXiv 2605.06066** [V] — a masked fixed 478-action space
  with PPO; evidence that fixed masked action spaces, not pointer scoring,
  are the field default. Not search distillation.

**Diagnosis, measured, not inferred.** The teacher's candidate values inside
one decision differ by a median of 0.062 = exactly 1/16, one sampled world at
K=16 (mean 0.138). The absolute-value target is therefore near-constant and
noise-dominated, and its minimiser is "predict the decision's mean" — the
collapse we measured. Queued as `bot-l9b-fix2-argmax-ce`: drop the value
term, train pure argmax cross-entropy with log-sum-exp stability, and only
then consider a residual-on-the-heuristic head.

**Deliberately NOT doing** soft-distribution (temperature/KL) targets yet:
with a one-world median spread the soft target degenerates toward uniform,
which is the failure we are escaping.

**Standing caveat from the same literature**: LOCM's search still beat its own
distilled network by a wide margin at decision time. If the student cannot
absorb the teacher, the fallback is to make the teacher cheaper (MageZero's
own open issue #3 is exactly that) rather than to keep distilling.

## Parallel (unchanged, lower priority)

AR8 combined-attacker lethal, block assignment, trace-family comparison
report, and the known red tests (stale `avengers-assemble` deck-list
assertions in `cmd/botbench`/`cmd/gorged`) — the last is a prerequisite for
any full-suite gate and should be queued first.

## Operator decisions (2026-09-19)

- Small JSON weight profiles fitted against `.cards/` **may be committed**.
  Revisit before committing any neural checkpoint (L6).
- **Promotion on gate pass is authorized**: a profile that clears L4's
  held-out gate (+3pp pooled vs `bot`, no pair below −5pp, zero new
  stalls/errors) may become the default `bot` in the same merge that
  regenerates the three `TestHeads` values, naming the measured cause.
- L0–L3 queued at priority 1 with the paid DeepInfra seat enabled.

## References

External pass, 2026-09-19. [V] = read at source; [S] = search/secondary only.

- **MageZero** — https://github.com/WillWroble/MageZero (XMage fork
  https://github.com/WillWroble/mage). [V] AlphaZero-style, PUCT c=1.0,
  **one agent per deck**; Weinberger-hashed sparse state (~2M buckets, ~200
  active/state) → EmbeddingBag → one transformer layer (summed embeddings lost
  co-occurrence: the "Skrelv problem"); fixed per-decision-type heads;
  bootstrapped from XMage minimax; 1,000 self-play games/gen; **no
  determinization/ISMCTS** (listed as open problem); ~250 games/hour on 13
  threads; ~48% avg vs minimax over 4 decks, UW Tempo 16%→66%. Author's
  lessons: strong bootstrap needed for long-horizon credit; simulator
  throughput is the bottleneck. *For gorge:* its fixed per-deck logits are
  what L6 must avoid (gorge plays arbitrary decks — per-option scoring
  instead); its throughput bottleneck is where gorge is strongest (17.5 ms/game
  in-process vs ~14 s/game). Matches the mtgbld result that naive MageZero-style
  iteration without gating did not compound.
- **Cowling, Ward & Powley 2012, Ensemble determinization in MCTS for MTG** —
  https://eprints.whiterose.ac.uk/75050/1/EnsDetMagic.pdf [V abstract]:
  many determinizations × shallow search, expert-knowledge rollouts, and
  move generation split into a binary yes/no tree. Relevant to why R1 §5.3's
  1-ply probe failed (single cheating determinization, 0.3-step leaf,
  heuristic-free evaluator).
- **ISMCTS** (Cowling, Powley & Whitehouse 2012) —
  https://eprints.whiterose.ac.uk/id/eprint/75048/ [S]. **PIMC analysis**
  (Long et al. 2010) — https://ojs.aaai.org/index.php/AAAI/article/view/7562 [S].
- **Learning to Beat ByteRL (LOCM)** — https://arxiv.org/html/2404.16689 [V]:
  BC from 3.5M pairs reached ~42% vs teacher; PPO fine-tune passed 50% within
  100 episodes, ~75% by 500. BC as *warm start*, not endpoint — consistent
  with mtgbld's BC ceiling and its PPO self-distillation result.
- **MTG-Causal-RL** — https://arxiv.org/abs/2605.06066 [V]: Gymnasium MTG
  benchmark, 478-action masked space, masked-PPO baseline.
- **Expert Iteration** — https://arxiv.org/abs/1705.08439 [S];
  **DAgger** — https://arxiv.org/abs/1011.0686 [S];
  **AlphaStar pointer-network action selection** —
  https://deepmind.google/blog/alphastar-mastering-the-real-time-strategy-game-starcraft-ii/ [S].
- **Forge AI** (rule-based, per-effect logic) —
  https://github.com/Card-Forge/forge/wiki/AI [V]. XMage AI players incl.
  AIMCTS — https://github.com/magefree/mage/tree/master/Mage.Server.Plugins [V].
- Pure-Go inference: hand-written forward pass preferred (bit-deterministic,
  zero deps; R1 §3); gonnx https://github.com/AdvancedClimateSystems/gonnx [S]
  only as a fallback.

### What the references change in this plan

1. L6's architecture is a **per-option scorer** (state embedding · option
   embedding, softmax over the legal set only) — AlphaStar pointer shape, not
   MageZero's fixed heads.
2. L6 is **warm-started** from the L3/L5 profile (as a relabelling teacher,
   DAgger-style, on states the learner visits), then improved with R1's
   PPO+KL self-distillation — the LOCM/ByteRL and mtgbld results both say a
   warm start is what makes the RL phase converge.
3. A later search teacher (L7, not scheduled) must use **ensemble
   determinization** (resample hidden zones per determinization, shallow
   search, profile-policy rollouts), never the single cheating clone R1
   measured.

## L9 measured on the full corpus (controller, 2026-09-20, main @ b007b396)

`bot-l9b-fix2-argmax-ce` merged (b007b396): pure argmax cross-entropy is the
default loss, with an optional fixed bot-prior residual (`-residual-init`).
The value-regression collapse is genuinely fixed — the model now learns.

The seat measured on a 1,426-decision fallback corpus, because the
pi-agent jail cannot read the controller's scratchpad
(`[[pi-agent-jail-blocks-tmp]]`). Re-measured here on the real 14,588-record
dev2 corpus (13,130 train / 1,458 holdout), 30 epochs, seed 1, per-kind
top-1 among labelled options:

| config | attackers | priority |
|---|---|---|
| bot baseline | 0.887 | 0.678 |
| first-option baseline | 0.897 | 0.425 |
| `-loss ce` (lr 0.1) | 0.924 | 0.355 |
| `-loss ce -lr 0.01` | 0.926 | 0.362 |
| `-loss ce -lr 0.003` | 0.921 | 0.371 |
| `-loss ce -residual-init 2` | **0.968** | 0.669 |

Two conclusions, both confirming the seat at 10x the corpus:

1. **`attackers` is learnable** and beats the bot baseline by 4-8pp.
2. **`priority` is not** — flat at 0.355-0.371 across three orders of
   magnitude of learning rate, so it is not a learning-rate artefact; with
   the residual prior it recovers to the bot baseline and supplies nothing
   beyond it. The seat's hypothesis (the teacher's override is a property of
   the PIMC rollout, and the option encoding carries only static board
   facts) survives the bigger corpus. Owned by
   `agent-20260921T012459Z-cb7a7077`.

### The L9c inference path is broken — 0/1000 in play

Neither checkpoint can play. Ten approved pairs, 100 games/pair, dev seed
10,000,000: **policynet 0 wins, bot 1000**, on the CE checkpoint AND the
residual one.

Cause: `seat/policynet.go`'s `attackersFromScores` admits an option at
`score > 0` (the value-regression era's calibrated sigmoid). A CE-trained
score is a softmax logit — shift-invariant, absolute level untrained.
Measured over the 5,152 labelled attackers decisions (9,016 options), the
scorer's range is [11575, 15052] and **100% of options score > 0**, preferred
and not alike, so the seat declares every legal attacker every combat and
empties its board into bad attacks (`-decision-stats`, 20 games: 68 attackers
decisions at 100% first-option, zero blockers decisions, 78 `choose/discard`,
against 256 / 38.7% / 90 / 0 for the bot self-control).

Neither gate caught it: the holdout metric is an argmax (shift-invariant, so
it measures exactly what CE trains and reports 0.968 for a checkpoint that
loses every game), and `seat/policynet_test.go`'s `zeroCheckpoint` scores
every option 0.0, where `0 > 0` is false — the tests only ever exercised the
empty declaration.

Filed as `bot-l9d-attack-admission` (P1). Until it lands, **no policynet
checkpoint can be benched or gated**, so L9's promotion question is not yet
askable. The `+5.80pp ± 1.25` combined-teacher result is unaffected — it was
measured with the teacher itself in the loop, not a distilled checkpoint.
