# Engine profile optimization checkpoint and next-task resume

## Completed checkpoint

The ten engine-only iterations described below are complete. Nine were
retained; the fixed-capacity tokenizer in iteration 4 was rejected and reverted
after the midpoint profile showed a real allocation-space regression. The
final 500-game result is exactly equal to the immutable baseline in every
non-timing field under the prescribed comparison (`true`): 500 games, 499
eligible roots, one no-root game, 114 covered roots, and zero errors.

| Measurement | Baseline | Retained midpoint | Final | Baseline to final |
|---|---:|---:|---:|---:|
| Wall time | 86.275419953 s | 73.382243556 s | 69.463003930 s | -19.5% |
| Profiled CPU | 405.30 CPU-s | 356.68 CPU-s | 338.66 CPU-s | -16.4% |
| Runtime allocated bytes | 58,309,831,952 | 53,115,788,624 | 49,603,764,312 | -14.9% |
| Sampled `alloc_space` | 54.65 GiB | 49.85 GiB | 46.36 GiB | -15.2% |
| Sampled allocated objects | 502,265,036 | 409,718,709 | 317,072,050 | -36.9% |
| End `HeapAlloc` / `HeapSys` | 18,336,000 / 255,328,256 B | 19,998,624 / 238,518,272 B | 17,349,376 / 242,712,576 B | -5.4% / -4.9% |

Final artifacts:

```text
/tmp/gorge-searchprobe-engine-final-500-20260917.json
/tmp/gorge-searchprobe-engine-final-bin-20260917
/tmp/gorge-searchprobe-engine-final-cpu-500-20260917.pprof
/tmp/gorge-searchprobe-engine-final-heap-500-20260917.pprof
```

The full experiment table, remaining profile leaders, verification evidence,
and artifact interpretation are in
`docs/superpowers/reports/2026-09-17-search-probe-running.md` under
**Engine-only profile optimization pass**. The historical prompt below is kept
as the protocol record; do not execute it again.

## Next task: compile `cards.Face` metadata

The next implementation task is an architectural pass over immutable
`cards.Face` metadata. Write and approve a design before changing the card IR
or introducing a registry. The design should answer these questions:

1. Which closed string domains become stable integer-backed types: face IDs,
   card/supertypes, common keyword heads, `SA.Kind`, implemented API names,
   trigger modes, replacement events, zones, colours, and boolean flags?
2. Which open-ended values must remain deterministic strings or explicit
   unknown/fallback records: names, descriptions, SVar names/bodies, novel
   Forge parameters, dynamic expressions, and unsupported syntax?
3. Which immutable values should compile further into masks or bytecode:
   WUBRG, known types/keywords, cost tokens, selectors/filters, conditions,
   and event eligibility?
4. How are flat tables laid out with integer IDs and offset/count spans so the
   CPU avoids map hashing and pointer chasing while source diagnostics can
   still recover the original text?
5. How are schema version, deterministic assignment order, gob-cache
   invalidation, unknown sentinels, and corpus identity represented so an
   index can never silently acquire a different meaning across builds?

Do not replace strings everywhere. Closed-domain opcodes and indexes are useful
only when unknown syntax remains distinguishable and the textual IR stays
available at compile/report boundaries. No runtime index should be written to
an event, replay, or external protocol unless its schema/corpus identity makes
it stable there.

The representation should deliberately support a future CUDA accelerator:
use pointer-free flat arrays or structure-of-arrays buffers for immutable face
rows, opcode streams, masks, and offset/count adjacency. Start with batchable,
pure queries such as object/filter matching, trigger preselection, and
cost/mana feasibility. Keep authoritative event application, deterministic
ordering, resolution continuations, decisions, and RNG on the CPU, and validate
accelerated candidate masks against the CPU oracle. The rules/card pipeline
remain pure Go with no third-party dependencies; CUDA integration therefore
belongs behind an optional external adapter/process or future plugin-tier
boundary consuming a versioned plain-data snapshot, never as a dependency of
the core.

