# Compiled Face Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a deterministic registry-owned compiled metadata catalog for `cards.Face`, migrate selected CPU hot paths to it, and preserve textual and replay behavior exactly.

**Architecture:** The pointer-rich Forge IR remains authoritative. After parsing or gob decoding, `cards.Registry` builds immutable flat rows, integer codes, masks, offset/count spans, a string blob, and a schema-qualified corpus hash. Faces and linked abilities receive unexported runtime bindings and fall back to text when unbound or unknown.

**Tech Stack:** Go standard library only: `crypto/sha256`, `encoding/binary`, `sort`, and `testing`.

**Spec:** `docs/superpowers/specs/2026-09-17-compiled-face-metadata-design.md`

## Global Constraints

- Keep the card pipeline and rules core pure Go, with no cgo or third-party dependencies.
- Never add Forge scripts or `.cards` contents to git.
- Sort every source map before emitting catalog data.
- Keep `TriggerPush.Amount` and `AbilityPush.Amount` as face-local ordinals.
- Do not change event kinds, replay/decision schemas, RNG, or state mutation paths.
- Unknown vocabulary retains its original text and existing fallback behavior.
- Do not regenerate goldens or run race tests without separate authorization.

---

### Task 1: Pin pre-catalog costs

**Files:**
- Create: `cards/compiled_bench_test.go`
- Create: `docs/superpowers/reports/2026-09-17-compiled-face-metadata-running.md`

**Interfaces:**
- Consumes: `LoadRegistry`, `Face.Is*`, `Face.HasKeyword`, `Face.SpellAbility`, `Face.ManaAbilities`, and `state.Object.Face`.
- Produces: stable benchmark names used after every migration.

- [ ] **Step 1: Add benchmark fixtures**

Add `BenchmarkLoadRegistry`, `BenchmarkFaceTypeQueries`,
`BenchmarkFaceKeywordQueries`, and `BenchmarkFaceAbilityQueries`. Resolve
`../.cards/ir.gob.gz` and skip when absent. Use package-level sinks. The query
benchmarks use positive and negative checks, including a parameterized keyword:

```go
var benchmarkBool bool

func BenchmarkFaceKeywordQueries(b *testing.B) {
    f := &Face{Keywords: []string{"Flying", "Ward:2", "Trample"}}
    b.ReportAllocs()
    for range b.N {
        benchmarkBool = f.HasKeyword("Ward") && !f.HasKeyword("Haste")
    }
}
```

`BenchmarkLoadRegistry` must call `LoadRegistry`, not `OpenCorpus`, so source
staleness cannot switch the measured path.

- [ ] **Step 2: Run and record the baseline**

```bash
go test ./cards -run '^$' -bench 'Benchmark(LoadRegistry|FaceTypeQueries|FaceKeywordQueries|FaceAbilityQueries)$' -benchmem -count=5
go test ./rules -run '^$' -bench 'BenchmarkFaceTriggerScanDistinctFaces$' -benchmem -count=5
go test ./rules -run '^$' -bench 'BenchmarkNewEngineObjectArena$' -benchmem -count=5
```

Record Go version, cache size, medians, and a one-process `runtime.MemStats`
load measurement. Keep the registry live through the second collection with
`runtime.KeepAlive`.

- [ ] **Step 3: Verify and commit**

```bash
go test ./cards ./rules
git diff --check
git add cards/compiled_bench_test.go docs/superpowers/reports/2026-09-17-compiled-face-metadata-running.md
git commit -m "test: benchmark face metadata baseline"
```

---

### Task 2: Define codes, masks, and conservative unknowns

**Files:**
- Create: `cards/compiled_codes.go`
- Create: `cards/compiled_codes_test.go`

**Interfaces:**
- Produces: `FaceID`, `AbilityID`, `StringID`, `SAKind`, `APICode`, `TriggerModeCode`, `StaticModeCode`, `ReplacementEventCode`, `TypeMask`, `KeywordMask`, `ColourMask`, `FaceFlags`, and `TriggerInterest`.

