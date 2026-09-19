# Hosted bot policy selection

## Purpose

Hosted games must create credible, reproducible bot opponents by name rather
than by an implicit implementation detail. A host requesting a bot gets the
production deterministic `bot` policy unless it deliberately selects a known
experimental policy. The selected identity is durable and visible without
exposing private game information.

This design wires policy selection only. It does not promote
`lethal-pressure`, change policy heuristics, regenerate golden heads, or
implement AR8 combined-attacker combat evaluation.

## Current paths and problem

`host.TableConfig` records table construction, including human slots, but not
the bot policy. When `host.Options.Seats` is absent, `host.defaultSeats`
constructs `seat.NewBot` for every slot. `match.play` separately constructs
`seat.NewBot` as every human seat's timeout caretaker. `cmd/gorged` creates
a play-vs-bot table without policy information, so it also receives those
implicit defaults.

The two production decision adapters are already unified in `seat.Bot`:
`Decide` consumes a player-projected view and `DecideBoard` consumes the
game-shaped public board prepared under the host lock. Both dispatch through
the same private policy choice. This is the correct implementation boundary
to preserve.

`cmd/botbench` has a separate, private names-to-constructors map. It includes
`legacy`, the frozen pre-B2 benchmark policy. `legacy` has reproducible
watchdog stalls and is not suitable for hosting.

## Policy model

The `host` package owns the durable hosted-policy vocabulary and resolves it
to a fresh `seat.Seat` from a supplied per-seat seed.

* `bot` is the stable production policy and the effective default.
* `lethal-pressure` is the explicit experimental policy. It constructs the
  same `seat.Bot` implementation with its AR7 opt-in behavior enabled.
* `legacy` is not a hosted policy. It remains an explicitly labelled
  diagnostic/development option in botbench only.

The resolver accepts only stable hosted names. It never substitutes another
policy for an unknown input. Its known-name reporting is deterministic.
Construction uses the existing `matchSeed ^ uint64(seat+1)` derivation, so
policy selection does not alter the engine RNG stream or introduce ambient
randomness.

## Table, match, and HTTP contract

`host.TableConfig` gains `BotPolicy string` serialized as `bot_policy`.
Absent or empty configuration resolves to `bot`; the normalized effective name
is what the host uses and reports. This preserves existing persisted tables
and all existing production matches.

`TableConfig.validate` resolves the policy before a table is accepted, so an
unknown policy cannot be registered, started, persisted, or silently routed
to a default. The same validation applies when a registry reloads its table
configuration.

At match setup, all non-human slots are created with the resolved factory.
Every `HumanSeat` timeout caretaker is built from that identical factory and
the same per-seat seed. A timeout therefore produces precisely the intent the
configured bot would have produced at that point, preserving the established
event-log and replay contract.

`POST /api/games` accepts an optional `bot_policy` field. Omission selects
`bot`; `lethal-pressure` is an explicit opt-in; `legacy` and every other
unknown name produce a clear HTTP 400. The gorged create-game closure persists
the selected name in the new table configuration.

The effective policy name is public configuration metadata. It is added to
the table and match metadata delivered through the REST and stream shapes so
spectators and the hosting client can identify a game. It is not emitted as a
game event, does not affect chain hashing, and does not contain any decision
or card information. Protocol additions are additive and are regenerated into
the TypeScript twins as appropriate.

## Privacy and replay invariants

No adapter or policy-selection branch may inspect an opponent's hand, library
order, hidden face, or unredacted decision options. The existing view-shaped
adapter reads only the acting seat's projection; the game-shaped adapter must
continue to construct exactly the corresponding public `botpolicy.Board`.

All decisions remain normal intents consumed by rules and all game-state
mutation remains through `events.Apply`. Policy metadata stays outside game
state. For a fixed engine seed, table configuration, and policy name, a run
must produce the same event log, head, result, and replay. The empty/default
policy path must preserve the existing production golden heads.

## Botbench boundary

Botbench should use the shared named-policy construction for `bot` and
`lethal-pressure` so its normal policies cannot drift from hosting. Its
diagnostic `legacy` constructor stays explicitly local or is provided through
a deliberately diagnostic-only extension; it must not become reachable from a
host table or the play-vs-bot API.

No policy-strength result is claimed by this wiring change. Existing AR7
evidence remains the basis for `lethal-pressure`'s opt-in status. Any claim
about a new policy requires the established botbench matrix: ten approved
mono-deck pairs, development seed 0 / 100 games per pair, held-out seed
1,000,000 / 400 games per pair, same-policy controls, starting-player split,
pair-level confidence intervals, stalls/errors/livelocks, replay status, and
trace-family diagnostics. Pooled win rate alone is insufficient.

## Verification

Focused tests will prove:

1. table parsing/validation defaults missing `bot_policy` to `bot`, persists
   it, and rejects unknown or diagnostic-only names;
2. controller construction uses the selected policy for bot slots and human
   caretakers, with no implicit random or fallback path;
3. both `seat.Bot` adapters make identical choices for `bot` and
   `lethal-pressure` from equivalent public facts;
4. HTTP create-game parsing defaults to `bot`, accepts the experimental name,
   exposes the effective metadata, and rejects `legacy`/unknown names;
5. a fixed seed and configuration for each hosted policy yields identical
   logs, chain head, outcome, and replay; and
6. `go test ./rules -run TestHeads -count=1` retains all current production
   heads.

The implementation verification also includes the targeted `botpolicy`,
`seat`, `host`, `host/httpapi`, and `cmd/gorged` tests, the botbench unit
suite without its known long default leaves, `go vet ./...`, `make sim`, and
`git diff --check`. Any broad-suite failures are compared with their known
stale-deck-pool/host/archtest/searchprobe baseline before being attributed to
this work.

## Rejected alternatives

Putting an unlabelled seat factory only on `host.Options` cannot persist or
report a policy identity and leaves validation too late. Handling policy
selection only in `cmd/gorged` would duplicate semantics between ordinary
tables, on-demand games, and caretakers. Exposing `legacy` for casual hosting
would make a known stalling diagnostic policy a normal opponent.

## Follow-up

AR8, combined-attacker lethal pressure against a defender's minimum legal
blocker set, is the next separate opt-in experiment. It needs its own design,
botbench evidence, and explicit authorization before any production promotion
or golden update.
