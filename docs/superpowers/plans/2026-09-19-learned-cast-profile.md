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
