# Compiled face metadata running report

**Date:** 2026-09-17  
**Branch:** `perf/hotspot-optimization-2026-09-17`  
**Starting HEAD:** `29fd5f9d63e5b43b6b0b90384edb3259535bab69`  
**Go:** `go1.25.11 linux/amd64`  
**Corpus cache:** `.cards/ir.gob.gz`, 8.6 MB

This report records the task-by-task measurements for
`docs/superpowers/plans/2026-09-17-compiled-face-metadata.md`. The original
immutable end-to-end artifact is
`/tmp/gorge-searchprobe-engine-final-500-20260917.json`; Task 8 explains why
the required pre-catalog replay fix moved the semantic comparison point to a
fresh control built at `1b703c0`.

## Task 1: pre-catalog baseline

Commands:

```sh
go test ./cards -run '^$' -bench 'Benchmark(LoadRegistry|FaceTypeQueries|FaceKeywordQueries|FaceAbilityQueries)$' -benchmem -count=5
go test ./rules -run '^$' -bench 'BenchmarkFaceTriggerScanDistinctFaces$' -benchmem -count=5
go test ./rules -run '^$' -bench 'BenchmarkNewEngineObjectArena$' -benchmem -count=5
```

Five-run medians on an Intel Xeon Platinum 8358 (`linux/amd64`):

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkLoadRegistry` | 778,270,900 | 279,452,784 | 3,016,325 |
| `BenchmarkFaceTypeQueries` | 35.43 | 0 | 0 |
| `BenchmarkFaceKeywordQueries` | 73.69 | 0 | 0 |
| `BenchmarkFaceAbilityQueries` | 92.01 | 24 | 2 |
| `BenchmarkFaceTriggerScanDistinctFaces` | 10,300 | 0 | 0 |
| `BenchmarkNewEngineObjectArena` | 554,800 | 192,604 | 135 |

The one-process load probe clears the timed registry, collects a baseline,
loads one registry, performs two garbage collections, reads `runtime.MemStats`,
and calls `runtime.KeepAlive` on the registry. Its five-run median was
92,897,784 retained bytes and 1,277,695 retained objects; total process
`HeapAlloc`/`HeapObjects` at that point were 93,279,488 bytes / 1,278,671
objects.

The trigger benchmark initially exposed a stale fixture: it constructed an
`Engine` without an event log, while the current granted-static-trigger scan
uses `active()` and therefore the log epoch. Real engines and the analogous
observer fixture have a log. Adding `events.NewLog(1)` to the benchmark made
the real current path measurable; the 10.30 microsecond result supersedes the
older 3.2--4.5 microsecond note from before that scan was added.

### Baseline blocker

`go test ./cards ./rules` passes `cards` but fails many `rules` replay checks
on the untouched production tree. A focused reproduction is
`TestManaCostAbilityGoesOnTheStackAndResolves`: live state ends with
`ActivatedThisTurn == 0`, while replay ends with `1`.

The root cause is `events.Apply`'s `AbilityPush` case retaining `src :=
g.Obj(e.Obj)`, then calling `g.AddObject`, then incrementing through the old
`src` pointer. `New` deliberately allocates the initial object arena at exact
deck capacity, so the append reallocates and the live increment writes through
a stale slice pointer. The log-only fixture grows its arena geometrically and
often has spare capacity, so replay mutates the current object and diverges.
This predates the catalog work (introduced by `ef0a843`) and is not one of the
documented accepted baseline failures.

## Task 4: registry and gob lifecycle

`CompileDir`, `LoadRegistry`, and therefore both `OpenCorpus` paths now publish
a catalog only after derivation, relinking, keyword/intrinsic expansion, and
token compilation. The gob shape and cache version remain unchanged; runtime
bindings and rows are rebuilt after decode. `Registry.Add` clears the published
catalog and every old face/ability binding before mutating a finalized
registry.

The full pinned corpus produced 35,385 faces, 54,671 linked ability rows,
17,664 triggers, 7,164 statics, 2,589 replacements, and 100,093 deduplicated
strings. Its canonical byte encoding is 20,908,489 bytes.

Post-catalog five-run medians:

| Measurement | Baseline | Compiled catalog | Change |
|---|---:|---:|---:|
| `BenchmarkLoadRegistry` | 778.271 ms | 1,249.571 ms | +60.6% |
| load allocation | 279,452,784 B/op | 499,555,616 B/op | +78.8% |
| load allocations | 3,016,325/op | 3,209,919/op | +6.4% |
| retained registry bytes | 92,897,784 | 125,159,208 | +34.7% |
| retained registry objects | 1,277,695 | 1,278,114 | +0.03% |

`BenchmarkCompileMetadata` isolates the new build at 379.652 ms,
217,338,888 B/op, and 193,600 allocs/op (five-run medians). The approximately
32.3 MB retained increase is explained by the flat rows, adjacency tables,
deduplicated string blob/index, and CPU reverse-pointer indexes. The 20.9 MB
canonical serialization is generated for hashing/export and is not retained;
an initial implementation retained that duplicate and measured about 148 MB
total live heap, so it was removed before accepting the lifecycle task.

## Task 5: compiled face queries

Bound faces now answer recognized type and keyword queries from their catalog
masks. `KeywordParam` uses the mask only to reject an absent recognized head,
then preserves textual parameter extraction. Spell and mana ability queries
use the catalog's one-based IDs and return the original `*SA` pointers in face
order. Unbound faces and unknown syntax retain the textual paths.

The benchmark fixtures are now finalized through a real registry catalog. The
keyword fixture changed from uncompiled `Ward`/`Haste` to the engine-consumed,
parameterized `Kicker:2` and absent `Madness`; its Task 1 number is therefore
context rather than a strict like-for-like comparison.

Five-run medians:

| Benchmark | Task 1 textual | Compiled | Allocations |
|---|---:|---:|---:|
| `BenchmarkFaceTypeQueries` | 35.43 ns/op | 10.82 ns/op | 0 B/op, 0 allocs/op |
| `BenchmarkFaceKeywordQueries` | 73.69 ns/op | 14.73 ns/op | 0 B/op, 0 allocs/op |
| `BenchmarkFaceAbilityQueries` | 92.01 ns/op | 57.46 ns/op | 16 B/op, 1 alloc/op |

The type and ability fixtures retain their Task 1 query shapes. Ability queries
previously allocated 24 B in 2 allocations; the remaining allocation is the
returned mana-ability slice. Corpus parity covers every bound face, including
tokens, and synthetic fixtures cover mixed-case recognized queries, unknown
type/keyword fallback, empty ability sets, and multiple mana abilities with
pointer/order equality.

## Task 6: compiled trigger interests

Bound faces now bypass the engine's pointer-keyed textual trigger-mask cache.
Rules maps every current `events.Kind` explicitly to the catalog's semantic
interest classes; a future event kind takes the conservative path until it is
audited. Unknown trigger modes and Phase-bearing diagnostics remain catch-all.
The object-local two-face cache retains its face-pointer validation for Rooms,
transforms, and synthetic replacement while storing compiled interests for
bound faces and textual masks for unbound fixtures.

The semantic interest mask is an over-approximation, not the final trigger
matcher. In particular, attacker and blocker declarations intentionally share
one interest bit; tests require that the compiled prefilter never rejects a
candidate admitted by the textual mask, and the existing matcher performs the
exact mode check afterward.

`BenchmarkFaceTriggerScanDistinctFaces`, now backed by 240 catalog-bound
faces, measured a five-run median of 6.983 us/op with 0 B/op and 0 allocs/op,
down from the 10.300 us/op textual baseline (32.2%).

## Task 7: dense effect API dispatch

The effect registry now publishes one immutable snapshot containing both the
existing name map and a dense `APICode` slice. Registration, replacement, and
unregistration clone and update both views under the existing writer mutex,
then atomically publish them together. Resolution uses a bound ability's
nonzero opcode first and falls back to its textual API name for unbound,
unknown, or extension APIs. `Supported` continues to enumerate the name map.

Five-run medians, using a nonallocating effect and host:

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkResolveKnownCompiledAPI` | 87.30 | 0 | 0 |
| `BenchmarkResolveKnownTextAPI` | 89.38 | 0 | 0 |
| `BenchmarkResolveUnknownAPI` | 89.67 | 0 | 0 |

