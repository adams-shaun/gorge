# Performance and bot-effectiveness strategy

## Purpose

Improve the engine's throughput and the quality of deterministic bot players
without letting performance noise, deck bias, or replay drift masquerade as
progress. This is a strategy and investigation backlog, not authorization to
change bot policy or rules behaviour.

## Measurement contract

The semantic oracle for engine work remains the fixed 500-game
`searchprobe` workload, run with `GOMAXPROCS=5 GOMEMLIMIT=5GiB`. A candidate
must compare exactly equal after removing only timing and memory telemetry.
Wall-clock decisions use interleaved A/B/A or A/B/A/B samples on the same
machine; a single profile or a two-run median is diagnostic evidence, not a
ship criterion. CPU and heap profiles explain a change but cannot substitute
for the exact replay comparison.

No future optimization may enlarge `state.Object` merely to cache immutable
card data. The active-face pointer and card-owned sidecar probes both regressed
through `runtime.duffcopy`. The retained face-bound trigger-interest value is
the accepted boundary: immutable `cards.Face` data may carry a compact bound
value, while objects continue to carry only card identity and face index.

## Performance investigation queue

1. Re-profile the retained tree before proposing code. Attribute copies and
   allocations to callers, not just `runtime.duffcopy`; keep `Game.Clone` and
   `Object.CloneDeep` as measurements, not assumed causes.
2. Compare event-log growth (`events.growEvents`) with a bounded capacity or
   reservation hypothesis. The event sequence, hash chain, and replay bytes
   are invariants; capacity may change only allocation behaviour.
3. Examine projection and search collection separately from rules execution.
   `view` construction, JSON encoding, and searchprobe collection are valid
   end-to-end costs but require their own benchmark so an engine optimization
   is not claimed for a harness-only win.
4. Audit repeated short-lived slices, beginning with `Face.ManaAbilities` and
   legal-action candidate construction. Prefer iterator or caller-owned-buffer
   APIs only if they preserve ordering, re-entrancy, and clone independence.
5. Revisit predicate and static evaluation only from a fresh profile. The next
   compiler expansion must retain textual fallback (`yes`/`no`/`maybe`), and a
   cache may not memoize context-dependent legality or layer results.

Each item starts as an approved spike with a focused benchmark, a red/green
semantic test, a profile, an exact 500-game oracle comparison, and an
interleaved timing decision. Rejected probes stay documented; no goldens are
regenerated to make a performance change appear valid.

## Bot effectiveness strategy

The initial goal is a stronger deterministic heuristic policy, not online
learning or an opaque model. The existing `cmd/botbench` already provides the
right experiment skeleton: fixed game seeds, policy seat trading, deck-pair
matrices, starting-player attribution, confidence intervals, decision
statistics, and action coverage. It should be the acceptance harness for any
policy change.

### Phase 1: establish an honest baseline

- Run the current `bot` against `legacy` across the full deck-pair matrix,
  with enough games per pair to report confidence intervals rather than a
  pooled anecdote.
- Preserve the same-policy `bot` versus `bot` control to detect bench bias.
- Record wins, draws, stalls, turns, starting-player splits, decision-kind
  coverage, unchosen option shapes, and per-card/action coverage. A policy
  may not improve win rate by increasing invalid decisions, stalls, or
  unexercised decision families.

### Phase 2: make decisions observable before making them clever

Add a deterministic, opt-in decision trace for botbench. For every offered
decision it should record a versioned feature snapshot, legal option IDs and
shapes, selected option IDs, and terminal outcome attribution. It must use
only stable engine/view data, fixed order, and no wall clock; it is an
evaluation artifact, never an event or replay input. Redact hidden opponents'
information from a seat's feature snapshot.

Use these traces to rank errors by frequency and consequence: missed casts,
bad targets, mana sequencing, attacks, blocks, optional triggers, discard and
choice responses. Diagnose a decision family on held-out seeds and deck pairs
before changing its policy.

### Phase 3: controlled heuristic ladder

Make one decision family better at a time, retaining deterministic tie-breaks
by option index. The first candidates are:

1. mana and cast sequencing using the engine's offered/potential-action data;
2. target and removal valuation using the existing target-effect summary;
3. combat scoring that values trades, blockers, life totals and known board
   state before considering search;
4. optional-trigger, discard and modal choices using local card/board value.

Every change needs unit tests for its decision contract, determinism tests,
same-policy controls, a `bot`-versus-`legacy` matrix, and no regression in
stall/error rates. A positive pooled rate is insufficient when a deck-pair or
starting-player split reverses it.

### Phase 4: offline learned policy, only after trace quality is proven

If heuristics plateau, evaluate an offline scorer trained from deterministic
botbench traces and explicitly curated examples. The feature schema, model
version, weights, and inference must be checked in as reproducible data/code;
no network, ambient randomness, map iteration, or runtime training reaches a
match. Start as a shadow scorer that logs rankings without changing intents,
then promote it through the same paired matrix gates. Do not begin self-play
training until trace coverage, hidden-information boundaries, and evaluation
variance are demonstrably controlled.

## First proposed bot task

Design the opt-in botbench decision-trace schema and a report that aggregates
decision-family regret proxies without changing the policy. Its review must
define the exact public-information fields, versioning, file retention,
redaction test cases, and the matrix/seed split used for evaluation. Only then
choose the first heuristic family to improve.
