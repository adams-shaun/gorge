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

## Task 9: reuse effect-side brace normalizers

The next allocation profile identified 2.12 GB under
`strings.(*Replacer).build`, reached from `effects.effMana` and
`effects.parseCMC`. Both paths constructed an identical immutable
`strings.Replacer` on every invocation. They now use package-level immutable
normalizers; the parsing, validation, and emitted event sequence are
unchanged.

Regression tests pin the allocation budgets of the real braced CMC and literal
mana-production paths. The tests first failed on the former implementation:
`parseCMC("{2}{U}{B}")` allocated 10 objects and literal `effMana` allocated
6. With shared normalizers the corresponding steady-state counts are 7 and 2,
respectively. Five-run focused medians are:

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkParseCMCBraced` | 420.1 | 176 | 7 |
| `BenchmarkManaLiteralProduction` | 250.9 | 8 | 2 |

The exact normalized 500-game semantic comparison against the post-
`AbilityPush` control is `true`: 500 games, 498 eligible roots, two no-root
games, and zero errors. The candidate took 78.146 seconds and allocated
46.967 GB, versus the control's 84.076 seconds and 50.491 GB: improvements of
7.05% and 6.98%. `strings.(*Replacer).build` is absent from the candidate heap
profile's reporting table. The change is retained.

## Task 10: snapshot active statics for a trigger scan

`checkGrantedStaticTriggers` previously called the cached `active()` accessor
once for every object in an event's deterministic trigger walk. Trigger
matching cannot emit events or mutate continuous effects, and the only
post-walk emission (a deferred Phase diagnostic) happens after the walk, so
`checkFaceTriggers` now takes one `observer.active()` snapshot and passes it
through every granted-trigger match for that event. Queue construction,
matching, and scan order are unchanged.

The new semantic fixture requires a supplied active-static snapshot to queue
the same linked `GrantTriggerPush` candidate as the ordinary matcher. Its
first run is a compile failure against the missing snapshot-aware path. The
240-object granted-static scan benchmark is allocation-free and has a
five-run median of 32,682 ns/op. On the profiled workload, flat `active()` CPU
falls from 9.06 seconds to 1.49 seconds, while granted-static trigger
matching falls from 39.32 to 15.96 cumulative CPU-seconds.

Both candidate 500-game results compare exactly equal to the post-
`AbilityPush` semantic control: 500 games, 498 eligible roots, two no-root
games, and zero errors. Their wall times were 78.957 and 74.719 seconds; the
76.838-second median is 1.67% below Task 9's 78.146-second baseline.
Allocated-byte median is effectively flat at 47.008 GB (+0.087%). The CPU
reduction and median wall-clock improvement retain the change.

## Task 11: avoid copying continuous effects in granted-trigger scans

The Task 10 profile then localized 2.08 flat / 11.54 cumulative CPU-seconds
to `for _, ce := range statics` inside the granted-trigger walk. Each
iteration copied the large immutable `ContinuousEffect` value before testing
whether it carried `AddTrigger`. The loop now ranges by index and takes a
pointer to the existing snapshot element. It preserves slice order and never
mutates the snapshot.

The existing granted-static semantic fixture continues to require the linked
pending trigger, and the 240-object benchmark improves from a 35,152 ns/op
five-run median to 26,953 ns/op (23.3%), both with zero allocations. The
profiled full workload reduces the loop line to 0.44 flat CPU-seconds. Its
normalized 500-game result is exactly equal to the semantic control (498
eligible roots, two no-root games, zero errors), taking 71.878 seconds and
allocating 46.984 GB. Compared with Task 10's 76.838-second / 47.008-GB
two-run median, this is 6.45% faster with 0.051% fewer allocated bytes. The
change is retained.

## Rejected experiment: read-only trigger traversal

A `forEachObjectReadOnly` walker was prototyped for `checkFaceTriggers` to
avoid the general walker's defensive zone-slice copy. It preserved the
ordinary walk's deterministic order in a new regression test and its
normalized 500-game replay was exactly equal to the semantic control (498
eligible roots, two no-root games, zero errors). It is deliberately not
retained: the full workload took 74.579 seconds versus the copy-elision
baseline's 71.878 seconds (+3.76%) with effectively identical allocated
bytes. The defensive snapshot remains in use; its locality and general safety
outweigh the isolated traversal microbenchmark.

## Task 12: read catalog trigger interests directly

Catalog-bound faces already own immutable trigger-interest bits, but the
object trigger fast path first copied those same bits into a mutable,
per-engine two-face cache. `objectFaceMayTrigger` now reads the catalog row
directly for a bound face; synthetic/unbound and dynamically replaced faces
retain the original pointer-guarded cache. A red/green regression test
requires a compiled face not to grow `triggerObjectMasks`, while the existing
transform and clone tests cover the fallback.

The compiled lookup benchmark has a five-run median of 4.906 ns/op with zero
allocations. The profiled candidate reduces `objectFaceMayTrigger` from 6.96
to 2.93 CPU-seconds and `Object.Face` from 15.64 to 11.51, while both
normalized 500-game candidates are exactly equal to the semantic control
(498 eligible roots, two no-root games, zero errors). Allocation is
consistently lower: 46.558 and 46.543 GB versus the prior 46.984 GB
(-0.92% median). Wall-clock samples were 75.965 and 71.461 seconds; the
repeat is 0.58% faster than the prior 71.878-second run, while the two-run
median remains noisy. The CPU and allocation reductions retain the change.

## Rejected experiment: event-maintained active-face pointer

`state.Object.Face` still accounted for 11.51 flat CPU-seconds, with 5.75
seconds reached directly from `checkFaceTriggers`. The probe added a derived
active `*cards.Face` cache to every `state.Object`, initialized at object
creation and maintained by `events.Apply` on `FlipFace`, Myriad, CardToken,
and StackCopy. It retained the original defensive lookup for manually
assembled or externally replaced test fixtures. A red/green test required
new objects to initialize the cache; focused state/events tests, the regular
targeted gate, vet, and whitespace checks passed.

The cache is deliberately not retained. The fixed `GOMAXPROCS=5` 500-game
artifact (`/tmp/gorge-active-face-cache-gomax5-500-20260918.json`) is exactly
equal to the post-`AbilityPush` semantic oracle: 498 eligible roots, two
no-root games, 100 covered roots, and zero errors. It took 90.270 seconds
and allocated 47.006 GB. The immediately preceding retained direct-interest
candidate measured 71.461 seconds / 46.543 GB; wall samples are noisy, but
the new representation adds allocation and has no plausible end-to-end
benefit. The CPU profile explains the regression: `Object.Face` fell from
11.51 to 9.90 flat CPU-seconds, but the larger, frequently copied Object made
`runtime.duffcopy` rise from about 30.40 to 36.68 CPU-seconds. The trial code
and its cache-specific test were reverted exactly.

An initial run inherited `GOMAXPROCS=16` while the fixed control records 5.
Its 500 per-result payloads were identical after timing fields were removed,
but the outer JSON comparator correctly rejected the environment mismatch;
it is not used for this decision.

## Known baseline failures

`go test ./...` reproduces the prior checkpoint's unrelated failures:

- `cmd/botbench`: `TestFullPairsIteratesSorted` and
  `TestConstructedDefaultIsByteIdentical`.
- `cmd/gorged`: `TestDeckDirectoryListingIsSortedAndComplete`.

They are outside this rules/effects work and no goldens or host behavior were
changed.
