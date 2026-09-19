# Compiled Predicate and Cost Design

**Date:** 2026-09-18  
**Status:** Approved for implementation planning

## Purpose

Reduce repeated interpretation of immutable Forge filter and cost text on the
engine's hottest paths without changing game semantics. The current fixed
500-game compiled-face profile spends 21.02 CPU-seconds (5.10%) in
`effects.MatchesSpecCtx`/`MatchesObjectCtx` and 18.22 CPU-seconds (4.42%) in
`rules.ParseCost`. The latter includes 7.83 CPU-seconds in regexp matching.
`manaFeasible` is a further 8.75 CPU-seconds (2.12%), most of which is the
cost parsing and payment work it invokes.

This milestone compiles the two immutable textual inputs before migrating
their consumers:

1. Forge object-filter expressions used by `MatchesSpecCtx`.
2. Literal cost strings consumed through `ParseCost` while an engine runs.

The textual IR remains authoritative for diagnostics, unsupported syntax,
dynamic strings, and every fallback. Conditions, full mana-feasibility
algorithms, and rules behavior are not reimplemented here.

## Constraints

- The core remains pure Go with no cgo or third-party dependencies.
- Forge card scripts and `.cards` content remain untracked.
- All state mutation remains in `events.Apply`; the sidecar is read-only
  engine state and never causes an event.
- No compiled identifier or program enters an event, replay, decision,
  snapshot, or existing protocol.
- Collection and compilation are deterministic: deck/card traversal follows
  `Config.Decks` order, token keys are sorted, nested source maps are sorted,
  and the deduplicated source-text set is compiled in lexical order.
- Engine clones share the immutable sidecar. They never share mutable
  scratch, payment, or decision state.
- The post-`AbilityPush` 500-game control at
  `/tmp/gorge-searchprobe-post-ability-fix-500-20260918.json` remains the
  semantic oracle. Its normalized output must equal the compiled run exactly.

## Architecture

### Engine-owned compiled text

Add an unexported immutable `compiledText` field to `rules.Engine`. `New`
constructs it after receiving `Config`; `Clone` shares it by pointer. It is a
cache of Forge text reachable from the configured decks and token table, not
game state. A card or token introduced outside that configuration, an
on-demand SVar, and dynamically assembled text simply use the existing
textual implementation.

This sidecar intentionally does **not** live in `cards.CompiledCatalog`.
`cards` cannot import `state` or `effects` without a dependency cycle, while
filter evaluation requires live `state.Object` and `effects.SpecContext`.
Keeping the sidecar at the engine boundary also lets hand-built cards and
unfinalized registries retain their current behavior with zero setup.

The collector walks every configured face and its linked ability graph,
triggers, statics, replacements, and sorted token cards. It deduplicates raw
filter and cost strings before compiling. The sidecar exposes no numeric IDs:
it uses immutable string-keyed lookup, so there is no identifier whose order
could escape into replay-sensitive data.

### Predicate programs

`effects` owns a compact, immutable `PredicateProgram` and a
`PredicatePrograms` lookup set. `rules.compiledText` owns one such set and
attaches it to the `SpecContext` produced by `Engine.specCtx`. Calls that
construct an ordinary `SpecContext` without an engine automatically retain
the current textual matcher.

The evaluator returns exactly one of:

- `yes`: a compiled alternative proves that the candidate matches.
- `no`: every alternative is proved false.
- `maybe`: at least one alternative needs the textual oracle.

`MatchesObjectCtx` evaluates a matching bound program first. It returns on
`yes` or `no`; on `maybe`, and for an unbound spec, it runs the current
textual implementation unchanged. Thus a compiler error cannot silently
discard a candidate: only a result proved by the narrow program avoids the
oracle.

The initial instruction set recognizes only exact grammar already modeled by
the textual matcher: ordinary bases (including known card-type masks and the
zone-local `Any`, `Permanent`, `Spell`, and `SpellAbility` forms), literal
type/colour predicates and their generic `non`/`!` negations, object-local
state (`token`, tapped/untapped, attacking, kicked/surged/escaped), ownership
and controller predicates, `Self`/`Other`, and the fixed keyword predicates
whose head is already catalog-compiled. It preserves alternatives and
conjunctions directly rather than allocating split slices at match time.