The first measurements should pin the current final profile above and add
focused corpus-size, load-time, resident-size, `Object.Face`, `hasType`,
keyword, API dispatch, trigger-scan, filter, and cost benchmarks. Migration
should be incremental and dual-path-tested, with exact replay/search JSON as
the semantic gate.

## Historical engine-pass prompt

Paste the following into a fresh session with `/tmp/gorge` as the workspace.

---

Continue the gorge performance work with **ten new measured optimization
iterations focused on the engine itself**. The previous ten-iteration pass is
complete and is the immutable baseline for this pass. Do not reopen search
proposal design, change sampling behavior, or change explicit mana tapping.

## Establish the checkpoint

Read, in order:

1. `AGENTS.md`.
2. `docs/superpowers/reports/2026-09-17-search-probe-running.md`, especially
   **Ten-iteration CPU/allocation optimization**.
3. `docs/superpowers/specs/2026-09-17-search-probe-profile-optimization-design.md`.
4. `docs/superpowers/plans/2026-09-17-search-probe-profile-optimization.md`.
5. This resume prompt.

Inspect branch, HEAD, upstream, worktree status, and the complete diff before
editing. Expected branch: `perf/hotspot-optimization-2026-09-17`. The worktree
is intentionally dirty with the completed sampler/profile work. Preserve every
existing change. Do not commit, push, merge, rebase, force-push, create/edit a
PR, regenerate goldens, or run race tests without fresh authorization.

The final baseline artifacts are:

```text
/tmp/gorge-searchprobe-profile-final-500-20260917.json
/tmp/gorge-searchprobe-profile-final-bin-20260917
/tmp/gorge-searchprobe-profile-final-cpu-500-20260917.pprof
/tmp/gorge-searchprobe-profile-final-heap-500-20260917.pprof
```

If any artifact is absent, reproduce it from the untouched current tree before
starting iteration 1, using the exact command below and new
`engine-pass-baseline` names. Its non-timing JSON must match the surviving
final JSON (or, if that too is absent, the exact aggregate/result invariants in
this prompt and the running report). Do not silently substitute an older
land-isolation, post-land-revert, or midpoint artifact; if the current baseline
cannot be reproduced, report the blocker before optimizing.

The exact baseline result is:

| Measurement | Current engine baseline |
|---|---:|
| Games / eligible roots / no-root games | 500 / 499 / 1 |
| Covered roots / errors | 114 / 0 |
| Wall time | 86.275419953 s |
| Profiled CPU | 405.30 CPU-s |
| Runtime allocated bytes | 58,309,831,952 |
| Sampled `alloc_space` | 54.65 GiB |
| Sampled allocated objects | 502,265,036 |
| End `HeapAlloc` / `HeapSys` | 18,336,000 / 255,328,256 B |

The current result already matches its pre-optimization baseline in every
non-timing JSON field after deleting only top-level `LoadSeconds`,
`TotalSeconds`, `AllocatedBytes`, `HeapAllocBytes`, `HeapSysBytes` and each
result's `SampleNS`/`SearchNS`. Preserve that exact equality throughout.

## Scope: engine production code only

Production optimizations may touch only:

```text
rules/
effects/
events/
state/
```

Tests may exercise other packages, but do not modify production code in
`internal/searchprobe`, `cmd/searchprobe`, `botpolicy`, `view`, `cards`, the
host/server, or the card pipeline during these ten iterations. The existing
searchprobe profiling flags are the measurement harness, not an optimization
target.

The objective is not ten predetermined refactors. It is ten profile-led
experiments, alternating CPU and allocation pressure:

```text
1 CPU, 2 allocation, 3 CPU, 4 allocation, 5 CPU,
6 allocation, 7 CPU, 8 allocation, 9 CPU, 10 allocation.
```

