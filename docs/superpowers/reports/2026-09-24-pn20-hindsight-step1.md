# pn20 step 1: hindsight branch mining — measurement (2026-09-24)

Brief: `.superpowers/briefs/policynet-exit/20-hindsight-branch-mining.md`.
Raw records: `/mnt/sata/gorge-training/pn20/smoke/records.jsonl`.

## TL;DR

**KILL, but confounded.** On a 3-game smoke sample (864 recorded decisions), the clear-decision
rate is 0.00% and the kill criteria fire. The direct cause is not a lack of decisive positions: it
is that 73 of the 77 decisions with ≥2 candidates (94.8%) hit `no_world` — the PIMC sampler
(`searchprobe.Sample`) could not produce a consistent hidden-information world to roll out from,
so only 4 decisions were ever actually scored. This is the same sampler-ESS starvation the L10
teacher seat already measured (t13+ coverage 4.8%). **Recommendation: do not run the 300-game
step further until pn16-b4 (redeal fallback) merges, then re-run this measurement** — the smoke
result says nothing about whether clear-margin labels exist once the sampler can actually answer
late-game decisions; it only shows they can't be measured without it. The 300-game run was
started, then stopped deliberately (operator decision, not a seat failure) once this was clear,
to avoid spending ~4.5 projected CPU-hours reproducing the same starvation.

## What was built

- **`internal/hindsight`** (new, pure Go): walks a recorded bot-vs-bot game backward from its last
  decision, clones the engine at each decision (`rules.Engine.Clone`), and for each untried option
  runs rollouts to game end. Every rollout re-samples the hidden information the deciding seat
  cannot see (`searchprobe.Sample`) with a fresh, deterministic seed per (game, decision, option,
  rollout) — it never reuses the real recorded hand/library order, so a rollout cannot see future
  draws it wouldn't otherwise know. A small omniscient arm (real hidden state, no resampling) is
  run separately and labelled leaked, purely to measure how much that leak would have inflated the
  clear-margin rate.
  - Adaptive rollout budget: 16 rollouts per option, +16 per block, up to 128, stopping once the
    top two options' Wilson 95% CIs stop overlapping.
  - "Clear margin": best alternative ≥ chosen + 0.10 win rate AND the paired-difference 95% CI
    excludes 0.
  - A `no_world` outcome is counted and skipped, never silently fallen back to the real state.
- **`internal/searchprobe`** extension (additive, both old entry points kept as thin wrappers):
  - `Collector.IntentActions`/`MatchIntent`: like `Actions`/`Match`, but also carry KArrange's
    independently-ordered `Rest` pile, which `Action`/`Choices` alone cannot represent — needed so
    hindsight can branch on an arrange decision's non-chosen ordering.
  - `TeacherIntentChoice`/`SemanticIntent`: like `TeacherChoice`, with the same `Rest` support, plus
    `TeacherResult.WinsOverBaseline`/`LossesToBaseline` (paired win/loss counts against
    candidate 0 in the same sampled world) — the raw counts hindsight's paired-CI math needs.
  - `TestTeacherChoiceRealDeckGolden`'s pinned digest moved (`fa6bb92b...` → `d97fc053...`)
    because `TeacherResult` gained two hashed fields; `Index`/`Values`/`Rollouts`/`Terminal`/
    `Capped`/the 8/8/8/8 wins split are unchanged (both new fields are `[0,0,0,0]` on the pinned
    fixture). Golden updated in `bench_test.go` with the same "re-measured" comment convention the
    file already uses for prior digest moves. `go test ./internal/searchprobe/... ./internal/searchseat/... ./view/...`
    and `go build ./...`/`go vet ./...` for the whole module all pass with this change.
- **`cmd/hindsight`**: CLI wrapping the package; deterministic JSONL + Markdown report output.

## 1. Clear-decision and no-world rates (3 smoke games: 2 losses, 1 win)

864 recorded decisions; 77 had ≥2 candidate answers (787 had a single legal answer and were not branched). The smoke ran with `-block 16 -max-rollouts 16`, i.e. a fixed 16 rollouts per option; the adaptive budget (up to 128) was only configured for the stopped full run.
**0 of 77 (0.00%) were clear.** **73 of 77 attempted (94.81%) were `no_world`.**

