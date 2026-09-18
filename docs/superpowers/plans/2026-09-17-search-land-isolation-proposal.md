# Search Probe Land-Isolation Proposal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce opponent land-play prefix rejection with an exactly weighted, support-preserving mixture proposal that leaves the frozen policy in control.

**Architecture:** Extend the exact physical-permutation solver with cumulative upper bounds, compile supported public land plays into per-epoch isolation facts, and mix the current constrained distribution with a stricter land-isolation subset. The mixture keeps the current component at probability one half, computes exact density from both set sizes, and retains full-prefix rejection and replay as final correctness checks.

**Tech Stack:** Go 1.25 standard library, existing `internal/searchprobe`, `rules`, `events`, `cards`, and `botpolicy` packages, `math/big` for exact counts.

**Spec:** `docs/superpowers/specs/2026-09-17-search-land-isolation-proposal-design.md`

## Global Constraints

- Consume only owned `History`, public deck definitions, and the hypothetical shuffle context; never consume the source engine, original seed, hidden source zones/hash, opponent-private intent indices, or source callbacks.
- Never force a recorded opponent choice. Every opponent action remains the output of `botpolicy.Decide` in that hypothetical information set.
- Preserve positive proposal support and exact target/proposal likelihoods over distinct physical permutations.
- All state mutation remains in ordinary events through `events.Apply`; preserve ordinary event bytes, ordinals, heads, RNG behavior, and clone independence.
- Preserve fixed attempt counts, ESS gating, selected-world replay, independent duplicate worlds, all-root accounting, and explicit baseline fallback.
- Add no third-party dependency or cgo and never track Forge/token script text.
- Use `GOMAXPROCS=5 GOMEMLIMIT=5GiB` for tests and calibration.
- Do not run race tests or regenerate goldens.
- Do not commit, push, merge, rebase, edit a PR, or deploy without fresh authorization.

---

### Task 1: Exact cumulative upper bounds and mixture arithmetic

**Files:**
- Modify: `internal/searchprobe/constraints.go`
- Modify: `internal/searchprobe/constraints_test.go`

**Interfaces:**
- Produce: `upperDeadlineConstraint{Through int, Name string, Count int}`
- Extend: `newConstraintCounter(cards, positions, deadlines, upperDeadlines)`
- Extend: `sampleConstrainedPermutation(cards, positions, deadlines, upperDeadlines, r)`
- Produce: `constraintCounter.total(available []proposalCard) *big.Int`
- Produce: `constraintCounter.contains(order []state.ObjID) bool`
- Produce: `mixtureLogTargetOverProposal(n int, baseCount, isolatedCount *big.Int, inIsolated bool) float64`

- [x] **Step 1: Add exhaustive upper-bound tests**

  Extend the existing four-card enumeration table with exact physical counts:

  ```go
  upper := []upperDeadlineConstraint{{Through: 2, Name: "B", Count: 0}}
  // [A1,A2,B,C]: B excluded from the first two positions => 12 orders.
  ```

  Add rows combining a lower bound and an upper bound at the same deadline,
  nested upper bounds, a fixed physical object, and duplicate `A` copies.
  Enumerate every rank and assert each satisfying physical permutation appears
  once and only once.

- [x] **Step 2: Add invalid and impossible upper-bound tests**

  Cover negative `Through`, `Through > len(cards)`, negative `Count`, a lower
  bound greater than an upper bound, and a fixed-position card that violates an
  upper bound. Invalid syntax must return an error; a valid but empty set must
  return `compatible == false`.

- [x] **Step 3: Add exact mixture-density tests**

  Use `C` with 24 physical orders and `L` with 12. Assert:

  ```go
  outside := math.Exp(mixtureLogTargetOverProposal(4, big.NewInt(24), big.NewInt(12), false))
  inside := math.Exp(mixtureLogTargetOverProposal(4, big.NewInt(24), big.NewInt(12), true))
  // outside = (1/24)/(1/(2*24)) = 2
  // inside  = (1/24)/(1/(2*24)+1/(2*12)) = 2/3
  ```

  Also assert that an absent isolation component reproduces the existing
  `|C|/n!` factor.