- [ ] **Step 1: Write failing mapping tests**

Cover all four SA kinds; every non-test API registered in `effects`; all modes
handled by `rules.triggerModeEvents`; every static/replacement mode handled by
rules; all `Face.Is*` type names; and every literal keyword head queried by
production code. Assert zero for unknowns, nonzero unique known codes, and:

```go
func TestUnknownTriggerInterestIsCatchAll(t *testing.T) {
    if got := triggerInterestForMode("FutureTrigger"); got != TriggerInterestAny {
        t.Fatalf("unknown interest = %x, want catch-all", got)
    }
}
```

- [ ] **Step 2: Observe the expected failure**

```bash
go test ./cards -run 'TestCompiledCode|TestUnknownTriggerInterest' -count=1
```

Expected: compile failure because the types do not exist.

- [ ] **Step 3: Implement fixed constants and switch classifiers**

Reserve zero for unknown. `TypeMask` covers Magic card types plus `Basic`,
`Legendary`, `Ongoing`, `Snow`, and `World`; subtypes set no bit. `KeywordMask`
covers only literal heads queried by production code. Define semantic trigger
bits independent of `events.Kind`:

```go
const (
    TriggerInterestAny TriggerInterest = 1 << iota
    TriggerInterestZoneChange
    TriggerInterestStackPut
    TriggerInterestAbilityPush
    TriggerInterestAttackDeclaration
    TriggerInterestTargetsChosen
    TriggerInterestTap
    TriggerInterestDamage
    TriggerInterestDraw
    TriggerInterestLifeChange
    TriggerInterestStepChange
)
```

Map unknown/always-observing trigger modes to `TriggerInterestAny`. Assign API,
mode, and replacement constants explicitly; never derive them from corpus sort
order.

- [ ] **Step 4: Verify and commit**

```bash
go test ./cards -count=1
git add cards/compiled_codes.go cards/compiled_codes_test.go
git commit -m "feat: define compiled card metadata codes"
```

---

### Task 3: Compile deterministic flat rows

**Files:**
- Create: `cards/compiled_catalog.go`
- Create: `cards/compiled_catalog_test.go`
- Modify: `cards/ir.go`

**Interfaces:**
- Consumes: Task 2 types and finalized textual card graphs.
- Produces: `CompiledCatalog`, `CatalogIdentity`, `Span`, row types, `Registry.CompileMetadata() error`, `Registry.Catalog() *CompiledCatalog`, `Face.CompiledID() FaceID`, `Face.CompiledTriggerInterests() (TriggerInterest, bool)`, `SA.CompiledKind() SAKind`, and `SA.CompiledAPI() APICode`.

- [ ] **Step 1: Write failing deterministic-layout tests**

Build equivalent fixture registries with different `Params`, `SVars`, and
token map insertion order. Assert equal `CanonicalBytes` and identity, card
faces before lexically sorted tokens, one-based IDs, in-range spans, recursive
sub-ability binding, unknown code plus retained string ID, and a changed hash
after changing one parameter value.

- [ ] **Step 2: Observe the expected failure**

```bash
go test ./cards -run 'TestCompiledCatalog' -count=1
```

- [ ] **Step 3: Define the row schema**

```go
type Span struct { Start, Count uint32 }
type StringRef struct { Offset, Length uint32 }
type ParamRow struct { Key, Value StringID }
type KeywordRow struct { Head, Full StringID }
type SVarRow struct { Name, Body StringID }
type AbilityRow struct {
    Kind SAKind
    API APICode
    Params Span
    Sub AbilityID
    Line, UnknownKind, UnknownAPI StringID
}
type TriggerRow struct { Mode TriggerModeCode; Params Span; Effect AbilityID; UnknownMode StringID }
type StaticRow struct { Mode StaticModeCode; Params Span; UnknownMode StringID }
type ReplacementRow struct { Event ReplacementEventCode; Params Span; With AbilityID; UnknownEvent StringID }
```

