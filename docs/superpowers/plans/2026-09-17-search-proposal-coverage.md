# History-Conditioned Proposal Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Raise search-probe sampling coverage by exactly conditioning genesis and later library shuffles on opponent public plays and actor-visible arrange windows.

**Architecture:** Compile the actor's allowed observation history into per-player shuffle epochs. An opt-in hypothetical-engine shuffle planner samples uniformly from exactly counted compatible physical permutations, records ordinary replayable chance draws, and leaves unsupported shapes to prior sampling plus full-prefix rejection.

**Tech Stack:** Go 1.25 standard library, existing gorge rules/events/searchprobe packages, `math/big` for exact completion counts.

**Spec:** `docs/superpowers/specs/2026-09-17-search-proposal-coverage-design.md`

## Global Constraints

- Never consume the actual engine, original seed, hidden zones/hash, opponent-private intent indices, or callbacks into the source game.
- All game mutation continues through ordinary events and `events.Apply`.
- No cgo or third-party dependencies; never track Forge/token scripts.
- Preserve ordinary event bytes, ordinals, heads, RNG behavior, and clone independence.
- Use fixed work counts and explicit independent seeds; clocks remain diagnostic only.
- Keep every calibration root and preserve baseline fallback accounting.
- Do not commit, push, merge, rebase, change a PR, deploy, run race tests, or regenerate goldens without new authorization.

---

### Task 1: Deterministic rejection diagnostics

**Files:**
- Modify: `internal/searchprobe/sample.go`
- Modify: `internal/searchprobe/sample_test.go`
- Modify: `internal/searchprobe/experiment_test.go`

**Interfaces:**
- Produce: `RejectionBucket{Frame int, Component, Shape string, Count int}`
- Extend: `SampleResult.Rejections []RejectionBucket`

```go
type RejectionBucket struct {
	Frame            int
	Component, Shape string
	Count            int
}
```

- [x] Add failing observed-shape fixtures and a sampler rejection-count regression. Exercise the real aggregation path with multiple identity/event/action/board keys, repeated counts, and deterministic frame/component/shape ordering; distinguish later differing identities from earlier matching ones.
- [x] Run `GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./internal/searchprobe -run 'TestSamplerRejectionHistogram' -count=1` and confirm the missing field/behavior failure.
- [x] Implement bounded aggregation at rejection sites. Classify only from `Frame` values: identity use in observed events, first differing event kind, board, decision, action, and missing decision. Do not include card names or hypothetical hidden IDs.
- [x] Sort buckets lexically by frame, component, then shape before returning. Keep `FirstRejection` for compatibility and verify histogram collection cannot influence acceptance or weights.
- [x] Run focused package tests and compare a small repeated command output after deleting timing fields.

### Task 2: Exact constrained-permutation solver

**Files:**
- Create: `internal/searchprobe/constraints.go`
- Create: `internal/searchprobe/constraints_test.go`
- Modify: `internal/searchprobe/permutation.go`

**Interfaces:**
- Produce: `positionConstraint{Index int, Name string, Obj state.ObjID}`
- Produce: `deadlineConstraint{Through int, Name string, Count int}`
- Produce: `sampleConstrainedPermutation(cards []proposalCard, positions []positionConstraint, deadlines []deadlineConstraint, r boundedRandom) (order []state.ObjID, logTargetOverProposal float64, compatible bool, err error)`

```go
type proposalCard struct {
	ID   state.ObjID
	Name string
}
type positionConstraint struct {
	Index int
	Name  string
	Obj   state.ObjID
}
type deadlineConstraint struct {
	Through int // exclusive prefix bound
	Name    string
	Count   int
}
type proposalRandom interface {
	IntN(int) int
	Uint64() uint64
}
```

- [x] Write exhaustive three- and four-card tests that enumerate every RNG path for: one fixed name, duplicate-name fixed slots, one deadline, nested deadlines, exact physical object plus name deadline, and no constraints. Assert each allowed physical permutation has equal proposal probability and returned factor equals `|C|/n!`.
- [x] Write contradiction tests for impossible names/counts, conflicting exact slots, duplicate exact objects, expired deadlines, and out-of-range positions.
- [x] Run the focused tests and confirm they fail before implementation.
- [x] Implement memoized completion counting with `math/big.Int`. State includes position, remaining counts of constrained names, remaining unconstrained physical cards, and deadline progress. Fixed physical IDs are removed exactly once and still contribute their names to deadline counts.
- [x] Implement uniform sampling by weighting each next physical-card category by its exact completion count, choosing a physical copy uniformly inside the selected category, and Fisher-Yates shuffling the unconstrained suffix.
- [x] Convert exact compatible-count and `n!` values to a stable log ratio. Retain the existing fixed-placement helper until integration tests prove parity.
- [x] Run `go test ./internal/searchprobe -run 'TestConstrainedPermutation|TestPermutation' -count=1` under the global environment.