- [x] **Step 4: Run the focused tests and confirm the new cases fail**

  Run:

  ```sh
  GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./internal/searchprobe \
    -run 'TestConstrainedPermutation|TestMixtureLogTarget' -count=1
  ```

  Expected: compile failures for the missing upper-bound interfaces.

- [x] **Step 5: Implement upper-bound pruning and membership**

  Add upper bounds to the counter's relevant-name set and stop position. At
  each reached prefix, calculate the already placed count using the same
  physical-copy accounting as lower deadlines:

  ```go
  placed := c.exactPrefix[pos][i] + c.initialFree[i] - remaining[i]
  if placed < lower.Count || placed > upper.Count {
      return false
  }
  ```

  Validate lower/upper intersections before unranking. `contains` must validate
  the complete physical permutation, fixed positions, lower bounds, and upper
  bounds without collapsing duplicate IDs.

- [x] **Step 6: Implement stable exact-count mixture arithmetic**

  Keep set sizes as `big.Int`. Convert only their logarithms and combine
  density terms with a two-term log-sum-exp:

  ```go
  logQ := -math.Ln2 - logBigInt(baseCount)
  if inIsolated {
      logQ = logAddExp(logQ, -math.Ln2-logBigInt(isolatedCount))
  }
  return -logFactorial(n) - logQ
  ```

  Reject zero base counts as an invariant; an empty isolated count is handled
  by the caller as the single-component distribution.

- [x] **Step 7: Run focused tests and existing permutation/weight tests**

  Run:

  ```sh
  GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./internal/searchprobe \
    -run 'TestConstrainedPermutation|TestMixtureLogTarget|TestWeights|TestPermutation' -count=1
  ```

  Expected: PASS.

---

### Task 2: Compile supported public land-isolation facts

**Files:**
- Modify: `internal/searchprobe/epochs.go`
- Modify: `internal/searchprobe/epochs_test.go`

**Interfaces:**
- Produce: `nameCount{Name string, Count int}` with deterministic name ordering
- Produce: `landIsolationConstraint{Through int, Name string, PriorExits []nameCount}`
- Extend: `epochConstraints.LandIsolation []landIsolationConstraint`
- Extend: `epochConstraints.LandIsolationUnsupported int`

- [x] **Step 1: Add a failing ordinary-land-play compiler test**

  Build a history with opponent genesis draws followed by frames containing:

  ```go
  ObservedEvent{Kind: events.MoveZone, Obj: landRef, From: state.ZHand, To: state.ZBattlefield}
  ObservedEvent{Kind: events.LandPlayed, Player: opponent}
  ```

  Assert the epoch records the observed name, current draw count, and a sorted
  snapshot of named prior hand exits. Add a second differently named land play
  after a draw and verify its snapshot includes the first exit exactly once.

- [x] **Step 2: Add failing eligibility-boundary tests**

  Assert no isolation fact is emitted for:

  - Hand-to-Battlefield without a matching `LandPlayed`;
  - multiple ambiguous Hand-to-Battlefield moves in the same burst;
  - actor-owned land plays;
  - a play after any post-shuffle hand entry;
  - a play after an unnamed/secret hand exit; and
  - a play after existing library reliability is lost.

  Each skipped candidate increments `LandIsolationUnsupported`; earlier
  supported plays in the same epoch remain present.

- [x] **Step 3: Run epoch tests and confirm the new assertions fail**

  Run:

  ```sh
  GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./internal/searchprobe \
    -run 'TestCompileEpochs.*Land|TestCompileEpochs.*Hand' -count=1
  ```

  Expected: compile failures for the missing land-isolation fields.

- [x] **Step 4: Implement ordered hand accounting and land-play recognition**

  Extend each player's epoch cursor with `handReliable` and sorted public exit
  counts. At each frame, precompute whether exactly one visible
  Hand-to-Battlefield object owned by the `LandPlayed.Player` pairs with that
  event. Process events in their recorded order: snapshot prior exits before
  counting the current exit, and invalidate subsequent candidates on any hand
  entry or unnamed exit.

  Keep names in epoch constraints because they are actor-observed facts. Never
  add names to aggregate diagnostics. Reset hand reliability and exit counts at
  each shuffle.