The compiled microbenchmark is only 2.3% below the matched textual path, so
the final fixed workload remains the acceptance gate for retaining it.

## Task 8: end-to-end verification and checkpoint

The implementation HEAD before this report-only checkpoint was `dea13a5`
(`perf: dispatch compiled effect APIs by opcode`). Focused verification passed:

```sh
go test ./cards ./effects ./rules ./state ./internal/searchprobe ./cmd/searchprobe -count=1
go vet ./...
git diff --check
```

The full suite was also run with the plan's resource limits. Its failures were
reproduced at the `1b703c0` control and are not introduced by compiled
metadata: botbench deck ordering and its 9/11 versus 16/4 golden, gorged deck
listing, five host overshoot-tail fixtures, the stall-guard timeout, and the
architecture resume-writer allowlist. No golden was regenerated and no race
test was run.

### Semantic baseline correction

Task 1 found and fixed a pre-existing `AbilityPush` stale-pointer replay bug.
That correction necessarily changes deterministic search results, so the
original `29fd5f9` artifact cannot be the exact semantic oracle for the later
metadata commits:

| Result | Original engine artifact | Post-`AbilityPush` control |
|---|---:|---:|
| Eligible roots | 499 | 498 |
| No-root games | 1 | 2 |
| Covered roots | 114 | 100 |
| Errors | 0 | 0 |