Every grammar that needs an unmodeled computation remains `maybe`, including
numeric or resolved RHS values, remembered/chosen/targeted/triggered
referents, name comparisons, attachments, blocking scans, greatest-power,
zone-history predicates, unusual keyword arguments, ambiguous name commas,
and unknown words. A mixed alternative may still be conclusively `no` when a
recognized local term already fails; it is `maybe`, never `yes`, when all
known terms hold but an unsupported term remains.

### Parsed-cost programs

`compiledText` also maps a literal raw cost string to the exact `Cost` that
`ParseCost` currently produces. The engine adds a private `parseCost` wrapper:
it returns a cached value for a collected string and calls public `ParseCost`
otherwise. The public parser remains the diagnostic and fallback API.

Cached `Cost` values are frozen before publication: every slice has capacity
equal to length. A caller receives a value copy, so subsequent `append` in a
cast/payment path cannot mutate cached backing storage. Existing functional
helpers (`Plus`, `WithX`, announced-cost transforms) retain their current
copying semantics. No cache entry contains player, object, RNG, or decision
state.

Initial migration replaces only repeated calls that have an `Engine` receiver:
spell and activated-ability offers, mana-ability availability and activation,
cast/alternative-cost construction, static cost views, ward/unless-payment
paths, and cumulative-upkeep payment. Standalone parsing and dynamically
joined costs keep calling `ParseCost` through the wrapper fallback. The
implementation must not change a cost token's current modeled or `Unknown`
behavior.

## Error handling and lifecycle

Compilation is best-effort and never rejects a match configuration. An
unparseable or unsupported filter program is absent; the textual matcher is
therefore authoritative exactly as today. A cost compilation panic or
malformed value is not possible through the existing parser contract; if a
future compiler rejects a string, it must omit that entry and use `ParseCost`.

The sidecar contains no maps that are mutated after `New` returns. It is safe
for the engine's existing clone and hypothetical-search use because every
lookup is read-only. Engine creation may spend extra time parsing each unique
reachable text once; the benchmark and end-to-end run below decide whether
that startup cost is acceptable.

## Verification and acceptance

1. Add focused predicate benchmarks for a compiled `yes`, compiled `no`,
   `maybe` fallback, and unbound textual fallback. Preserve the existing
   zero-allocation matcher budget for the direct paths.
2. Add a corpus-backed differential test over every collected filter string
   and representative object/context matrix: whenever a program returns
   `yes` or `no`, it must equal `MatchesObjectCtx`'s textual result. Assert
   mixed alternatives and unknown predicates fall back correctly.
3. Add a corpus-backed cost differential test: every cached value must be
   deeply equal to `ParseCost(raw)`, including `Unknown`, hybrid, Phyrexian,
   cost parts, and malformed-token fallbacks. Verify that mutation-prone
   follow-up operations cannot alter a later cache hit.
4. Add clone tests proving a clone shares the immutable sidecar while a
   suspended cast/payment retains independent mutable `Cost` state.
5. Benchmark cached `Engine.parseCost` against public `ParseCost` on
   representative mana, hybrid, and non-mana costs before migrating each
   consumer. Record five-run medians for construction time, retained bytes,
   and allocations.
6. Run focused package tests, `go vet ./...`, and `git diff --check`.
7. Re-run the 500-game fixed workload with CPU and heap profiles. Normalize
   only the established timing and allocation fields, then compare the JSON
   exactly with the post-`AbilityPush` control. Do not regenerate goldens,
   run race tests, push, merge, rebase, or create a PR without authorization.

## Non-goals

- No compiled condition-expression language, active-object predicate index,
  general memoization of dynamic strings, or change to the rules' mana
  payment solver.
- No change to card-cache gob format, `cards.CompiledCatalog` schema, or its
  canonical bytes.
- No acceleration claim without the focused benchmark and the normalized
  fixed-workload comparison.