Rejected experiments count toward ten only after they have a concrete
hypothesis, a focused benchmark or profile, and a recorded before/after result.
Revert rejected production code and its implementation-specific tests before
continuing. Retain a change only when it improves its stated target, preserves
semantics, and does not materially regress the other axis or live heap.

## Current engine long poles

Use the profiles, not this list alone, when selecting each iteration. The list
is the starting evidence and exposes the common threads seen in the flame
graph.

### CPU

Final flat/cumulative leaders:

| Node | Flat | Cumulative |
|---|---:|---:|
| `runtime.duffcopy` | 23.67 s | 23.67 s |
| `rules.(*Engine).checkFaceTriggers.func1` | 22.24 s | 73.08 s |
| `state.(*Object).Face` | 13.45 s | 13.48 s |
| `aeshashbody` | 8.94 s | 8.94 s |
| `rules.(*Engine).faceMayTrigger` | 7.57 s | 14.19 s |
| small-string map lookup | 7.48 s | 14.25 s |
| `state.(*Game).Obj` | 6.47 s | 6.49 s |
| `rules.(*livelockWatcher).detect` | 6.12 s | 6.14 s |
| `rules.(*Engine).forEachObject` | 4.80 s | 86.86 s |
| `rules.(*Engine).derivedWith` | 4.70 s | 56.05 s |

Broad cumulative paths overlap; do not add them together:

```text
Engine.Submit        271.61 CPU-s
Engine.Advance       176.32 CPU-s
Engine.emit          119.57 CPU-s
Engine.legalActions  114.64 CPU-s
Engine.forEachObject  86.86 CPU-s
Engine.checkTriggers  79.47 CPU-s
Engine.checkFaceTriggers 79.04 CPU-s
```

The flame graph shows repeated object traversal, face lookup, dynamic derived
state, cost/action construction, trigger eligibility, map hashing, and event
emission feeding one another. Optimize the shared cause when evidence supports
it; do not micro-optimize a leaf that merely reflects an upstream repeated
walk.

### Allocation churn

Final flat `alloc_space` leaders inside the allowed engine scope:

| Node | Flat allocation |
|---|---:|
| `rules.(*livelockWatcher).observe` | 4.97 GiB |
| `state.(*Game).AddObject` | 4.75 GiB |
| `events.growEvents` | 4.59 GiB |
| `rules.(*Engine).legalActions.func1` | 2.71 GiB |
| `strings.(*Replacer).build` | 2.31 GiB |
| `events.(*Log).Reserve` | 2.13 GiB |
| `rules.splitCostTokens` | 1.24 GiB |
| `rules.(*Engine).paymentConv` | 0.83 GiB |
| `rules.(*Engine).derivedWith` | 0.56 GiB |

The dominant engine allocation-object counts are approximately:

```text
utf8.AppendRune                 72.7 million
rules.splitCostTokens           54.6 million flat / 91.5 million cumulative
rules.(*Engine).derivedWith     37.4 million flat / 80.4 million cumulative
strings.genSplit                34.1 million
strings.Fields                  19.0 million
rules.Cost.costPips             16.3 million
rules.(*Engine).paymentConv     13.9 million
cards.SplitKeywordList          12.2 million (measure callers; cards is out of scope)
rules.(*Engine).activeStatics    8.7 million
regexp matching                 8.4 million
```

`encoding/json`, `view.cardViews`, `Collector.cards`, and
`Collector.Capture` remain large but are out of scope for this pass. Do not
move work into those packages to make engine profiles look smaller.

The sampled final live heap is only about 2 MiB and consists of runtime thread
storage, JSON type metadata, and scavenger state. This pass targets churn.
Any cache or index that materially raises retained `HeapAlloc`, retains game
objects across engines, or grows without an explicit lifetime loses even if it
reduces CPU.

## Candidate hypothesis ladder

This is a prioritized investigation queue, not permission to implement all of
it blindly. Re-profile after retained changes and always choose the largest
remaining engine-owned cause.