`FaceRow` contains spans for types, keywords, abilities, triggers, statics,
replacements, and SVars; masks/flags; power, toughness, mana value; colour and
colour-identity masks; and trigger interests. `CompiledCatalog` owns all row
slices, `TypeTokens []StringID`, string refs/blob, identity, and unexported CPU
reverse indexes for `*Face` and `*SA`.

- [ ] **Step 4: Implement build-then-publish compilation**

Walk cards/faces in slice order and tokens by sorted key. Sort every parameter
and SVar map. Deduplicate linked SA pointers. Check all lengths and offsets
before converting to `uint32`. Encode canonical fields explicitly in
little-endian order; never hash Go padding or gob output. Compute SHA-256 with
the hash field excluded, then bind faces/SAs and publish the catalog only after
all checks pass.

- [ ] **Step 5: Verify and commit**

```bash
go test ./cards -run 'TestCompiledCatalog' -count=20
go test ./cards -run 'TestWholeCorpusCompiles|TestCorpusPrimitiveSurface' -count=1
git add cards/compiled_catalog.go cards/compiled_catalog_test.go cards/ir.go
git commit -m "feat: compile flat face metadata catalog"
```

---

### Task 4: Integrate registry and gob lifecycle

**Files:**
- Modify: `cards/registry.go`
- Modify: `cards/registry_test.go`
- Modify: `cards/open_test.go`
- Modify: `cards/tokens_test.go`

**Interfaces:**
- Consumes: `Registry.CompileMetadata()`.
- Produces: compiled `CompileDir`, `LoadRegistry`, and `OpenCorpus` results; safe `Add` invalidation.

- [ ] **Step 1: Write failing lifecycle tests**

Assert that compile and load return non-nil catalogs with identical identity.
Finalize a small registry, call `Add`, assert `Catalog() == nil` and old faces
are unbound, then rebuild and assert old/new faces are bound. Round-trip a
registry containing tokens and verify deterministic token IDs.

- [ ] **Step 2: Observe the failures**

```bash
go test ./cards -run 'Test.*(Catalog|Registry|OpenCorpus|Token)' -count=1
```

- [ ] **Step 3: Wire finalization after all repair work**

Call `CompileMetadata` only after parse/decode, derive, link, keyword expansion,
intrinsics, and token compilation finish. Make `Add` clear all existing
face/SA bindings before clearing `r.catalog`. Do not encode bindings/catalog;
the gob version stays unchanged because its encoded shape does not change.
Document the independent catalog schema version.

- [ ] **Step 4: Verify, measure, and commit**

```bash
go test ./cards -count=1
go test ./cards -run '^$' -bench 'BenchmarkLoadRegistry$' -benchmem -count=5
git add cards/registry.go cards/registry_test.go cards/open_test.go cards/tokens_test.go docs/superpowers/reports/2026-09-17-compiled-face-metadata-running.md
git commit -m "feat: bind compiled metadata during corpus load"
```

Record catalog build time, allocations, retained heap, row counts, and
canonical byte size. Stop and profile if compact rows/blob do not explain the
retained-memory increase.

---

### Task 5: Migrate face queries with textual parity

**Files:**
- Modify: `cards/face.go`
- Modify: `cards/compiled_catalog_test.go`
- Modify: `cards/compiled_bench_test.go`

**Interfaces:**
- Consumes: bound face rows and CPU reverse indexes.
- Produces: unchanged `Face.Is*`, `HasKeyword`, `KeywordParam`, `SpellAbility`, and `ManaAbilities` signatures with compiled fast paths.

- [ ] **Step 1: Write corpus parity and unbound fallback tests**

Add test-only textual reference functions. Compare every corpus face for every
known type and keyword head. Cover mixed case, parameterized/unknown keywords,
no spell ability, multiple mana abilities, and unbound synthetic faces.

