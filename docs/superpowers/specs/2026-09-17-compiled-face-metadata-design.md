# Compiled Face Metadata Design

**Date:** 2026-09-17
**Status:** Approved for implementation planning

## Purpose

Compile immutable `cards.Face` data into a registry-owned, integer-backed
catalog that speeds current CPU hot paths and can later be exported as flat
buffers to an optional CUDA worker. The textual Forge IR remains authoritative
and available for diagnostics, coverage, unsupported syntax, and incremental
migration.

This is deliberately not a conversion from strings everywhere. Integer codes
are appropriate only for closed domains whose meanings the engine owns.
Open-ended Forge text remains text and every compiled classifier has an
explicit unknown fallback.

## Current baseline

At the current pinned corpus, loading `.cards/ir.gob.gz` and forcing a garbage
collection measured:

- 33,669 cards and 839 token scripts
- 35,385 faces
- 20,992 top-level abilities, 17,664 triggers, 7,164 statics, and 2,589
  replacements
- 54,671 linked `SA` nodes
- approximately 0.87 seconds to load and relink
- approximately 93 MB of retained heap and 1.28 million retained heap objects

Corpus domain sizes explain which representations are safe:

- `SA.Kind`: 4 values
- API name: 194 observed values
- trigger mode: 137 observed values
- static mode: 83 observed values
- replacement event: 34 observed values
- keyword head: 255 observed values
- type token: 620 observed values, mostly open-ended creature and other
  subtypes
- parameter key: 1,156 observed values

The final 500-game profile remains the performance and semantic baseline. Its
non-timing JSON must remain exactly equal during migration.

## Goals

1. Give every corpus face and linked behavior row a compact runtime identity.
2. Replace repeated comparisons in selected hot paths with masks or integer
   dispatch while preserving existing behavior.
3. Store compiled relationships as contiguous arrays and `offset + count`
   spans, suitable for direct serialization or conversion to structure-of-
   arrays buffers.
4. Retain the original text for diagnostics and for syntax the compiler does
   not understand.
5. Make stale or mismatched compiled identities detectable through explicit
   schema and corpus identities.
6. Permit a measured, consumer-by-consumer migration with a textual oracle.

## Non-goals

- No CUDA runtime, cgo, third-party dependency, plugin, or external process is
  added in this milestone.
- No event, replay, decision, saved game, or public protocol stores a runtime
  catalog ID.
- Filters, conditions, and costs are not converted to bytecode yet.
- Dynamic characteristics and CR 613 layer evaluation remain authoritative in
  `rules`.
- The textual `Face`, `SA`, `Trigger`, `Static`, and `Repl` structures are not
  removed.
- This milestone does not introduce an active-object trigger index or batched
  search frontier. It supplies the immutable metadata those later changes
  need.

## Architecture

### Registry-owned sidecar

`cards.Registry` owns one immutable `CompiledCatalog`. Production construction
has two phases:

1. Parse or decode the textual IR, then perform all existing derivation,
   linking, keyword expansion, and intrinsic expansion.
2. Build and bind the compiled catalog from that final textual graph.

`CompileDir` and `LoadRegistry` perform both phases before returning. This is
important because loading currently re-runs idempotent keyword expansion so a
reused cache benefits from newly implemented expansions. Catalog compilation
must happen after that repair or its spans and masks could describe the stale
pre-expansion graph.

`NewRegistry` and `Add` remain usable as builder/test APIs. A registry that has
not been finalized has no catalog, and its faces use the existing textual
paths. Adding to a finalized registry invalidates its catalog rather than
leaving a partially correct one. Production registries are treated as
immutable after construction.

Each compiled corpus `Face` receives an unexported binding containing its
`FaceID` and catalog pointer. These fields are derived runtime state and are
not gob encoded. Synthetic faces and hand-built test fixtures remain unbound
and continue to work through textual fallback behavior.

### Deterministic identity

The following internal integer types are introduced:

```go
type FaceID uint32
type AbilityID uint32
type StringID uint32
type SAKind uint8
type APICode uint16
type TriggerModeCode uint16
type StaticModeCode uint16
type ReplacementEventCode uint8
```

Zero is the unknown or unbound value for every ID/code type. Catalog row IDs
are one-based so a zero-initialized value can never accidentally name a real
row.

Face IDs are assigned in this exact order:

1. `Registry.Cards` order, which compilation already derives from sorted
   script paths.
2. Face slice order within each card.
3. Token cards in lexically sorted token-key order.
4. Face slice order within each token card.

Ability, trigger, static, replacement, parameter, type-token, keyword, and
string entries are emitted while walking faces in that order. Any source map
is traversed using sorted keys. Go map iteration can therefore never affect an
ID or exported byte sequence.