The valid control was therefore built from `1b703c0` in the preserved checkout
`/tmp/gorge-post-ability-fix-baseline-20260918`. After deleting only top-level
`LoadSeconds`, `TotalSeconds`, `AllocatedBytes`, `HeapAllocBytes`,
`HeapSysBytes` and each result's `SampleNS`/`SearchNS`, its JSON and the
compiled branch JSON compare exactly equal. Both contain 500 games, 498
eligible roots, two no-root games, 100 covered roots, and zero errors.

Artifacts:

```text
/tmp/gorge-searchprobe-post-ability-fix-500-20260918.json
/tmp/gorge-searchprobe-post-ability-fix-cpu-500-20260918.pprof
/tmp/gorge-searchprobe-post-ability-fix-heap-500-20260918.pprof
/tmp/gorge-searchprobe-post-ability-fix-bin-20260918
/tmp/gorge-compiled-face-500-20260918.json
/tmp/gorge-compiled-face-cpu-500-20260918.pprof
/tmp/gorge-compiled-face-heap-500-20260918.pprof
/tmp/gorge-compiled-face-bin-20260918
```

### Fixed-workload result

| Measurement | Post-fix textual control | Compiled metadata | Change |
|---|---:|---:|---:|
| Load time | 0.8265 s | 1.0630 s | +28.6% |
| Wall time | 84.0761 s | 83.5442 s | -0.6% |
| Profiled CPU | 410.99 CPU-s | 412.43 CPU-s | +0.35% |
| Runtime allocated bytes | 50,490,551,912 | 50,553,352,192 | +0.12% |
| Sampled `alloc_space` | 48,517.47 MB | 48,535.32 MB | +0.04% |

The full workload is effectively neutral. The clearest profile movement is
`strings.EqualFold`, down from 6.14 CPU-s flat in the post-fix control to 2.71
CPU-s in the compiled run; total CPU sampling variance and garbage-collector
movement absorb that local improvement. The retained catalog also raises the
end-of-run heap, as expected from Task 4, so this milestone is retained for its
deterministic flat representation and measured focused-path gains rather than
claimed as an end-to-end speedup.

## Outcome and next boundary

The milestone preserves textual IR and replay-sensitive ordinals while adding
one-based face/ability identities, deterministic flat tables, compiled face
queries, conservative trigger interests, and dense known-API dispatch. No
consumer added steady-state allocations.

Further migration should begin with a separately designed predicate/cost
compiler. It must keep a three-way `yes`/`no`/`maybe` result, route `maybe` to
the textual oracle, benchmark filters and cost/mana feasibility before changing
them, and re-run the fixed semantic workload against the post-`AbilityPush`
control. CUDA, cgo, plugins, and external workers remain out of scope for the
pure-Go core.
