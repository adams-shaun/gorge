# Compiled predicate and cost running report

**Date:** 2026-09-18  
**Branch:** `perf/hotspot-optimization-2026-09-17`  
**Baseline HEAD:** `89d4a5f`  
**Go:** `go1.25.11 linux/amd64`

## Baseline

The post-`AbilityPush` 500-game compiled-face profile is the semantic
baseline. It attributes 21.02 CPU-seconds (5.10%) to
`effects.MatchesSpecCtx`/`MatchesObjectCtx`, and 18.22 CPU-seconds (4.42%) to
`rules.ParseCost`, including 7.83 CPU-seconds in regexp matching.

Task 1 added focused benchmarks. Five-run medians on an Intel Xeon Platinum
8358 (`linux/amd64`) are:

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkFilterTextualMatch` | 197.5 | 0 | 0 |
| `BenchmarkFilterTextualReject` | 200.9 | 0 | 0 |
| `BenchmarkFilterTextualUnknown` | 267.5 | 0 | 0 |
| `BenchmarkParseCostMana` | 283.0 | 0 | 0 |
| `BenchmarkParseCostHybridNonMana` | 1,612 | 176 | 5 |
| `BenchmarkParseCostGraveyardLife` | 2,365 | 192 | 4 |

The filter work must remove repeated grammar scanning without regressing its
zero-allocation property. The parsed-cost cache has the clearest focused
allocation opportunity.

## Known baseline failures

`go test ./...` reproduces the prior checkpoint's unrelated failures:

- `cmd/botbench`: `TestFullPairsIteratesSorted` and
  `TestConstructedDefaultIsByteIdentical`.
- `cmd/gorged`: `TestDeckDirectoryListingIsSortedAndComplete`.

They are outside this rules/effects work and no goldens or host behavior were
changed.