Runtime row IDs are meaningful only together with a `CatalogIdentity`:

```go
type CatalogIdentity struct {
    Schema     uint32
    CorpusHash [32]byte
}
```

`Schema` changes whenever a code table, row layout, ordering rule, or compiled
semantic changes. `CorpusHash` is SHA-256 over the canonical compiled catalog
content, excluding addresses and runtime bindings. It identifies the actual
linked corpus rather than trusting filesystem timestamps or a caller-supplied
label.

No runtime ID is written to an event or existing external protocol. In
particular, `TriggerPush.Amount` and `AbilityPush.Amount` retain their current
meaning as face-local slice ordinals. Changing them would break replay and is
outside this design.

### Flat catalog tables

The catalog uses immutable slices of fixed-width rows. Variable-length
relationships use `uint32` start/count spans.

`FaceRow` contains:

- card/supertype mask
- printed colour mask and existing colour-identity mask
- engine-consumed keyword mask
- flags such as permanent and characteristic-defining
- parsed power, toughness, and mana value
- spans for type tokens, keywords, abilities, triggers, statics,
  replacements, and SVars
- the conservative trigger-event mask already computed by
  `triggerMaskForFace`

`AbilityRow` contains the kind code, API code, parameter span, sub-ability ID,
and source-line string ID. `TriggerRow`, `StaticRow`, and `ReplacementRow`
contain their mode/event code, parameter span, and linked ability ID where
applicable. `ParamRow` contains key and value `StringID`s. Keyword and type
rows preserve their full text or token ID even when a fast mask bit also
exists.

Strings are represented by a deterministic byte blob plus offset/length rows,
not Go pointers, in the compiled sidecar. The textual IR still owns normal Go
strings and maps. The initial CPU migration need not route general parameter
access through the blob; flattening it now establishes an exportable schema
and permits later compiled filters without redesigning adjacency.

All offsets and counts are range-checked while building. A corpus too large
for `uint32` is rejected explicitly instead of wrapping.

## Closed and open domains

### Fixed masks

The type mask contains only Magic card types and supertypes that the engine
recognizes, including the types currently queried by `Face.Is*`. Creature
types and other subtypes are not a closed enum: they remain token IDs/text in
the type span. An unknown type cannot alias a known mask bit.

The keyword mask contains only keyword heads directly consumed by engine hot
paths. Parameterized and uncommon keywords remain in the keyword span with
their original spelling and parameter. Adding a mask bit is a schema change;
unknown keywords remain queryable through the textual fallback.

Colours use a fixed WUBRGC bit representation. The existing WUBRG colour
identity values retain their meanings.

### Fixed opcodes

`SA.Kind` has fixed codes for `SP`, `AB`, `DB`, and `ST`.

API, trigger-mode, static-mode, and replacement-event code tables contain only
engine-owned, implemented names. Codes are explicit constants, not positions
derived from the current corpus. New codes are appended; renumbering or
changing meaning requires a catalog schema bump.

The effects registry binds known `APICode`s to a dense function table for
dispatch. Its existing string registry remains authoritative for unknown API
names and plugin overrides. Registration updates both views when a name has a
known code. Thus an accelerator-friendly opcode never prevents a newly
registered textual API from working.

### Unknown and dynamic values

An unrecognized closed-domain string compiles to code zero and retains its
original `StringID`. It never maps to a default known code. Callers either use
the textual implementation or report the same unsupported diagnostic they do
today.

Names, descriptions, SVar names and bodies, arbitrary parameter keys/values,
dynamic expressions, selectors, and unsupported Forge syntax remain textual.
Later predicate compilation must use three outcomes:

- `yes`: the compiled predicate proves a match
- `no`: the compiled predicate proves no match
- `maybe`: dynamic or unsupported syntax requires the Go textual oracle

CUDA preselection may discard only `no`. Treating unknown syntax as false
would silently remove legal candidates and is forbidden.

## CPU migration

Migration is consumer-by-consumer, with the old textual result retained as a
test oracle:

1. Build catalog identity, rows, spans, masks, and bindings without changing
   behavior.
2. Route `Face.Is*`, `HasKeyword`, `KeywordParam`, `SpellAbility`, and
   `ManaAbilities` through compiled metadata when bound, retaining textual
   fallback for synthetic faces.
3. Replace per-engine face-pointer trigger-mask caching with the catalog's
   immutable face trigger mask. Object-local caching may remain temporarily
   where it avoids repeated face lookup.
4. Add dense known-API dispatch while preserving string dispatch and plugin
   replacement semantics.