- [ ] **Step 2: Run the tests on the textual implementation**

```bash
go test ./cards -run 'TestCompiledFaceQueryParity|TestUnboundFaceFallback' -count=1
```

Expected: PASS; also assert the corpus fixtures really have nonzero compiled
IDs so the post-change test cannot accidentally exercise only fallback.

- [ ] **Step 3: Add compiled fast paths**

Use row masks for recognized types/keywords. Use textual loops for unbound
faces and unrecognized queries. Preserve case-insensitive matching. Use the
spell/mana ability IDs and CPU reverse SA index to return the identical `*SA`
pointers in identical order.

- [ ] **Step 4: Verify, benchmark, and commit**

```bash
go test ./cards -count=1
go test ./cards -run '^$' -bench 'BenchmarkFace(Type|Keyword|Ability)Queries$' -benchmem -count=5
git add cards/face.go cards/compiled_catalog_test.go cards/compiled_bench_test.go docs/superpowers/reports/2026-09-17-compiled-face-metadata-running.md
git commit -m "perf: use compiled face query metadata"
```

Reject a fast path that regresses its target or adds steady-state allocation.

---

### Task 6: Consume compiled trigger interests

**Files:**
- Modify: `rules/trigger_eligibility.go`
- Modify: `rules/trigger_eligibility_test.go`
- Modify: `rules/trigger_hotspots_test.go`
- Modify: `rules/engine.go`
- Modify: `rules/clone.go`

**Interfaces:**
- Consumes: `Face.CompiledTriggerInterests()`.
- Produces: `eventTriggerInterest(events.Kind) cards.TriggerInterest`; bound faces bypass pointer-map compilation and unbound faces retain current behavior.

- [ ] **Step 1: Write event/mode parity tests**

Compare existing textual masks with semantic interests for every defined
`events.Kind`. Cover `ChangesZone`, phase-bearing triggers, unknown modes,
triggerless faces, Rooms, face changes, and unbound synthetic faces. Unknown
future event values must take the conservative path.

- [ ] **Step 2: Observe the compiled-path failure**

```bash
go test ./rules -run 'Test.*Trigger.*(Interest|Eligibility|Cache|Room|Face)' -count=1
```

- [ ] **Step 3: Wire the compiled branch**

Implement `eventTriggerInterest(events.Kind) cards.TriggerInterest` with an
exhaustive switch. In `faceMayTrigger`, use compiled interests when available
and the existing cached textual mask otherwise. Retain pointer validation in
the object cache for transforms and synthetic replacement. Do not alter
traversal/APNAP order, Room ordering, LKI behavior, phase diagnostics, firing
limits, or pending trigger ordinals.

- [ ] **Step 4: Verify, benchmark, and commit**

```bash
go test ./rules -run 'Test.*(Trigger|Room|Ward|Dethrone|Clone)' -count=1
go test ./rules -run '^$' -bench 'BenchmarkFaceTriggerScanDistinctFaces$' -benchmem -count=5
git add rules/trigger_eligibility.go rules/trigger_eligibility_test.go rules/trigger_hotspots_test.go rules/engine.go rules/clone.go docs/superpowers/reports/2026-09-17-compiled-face-metadata-running.md
git commit -m "perf: use compiled trigger interest masks"
```

---

### Task 7: Add dense known-API dispatch

**Files:**
- Modify: `effects/registry.go`
- Modify: `effects/context_test.go`
- Create: `effects/registry_bench_test.go`

**Interfaces:**
- Consumes: `SA.CompiledAPI()` and fixed API code count.
- Produces: one atomically published snapshot with name and opcode lookup; unchanged `Register`, `unregister`, `Supported`, and `Resolve` APIs.

- [ ] **Step 1: Write dispatch-equivalence tests**

Test a bound known API, unbound known API, registered unknown API, unregistered
unknown API, and replacement/unregistration of a known API. Preserve the
existing concurrent registration/read test. Verify that overriding `Draw`
changes both textual and compiled dispatch.