- [x] **Step 5: Preserve deterministic ordering and cloning semantics**

  Convert exit-count maps to sorted `[]nameCount` when storing a fact. Deep-copy
  slices when moving epoch constraints so later cursor mutations cannot change
  earlier facts. Do not range over a map to produce proposal decisions or
  serialized diagnostics.

- [x] **Step 6: Run all epoch and observation tests**

  Run:

  ```sh
  GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./internal/searchprobe \
    -run 'TestCompileEpochs|TestObservation|TestAction' -count=1
  ```

  Expected: PASS with existing deadline, arrange-window, and unguided behavior
  unchanged.

---

### Task 3: Integrate the support-preserving land mixture

**Files:**
- Modify: `internal/searchprobe/proposal.go`
- Modify: `internal/searchprobe/proposal_test.go`
- Modify: `internal/searchprobe/sample.go`
- Modify: `internal/searchprobe/weights_test.go`

**Interfaces:**
- Extend: `proposalState.landNames map[state.PlayerID]map[string]bool`
- Produce: `publicLandNames(setup PublicGame) map[state.PlayerID]map[string]bool`
- Produce: `isolationUpperDeadlines(epoch epochConstraints, handCounts map[string]int, landNames map[string]bool) ([]upperDeadlineConstraint, bool)`
- Extend: `SampleResult` with `LandIsolationEligible`, `LandIsolationSelected`, `LandIsolationEmpty`, and `LandIsolationUnsupported` integer counters

- [x] **Step 1: Add failing upper-bound construction tests**

  For an observed land `A` at draw count 7, deck land names `A/B`, no previous
  exits, and an empty boundary hand, assert `B <= 0 through 7`. Add cases where
  one `B` already exited (`B <= 1`), one `B` is in the later-shuffle hand
  (`B <= -1`, empty isolation), and two successive observed land names produce
  both required bounds.

- [x] **Step 2: Add failing mixture integration tests**

  Use a tiny physical deck with two differently named lands and duplicate
  copies. Enumerate both component-selection branches with deterministic RNGs.
  For every resulting order, recompute `q(x)` from `|C|`, `|L|`, and membership
  and assert the returned log factor exactly matches the Task 1 helper.

  Assert that:

  - both components can emit an order inside `L`;
  - only the base component emits an order in `C \\ L`;
  - every order in `C` retains positive support;
  - an empty `L` consumes no mixture selection and reproduces the old order and
    weight for the same proposal RNG stream; and
  - duplicate-name lands are distinct orders but never treated as competitors.

- [x] **Step 3: Add a real-engine frozen-policy regression**

  Construct a two-player history whose opponent opening hand can contain the
  observed land plus a differently named land that `chooseLand` prefers. Pin a
  deterministic attempt where the base component rejects at that land play and
  the isolation component reaches beyond it. Assert the opponent intent still
  comes from `botpolicy.Decide`, the accepted world passes the complete prefix,
  and Config + checked chance transcript + intents replay without a planner.

- [x] **Step 4: Run focused proposal tests and confirm failure**

  Run:

  ```sh
  GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./internal/searchprobe \
    -run 'TestLandIsolation|TestProposal.*Land|TestSelectedWorldReplay' -count=1
  ```

  Expected: failures for missing mixture construction and counters.

- [x] **Step 5: Implement public land classification**

  Build a per-player set from `PublicGame.Decks` using each card's playable
  front-face land type, matching the existing legal land-play classification.
  Reject malformed public definitions through the existing invariant boundary.
  Pass this immutable set into every attempt's `proposalState`.

- [x] **Step 6: Implement attempt-local isolation bounds**

  At each guided shuffle, translate each compiled land-isolation fact into
  upper deadlines for every differently named public-deck land. Subtract the
  current hypothetical `handCounts` from the prior-exit allowance. Return
  `eligible == false` when the epoch has no supported land facts; return an
  empty exact set, not an invariant, when any computed maximum is negative.