| outcome | turn | kind | decisions | clear | clear % | no_world | no_world % |
|---|---:|---|---:|---:|---:|---:|---:|
| loss | 1-3 | choose | 1 | 0 | 0.00 | 1 | 100.00 |
| loss | 1-3 | priority | 54 | 0 | 0.00 | 0 | 0.00 |
| loss | 4-6 | choose | 1 | 0 | 0.00 | 1 | 100.00 |
| loss | 4-6 | priority | 60 | 0 | 0.00 | 1 | 1.67 |
| loss | 7-12 | attackers | 3 | 0 | 0.00 | 3 | 100.00 |
| loss | 7-12 | blockers | 1 | 0 | 0.00 | 1 | 100.00 |
| loss | 7-12 | choose | 6 | 0 | 0.00 | 5 | 83.33 |
| loss | 7-12 | modes | 1 | 0 | 0.00 | 1 | 100.00 |
| loss | 7-12 | priority | 149 | 0 | 0.00 | 10 | 6.71 |
| loss | 7-12 | target | 1 | 0 | 0.00 | 0 | 0.00 |
| loss | 13+ | attackers | 11 | 0 | 0.00 | 11 | 100.00 |
| loss | 13+ | blockers | 4 | 0 | 0.00 | 4 | 100.00 |
| loss | 13+ | modes | 1 | 0 | 0.00 | 0 | 0.00 |
| loss | 13+ | priority | 286 | 0 | 0.00 | 10 | 3.50 |
| loss | 13+ | target | 3 | 0 | 0.00 | 0 | 0.00 |
| win | 1-3 | priority | 32 | 0 | 0.00 | 0 | 0.00 |
| win | 4-6 | priority | 37 | 0 | 0.00 | 2 | 5.41 |
| win | 4-6 | target | 1 | 0 | 0.00 | 0 | 0.00 |
| win | 7-12 | attackers | 2 | 0 | 0.00 | 2 | 100.00 |
| win | 7-12 | blockers | 1 | 0 | 0.00 | 1 | 100.00 |
| win | 7-12 | choose | 2 | 0 | 0.00 | 2 | 100.00 |
| win | 7-12 | priority | 84 | 0 | 0.00 | 6 | 7.14 |
| win | 7-12 | target | 1 | 0 | 0.00 | 1 | 100.00 |
| win | 13+ | attackers | 5 | 0 | 0.00 | 5 | 100.00 |
| win | 13+ | blockers | 1 | 0 | 0.00 | 1 | 100.00 |
| win | 13+ | priority | 115 | 0 | 0.00 | 4 | 3.48 |
| win | 13+ | target | 1 | 0 | 0.00 | 1 | 100.00 |

The pattern is
consistent with the known L10 finding: `priority` decisions (small hidden-info surface, often
resolved with mana/land info already public) mostly get a world; non-priority decisions late in
the game (attackers, blockers, targets, seven-plus turns of hidden draws) mostly don't.

## 2. Reliability

0/0 — no clear decisions existed to re-run. Not evaluable from this sample; the metric needs a
non-empty clear set, which requires enough successfully-sampled decisions in the first place.

## 3. Pivot structure in losses

0.000 mean clear decisions/game (0/2 losses had any). Not evaluable — same root cause.

## 4. Cost

97.9 CPU-s / 49.2 wall-s for 3 games → 32.6 CPU-s/game. Linear projection for 10k games on 20
cores: **4.53h** (PASS against the 48h kill criterion). Cost is not the blocker.

## 5. Hidden-information leak inflation

0/70 clear resampled vs 0/70 clear leaked (omniscient) on the same 2 games: +0.00pp inflation.
Uninformative at this sample size/starvation level, but confirms the leaked arm isn't hiding a
sampler bug that makes resampled decisions look artificially worse.

## 6. Hand-read examples

None — no clear decisions were produced.

## Kill criteria

- Clear decisions <2%: **KILL** — 0.00% (measured on 4 actually-scored decisions out of 864).
- Reliability <70%: **KILL** — 0/0, undefined; treated as failing since there was nothing to
  confirm.
- Projected 10k-game cost >48h: **PASS** — 4.53h.

## Recommendation

**Do not proceed to pattern mining/distillation now, and do not scale this measurement past
smoke.** The kill verdict is real but not informative about the hindsight-mining premise itself —
it mostly re-measures that `searchprobe.Sample` can't find a consistent world for most non-priority
late-game decisions, which is exactly what pn16-b4 (the redeal fallback,
`wt/spike-redeal`/commit 9e2321ce2) exists to fix. Once pn16-b4 merges:
1. Re-run this same smoke procedure (3–20 games) with the redeal fallback wired into
   `internal/hindsight`'s per-rollout sampling, and confirm the `no_world` rate drops.
2. Only then decide whether to spend the 4.53 projected CPU-hours on the full 300-game run —
   the cost estimate here should still roughly hold, since redeal adds a fallback path, not more
   rollouts.
3. `priority` decisions already sample well; a `-kinds priority` first pass could get partial
   signal before pn16-b4 lands, if useful as a smaller side measurement. Not attempted here.

## Method and limitations

- Branches were visited backward from the last recorded decision. `rules.Engine.Clone` retains the
  exact branch boundary; every rollout world was independently sampled consistent with the
  deciding seat's observation history — no rollout used the real recorded hand or library order.
- Wilson 95% intervals per option; a clear label additionally requires the paired {-1,0,+1} outcome
  difference's 95% CI to exclude 0.
- A `no_world` decision is excluded from the clear-rate denominator's "attempted" count but
  included in the raw decision count, matching the brief.

## Issues

No engine defect found. The `internal/searchprobe` golden digest move (§ above) is expected
collateral of adding fields to a hashed struct, not a behavior change; already fixed in this
commit.

## What was not done

- The 300-game full run was started and deliberately stopped (see TL;DR) rather than left running
  to completion, since the smoke result already determines the kill verdict and cost projection.
- Step 2 (pattern mining) and step 3 (distillation/bot rules) are out of scope per the kill
  criteria above.
