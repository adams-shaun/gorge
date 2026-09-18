# Resume prompt: continue post-rebase search-probe diagnosis

Paste the following into a fresh session with `/tmp/gorge` as the workspace.

Implementation checkpoint: `fc9f285` (`feat: diagnose search proposal
rejection`). This resume document is committed immediately after that
checkpoint. Both commits were requested for the existing remote branch after
rebasing it onto `origin/main` `0695432`. Verify the remote tip before assuming
publication. The rebase makes an ordinary push likely to reject; never
force-push to repair it without explicit authorization.

---

Continue the runnable search-probe work on gorge from the post-rebase census.
The exact land-isolation mixture was measured and then removed after its
negative coverage result; its reports and artifacts remain as evidence. The
identity-free weight/stack diagnostics remain implemented. The fixed 50/50,
2:1, and 4:1 mixtures were rejected, and the complete
`hand_to_stack` census does not justify spell isolation. Upstream engine and
bot changes materially moved the fixed protocol, so use the post-rebase
artifact as the current checkpoint. Do not describe the change as a coverage
win, force recorded opponent choices, or jump to production search/training.

## Establish the checkpoint

Read `AGENTS.md`, applicable skills, and these documents:

1. `docs/superpowers/reports/2026-09-17-search-probe-running.md` — both fixed
   calibrations, exact commands, costs, and the land-isolation comparison.
2. `docs/superpowers/specs/2026-09-17-search-land-isolation-proposal-design.md`
   — approved observable inputs, exact mixture, and fallback behavior.
3. `docs/superpowers/plans/2026-09-17-search-land-isolation-proposal.md` —
   completed test-first implementation plan.
4. `docs/superpowers/specs/2026-09-17-search-proposal-coverage-design.md` and
   `docs/superpowers/plans/2026-09-17-search-proposal-coverage.md` — underlying
   history-conditioned proposal and shuffle-planner contract.
5. `docs/superpowers/specs/2026-09-17-search-world-sampler-design.md` and
   `docs/superpowers/plans/2026-09-17-search-world-sampler.md` — original
   sampler boundary and completed plan.

Inspect git status, branch, HEAD, upstream, and remote before changing
anything. Expected branch and upstream are
`perf/hotspot-optimization-2026-09-17` and
`origin/perf/hotspot-optimization-2026-09-17`; origin is
`git@github.com:adams-shaun/gorge.git`. Do not assume temporary calibration
artifacts still exist.

The user authorized committing and normally pushing this checkpoint and this
resume handoff. That is not standing authorization to merge, rebase,
force-push, create/edit a PR, deploy, run race tests, regenerate goldens, or
publish later work. Preserve unrelated user changes.

## What is now implemented

- The active sampler is the history-conditioned proposal from before land
  isolation: exact physical permutations satisfy fixed positions and
  cumulative lower deadlines, with exact target/proposal weights.
- The proposal never forces an opponent action. Reconstruction still calls
  `botpolicy.Decide`, validates the complete observed prefix, clears the
  planner at accepted roots, and replays from Config + complete chance
  transcript + intents. Duplicate worlds remain independently mutable.
- `SampleResult` retains identity-free weight-distribution diagnostics and the
  complete normalized `hand_to_stack` cause census.
- A completed baseline game with no qualifying decision records
  `NoRootReason`; the CLI reports `EligibleRoots` and `NoRootGames` separately.
- The removed land-isolation design, implementation plan, and measurements are
  retained as historical evidence, not executable proposal behavior.

Final verification on the implementation tree passed:

```text
GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./... -count=1
GOMAXPROCS=5 GOMEMLIMIT=5GiB go vet ./...
git diff --check
```

No race suite or golden regeneration ran.

## Final fixed calibration

Protocol: death-n-taxes versus dimir-tempo, seeds 10000--10499,
current/current, no mulligans, actor `seed % 2`, first eligible turn>=5
cast/ability/pass root, sampler seed 54321, 64 proposal attempts/root, four
worlds, ESS gate >=4, 5,000 submits, five workers, `GOMAXPROCS=5`, and
`GOMEMLIMIT=5GiB`.

