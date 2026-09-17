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
