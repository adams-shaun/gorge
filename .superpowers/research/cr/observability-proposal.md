# Rules observability proposal — judge_jebbediah

CR reference: **2026-08-07** revision. Engine inspected: **`63ee22d`**.
Status: **proposal only**; no hook, event, field, protocol or engine change in jj1.

## Problem grounded in this audit

`rules/cr601_conformance_test.go` could establish its assertions without a new
hook, but only by living inside `rules`:

- I-2's oracle reads `e.G.Stack`, the priority player's hand, compiled
  `Face().SpellAbility()` and the actual priority options. An empty stack proves
  no spell target exists independently of `askTarget`. The public option says
  “cast”; it does not say which targeting or payment predicates were evaluated
  (or never evaluated). Reusing `legalTargets` as the oracle would merely test
  agreement with another implementation, not CR 601.2c.
- The payment-order test supplies mana through `e.emit`, re-asks priority with
  `e.askPriority`, submits an ordinary cast intent and compares the actual pool
  before/after. It reads the new log suffix to check no `TargetsChosen` occurred.
  A real target decision is still unanswered, yet the pool is already debited
  (CR 601.2c precedes CR 601.2h). No log reconstruction was needed for the live
  assertion; reconstructing this boundary from a host view alone would require
  correlating decision, source, payment and intent boundaries.
- There is no public casting-stage snapshot. `pendingCast` holds player, card,
  origin, chosen cost/mode, X, delve/sacrifice selections and stage cursors;
  `commitCast` clears `e.cast` before it pays and asks targets. A nil `cast` at
  a target decision is therefore not proof that the CR casting process finished.

I-1 asks a harder question: why did a resolving spell remain on the stack?
The issue's original event/intent files were lost on redeploy. Its surviving
metadata and reproduction instructions must be preserved; jj1 has **not**
reproduced it, and establishes **no root cause**. The issue records disproved
resume hypotheses. Do not promote those hypotheses back into findings.

### What inspection changed about the premise

An absent persisted `MoveZone` does **not** prove the caller never attempted
one. `Engine.emit` calls `applyReplacements` **before** appending; a handled
move can disappear from the authoritative log. We need both the attempted
operation and its disposition, not another scan of emitted kinds.

At this base, `resolveTop` and `resumeResolution` do emit `MoveZone` to exile
for resolved ability objects. The charter's general warning about invisible
ability cessation is not evidence that these current exits are unlogged. The
engine represents cessation with exile; diagnostics should report that actual
representation, not invent a CR deletion event. `AbilityPush`/`TriggerPush`
mint wrapper IDs in `events.Apply`, so source IDs in the push event and the
new stack object's identity must be explicitly linked for an investigator.

## Proposed read-only boundary snapshot

Define a versioned, immutable diagnostic DTO in `rules` (no imports of host,
view or replay); the host serialises it. Never expose an `*Engine`, `*Game`,
mutable slices/maps or SA pointers to a diagnostic consumer. All collections
are ordered: stack order and zone order are retained, unordered IDs/keys sorted.
Each optional field distinguishes **absent** from **not captured**.

| Snapshot section | Required facts / question it answers |
|---|---|
| Identity | schema version, engine build/source SHA, corpus/IR identity, match identity, config identity, authoritative head and event count, intent cursor, diagnostic ordinal; are these facts from the same execution? |
| Boundary | returned from New/Advance/Submit, pending decision seq/kind/player/source/options/constraints, active/priority player, passes, turn/step, game-over/lost players; who is owed an answer? |
| Objects/zones | ordered stack IDs plus each object's actual Zone, owner/controller, card/face identity, spell vs ability/copy, wrapper source, ability provenance, targets, remembered objects/LKI, X/cast flags; expose contradictions between stack membership and object.Zone rather than normalising them away |
| Casting | presence of pendingCast, all choice cursors and selections, cost components, suppressed-cast IDs, choosing tag; separately whether targets are outstanding and payment has occurred; do not infer CR stage solely from `cast != nil` |
| Continuations | presence of resumePoint, kind/object/SA provenance, linked pending decision, current resolution span; commander parked moves, trigger-drain wait state, orderedTriggers cursor, pending trigger contexts/controller/source/LKI in queue order; is a pending decision actually able to resume its owner? |
| Effects | registered continuous effects including duration/expiry, derived active layer ordering and provenance, delayed registrations, replacement re-entry flag; show both recorded and derived facts, with a derivation boundary |
| Players/permanents | mana, life, counters, relevant payment candidates, damage/regeneration, attachments and combat participation; support inspection without reparsing display labels |