1. **Trigger/object traversal:** determine why each event repeatedly reaches
   every object/face. Prefer immutable event-eligibility metadata or an
   engine-local deterministic index over another string prefilter. Preserve
   unlocked Rooms, granted Ward/Dethrone, LKI observers, trigger ordering, and
   zone semantics.
2. **Livelock history:** price the retained snapshot/copy work in `observe` and
   the repeated comparison work in `detect`. Explore bounded ring storage,
   compact fingerprints, or reuse only if collision handling and exact stall
   behavior remain unchanged.
3. **Derived/legal-action work:** identify repeated `derivedWith`, active
   static, cost-modifier, and candidate walks within one unchanged engine
   epoch. Consider short-lived epoch caches only with complete invalidation
   and clone independence.
4. **Cost parsing and formatting:** find the callers responsible for
   `splitCostTokens`, `strings.Fields`, `utf8.AppendRune`, replacer creation,
   `costPips`, and regex work. Prefer immutable compiled cost metadata or
   stack-local reusable buffers. Never cache a result that depends on mutable
   game, source, controller, targets, X, alternate costs, or statics.
5. **Object lookup/creation:** measure `Game.Obj`, `Object.Face`, map hashing,
   and `AddObject` together. A denser ID lookup or capacity plan must preserve
   object identity, stable deterministic traversal, clone isolation, token
   creation, LKI, and replay.
6. **Event-log allocation:** revisit `growEvents` and `Reserve` as one system.
   The previous exact prefix reserve shifted about 2.00 GiB out of growth into
   2.13 GiB of reserve allocation while coinciding with a large copy reduction;
   do not call that a heap win. Optimize total bytes and copy CPU together,
   including clone cap isolation and append-only history ownership.
7. **Midpoint-selected engine nodes:** iterations 6-10 must be selected from a
   fresh midpoint profile. Do not continue this ladder after evidence moves.

Architectural changes such as replacing the object store, changing event-log
ownership, or introducing global compiled registries require a written design
and explicit user approval before implementation. An iteration may investigate
one as a rejected/prototype result, but must not leave it in the worktree
without that approval.

## Hard correctness boundaries

- All authoritative `state.Game` mutation remains in `events.Apply`.
- `events.Kind` is append-only. Do not reorder kinds or change ordinals.
- Event bytes, order, hash heads, replay results, RNG draws, decisions, and
  non-timing searchprobe output remain exact.
- No wall clock, ambient randomness, or map iteration may influence an event,
  decision, object order, trigger order, or serialized result.
- Preserve clone independence. Mutable caches, scratch, indexes, maps, slices,
  hashers, and watchers must never alias between engines unless the shared data
  is demonstrably immutable.
- Preserve hypothetical-engine independence and replay from Config + chance
  transcript + intents.
- Preserve manual mana activation. Do not add tap-to-pay, cast hints, or future
  mana planning.
- Do not weaken trigger checks, SBA passes, legality, targeting, cost payment,
  livelock detection, observation validation, or replay to gain speed.
- No cgo or third-party dependencies. Never track Forge scripts or tokens.
- Do not regenerate heads/goldens to accept changed behavior.
- Keep caches explicitly bounded by an engine epoch, immutable card/face
  lifetime, or another proved lifetime. No process-global mutable cache.

## Iteration protocol

For each of the ten iterations:

1. Record the iteration number and whether it is CPU- or allocation-led.
2. Select one concrete engine-owned node from the latest profile.
3. Trace its callers and identify the repeated work; distinguish a cause from a
   wrapper or runtime symptom.
4. Write a focused benchmark that executes the real path and reports time,
   bytes/op, and allocations/op. Include a semantic digest or independent
   assertions so a faster no-op cannot pass.
5. Use TDD for every behavior/refactor boundary: write the focused regression
   or ownership/invalidation test first and observe the expected failure.
6. Implement the smallest change that tests the hypothesis.
7. Run the focused benchmark at least five times before and after. Use
   `benchstat` only if already installed; do not add it as a repository
   dependency.
