# Search-probe Profile Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete ten measured CPU/allocation optimization iterations and produce an exact final profiled search-probe comparison.

**Architecture:** Preserve the existing sampler and engine contracts while removing repeated computation and short-lived allocation from the measured hot paths. Use targeted benchmarks for each iteration, exact deterministic artifact comparison at midpoint/final, and retain only measured wins.

**Tech Stack:** Go standard library, `testing`, `runtime/pprof`, `go tool pprof`, `jq`.

**Spec:** `docs/superpowers/specs/2026-09-17-search-probe-profile-optimization-design.md`

## Global Constraints

- Ten total iterations: five CPU-led and five allocation-led.
- Preserve all non-timing search-probe output exactly.
- Preserve RNG draw order, exact proposal density, replay, and clone isolation.
- No `state.Game` mutation outside `events.Apply`.
- No Forge scripts, third-party dependencies, commits, pushes, race tests, or golden regeneration.
- Record rejected experiments; revert their code before continuing.

---

### Task 1: CPU iteration — factorial table

**Files:** Modify `internal/searchprobe/constraints.go`; test `internal/searchprobe/constraints_test.go`.

- [x] Add an allocation/benchmark case exercising a 60-card constrained deck and record the baseline.
- [x] Add a failing unit test for a counter-owned factorial table whose entries equal independently computed factorial literals for small values and do not alias mutable results.
- [x] Precompute factorials `0..n` once in `newConstraintCounter`; return immutable entries from the recursive base case.
- [x] Run constrained-permutation tests and benchmark; retain only on CPU improvement with identical orders/weights.

### Task 2: Allocation iteration — immutable memo values

**Files:** Modify `internal/searchprobe/constraints.go`; test `internal/searchprobe/constraints_test.go`.

- [x] Add a failing allocation ceiling around repeated `count` memo hits.
- [x] Make cached counts immutable after insertion and return the cached pointer without `Set` copies; ensure every arithmetic receiver is distinct from its operands.
- [x] Run the exact-distribution tests and allocation benchmark; record bytes/op and allocations/op.

### Task 3: CPU iteration — compact count keys

**Files:** Modify `internal/searchprobe/constraints.go`; test `internal/searchprobe/constraints_test.go`.

- [x] Add literal collision tests covering multi-digit positions/counts and differing `other` values.
- [x] Replace `strconv` decimal construction with an unambiguous fixed-width binary key encoding.
- [x] Run key tests, distribution tests, and the targeted benchmark; record CPU and allocation effects.

### Task 4: Allocation iteration — in-place recursive count state

**Files:** Modify `internal/searchprobe/constraints.go`; test `internal/searchprobe/constraints_test.go`.

- [x] Add a test proving `count` restores its caller-owned remaining-count vector.
- [x] Decrement/recurse/restore each relevant count in place instead of allocating a branch slice.
- [x] Apply the same discipline to unranking, copying counts only when a candidate is selected if required.
- [x] Run exact-distribution, input-mutation, and allocation tests; record the result.

### Task 5: CPU iteration — reuse compiled constrained plans

**Files:** Modify `internal/searchprobe/constraints.go`, `internal/searchprobe/proposal.go`, and their tests.

- [x] Add a test that two identical proposal inputs reuse counting state while different IDs, constraints, or hand-adjusted deadlines do not.
- [x] Split compilation from random unranking and add a per-`Sample` cache owned by proposal reconstruction, keyed only by complete public proposal inputs.
- [x] Verify distinct attempt RNG streams still choose exactly the same orders as the uncached implementation.
- [x] Run focused tests and the exact midpoint 500-game CPU/heap profile; compare all non-timing fields to iteration zero.

### Task 6: Allocation iteration — reuse opponent bot boards

**Files:** Modify `internal/searchprobe/sample.go`; test `internal/searchprobe/sample_test.go`.

- [x] Add an allocation test demonstrating repeated hypothetical opponent decisions currently allocate fresh board maps.
- [x] Allocate one `botpolicy.Board` per player per attempt and fill it through `BoardFromGameInto` before `Decide`.
- [x] Run sampling determinism tests and targeted allocation measurement.

### Task 7: CPU iteration — constant-time turn counts

**Files:** Modify `rules/engine.go`, `rules/stack.go`, `rules/clone.go`, and rules tests.

- [x] Add a test comparing `TurnsTaken` against a hand-built log across turn changes and cloning/replay.
- [x] Maintain an engine-only per-player derived counter when a `TurnChange` event is emitted; rebuild it from an existing log in constructors that accept history.
- [x] Preserve event bytes and keep authoritative game mutation in `events.Apply`.
- [x] Run trigger/count, replay, clone, and rules acceptance tests plus a targeted benchmark.

### Task 8: Allocation iteration — reserve hypothetical logs

**Files:** Modify `internal/searchprobe/sample.go`; test `internal/searchprobe/sample_test.go`.

- [x] Add a test that a reconstruction reserves enough event capacity for the complete observed prefix without changing length or head.
- [x] Sum observed frame event counts once and call `events.Log.Reserve` on each hypothetical engine before replaying the prefix.
- [x] Run sampling determinism/replay tests and record event-log allocation change.

### Task 9: CPU iteration — trigger prefilter

**Files:** Modify `rules/trigger_match.go` and focused trigger tests.

- [x] Use the midpoint CPU profile to select the largest semantics-neutral repeated prefilter inside `checkFaceTriggers`.
- [x] Add a failing equivalence/call-count test covering ordinary faces, unlocked Rooms, granted Ward/Dethrone, and LKI walks.
- [x] Cache or hoist that prefilter without changing deterministic object/trigger traversal order.
- [x] Run trigger, replay, and acceptance tests plus a targeted benchmark; revert if the profile does not improve.

### Task 10: Allocation iteration — midpoint-selected reconstruction temporary

**Files:** Modify only the file owning the largest remaining avoidable allocation and its focused test.

- [x] Select the target from midpoint `alloc_space`, excluding required owned history/output and runtime bookkeeping.
- [x] Add a failing allocation ceiling and an ownership test proving returned frames/logs do not alias reusable scratch.
- [x] Reuse or eliminate the temporary while retaining independent frame/world ownership.
- [x] Run focused tests and record allocation improvement; revert if it regresses CPU materially.

### Task 11: Final profile and report

**Files:** Update `docs/superpowers/reports/2026-09-17-search-probe-running.md` and `docs/superpowers/reports/2026-09-17-search-probe-resume.md`.

- [x] Run focused tests, full `go vet ./...`, and `git diff --check`.
- [x] Build the instrumented `searchprobe` binary.
- [x] Run the exact 500-game CPU/heap profile into new `/tmp` artifacts.
- [x] Compare every non-timing JSON field against iteration zero with `jq -e`.
- [x] Extract CPU flat/cumulative, alloc_space, alloc_objects, and inuse_space tops.
- [x] Report all ten iteration outcomes and cumulative improvements; leave all work uncommitted.