Use stable compiled provenance `(card identity, face index, ability slot or SVar
path, IR digest)` rather than a pointer address. A sub-ability needs a stable
path, not just its API name. Never require copying Forge scripts into this
repository to interpret a diagnostic record.

### Live access and consistency

Capture only when the single engine-driving goroutine has returned to a safe
boundary, under the match's existing mutex discipline. Requests read the last
completed snapshot with its head; they cannot interrupt a half-written engine.
Follow `host/viewat.go`'s copy-under-lock / project-outside-lock discipline.
Avoid cloning the entire event log merely to get a small state DTO. Compute
expensive derived details on an isolated clone, labelled with the captured head,
not by rerunning option enumeration or replacement predicates against live state.

`Engine.Clone` already knows the private continuation state, but it is not a
serialisation contract. A dedicated copied DTO avoids pointer aliasing and
makes omitted fields reviewable. `host.viewAt` currently projects raw zones at
an arbitrary event seq but derived effects at a burst boundary, and omits the
historical pending decision. Preserve that distinction: an arbitrary event
view must not be advertised as an exact historical continuation snapshot.
The existing `recover()` returns reader errors; keep failures visible and
read-only rather than trying to repair a match during inspection.

An unredacted judge snapshot exposes opponents' hands and hidden library order.
It must be privileged, local/admin-only by default, never piggybacked onto the
spectator/seat SSE channel. Seat-safe projection is a separate, explicitly
redacted product. Cap retained snapshots and request size; pagination must pin
one head. Do not serve slices into a live engine.

## Parallel diagnostic channel — no events.Kind change

Record structured diagnostic records outside `events.Log`. Do not add a Kind,
change Event encoding, emit diagnostic Notes or alter decisions/options: even
“just logging” through those paths changes the chain or clients' choices.
Records use a monotonic **diagnostic** ordinal, current event count/head and
intent cursor. The event count is an anchor, not a new event Seq; many branch
records can exist between adjacent game events. No wall clock or randomness is
needed inside the core. Span IDs and parent span IDs correlate nested work.

Capture the branch result at the branch that actually ran, not by re-evaluating
it in a reader. Proposed instrumentation sites:

| Site | Record |
|---|---|
| `legalActions` / `castable` | candidate card + variant + cost, offered/withheld, reason (timing, zone, restriction, suppression, mana, nonmana components), predicates evaluated/skipped/not implemented; candidate targeting `not evaluated` is distinct from `no legal target` |
| `beginCast` / `continueCast` / `commitCast` / `handleTarget` | casting transaction ID, stage entry/exit, selections, payment before/after, stack insertion and target selection; aborted-with-reason vs completed vs suspended |
| `resolveTop` | entry with top object's ID/Zone/type and resume/pending summaries; exactly one normal exit classification: empty stack, missing object, fizzle, ability complete, spell complete, suspended, already moved; distinguish panic/incomplete spans without swallowing the panic |
| `Ask` / `handleModes` / `resumeResolution` / departed-player release | resume created/cleared/re-entered, precise SA path, decision linkage, missing object or missing continuation; whether an outer caller remains to be run (do not pretend the current single resume point is a full continuation stack) |
| `emit` / `applyReplacements` | attempt ID and original operation, replacement candidates actually considered and predicate outcomes, matched source/provenance, re-entry bypass, commander parking, Updated vs Replaced, output event seqs or explicit discarded/no output |
| `moveResolvedOffStack` / `ensureLeftTheStack` | attempted resting zone, observed membership/Zone afterward, fallback used or unnecessary, final disposition |
| Trigger discovery/drain and layer evaluation | trigger captured/ordered/pushed/discarded with reason and minted wrapper ID; continuous/replacement effects considered and applied, explicit short-circuit/unexamined tail rather than a fabricated exhaustive result |