5. Re-profile before choosing any additional consumer.

The catalog does not cache derived object characteristics. Printed face data
is immutable, while granted types, colours, keywords, power/toughness,
controller, face state, and continuous effects remain game-epoch-dependent.
Those values continue through the current rules layer.

Pointer identity remains valid during the migration. `*cards.SA` continues to
flow through resolution continuations and ability objects, and trigger lookup
continues to preserve face-local ordinals. Integer row IDs are an additional
lookup path, not replacements for replay-sensitive identity.

## Gob cache lifecycle

The first implementation rebuilds the compiled catalog after gob decode and
does not encode runtime bindings. This preserves the current rule that decoded
data is repaired by the current linker and avoids trusting compiled metadata
created by older expansion logic.

The gob cache version is bumped only if its encoded shape changes. Catalog
schema version is independent from gob cache version. If a future optimization
stores flat catalog bytes in the cache, that cache must carry and validate both
the catalog schema and corpus hash, and the cache version must be bumped.

Failure to compile metadata is a registry-load error for malformed spans,
overflow, or internal inconsistency. Unknown Forge vocabulary is not a load
error; it is represented by the unknown-code/text fallback.

## CUDA boundary enabled by this design

CUDA remains outside the pure-Go core. A future optional worker or plugin-tier
adapter receives:

- `CatalogIdentity`
- pointer-free catalog rows and string/blob metadata once per corpus
- structure-of-arrays snapshots of mutable objects per board epoch
- batched queries across many search worlds

Promising batches are world-by-object filter preselection, world-by-trigger
event eligibility, and world-by-action cost feasibility. The worker returns
bitsets, candidate row IDs, or feasibility results only. Go remains
authoritative for event application, ordering, continuations, decisions, RNG,
unsupported predicates, and continuous-effect layer semantics.

A single engine operation is not a CUDA target. The measured scan of 240
distinct faces takes only about 3.2–4.5 microseconds, and current roots contain
only a few candidate worlds. CUDA work begins only after a batched frontier can
offer hundreds or thousands of independent predicate evaluations per dispatch
and an end-to-end IPC benchmark demonstrates a win.

Every accelerator response must echo the catalog identity and board epoch.
Mismatched or stale responses are rejected. During validation, sampled or all
GPU candidate masks are compared with the CPU oracle; a disagreement disables
the accelerated result rather than changing game behavior.

## Testing and measurement

### Correctness tests

- Build the catalog twice from identical textual registries and compare its
  canonical bytes exactly.
- Randomize source-map insertion order in fixtures and verify identical rows,
  IDs, and hashes.
- Compare compiled and textual type, keyword, ability, trigger-mask, and API
  classifications across every corpus face.
- Exercise unknown types, keywords, APIs, modes, events, and parameters with
  synthetic faces and prove textual fallback remains active.
- Round-trip the gob cache and verify identical catalog identity and compiled
  query results after decode/re-link/rebuild.
- Verify tokens receive deterministic IDs independent of Go map iteration.
- Assert that serialized events and face-local `TriggerPush`/`AbilityPush`
  ordinals are unchanged.

### Benchmarks

Add focused benchmarks for:

- corpus load/relink/catalog-build time and allocation
- retained registry heap and object count
- `Object.Face`
- known and unknown type checks
- known, parameterized, and unknown keyword checks
- known API dispatch and unknown/plugin fallback
- distinct-face trigger-event preselection and full trigger scan

Later bytecode milestones add filter and cost benchmarks before changing those
paths.

### Semantic gates

Each consumer migration must pass focused package tests and `go vet ./...`.
The full suite is run with known baseline failures classified rather than
silently accepted. Before the milestone is declared complete, the search probe
is rerun and every non-timing JSON field is compared exactly with
`/tmp/gorge-searchprobe-engine-final-500-20260917.json`. Replay goldens are not
regenerated to conceal differences.

## Rollout and rollback

Each migration step is independently revertible. Unbound or unknown metadata
uses the old textual implementation, which permits tests and synthetic cards
to coexist with the compiled corpus throughout the rollout.

Performance is measured after each step. A compiled structure that increases
retained memory without producing a meaningful CPU/allocation benefit is
reworked or rejected; CUDA-oriented data layout alone is not sufficient
justification for a regression in the CPU engine.

The next architectural milestones, each requiring its own design review, are:

1. normalized tri-state filter/condition programs
2. compiled bounded cost vectors with dynamic fallback
3. event-indexed active trigger membership
4. dense per-world object snapshots and epoch-based effective
   characteristics
5. batched search frontiers and an optional external accelerator protocol
