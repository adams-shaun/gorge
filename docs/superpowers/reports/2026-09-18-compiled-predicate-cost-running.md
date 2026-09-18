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
| `BenchmarkFilterTextualMatch` | 183.4 | 0 | 0 |
| `BenchmarkFilterTextualReject` | 200.9 | 0 | 0 |
| `BenchmarkFilterTextualUnknown` | 267.5 | 0 | 0 |
| `BenchmarkParseCostMana` | 283.0 | 0 | 0 |
| `BenchmarkParseCostHybridNonMana` | 1,612 | 176 | 5 |
| `BenchmarkParseCostGraveyardLife` | 2,365 | 192 | 4 |

The filter work must remove repeated grammar scanning without regressing its
zero-allocation property. The parsed-cost cache has the clearest focused
allocation opportunity.

## Task 2: conservative predicate programs

The initial immutable program evaluator recognizes only local grammar and
returns `maybe` for every unmodeled base or predicate; the public matcher then
uses the original textual evaluator. This makes an unknown base conservative
as well as an unknown predicate -- neither can be silently rejected.

Five-run compiled-path medians were:

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkFilterCompiledYes` | 131.1 | 0 | 0 |
| `BenchmarkFilterCompiledNo` | 75.24 | 0 | 0 |
| `BenchmarkFilterCompiledMaybe` | 329.3 | 0 | 0 |

The direct yes/no paths improve over the separately measured textual baseline;
the fallback pays the expected lookup overhead and is retained only for
correctness. Broader corpus reachability and end-to-end benefit remain open.

## Task 3/5: configured cost cache

The engine constructs and clones an immutable cache of configured printed,
activation, and unless costs. A hybrid/non-mana cache hit measured 79.38
ns/op (five-run median), 0 B/op, and 0 allocs/op, against the Task 1 direct
parser median of 1,612 ns/op, 176 B/op, and 5 allocs/op. The first migration
batch covers mana activation/availability, activation, speed, and ability
offer paths; raw strings a match did not precollect still call `ParseCost`.

## Task 6: reuse immutable sidecars across hypothetical engines

The first 500-game run after adding the engine-local sidecar regressed to
91.659 seconds and 59.49 GB allocated. Its CPU profile showed why:
`rules.newCompiledText` spent 45.70 cumulative CPU-seconds (12.12%) rebuilding
the same configured card interpretation for each
`NewHypotheticalPlanned` engine.

`rules.compiledTextCache` now holds immutable sidecars behind a mutex. Cache
identity is an exact check, not a hash: the ordered deck dimensions and card
pointers plus every token key/value card pointer must match. The entry owns a
snapshot to protect its identity from later caller slice/map edits; hit lookup
compares that snapshot directly with the supplied configuration and does not
copy it. The cache contains no state, event, replay, decision, or runtime
catalog IDs.

The regression test first demonstrated that equivalent configurations rebuilt
the sidecar, then now requires pointer reuse. Its companion test proves that
one different deck card or token card does not reuse a sidecar.

The exact semantic oracle comparison is `true` after deleting only timing and
memory telemetry (and per-result timing fields): 500 games, 498 eligible
roots, 2 no-root games, 100 covered roots, and zero errors. On the final
profiled run:

| Metric | post-AbilityPush control | sidecar cache | Change |
|---|---:|---:|---:|
| Wall time | 84.076 s | 83.015 s | -1.26% |
| Runtime allocated bytes | 50.491 GB | 50.103 GB | -0.77% |

The cache path no longer appears in the CPU profile's 2.05-second reporting
threshold. The remaining direct parser cost is 9.80 cumulative CPU-seconds
(2.39%); `MatchesSpecCtx`/`MatchesObjectCtx` remain 22.24/20.89 seconds
(5.43%/5.10%) and are the next candidates for a separately designed,
conservative grammar expansion.

## Task 7: carry compiled predicates through layer evaluation

The first predicate sidecar only helped call sites that already constructed an
engine `SpecContext`. The profile showed 18.87 cumulative CPU-seconds still
in `matchesObjectText`; its largest source was `Engine.derivedWith`, whose
calls to the public `effects.MatchesSpecFrom` necessarily omitted the
engine-owned sidecar.

`Engine.matchesSpecFrom` preserves the public helper's source-relative
semantics while attaching `Engine.specCtx`. The layer/continuous/static paths
now use it, including the `AsStack` and remembered-context paths that need
additional fields. A regression test covers the engine-owned helper.

The initial program representation still re-dispatched each base and term
through string grammar at evaluation time. The profile measured 2.55 CPU-s in
the program map lookup, 2.31 in `matchesBase`, and 0.93 in `matchPredicate`.
The compiler now stores the existing conservative subset as base/term opcodes
with only the literal type/colour argument retained. Unsupported grammar
remains `maybe` exactly as before. Five-run focused medians improved from
131.1 to 58.6 ns/op for definite yes and from 75.2 to 58.3 ns/op for definite
no, both zero-allocation.

A corpus fallback census identified source-relative attachment predicates as
the dominant remaining `Affected$` forms: `EnchantedBy` 926 occurrences and
`EquippedBy` 617. The compiler now uses their pre-existing shared
`attachedBy` implementation for all three aliases (`EnchantedBy`,
`EquippedBy`, `AttachedBy`); a red/green regression test requires definite
yes/no results for an attached and unattached object.

Both 500-game attachment runs compare semantically equal to the fixed
post-`AbilityPush` control. They recorded 86.174 and 81.200 seconds; the
83.687-second median is 0.46% faster than the control's 84.076 seconds. The
repeat run used 50.084 GB allocated, 0.81% below the control's 50.491 GB.
The repeat profile reduced filter work from the pre-context 19.78 combined
seconds (`matchesObjectText` plus compiled evaluator) to 12.25 seconds
(6.49 text + 5.76 compiled), a 38.1% reduction. The fixed workload remained
exactly 500 games, 498 eligible roots, 2 no-root games, 100 covered roots,
and zero errors.

## Task 8: cache loyalty and projection cost reads

The next profile attributed 5.05 CPU-seconds of direct parsing to the
receiver-owned `isLoyaltyAbility` helper: its fixed loyalty-counter check
reparsed every configured activated-ability cost. `Engine.isLoyaltyAbility`
now supplies `parseCost` to a shared classifier, while the public helper
retains direct parsing for standalone/dynamic callers. `rawBaseCost` and the
view projection `AbilityCosts` now likewise use the configured cache.

The new regression test covers a configured `AddCounter<1/LOYALTY>` cost.
Focused tests, vet, and the fixed workload pass. The final normalized
500-game result is exactly equal to the control (500 games, 498 eligible
roots, 2 no-root games, 100 covered roots, zero errors). It took 81.021
seconds, 3.63% below the 84.076-second control, and allocated 49.630 GB,
1.70% below the control's 50.491 GB. `ParseCost` is no longer present in the
CPU profile's 2-second reporting table.

## Known baseline failures

`go test ./...` reproduces the prior checkpoint's unrelated failures:

- `cmd/botbench`: `TestFullPairsIteratesSorted` and
  `TestConstructedDefaultIsByteIdentical`.
- `cmd/gorged`: `TestDeckDirectoryListingIsSortedAndComplete`.

They are outside this rules/effects work and no goldens or host behavior were
changed.
