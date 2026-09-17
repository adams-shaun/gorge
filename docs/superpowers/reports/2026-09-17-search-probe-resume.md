# Resume prompt: reduce remaining history-conditioned prefix rejection

Paste the following into a fresh session with `/tmp/gorge` as the workspace.

Implementation checkpoint: `ed50325` (`feat: condition search proposals on
observed history`). This resume document is committed immediately after that
checkpoint. Both commits were requested for the existing remote branch. Verify
the remote tip before assuming publication; never force-push to repair a
mismatch.

---

Continue the runnable search-probe work on gorge. The exact
history-conditioned proposal mechanism is complete. The next problem is the
remaining full-prefix rejection tail, not another rewrite of the sampler, a
perfect-information shortcut, a trainer, or production MCTS.

## Establish the checkpoint

Read `AGENTS.md`, applicable skills, and these documents:

1. `docs/superpowers/reports/2026-09-17-search-probe-running.md` — final
   measured calibration and reproduction commands.
2. `docs/superpowers/specs/2026-09-17-search-proposal-coverage-design.md` —
   information boundary, proposal distribution, and shuffle-planner design.
3. `docs/superpowers/plans/2026-09-17-search-proposal-coverage.md` — completed
   implementation plan.
4. `docs/superpowers/specs/2026-09-17-search-world-sampler-design.md` and
   `docs/superpowers/plans/2026-09-17-search-world-sampler.md` — original
   sampler contract and completed prior plan.

Inspect git status, branch, HEAD, upstream, and remote before changing
anything. Expected branch and upstream are
`perf/hotspot-optimization-2026-09-17` and
`origin/perf/hotspot-optimization-2026-09-17`; origin is
`git@github.com:adams-shaun/gorge.git`. Do not assume temporary calibration
artifacts still exist.

The user authorized committing and pushing this checkpoint. That is not
standing authorization to merge, rebase, force-push, create/edit a PR, deploy,
run race tests, regenerate goldens, or publish later work. Preserve unrelated
user changes.

## What is now implemented

- Deterministic rejection histograms keyed by frame, component, and normalized
  public shape. No card names or hypothetical hidden IDs enter diagnostics.
- Exact constrained physical-card permutation counting and uniform unranking
  with `math/big`, including duplicate names, exact objects, fixed positions,
  cumulative deadlines, contradictions, and exact `|C| / n!` log weights.
- Compilation of allowed observer history into per-player shuffle epochs:
  actor draws, public opponent hand exits, later shuffles, exact known-object
  redraws, and actor-visible `KArrange` windows. Supported actor arrange answers
  preserve original-shuffle positions across the matching `LibraryOrder`;
  unsupported or private ordering changes fall back to prior sampling.
- An opt-in hypothetical `ShufflePlanner` used by genesis, mulligan, search,
  and Shuffle effects. Planned permutations are validated, translated into the
  canonical Fisher-Yates draws, advance PCG state, and are stored in the normal
  replayable chance transcript. Planner state is absent from clones and is
  cleared at accepted roots.
- Multi-epoch proposal integration with separate deterministic streams for
  engine randomness, proposal selection, opponent policies, and weighted
  resampling. Later-shuffle opponent hand contents satisfy public name
  requirements before any library deficit is imposed.
- Attempt-local incompatible completions, typed public-contradiction baseline
  fallback, full-prefix validation, ESS gating, independent duplicate worlds,
  and Config + chance-transcript + intent replay all remain enforced.
- Ordinary shuffle event bytes, chain heads, RNG counts, and clone behavior are
  covered across genesis, mulligan, search, and Shuffle effects.

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

- 500/500 roots retained; zero experiment errors, invariant failures, budget
  exhaustions, or nonterminal outcomes.
- 2,473/32,000 proposals accepted; 29,421 prefix rejected; 106 incompatible.
- 138/500 roots covered (27.6%); 362 explicit baseline fallbacks.
- Death-n-taxes actor: 31/250 covered. Dimir actor: 107/250 covered.
- Later-epoch roots: 41/258 covered, up from 6/258.
- 500 baseline and 2,000 paired terminal outcome replays matched; selected
  one-/four-world replay entries were 138/552.
- Total elapsed 114.186 s; allocations after corpus load 83,695,350,608 bytes.
- A one-worker 125-root run matched every corresponding five-worker per-game
  field after removing only `SampleNS` and `SearchNS`.

This improves the fixed checkpoint from 164 accepted proposals and 17/500
covered roots to 2,473 and 138/500. It is coverage/correctness evidence only,
not evidence of action quality, search strength, or promotion readiness.

Final artifacts:

```text
/tmp/gorge-searchprobe-finalwave-500-20260917.json
/tmp/gorge-searchprobe-finalwave-125-20260917.json
```

If absent, reproduce them using the exact commands in
`docs/superpowers/reports/2026-09-17-search-probe-running.md`; output creation
is exclusive, so choose new paths.

## Next bounded work

Start with diagnosis, not an implementation assumption. The 29,421 remaining
prefix rejections comprise 26,967 identity and 2,454 event mismatches. Leading
shapes are:

- `identities/hand_to_battlefield`: 11,048
- `identities/hand_to_stack`: 10,625
- `identities/other`: 3,686
- `events/priority_to_stack_resolve`: 1,679
- `identities/hand_to_exile`: 1,583

Determine why hand-exit mismatches remain after the required card names are
available by their public deadlines. Separate at least these hypotheses:

1. the frozen opponent policy chooses a different legal play because the rest
   of its sampled hand or board differs;
2. observer-reference novelty for interchangeable same-name physical copies
   causes avoidable rejection;
3. a supported public constraint is missing or is assigned to the wrong epoch;
4. the divergence is an unavoidable consequence of information-consistent
   prior uncertainty and should remain rejection.

Use bounded, deterministic diagnostics derived only from `Frame` values.
Inspect representative failures for both actors and later epochs. Before any
new proposal mechanism, write a narrow design that defines its observable
inputs, exact target/proposal probability, duplicate-copy treatment, and
fallback behavior. Do not force recorded opponent choices: they remain outputs
of the frozen policy in each hypothetical information set unless a separately
approved, correctly weighted history-likelihood model replaces that contract.

Repeat the fixed 500-root calibration and one-worker deterministic prefix only
after a test-first correctness change. Preserve all roots and compare against
2,473/32,000 accepted, 138/500 covered, actor coverage 31/250 and 107/250, and
41/258 later-epoch coverage. Do not make strength claims.

## Non-negotiable boundaries

- Never give sampling the actual engine, original seed, hidden zones/hash,
  opponent-private intent indices, or callbacks into the source game.
- All state mutation goes through `events.Apply`; do not create artificial
  present-day zone moves to manufacture a compatible history.
- Exact proposal likelihoods are mandatory. Unknown shapes use the prior;
  impossible hypothetical completions reject only that attempt; public
  contradictions retain explicit baseline fallback.
- Every selected world must replay without a planner from Config, the complete
  checked chance transcript, and intents. Duplicate selections must own
  independent mutable engines and observer maps.
- No new core/card dependencies or cgo; never track Forge/token script text.
- Preserve event ordinals/bytes/heads, ordinary RNG behavior, clone
  independence, fixed work counts, and baseline fallback accounting.
- No merge, rebase, force-push, PR changes, deploy, race runs, golden changes,
  or further commit/push without fresh authorization.

Start with a concise verified status and the single highest-value residual
rejection mechanism. Do not reopen the completed exact-proposal design unless
new evidence demonstrates a correctness defect.
