# gorge M2d (decision-kind completeness) + M2e (local fixture) — design

2026-09-05. Supersedes nothing; amends FL-73 and the sequencing of M2r Task 21.

Decisions in this document were settled with the user on 2026-09-05 and are
recorded here so they survive a context compaction.

## 1. Delivery model (settled)

**gorge ships as an imported Go library inside mtgserve.** Not a pod, not a
service. mtgserve calls `host.Registry` in-process.

**`cmd/gorged` is the local test fixture** — for playing the engine by hand,
debugging rules bugs, and demoing. It is not a production delivery target.

The user's words: *"plan on running gorge as an imported library in mtgserve,
but fixture the binary for local testing, agent training etc."*

A separate pod remains possible later. The endpoint shapes below are chosen so
that if it happens, only the seat-claim resolver changes. We are not building
auth, retry or multi-tenancy for that day.

### What this removes from earlier plans

An earlier draft of this design assumed gorge would be a private service behind
mtgserve and worked through a wire protocol, a principal/authz model and a
respec of M2c-1. **All of that is dropped.** With an in-process import:

- There is no production wire and no production authz boundary.
- `M2c-1`'s `OnBurst`/`OnMatchEnd` stay ordinary Go callbacks, exactly as
  specced. No respec.
- `ViewAtSeat` / `EventsSeat` / `Pending` / `SubmitIntent` are already the
  integration surface, and all four are merged.

### Agent training

In-process, through `seat.Seat`, the way `cmd/mtgsim` already drives bots. A
policy implements `Decide` and is called directly. Training never goes over
HTTP, so the fixture's HTTP never becomes a hot path and does not need
throughput engineering.

## 2. The wire already exists, and it already matches the survey

The 2026-09-04 UI survey (`docs/superpowers/reports/2026-09-03-m0-m1/ui-inspiration.md`)
made 22 recommendations for a player seat. `view.View` was designed against
them — `CardView.Token` is documented as *"tells two copies of one card apart in
the stack, the log and an arrow"* (survey #9), and `AttackingPlayer`/`BlockedBy`
say *"both exist for the arrow overlay (PL-17)"* (survey #12/#16).

Audited 2026-09-05 against the current types:

| Survey item | Wire support | Status |
|---|---|---|
| #4 status block (priority/turn/phase/stack) | `View.Turn/Step/Phase/Active/Priority` | present |
| #5 primary button label is the state | derivable from `Decision` + `View` | client-only |
| #7 type-label stack objects | `StackView.Kind` | present |
| #8 bound parameters in stack text | `StackView.Text` — *"when known"* | hedged |
| #9 short identity token | `CardView.Token` (`"#12"`) | present |
| #10 emphasise top of stack | `Stack` ordered, last is top | client-only |
| #11 stack depth | `len(View.Stack)` | present |
| #12 targeting arcs | `StackView.Targets []TargetView` | present |
| #13 pending-trigger tray | `View.Pending []PendingView` | present |
| #14 trigger order | `KTriggerOrder`, `Min==Max==n` permutation | present |
| #15 optional triggers | `KTriggerOptional`, yes/no | present |
| #16 combat arrows | `CardView.AttackingPlayer`, `BlockedBy` | present |
| #17 combat sub-step in prompt | `View.Step` | present |
| #18 prompts as text over the board | `Decision.Prompt` | client-only |
| #20 log as rules transcript | `view.Describe` + `EventsSeat` | present |
| #21 ring changed permanents | client derives from the event stream | client-only |
| #22 group permanents by type | `CardView.Types` | present |
| #1–3, #6 auto-pass, stops, held modifier, yield manager | none needed — client prefs plus an auto-submitted pass | client-only |
| #19 mana payment, manual + Cancel | none — gorge auto-pays server-side | **deliberate divergence** |
| mulligan | `KMulligan` declared, never constructed | **gap** |
| modes / modal spells | `KModes` declared, never constructed | **gap** |
| concede | nothing | **gap** |

Survey #13 is worth noting: a visible pending-trigger tray is something *no
surveyed platform does*, and the survey calls it a genuine differentiator.
gorge already carries it on the wire via requirement R3.

**Conclusion: the fixture is not blocked on protocol design.** It is blocked on
two missing decision kinds. A client that can never mulligan and cannot cast a
modal spell is a demo, not a fixture.

### Two dangling references found during the audit

Both point at work that shipped something else. Neither schedules the work.

- `effects/misc.go:160` — *"a real choice is Task 20's KModes decision."* M2r
  Task 20 shipped as hygiene minors (`Repeat`/`MaxRepeat$`, `applyCountOp`
  clamping, `ParseCost` clamping). It contains no modes work.
- M2r plan line 41, ruling R-8 — defers `UnlessCost$` to *"M2b's KModes/Charm
  machinery."* M2b-1..5 are all human-seat plumbing (`HumanSeat`, `Pending`/
  `SubmitIntent`, think-timeout caretaker, `ViewAtSeat`, integration test).
  There is no modes task in M2b.

