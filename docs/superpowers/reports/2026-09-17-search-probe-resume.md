# Resume prompt: diagnose land-mixture ESS loss and remaining rejection

Paste the following into a fresh session with `/tmp/gorge` as the workspace.

Implementation checkpoint: `60a47bcfa9f27698ba6bf4fcf2ddf263875d8d0c`
(`feat: add land-isolation search proposal`). This resume document is committed
immediately after that checkpoint. Both commits were requested for the existing
remote branch. Verify the remote tip before assuming publication; never
force-push to repair a mismatch.

---

Continue the runnable search-probe work on gorge. The exact land-isolation
mixture is implemented and verified, but its fixed calibration is a mixed,
non-promotable result: it reduces the targeted rejection and raises raw
acceptance while lowering the four-world/ESS coverage gate. Start from that
evidence. Do not describe the change as a coverage win, force recorded opponent
choices, or jump to production search/training.

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

The user authorized committing and pushing this checkpoint and this resume
handoff. That is not standing authorization to merge, rebase, force-push,
create/edit a PR, deploy, run race tests, regenerate goldens, or publish later
work. Preserve unrelated user changes.

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