- [ ] **Step 2: Observe the missing dense path**

```bash
go test ./effects -run 'Test.*(Registry|Resolve|Supported|CompiledAPI)' -count=1
```

- [ ] **Step 3: Publish both views atomically**

Use:

```go
type effectRegistrySnapshot struct {
    byName map[string]Effect
    byCode []Effect
}
```

Under one writer mutex, clone/update both views and publish one pointer.
`Resolve` loads one snapshot, tries a nonzero compiled code, then falls back to
`byName[sa.API]`. `Supported` continues returning names. Leave non-API support
on its current atomic map.

- [ ] **Step 4: Verify, benchmark, and commit**

Add `BenchmarkResolveKnownCompiledAPI`, `BenchmarkResolveKnownTextAPI`, and
`BenchmarkResolveUnknownAPI`, using a nonallocating host/effect. Run:

```bash
go test ./effects -run 'Test.*(Registry|Resolve|Supported|CompiledAPI)' -count=1
go test ./effects -run '^$' -bench 'BenchmarkResolve.*API$' -benchmem -count=5
git add effects/registry.go effects/context_test.go effects/registry_bench_test.go docs/superpowers/reports/2026-09-17-compiled-face-metadata-running.md
git commit -m "perf: dispatch compiled effect APIs by opcode"
```

---

### Task 8: End-to-end verification and checkpoint

**Files:**
- Modify: `docs/superpowers/reports/2026-09-17-compiled-face-metadata-running.md`
- Modify: `docs/superpowers/reports/2026-09-17-compiled-face-metadata-resume.md`

**Interfaces:**
- Consumes: all prior tasks and `/tmp/gorge-searchprobe-engine-final-500-20260917.json`.
- Produces: final measurements and a continuation prompt for predicate/cost compilation.

- [ ] **Step 1: Run focused/static verification**

```bash
go test ./cards ./effects ./rules ./state ./internal/searchprobe ./cmd/searchprobe -count=1
go vet ./...
git diff --check
```

- [ ] **Step 2: Run the full suite**

```bash
GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./... -count=1
```

Do not run races or regenerate goldens. Classify the documented botbench,
stall-guard, and occasional undo-stream failures against baseline; any new
failure blocks completion.

- [ ] **Step 3: Run the fixed workload**

```bash
GOMAXPROCS=5 GOMEMLIMIT=5GiB go run ./cmd/searchprobe \
  -games 500 -workers 5 -seed 10000 -sample-seed 54321 \
  -attempts 64 -worlds 4 -max-submits 5000 \
  -cpuprofile /tmp/gorge-compiled-face-cpu-500-20260917.pprof \
  -memprofile /tmp/gorge-compiled-face-heap-500-20260917.pprof \
  -out /tmp/gorge-compiled-face-500-20260917.json
```

- [ ] **Step 4: Enforce semantic and performance gates**

Compare against the immutable JSON after deleting only top-level
`LoadSeconds`, `TotalSeconds`, `AllocatedBytes`, `HeapAllocBytes`,
`HeapSysBytes`, and per-result `SampleNS`/`SearchNS`. Require `true`, 500 games,
499 eligible roots, one no-root game, 114 covered roots, and zero errors.
Record wall/CPU/allocation/heap results and hotspot changes. Revert losing
consumer migrations; catalog layout alone does not justify a regression.

- [ ] **Step 5: Complete the reports and commit**

Record exact HEAD, commands, artifacts, semantic comparison, known failures,
retained/rejected results, and next work. Then:

```bash
git add cards effects rules docs/superpowers/reports/2026-09-17-compiled-face-metadata-running.md docs/superpowers/reports/2026-09-17-compiled-face-metadata-resume.md
git commit -m "perf: compile immutable face metadata"
git status --short
```

Do not push without fresh authorization in the execution session.