- 500/500 roots retained; zero errors or budget exhaustions.
- 3,155/32,000 proposals accepted; 28,715 prefix rejected; 130 incompatible.
- 131/500 roots covered (26.2%); 369 explicit baseline fallbacks.
- Death-n-taxes actor: 39/250 covered. Dimir actor: 92/250 covered.
- Later-epoch roots: 35/258 covered.
- Land isolation: 31,335 eligible attempt-shuffles, 15,744 isolation-component
  selections, 17 empty-isolation fallbacks, and 119 unsupported public shapes.
- 500 baseline, 131 one-world, 524 four-world, and 2,000 terminal outcome
  replays matched.
- Total elapsed 125.395 seconds; allocations after corpus load
  101,631,260,040 bytes.
- The one-worker 125-root run covered 26 roots and matched every corresponding
  five-worker per-game field after removing only `SampleNS` and `SearchNS`.

Final artifacts:

```text
/tmp/gorge-searchprobe-land-isolation-opt-500-20260917.json
/tmp/gorge-searchprobe-land-isolation-opt-125-20260917.json
```

The prior fixed checkpoint remains:

```text
/tmp/gorge-searchprobe-finalwave-500-20260917.json
/tmp/gorge-searchprobe-finalwave-125-20260917.json
```

If absent, reproduce artifacts using the exact commands in
`docs/superpowers/reports/2026-09-17-search-probe-running.md`; output creation
is exclusive, so choose new paths.

## Measured comparison

Compared with the prior history-conditioned proposal:

- accepted proposals rose 2,473 -> 3,155 (+682);
- `identities/hand_to_battlefield` fell 11,048 -> 6,668 (-4,380, 39.6%);
- total prefix rejection fell 29,421 -> 28,715 (-706);
- usable roots fell 138 -> 131 (-7);
- Death-n-taxes actor coverage rose 31 -> 39, while Dimir fell 107 -> 92;
- later-epoch coverage fell 41 -> 35;
- `identities/hand_to_stack` rose 10,625 -> 13,034 and is now the largest
  rejection bucket;
- total elapsed rose 114.186 -> 125.395 seconds, and allocations rose
  83,695,350,608 -> 101,631,260,040 bytes.

The exact nonuniform mixture weights reduce ESS enough to lose usable roots
despite more accepted attempts. Raw acceptance is therefore not the objective
for the next change.

## Next bounded work

Start with diagnosis, not another proposal assumption.

1. Compare the 369 fallback roots against the prior artifact and identify the
   exact roots gained and lost. Separate failures due to fewer than four
   accepted proposals from failures due only to ESS below four.
2. Measure accepted-world log-weight distributions, component membership, and
   effective sample contributions for those lost roots using deterministic,
   aggregate diagnostics derived only from owned `Frame` data and proposal
   state. Do not add names or hypothetical object IDs to serialized output.
3. Inspect representative `hand_to_stack` failures for both actors and later
   epochs. Determine whether the now-leading bucket is policy competition,
   observer-reference novelty, missing public constraints, or unavoidable
   information-consistent uncertainty.
4. Before changing proposal probabilities or adding spell isolation, write a
   narrow design with observable inputs, exact target/proposal density,
   duplicate-copy treatment, ESS implications, and empty/ineligible fallback.
   Preserve positive support. Do not tune on the 500-root checkpoint without a
   separately justified evaluation protocol.

Do not assume the fixed 50/50 land mixture should be promoted unchanged. It is
a correct experiment with a negative coverage result; retain, revise, or revert
it only on explicit measured reasoning.

## Non-negotiable boundaries

- Never give sampling the actual engine, original seed, hidden zones/hash,
  opponent-private intent indices, or callbacks into the source game.
- Never force a recorded opponent choice. Every opponent action remains the
  output of `botpolicy.Decide` in the hypothetical information set.
- All state mutation goes through `events.Apply`; do not manufacture compatible
  history with artificial zone moves.
- Exact proposal likelihoods are mandatory. Unknown shapes use the prior;
  impossible hypothetical completions reject only that attempt; public
  contradictions retain explicit baseline fallback.
- Every selected world must replay without a planner from Config, the complete
  checked chance transcript, and intents. Duplicate selections must own
  independent mutable engines and observer maps.