`KMulligan` and `KModes` are scheduled nowhere in any plan on main.

## 3. M2d — decision-kind completeness (engine)

Engine work. Lands before the fixture.

### M2d-1: `KMulligan`

London mulligan with configurable free mulligans. Today `rules.New` deals seven
and begins the turn (`rules/engine.go:103`, `:141`); a mulligan round has to run
between genesis and turn 1.

`KMulligan` is already declared (`decision/decision.go:19`) and `seat/bot.go:75`
already treats it as a kind it must answer, so the bot has a fallback path.

London bottoming reuses the existing target/choose shape — after keeping at N
mulligans, the player puts N cards on the bottom, which is a `Min==Max==N`
distinct-index decision, exactly the shape `Validate` already enforces for
`KTriggerOrder`. No new wire format.

### M2d-2: `KModes`

Modal spells and Charm. `KModes` is declared (`decision/decision.go:20`) and
marked future at `effects/misc.go:143`.

This also delivers the **mid-resolution ask** machinery. M2r's R-8 declined
`UnlessCost$` ("may pay … if they do …") deterministically with a Note because
the engine cannot suspend a resolution. Chain Lightning's copy clause is the
corpus case. Once a resolution can ask, R-8's deferral closes — its own ruling
says the cost is *"one `if` once mid-resolution asks exist."*

### M2d-3: concede

Audit item 12. A `Registry` method plus, if it needs to be answerable mid-
decision, an option on the priority decision. Scope to be settled in the plan.

### The golden cost of M2d — read this before scheduling

**A mulligan changes the opening hand of every acceptance game.** Consequences:

1. **All four chain heads move.** Not one or two — the opening hand feeds every
   subsequent decision, so 2/4/6/8 all diverge.
2. **The ratchet can move in either direction.** A card counted as fully
   supported may simply stop being drawn in the new opening, and a card that was
   never reached may now be cast. The ratchet is a measurement of what the repo
   decks actually do, and M2d changes what they do.
3. Per **FL-76** the head cause is measured by build bisect, never by toggling a
   feature behind a flag. M2d is a single milestone with a known cause, so the
   bisect is main-before vs main-after.

**M2d owns exactly one golden regeneration**, done at the merge gate by the
controller, with the ratchet recomputed from the merged tree.

**Therefore M2r Task 21 must land after M2d.** R21 is the M2r end state — closed
ratchet, docs, final numbers. Any number it records before M2d is stale the
moment M2d merges. This is a sequencing change to a plan already on main.

## 4. M2e — the local fixture

### M2e-1: `TableConfig.Humans`

This is **M2c-2, already specced** in the library-first plan. It is listed here
because it blocks the fixture: gorged cannot seat a human without it. Today
`AddTable` builds all-bot tables.

Produces `TableConfig.Humans []int` (slot indices, default nil = all bots =
today), validated in-range, no duplicates, disjoint from bots; a table with
`Humans != nil` is single-shot rather than perpetual.

### M2e-2: seat HTTP

Thin. The View types already carry what a client needs, so this is plumbing the
four merged `Registry` methods onto the existing REST + SSE surface.

```
GET  /api/tables/{t}/matches/{k}/view?seq=N&seat=S    -> ViewAtSeat
GET  /api/tables/{t}/matches/{k}/events?since=N&seat=S -> EventsSeat
GET  /api/tables/{t}/matches/{k}/pending?seat=S        -> Pending
POST /api/tables/{t}/matches/{k}/intent                -> SubmitIntent
```

`?seat=` absent keeps today's spectator behaviour exactly, so every existing
endpoint and test is unchanged.

**Seat claim.** `httpapi.Options` already has `Authorize func(*http.Request) error`,
which is binary — allow the request or 401 it. Add a *separate*, non-breaking
resolver rather than changing that signature:

```go
// nil = nobody may act as a seat (spectator-only, today's behaviour)
Seat func(*http.Request) (SeatClaim, bool)
```

For local gorged the claim comes from the existing session: a session claims a
seat and holds it. In a pod it would come from a signed header instead. That
indirection is the whole of the pod-readiness, and it costs one field.

**Seat scoping is enforced even locally**, through `ViewAtSeat` rather than a
god view filtered client-side. Not for local security — because it is the only
way the fixture actually exercises the redaction mtgserve depends on. A local
god view would let a projection bug reach production untested. The existing
omniscient spectator view stays available as a deliberate debugging mode.