### Task 3: Compile allowed history into library epochs

**Files:**
- Create: `internal/searchprobe/epochs.go`
- Create: `internal/searchprobe/epochs_test.go`
- Modify: `internal/searchprobe/observation.go`

**Interfaces:**
- Produce: `epochKey{Player state.PlayerID, Ordinal int}`
- Produce: `epochConstraints{Positions []positionConstraint, Deadlines []deadlineConstraint, ArrangeWindows int, Unguided []string}`
- Produce: `compileEpochs(h History) (map[epochKey]epochConstraints, error)`
- Extend collector with an ordered reverse observer-reference lookup used only inside hypothetical reconstruction.

```go
type epochKey struct {
	Player  state.PlayerID
	Ordinal int
}
type epochConstraints struct {
	Positions      []positionConstraint
	Deadlines      []deadlineConstraint
	ArrangeWindows int
	Unguided       []string
}
```

- [x] Write table tests for genesis actor draws, opponent land/cast/exile first appearances, repeated public identities after bounce, duplicate-name cumulative deadlines, and independent player epochs.
- [x] Write later-history tests for a second Shuffle resetting draw positions, actor `KArrange` options fixing an ordered top window, deterministic post-`LibraryOrder` continuation, and unsupported position tracking after an unmodelled library mutation.
- [x] Write contradictory-prefix tests for unknown identity names, impossible deadline counts, and the same known object fixed twice.
- [x] Run focused tests and confirm missing compiler behavior.
- [x] Implement a chronological frame/event walker. Track shuffle ordinal, draw count, positional reliability, first introduction of every observer ID, and actor answers. Add only constraints licensed by observable events/decisions.
- [x] Store collector references in insertion-order reverse storage rather than recovering them through map iteration. Clone that storage independently.
- [x] Mark unsupported epoch portions for diagnostics and prior fallback; do not reject the whole root merely because one epoch cannot be guided.
- [x] Run observation, action, epoch, and noninterference tests.

### Task 4: Opt-in replayable hypothetical shuffle planning

**Files:**
- Modify: `rules/chance.go`
- Modify: `rules/chance_test.go`
- Modify: `rules/rng.go`
- Modify: `rules/engine.go`
- Modify: `rules/mulligan.go`
- Modify: `rules/stack.go`
- Modify: `effects/registry.go`
- Modify: `effects/shuffle.go`
- Modify: `effects/zone.go`
- Modify: `effects/context_test.go`

**Interfaces:**
- Produce: `rules.ShuffleCard{ID state.ObjID, Name string}`
- Produce: `rules.ShuffleContext{Player state.PlayerID, Ordinal int, Library, Hand []ShuffleCard}`
- Produce: `type ShufflePlanner func(ShuffleContext) ([]state.ObjID, error)`
- Produce: `rules.NewHypotheticalPlanned(Config, []ChanceDraw, ShufflePlanner) (*Engine, error)`
- Produce: `(*Engine).ClearHypotheticalPlanner()`
- Extend: `effects.Host.ShuffleLibrary(state.PlayerID, []state.ObjID) []state.ObjID`

```go
type ShuffleCard struct {
	ID   state.ObjID
	Name string
}
type ShuffleContext struct {
	Player        state.PlayerID
	Ordinal       int
	Library, Hand []ShuffleCard
}
type ShufflePlanner func(ShuffleContext) ([]state.ObjID, error)
```

- [x] Add failing rules tests proving a planner controls genesis and a later effect-driven shuffle, receives the correct player/ordinal/current hand/library, and rejects a non-permutation.
- [x] Add tests proving every forced Fisher-Yates value advances the independent PCG, the complete transcript replays without a planner, planner state does not leak through clones, and clearing the planner restores prior continuation.
- [x] Add ordinary-game parity tests comparing event bytes, head, draw count, and clone behavior before/after routing genesis, mulligan, search, and Shuffle effects through `ShuffleLibrary`.
- [x] Run the focused tests and confirm the planned APIs are absent.
- [x] Implement one engine shuffle path. With no planner, call the existing RNG shuffle unchanged. With a planner, validate the requested order, append checked values at the current transcript position, and execute the same Fisher-Yates loop so recording and PCG advancement remain canonical.
- [x] Route all library shuffle call sites through that path. Test hosts implement the same deterministic `Rand`-based fallback; effects continue to emit the sole state-mutating Shuffle event.
- [x] Run chance, shuffle, mulligan, search, clone, head, and architecture tests.