- [x] **Step 7: Implement the two-component proposal**

  Build and count `C`. When isolation is eligible, build and count `L`. If
  `|L| > 0`, draw one proposal-stream bit to choose the base or isolation
  component, sample uniformly from that component, test membership in `L`, and
  replace the component's standalone weight with
  `mixtureLogTargetOverProposal`. If `|L| == 0`, sample `C` exactly as before.

  Increment aggregate counters only after eligibility/count/selection is known.
  Preserve the existing incompatible-attempt path when `|C| == 0`.

- [x] **Step 8: Run proposal, weight, replay, and noninterference tests**

  Run:

  ```sh
  GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./internal/searchprobe -count=1
  GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./rules \
    -run 'TestHypothetical|TestShuffle|TestHeads|TestClone' -count=1
  ```

  Expected: PASS. Confirm no ordinary rules path sees or imports the proposal.

---

### Task 4: Full verification and fixed calibration

**Files:**
- Modify: `docs/superpowers/reports/2026-09-17-search-probe-running.md`
- Modify: `docs/superpowers/plans/2026-09-17-search-land-isolation-proposal.md`

**Interfaces:**
- Consume: final implementation and all deterministic counters from Tasks 1-3
- Produce: fresh 500-root and 125-root JSON artifacts at new exclusive paths

- [x] **Step 1: Run full verification**

  Run:

  ```sh
  GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./... -count=1
  GOMAXPROCS=5 GOMEMLIMIT=5GiB go vet ./...
  git diff --check
  git status --short
  ```

  Expected: both Go commands and `git diff --check` exit 0; status contains only
  intended Go tests/code and the spec/plan/report edits, with no Forge or token
  scripts.

- [x] **Step 2: Run the fixed five-worker calibration to a new path**

  Run:

  ```sh
  GOMAXPROCS=5 GOMEMLIMIT=5GiB go run ./cmd/searchprobe \
    -games 500 -workers 5 -seed 10000 -sample-seed 54321 \
    -attempts 64 -worlds 4 -max-submits 5000 \
    -out /tmp/gorge-searchprobe-land-isolation-opt-500-20260917.json
  ```

  Record every accounting category, actor split, later-epoch split, land-mixture
  counters, rejection histogram, elapsed time, and allocations. Keep all 500
  roots and report coverage/correctness only.

- [x] **Step 3: Run and compare the deterministic serial prefix**

  Run:

  ```sh
  GOMAXPROCS=5 GOMEMLIMIT=5GiB go run ./cmd/searchprobe \
    -games 125 -workers 1 -seed 10000 -sample-seed 54321 \
    -attempts 64 -worlds 4 -max-submits 5000 \
    -out /tmp/gorge-searchprobe-land-isolation-opt-125-20260917.json

  jq -e --slurp \
    '(.[0].Results[:125] | map(del(.SampleNS, .SearchNS))) ==
     (.[1].Results | map(del(.SampleNS, .SearchNS)))' \
    /tmp/gorge-searchprobe-land-isolation-opt-500-20260917.json \
    /tmp/gorge-searchprobe-land-isolation-opt-125-20260917.json
  ```

  Expected: both commands exit 0 and `jq` prints `true`.

- [x] **Step 4: Compare against the fixed checkpoint**

  Compare accepted proposals with 2,473/32,000; covered roots with 138/500;
  actor coverage with 31/250 and 107/250; later-epoch coverage with 41/258;
  and `hand_to_battlefield` rejections with 11,048. Also report whether
  `hand_to_stack`, total prefix rejection, runtime, or allocations regress.
  Do not claim stronger actions or promotion readiness.

- [x] **Step 5: Update documentation and perform the final diff audit**

  Add the new commands, artifact paths, exact mixture definition, counters, and
  measured comparison to the running report. Mark this plan's boxes accurately.
  Re-run:

  ```sh
  git diff --check
  git status --short
  git diff --stat
  ```

  Leave all changes uncommitted and unpushed pending explicit authorization.