8. Run focused semantic, replay, clone, trigger, and event-log tests appropriate
   to the touched code.
9. Retain or revert based on the stated target and cross-axis result. Record
   rejected experiments too.
10. Update a running iteration table with evidence before starting the next
    iteration.

Do not use fewer engine steps, fewer proposals, fewer frames, weaker checks, or
a changed corpus/workload as an optimization.

After iteration 5, build a new binary and run the exact 500-game midpoint
profile. Compare every non-timing JSON field to the baseline before selecting
iteration 6. After iteration 10, repeat for the final profile.

## Exact midpoint/final protocol

Use fresh exclusive artifact names. The command is:

```sh
GOMAXPROCS=5 GOMEMLIMIT=5GiB <instrumented-searchprobe-binary> \
  -games 500 -workers 5 -seed 10000 -sample-seed 54321 \
  -attempts 64 -worlds 4 -max-submits 5000 \
  -cpuprofile <CPU.pprof> -memprofile <HEAP.pprof> -out <REPORT.json>
```

Compare against
`/tmp/gorge-searchprobe-profile-final-500-20260917.json` with:

```sh
jq -e --slurp \
  '(.[0] | del(.LoadSeconds,.TotalSeconds,.AllocatedBytes,.HeapAllocBytes,.HeapSysBytes) |
    .Results |= map(del(.SampleNS,.SearchNS))) ==
   (.[1] | del(.LoadSeconds,.TotalSeconds,.AllocatedBytes,.HeapAllocBytes,.HeapSysBytes) |
    .Results |= map(del(.SampleNS,.SearchNS)))' \
  /tmp/gorge-searchprobe-profile-final-500-20260917.json <NEW.json>
```

Anything other than `true` is a correctness failure. Inspect the first differing
field and revert or fix it; never expand the deletion set.

For midpoint and final profiles extract:

- CPU flat and cumulative top nodes;
- CPU subtraction against the baseline and prior checkpoint;
- heap `alloc_space` flat and cumulative;
- heap `alloc_objects`;
- heap `inuse_space` and runtime `HeapAlloc`/`HeapSys`;
- total wall time, CPU samples, allocated bytes, covered roots, eligible roots,
  no-root games, and errors;
- an interactive pprof CPU flame graph (`go tool pprof -http=... -no_browser`)
  plus a PNG call graph if requested. Do not label the call graph itself a
  flame graph.

## Verification

After each iteration run the narrow affected tests. At midpoint and final run:

```sh
GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./internal/searchprobe ./cmd/searchprobe ./rules ./effects ./events ./state -count=1
GOMAXPROCS=5 GOMEMLIMIT=5GiB go vet ./...
git diff --check
```

Also run `GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./... -count=1` at the final
checkpoint. Two unrelated failures were previously reproducible on clean
`origin/main`: `cmd/botbench.TestConstructedDefaultIsByteIdentical` measured
18/2 against its 16/4 golden, and `host.TestStallGuardSetToZeroDoesNotHalt`
missed its 200-decision/30-second target. Do not update those tests or goldens.
Report whether those exact failures remain and treat any new failure as caused
by this work until disproved.

## Final report

Update:

```text
docs/superpowers/reports/2026-09-17-search-probe-running.md
docs/superpowers/reports/2026-09-17-engine-profile-optimization-resume.md
```

Report all ten experiments, including reverted ones, with hypothesis, files,
benchmark before/after, correctness tests, retain/revert decision, and measured
cross-axis effect. Include baseline/midpoint/final totals and the largest
remaining CPU, allocation-space, allocation-object, and live-heap nodes.

Lead with the outcome. Distinguish cumulative call-path costs from independent
costs and never add overlapping cumulative numbers. Distinguish allocation
churn from retained heap and do not infer peak RSS from end-of-run `HeapAlloc`.
Leave all work local and uncommitted unless the user separately authorizes an
integration action.

---