### Task 5: Multi-epoch proposal integration

**Files:**
- Create: `internal/searchprobe/proposal.go`
- Create: `internal/searchprobe/proposal_test.go`
- Modify: `internal/searchprobe/sample.go`
- Modify: `internal/searchprobe/history_effects_test.go`
- Modify: `internal/searchprobe/weights_test.go`

**Interfaces:**
- Produce: per-attempt planner state keyed by `(player, shuffle ordinal)` with independent deterministic RNG streams and accumulated log weight.
- Extend: `SampleResult` with guided genesis/later shuffle counts, arrange-window count, unguided constraint count, and incompatible proposal count.

```go
func taggedSeed(base uint64, history [32]byte, attempt int, tag uint64, values ...uint64) [2]uint64

type proposalState struct {
	epochs    map[epochKey]epochConstraints
	logWeight float64
	// Diagnostic counters are updated only after proposal decisions.
}
```

- [x] Add a tiny real-engine opponent-play test where prior sampling usually rejects but the guide always places a required public land/spell within its legal draw deadline. Verify exact weight against exhaustive physical permutations.
- [x] Add real-engine tests for a later Shuffle followed by an actor-visible draw and for `KArrange` top-window matching followed by the recorded reorder and draw.
- [x] Add tests where a hypothetical pre-shuffle hand already satisfies an opponent public-play requirement, where only part of a duplicate-name requirement remains, and where no completion exists for that attempt.
- [x] Add repeated-run, different-worker seed allocation, source-secret noninterference, selected-world replay, and duplicate-world independence tests.
- [x] Run the focused tests and confirm they fail before integration.
- [x] Replace `genesisProposal`'s fixed actor-only tape generation with compiled epoch constraints plus `NewHypotheticalPlanned`. Keep the public toss forced and weighted exactly.
- [x] Derive engine, proposal, opponent-policy, and resampling RNGs from separate tagged seed streams. Never use worker identity or execution timing.
- [x] At each planner call, translate known observer references through the current hypothetical collector, subtract matching hypothetical hand counts for opponent deadlines, sample the compatible permutation, and add its log factor. Return nil for explicitly unguided epochs.
- [x] Clear the planner at the accepted root. Preserve full-prefix validation, attempt count, ESS gate, baseline fallback, and complete chance-transcript replay.
- [x] Run all `internal/searchprobe`, `rules`, and `cmd/searchprobe` tests.

### Task 6: Verification and fixed calibration

**Files:**
- Modify: `docs/superpowers/reports/2026-09-17-search-probe-running.md`
- Modify: `docs/superpowers/plans/2026-09-17-search-proposal-coverage.md`

- [x] Run `GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./... -count=1`.
- [x] Run `GOMAXPROCS=5 GOMEMLIMIT=5GiB go vet ./...`.
- [x] Run `git diff --check` and inspect `git status --short` for accidental corpus or unrelated files.
- [x] Run the fixed 500-game command to a new output path. Record total and per-actor coverage, later-epoch coverage, accepted/rejected attempts, ESS, duplicates, failure buckets, submissions, elapsed time, and allocations.
- [x] Run a one-worker subset with the same `GOMAXPROCS=5`; compare every non-timing per-game field against the matching five-worker prefix.
- [x] Compare results to the 164/32,000 accepted, 17/500 covered, 0/250 Death-n-taxes actor, 17/250 Dimir actor, and 6/258 later-epoch checkpoint without making strength claims.
- [x] Update the measured report with commands, artifact names, supported/unguided constraint shapes, and any remaining bottleneck. Mark this plan's completed steps truthfully.
- [x] Leave all changes local. Report verification evidence and request separate authorization before any commit or push.

### Final review fix wave

- [x] Retain typed public contradictions as a distinct sampling fallback and continue baseline/outcome replay; invariants remain fatal.
- [x] Track original shuffle positions through supported actor `KArrange` semantic answers and the following matching `LibraryOrder`, including top/bottom placement and unseen post-window draws. Unsupported/private/invalid ordering remains unguided.
- [x] Reject a repeated exact observer reference at different positions before sampling; allow distinct duplicate-name objects.
- [x] Classify only differing identity declarations and prove multi-key histogram aggregation/sorting.
- [x] Run fresh full tests, full vet, and diff checks under the required environment.
- [x] Rerun the fixed 500/5-worker calibration and 125/1-worker prefix to new artifact names; compare all per-game fields except the two timing fields.
- [x] Replace the measured report with final-code values, preserve no-strength language, and explicitly request separate authorization before any commit or push.