The always-cheap tier should capture boundary/resolve/move/resume disposition.
Verbose per-candidate option and effect explanations are an opt-in tier because
candidate scans dominate some games. Estimate cost by measuring allocations,
bytes per intent and total runtime on acceptance and the I-1 reproduction;
no throughput/size claims have been measured in this session.

Delivery uses a bounded buffer of copied records owned by the match, not a
caller-supplied callback executed inside effect resolution. Flush outside the
rules core. Slow/broken diagnostic persistence must neither block the engine
indefinitely nor change the game chain. On overflow/failure, record dropped
ordinal ranges and mark capture incomplete through reserved status metadata;
never silently present a gapped trace as complete. A diagnostic failure cannot
masquerade as a persist failure of the authoritative log. No diagnostic input
is used to decide game behaviour.

## Post mortem: an evidence bundle, not events/intents alone

**The literal two-file goal is impossible with today's formats.** `rules.New`
creates genesis objects from Config; the events do not encode the deck lists or
names. `replay.Replay` explicitly requires `(Config, Log)`. Nor do authoritative
events record discarded move attempts or resume-point writes. Do not claim
that a new reader can recover facts never persisted.

Propose an independently versioned bundle alongside the existing files:

- Existing `.events` and `.intents`, byte-for-byte untouched.
- A manifest containing full replay Config (ordered card identities/decks,
  names, format, commanders, starting life, mulligan setting, seed), executable
  revision/build identity, corpus pin and compiled IR digest, token registry
  identity, schema versions and file checksums. Deck *names alone* are not
  immutable deck contents. Retain access to the matching executable and IR;
  a digest without retained material cannot reproduce anything.
- `.diagnostics` ordered branch trace and boundary snapshots with head anchors,
  final complete-through markers and explicit gaps. Crash tails are allowed;
  a started span without its exit is evidence of an incomplete capture, not
  evidence that a particular exit ran.

Use private permissions and retention policy for this hidden-information bundle.
Archive it outside redeploy-managed directories before any demo deployment.
Do not commit corpus/IR or private game captures as source. Old matches without
a manifest or diagnostics remain labelled partial evidence; a verified replay
on the matching revision can regenerate a diagnostic trace, but that is
**reconstructed**, not the original captured trace. Stop at the first event
mismatch and do not explain later events using a different engine's branches.

### What a future I-1 investigation would do

Locate the first repeated resolve for the object, inspect its boundary stack
and continuation snapshot, then follow the resolution span's exit. A suspended
exit must identify the decision/resume that owns it. A completed exit must
identify its move attempt, any intercepted replacement/park, resulting events
and final zone. A missing exit or dropped range is reported as insufficient
evidence. This distinguishes “tail never reached”, “tail attempted but replaced”,
“continuation still outstanding” and “object not removed after application”
without presuming which explains historical I-1.

## Gates and rollout requiring a later ruling

Start with the pure boundary DTO and coarse resolve/move/resume records, then
add verbose legality/cost provenance. No implementation is authorised by this
document. Acceptance must prove diagnostics disabled/enabled/overflowing give
identical authoritative events, intents, decisions and heads; replay must
reproduce the same facts at matched boundaries. Test missing files, truncated
trace, mismatched manifests, panic tails, private-data access, concurrent readers
and nil/slow/failing diagnostic sinks. Benchmark before choosing buffer or disk
budgets. The judge should be able to replace private-field reads in future
conformance tests with these facts, without widening mutation authority.