- Preserve fixed attempt counts, ESS gating, all-root accounting, ordinary
  event bytes/ordinals/heads, RNG behavior, clone independence, and baseline
  fallback.
- No new core/card dependencies or cgo; never track Forge card/token scripts.
- No merge, rebase, force-push, PR changes, deploy, race runs, golden changes,
  or further commit/push without fresh authorization.

Start with a concise verified status and the single highest-value cause of the
ESS-driven root losses. Do not reopen the completed exact-counting machinery
unless new evidence demonstrates a correctness defect.

## Resumed diagnosis (2026-09-17)

The root comparison and bounded prototypes are complete locally. The 43 roots
lost from the prior proposal split into 28 ESS-only losses and 15 losses with
fewer than four acceptances; 36 other roots were gained. Across all 50 roots
with at least four acceptances but ESS below four, outside-isolation accepted
worlds contributed more than 90% of squared-weight mass. On the 28 prior-covered
ESS-only losses they averaged 85.4% of normalized mass and 98.8% of squared
mass despite averaging only 2.07 accepted attempts. This is the highest-value
cause.

Exact 2:1 and 4:1 count-adaptive importance-ratio caps were evaluated on fresh
development seeds 10500--10624. They covered 36 and 37 roots respectively,
versus 37 for fixed 50/50. Both reduced ESS failures but lost acceptance; the
probability changes were reverted and no holdout was spent. The subsequent
full 500-root `hand_to_stack` census closed all 13,034 rejections: 6,175
hypothetical extra casts against an observed pass, 5,738 different-name cast
competitions, 1,118 missing observed casts, and 3 observer-reference novelties.
All were in main phase 1; 6,686/6,859 observed-cast mismatches already had a
supported public deadline. Spell isolation is therefore not justified by this
evidence. See the running report and rejected weight-cap design for commands,
artifacts, and exact densities.

## Post-rebase checkpoint (2026-09-17)

The branch was rebased onto `origin/main` `0695432`; its six earlier commits end
at `7c699ce`. The single constructor conflict was resolved by retaining
upstream's livelock watcher while using the injected RNG required by
hypothetical engines. The restored diagnostics and running report were then
committed as `fc9f285`; this resume handoff follows it.

The fixed 500-root census was rerun at
`/tmp/gorge-searchprobe-stackcensus-rebased-500-20260917.json`. It completed
with zero experiment errors and zero budget exhaustions, but upstream gameplay
drift makes it a new checkpoint rather than a deterministic reproduction of
the old artifact: only 28/500 roots match after stripping timing and the three
new diagnostic fields, and 459 baseline heads changed. Coverage is 112/499
eligible roots (39 Death-n-taxes actor, 73 Dimir actor), with 387 explicit
fallbacks and one game with no eligible root. There were 2,588 accepted
proposals out of 31,936 attempts, 29,198 prefix rejections, and 150 incompatible
proposals.

The post-rebase `hand_to_stack` census totals 11,776: 5,781 hypothetical extra
casts against pass, 4,931 policy competitions, 1,061 missing observed casts,
and 3 observer-reference novelties. All are still main phase 1; 5,973/5,995
observed-cast mismatches have a supported public deadline. The conclusion is
unchanged: spell isolation is not justified by the measured cause mix.

The post-rebase one-worker prefix at
`/tmp/gorge-searchprobe-stackcensus-rebased-125-resume-20260917.json` completed
125 games with zero errors and covered 24 roots. It matches the first 125
five-worker records exactly after removing only `SampleNS` and `SearchNS`.
The 35 post-rebase coverage losses split into 31 roots with fewer than four
accepted proposals and four ESS-only losses; 16 roots were gained.

Seed 10307 is not a sampling fallback and does not expose an engine casting
defect. Under the current explicit mana-activation flow, the bot tapped Island
for blue, played Underground Sea, selected blue from it, and reached `UU`.
Its black spells were not payable and therefore were not offered; the bot
passed, leaving no eligible turn>=5 cast/ability/pass root. The experiment now
records this state through per-game `NoRootReason` and top-level
`EligibleRoots`/`NoRootGames`; report its fixed result as 112/499 eligible roots
plus one no-root game. This bookkeeping change does not require another
500-game calibration.

