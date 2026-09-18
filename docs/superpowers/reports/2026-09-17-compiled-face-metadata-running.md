# Compiled face metadata running report

**Date:** 2026-09-17  
**Branch:** `perf/hotspot-optimization-2026-09-17`  
**Starting HEAD:** `29fd5f9d63e5b43b6b0b90384edb3259535bab69`  
**Go:** `go1.25.11 linux/amd64`  
**Corpus cache:** `.cards/ir.gob.gz`, 8.6 MB

This report records the task-by-task measurements for
`docs/superpowers/plans/2026-09-17-compiled-face-metadata.md`. The immutable
end-to-end comparison remains
`/tmp/gorge-searchprobe-engine-final-500-20260917.json`.

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
