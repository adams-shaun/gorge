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
The exact land-isolation mixture and identity-free weight/stack diagnostics are
implemented. The fixed 50/50 calibration remains mixed and non-promotable; 2:1
and 4:1 weight caps were measured and rejected, and the complete
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

- The exact physical-permutation solver supports cumulative upper deadlines,
  including validation, exact counting, uniform unranking, membership tests,
  fixed objects, duplicate names, and lower/upper intersections.
- Upper bounds prune impossible prefixes before their deadline, and the chosen
  component reuses its populated exact counter. This green refactor preserved
  every non-timing calibration result while reducing the first implementation's
  327.833 seconds / 370,156,069,016 allocated bytes to 125.395 seconds /
  101,631,260,040 bytes.
- Epoch compilation recognizes only unambiguous public opponent
  Hand-to-Battlefield moves paired with `LandPlayed`. It records the draw
  deadline, observed name, and a sorted snapshot of prior named hand exits.
  Actor plays, ambiguous moves, post-shuffle hand entries, unnamed exits, and
  lost library reliability remain unsupported and are counted without erasing
  earlier supported facts.
- Public front-face land names are classified from `PublicGame.Decks`. For a
  supported observed land `y` at deadline `d`, every differently named public
  land `x` receives `drawn(x,d) <= priorExit(x,d) - hand0(x)`.
- With base set `C` and nonempty isolated subset `L`, the proposal chooses each
  component with probability one half and uses the exact density
  `q(x)=1/(2|C|)+I[x in L]/(2|L|)`. Empty or ineligible `L` samples `C` exactly
  as before and consumes no component-selection draw.
- The proposal never forces an opponent action. Reconstruction still calls
  `botpolicy.Decide`, validates the complete observed prefix, clears the
  planner at accepted roots, and replays from Config + complete chance
  transcript + intents. Duplicate worlds remain independently mutable.
- `SampleResult` now reports `LandIsolationEligible`,
  `LandIsolationSelected`, `LandIsolationEmpty`, and
  `LandIsolationUnsupported` without exposing card names or object IDs.

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
new diagnostic fields, and 459 baseline heads changed. Coverage is 112/500
(39 Death-n-taxes actor, 73 Dimir actor), with 387 explicit fallbacks and one
game with no eligible root. There were 2,588 accepted proposals, 29,198 prefix
rejections, and 150 incompatible proposals.

The post-rebase `hand_to_stack` census totals 11,776: 5,781 hypothetical extra
casts against pass, 4,931 policy competitions, 1,061 missing observed casts,
and 3 observer-reference novelties. All are still main phase 1; 5,973/5,995
observed-cast mismatches have a supported public deadline. The conclusion is
unchanged: spell isolation is not justified by the measured cause mix.

Focused search-probe tests, `go vet ./...`, and `git diff --check` pass. The
full suite is blocked by two failures also reproduced on clean `origin/main`:
the botbench constructed-default golden measures 18/2 rather than 16/4, and
the host's stall-guard opt-out test misses its 200-decision/30-second target.
Do not update those unrelated tests or goldens as part of this work. The remote
feature branch may still name the pre-rebase history. An ordinary push was
authorized, but do not force-push without explicit authorization.

## Next bounded work

1. Verify the remote tip and local status. If the normal push was rejected
   because of the rebase, report that fact and request explicit authorization
   before using `--force-with-lease`.
2. Run the post-rebase 125-root, one-worker prefix to a new exclusive artifact
   and compare it with the first 125 results of the five-worker artifact after
   deleting only `SampleNS` and `SearchNS`. This worker-invariance check has not
   yet been repeated on the new engine.
3. Explain seed 10307's lack of an eligible turn>=5 root and decide whether the
   protocol should represent that state explicitly; do not silently count it
   as a sampled fallback.
4. Compare the 35 post-rebase coverage losses and 16 gains before changing the
   proposal again. Keep the rejected weight caps reverted and do not add spell
   isolation from the current evidence.
5. Treat the two clean-`origin/main` test failures as upstream work. Do not
   update their goldens or host behavior inside the search-probe task.