**Staleness needs no new mechanism.** `SubmitIntent` already validates stale
`Seq`, wrong player, out-of-range and duplicate indices, and min/max, and a
rejected intent leaves the game exactly where it was. The wire carries the
decision's `Seq` and gorge's existing validation is the fence. Audit item 17
("`Submit` returns an error with nowhere to send it") is answered by the HTTP
status plus a reason body.

### M2e-3: gorged

Un-deprioritized. `wt/a16` is the base — it is committed, rebased onto main, and
still owes **FL-40's Tokens proof obligation**, which must be discharged before
it merges.

Adds: a table plan that seats a human, and the seat endpoints mounted.

### M2e-4: the Svelte seat panel

MVP scope, survey items 4, 5, 7, 10, 18, 20:

- the four-line status block: `Priority / Turn / Phase / Stack: N to resolve`
- a primary button whose label is the state (`Resolve` / `Pass → Blockers` / `End Turn`)
- type-banded stack, top entry expanded and the rest dimmed
- prompts as text over the board, naming the source, never a modal
- the persistent transcript log in a right rail

Explicitly deferred: combat arrows (#12/#16 — the wire carries the bindings
already, so this is pure rendering whenever it is wanted), the per-step stops
strip (#2), the held-modifier full control (#3), the auto-yield manager (#6),
ringed permanents (#21).

## 5. Sequencing and the collision map

```
in flight   m2b5 (host/)              r17 (rules/, effects/)
then        R15  (rules/stack.go, combat.go, engine.go, effects/attach.go)
then        M2d-1..3 (rules/ heavy: cast.go, engine.go, turn.go)
then        R21  (final numbers, measured after M2d's regeneration)
then        M2e-1 -> M2e-2 -> M2e-3 -> M2e-4   (one slot, sequential)
parallel    M2c-1, M2c-3, M2c-4  (mtgserve side, independent of the fixture)
```

Collisions that force the order:

- **R15 ∩ R17** = `rules/stack.go`, `rules/acceptance_test.go`. Cannot run together.
- **M2d ∩ R15** = `rules/` core. M2d cannot overlap the remaining M2r rules work.
- **M2e-1..4** are a sequential chain: `TableConfig.Humans` blocks the endpoints,
  the endpoints block gorged, gorged blocks the client.
- **M2c-1/3/4** touch `host/registry.go`, `host/match.go` and test files; they
  can fill other slots but M2c-4 must land after M2b-4 (already merged).

## 6. Ledger amendments this design requires

- **FL-73 amended.** gorged stops being deprioritized scaffolding and becomes
  the library's local test fixture. It is still not a production delivery
  target — mtgserve imports the library — but it is no longer something to
  discard, and `wt/a16` becomes the base for M2e-3 rather than a parked branch
  awaiting a decision.
- **New ruling (to be numbered at merge).** A mulligan moves every golden, so
  M2d owns exactly one regeneration and M2r Task 21's final numbers are measured
  after M2d, not before. *Cost if wrong:* R21 publishes numbers that are stale
  on arrival and the ratchet has to be re-closed.

## 7. Testing

- **M2d-1/2/3**: TDD per decision kind, plus the acceptance decks. Mulligan and
  modes both change what the repo decks do, so `TestEveryRepoDeckIsFullySupported`
  and `TestRepoDecksPlayAtEverySeatCount` are the real gate, and `TestHeads` is
  regenerated once at the M2d merge with a build-bisected cause per FL-76.
- **M2e-2**: `host/httpapi` table tests for each endpoint, both with and without
  `?seat=`, asserting that a seat cannot read another seat's hand or decision —
  the redaction property, not just the status code.
- **M2e-3**: gorged already has `main_test.go` driving it on a random port; extend
  it to seat a human and take one decision end to end.
- **M2e-4**: the client is the fixture; its test is that a human can play a match
  to completion. M2b-5 (in flight) already proves the library can.

## 8. Open questions

1. Concede's shape — a `Registry` method, a priority option, or both. Settle in
   the plan.
2. Whether the mulligan round is configurable off for the acceptance decks. If
   it is, the golden move can be deferred until the fixture needs it; if it is
   not, M2d pays the regeneration immediately. Recommend **not** configurable
   off: a mulligan the acceptance suite never exercises is a mulligan nobody
   tests.
3. `StackView.Text`'s *"when known"* hedge (survey #8) — how often is it unknown
   in practice, and does the fixture surface the gap? Measure before deciding
   whether it needs work.
4. Free-mulligan count and whether it is a `rules.Config` field or a table
   setting.