The user then chose to remove land isolation from the active proposal while
retaining its reports as evidence. The fresh reverted-proposal artifacts are
`/tmp/gorge-searchprobe-post-land-revert-500-20260917.json` and
`/tmp/gorge-searchprobe-post-land-revert-125-20260917.json`. The 500-game run
had zero errors and covered 114/499 eligible roots, with one no-root game:
1,999 accepted proposals, 29,823 prefix rejections, 114 incompatible proposals,
and zero budget exhaustions. Against the post-rebase land-isolation run it
gained 40 roots and lost 38, a net gain of two. The one-worker prefix covered
26 roots and matched the five-worker prefix exactly after removing only
`SampleNS` and `SearchNS`.

Focused search-probe tests, `go vet ./...`, and `git diff --check` pass. The
full suite is blocked by two failures also reproduced on clean `origin/main`:
the botbench constructed-default golden measures 18/2 rather than 16/4, and
the host's stall-guard opt-out test misses its 200-decision/30-second target.
Do not update those unrelated tests or goldens as part of this work. The remote
feature branch may still name the pre-rebase history. An ordinary push was
authorized, but do not force-push without explicit authorization.

## Next bounded work

The ten-iteration CPU/allocation pass is complete locally and remains
uncommitted. Its exact final 500-game run preserved every non-timing result:
114/499 eligible roots covered, one no-root game, and zero errors. Relative to
the post-land-revert profiled baseline, wall time fell 114.862 -> 86.275 seconds
(-24.9%), profiled CPU fell 558.42 -> 405.30 CPU-seconds (-27.4%), and runtime
allocated bytes fell 89,925,803,696 -> 58,309,831,952 (-35.2%). Sampled
`alloc_space` fell 83.98 -> 54.65 GiB and sampled allocated objects fell 1.119
billion -> 502.3 million. End live heap did not improve (`HeapAlloc` 16.23 ->
18.34 MB, `HeapSys` 238.55 -> 255.33 MB), but the final sampled live heap is
only 2 MiB and contains runtime/JSON caches rather than a retained sampler
graph.

The first five iterations optimize exact constrained-permutation counting:
factorial precomputation, immutable memo values, compact keys, in-place
decrement/restore recursion, and cross-attempt compiled-plan reuse. The last
five reuse opponent boards, cache `TurnsTaken`, reserve observed-prefix logs,
prefilter granted Ward/Dethrone checks, and reuse observation redaction
scratch. The log reserve is the one mixed result: it replaces about 2.00 GiB of
geometric growth with 2.13 GiB of exact reserve allocation, while the
second-half profile removes 31.79 CPU-seconds of `runtime.duffcopy`; do not
describe it as an allocation win. Full iteration details and remaining profile
leaders are in `docs/superpowers/reports/2026-09-17-search-probe-running.md`.

Final artifacts:

```text
/tmp/gorge-searchprobe-profile-final-500-20260917.json
/tmp/gorge-searchprobe-profile-final-bin-20260917
/tmp/gorge-searchprobe-profile-final-cpu-500-20260917.pprof
/tmp/gorge-searchprobe-profile-final-heap-500-20260917.pprof
/tmp/gorge-searchprobe-final-cpu.svg
/tmp/gorge-searchprobe-cpu-diff.svg
/tmp/gorge-searchprobe-final-alloc-space.svg
/tmp/gorge-searchprobe-alloc-space-diff.svg
/tmp/gorge-searchprobe-final-inuse-space.svg
```

Fresh verification after iteration ten passed:

```text
GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./internal/searchprobe ./cmd/searchprobe ./rules -count=1
GOMAXPROCS=5 GOMEMLIMIT=5GiB go vet ./...
git diff --check
```

No race suite, golden regeneration, commit, push, merge, rebase, PR edit, or
deployment was run. If continuing optimization, the dominant CPU target is
object/trigger traversal (`forEachObject`, `checkFaceTriggers`, `Object.Face`),
and the dominant churn targets are livelock snapshots, object creation, event
log growth, JSON/view projection, and `Collector.Capture`. Preserve explicit
mana tapping and every current search-probe output invariant.
