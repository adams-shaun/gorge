# Interface review: plugging bots into gorge

This is an assessment, for the operator, of every surface a bot can use to
play gorge, written while building the [Python and Java examples](examples.md)
and the [native Go example](native-go.md) against commit `ad3777023` on
2026-10-20. It states what each surface is, measures it, lists the defects
found on the way, and ends with a ranked set of decisions on what to build
next for remote agents. The status of each item was rechecked against the
code at `513a2a0c3` on 2026-10-21. Field-level detail is in the
[HTTP API reference](../api/http.md) and the
[SpellBench v2 reference](../api/spellbench-v2.md).

## Verdict

The HTTP seat is the right foundation for remote agents: two endpoints, any
language's standard library, a server-enforced seat boundary, and about
0.25 ms of overhead per decision on loopback. The P0 defects that let a
buggy agent crash or wedge a match are fixed: a refused answer is now a
`409` and the decision stays pending, and a decision can be answered only
once. What remains is that it has no deadline, no client-side
reproducibility and no schema version (P1, P2 below). Add a long-poll so an
agent needs one round trip per decision, and leave WebSocket and multi-game
batching until a user needs them. SpellBench v2 stays as it is: stdio is its
specification, and it is a compatibility surface, not a remote-agent
transport.

| Rank | Decision | Cost | Status at `513a2a0c3` |
|---|---|---|---|
| P0-1 | An intent the rules refuse must not crash the match: validate "one defender per attacker" on the wire, and re-park any other refusal as a `409` | S | done |
| P0-2 | Claim the parked decision when an intent is admitted, so a duplicate POST is a `409`, never a second `204` or a hung handler | XS | done |
| P0-3 | Default spectator visibility to `public` on any server with human or HTTP seats | XS | done |
| P1-1 | Per-decision deadlines for HTTP seats, announced in the decision, answered by the caretaker on expiry; reap abandoned on-demand tables | S | open |
| P1-2 | Long-poll: `GET .../pending?seat=N&wait_ms=…` blocks until a decision exists | S | open |
| P1-3 | Authentication: an optional server key for `POST /api/games`; tokens stay per-seat capabilities; TLS by reverse proxy | S | open |
| P2-1 | A versioned decision schema: JSON Schema generated from the Go types, a schema-version field, and a grow-only ratchet | M | partly done since: the generated schemas are published ([`openapi.json`](../api/openapi.json), checked by `TestOpenAPISchemasMatchGo`); no schema-version field or grow-only ratchet yet (rechecked at `25d71cadc`) |
| P2-2 | Client-chosen seeds on `POST /api/games`, for reproducible remote experiments (decks can already be chosen) | S | open |
| P2-3 | Keep the Python and Java examples as the reference SDKs, with a CI job that plays one game with each | S | done (`scripts/examples-smoke.sh` in CI) |
| P3-1 | SpellBench v2: find why the neutral translation costs the default bot most of its strength; make `game_over` wait for its `ack` | M | open (still 1 of 20 against 17 of 20 on 2026-10-21) |
| P3-2 | Multi-seat batching endpoints, only when a training user needs more than about 10,000 decisions per second from outside Go | M | not needed yet |
| — | Rejected: gRPC (third-party dependency), WebSocket now (hand-written RFC 6455 for a gain long-poll already captures), a stdio transport for new bots | — | — |

Costs: XS under half a day, S one to two days, M about a week, all
including tests.

## The surfaces, side by side

| | Native Go seat | HTTP seat | SSE stream | SpellBench v2 seat (`-sb-agent`) | SpellBench v2 agent / env commands |
|---|---|---|---|---|---|
| **Code** | `engine/seat`, `bots/registry.go` | `server/host/httpapi/rest.go`, `server/host/humanseat.go`, `server/gorged/seats.go` | `server/host/httpapi/sse.go`, `server/host/fanout.go` | `server/spellbench/seat` | `cmd/spellbench-gorge-agent`, `cmd/spellbench-gorge-env` |
| **Transport** | function call | HTTP/1.1, JSON bodies | HTTP/1.1 `text/event-stream` | child-process stdin/stdout | stdin/stdout |
| **Framing** | Go values | one JSON document per request; intent body ≤ 64 KiB, unknown fields refused | SSE `id:`/`event:`/`data:` frames, a `{v,t,id,table,match,seq,body}` envelope | one strict JSON object per line, ≤ 8 MiB, integers only, depth ≤ 64 | same |
| **Auth** | none needed (trusted code) | per-seat bearer token (16 random bytes), header or `?token=`; the claim names table and seat; process memory only | none for a public table; a private play-vs-bot table needs the seat's token on the stream | the host spawns the process; no auth | none |
| **Hidden information** | View only by contract; `MatchContext.Engine` (live engine, read-only by convention) reaches any seat implementing `MatchSeat` | enforced server-side: `?seat=` must equal the token's seat; views are seat-projected | whatever the table's spectator visibility allows: **everything** on an `omniscient` table | neutral observation for the agent's seat only | as the host provides |
| **Versioning** | Go API, compiled together | `protocol.Version = 1` covers SSE frame bodies only; the decision and intent shapes are `engine/decision` structs that grow by `omitempty` fields, unversioned | `v` field in every frame | `protocol: "spellbench/v2"`, `protocol_minor` | same |
| **Deadlines** | hosted search seats: 2 s per decision (`hostedDecisionDeadlineMS`), then their heuristic fallback | **none** in `gorged` (`ThinkTimeout` 0); the caretaker answers only at table shutdown | 15 s keep-alive; 30 s resume grace | the host's budget: 2 s in `gorged`, none in `mtgsim`; startup 300 s | — |
| **Concurrency** | goroutines; `-bot-search-slots` bounds concurrent searches | one token per seat; games limited by `-max-on-demand-tables` (default 32 retained) | one session per connection, 256-frame ring | one process per seat per match | one process per caller |
| **Failure behaviour** | a returned error crashes the match; `RefusalAnswerer` and the fallback ladder catch refused answers | `POST /intent` waits for the engine's verdict; any refusal is a `409` and leaves the decision pending | overflow closes the session with an `overflow` frame | timeout, crash, bad JSON or bad candidate forfeits: concede at the next priority, default bot until then | `error` responses with a closed code list |
| **Observed latency** | 21–34 µs per decision, engine and both seats (`cmd/duel`, 40 games) | `GET /pending` 0.12 ms, `POST /intent` 0.11 ms, `GET /view` 0.50 ms median; next decision visible 0.13 ms after the answer (busy poll) | wake-up to visible decision: 0.24 ms median | `choose` 0.05 ms (Python minimal agent) to 0.18 ms (gorge bot, neutral world); 0.41 s process start per match | — |

All latencies: 2026-10-20, loopback, Ryzen 9 9950X, `gorged -pace 0`;
commands on [Playing a seat over HTTP](http.md#latency) and
[SpellBench v2 agents](spellbench.md#latency). Sixteen concurrent Python
agents on one `gorged` played 4,156 decisions in 1.27 s of wall time,
about 3,300 decisions per second including process start.

Two smaller surfaces are out of scope: the ManaBrew adapter
(`server/host/manabrewhttp`, opt-in with `-manabrew`, SSE plus POST for one
specific client) and `gorged.RegisterNetBot`, which is a native-Go seam for
network-backed policies and inherits the native column.

## Defects found while writing the examples

D1, D2 and D3 are fixed. Their write-ups stay here as the record of what an
HTTP client could do to a match before `513a2a0c3`.

### D1: a rules-refused HTTP intent crashed the match (fixed)

**Observed.** A random Python agent declared one creature (Strangleroot
Geist, object 38) as attacking both the opponent and the opponent's Jace.
`POST /intent` answered `204`; the engine then refused the declaration and
the table halted:

```text
table: g21
match: 1
intents: 1258
turn: 48
reason:
intent 1258 rejected: attacker 38 declared against more than one defender
```

**Cause.** An `attackers` decision has one option per (creature, defender)
pair. `Option.Group` on those options is the *defender's* attack-restriction
group (`engine/rules/combat.go`, `attackRestrictGroup`), so nothing on the
wire says two options share a creature, and `Decision.Validate` admits the
pair. The engine's own check (`combat.go`: "declared against more than one
defender") refuses it inside the match loop. A refused intent from a seat
that does not implement `bots.RefusalAnswerer` goes straight to
`r.crash` (`server/host/match.go`, the refusal branch after `attempt`).
`HumanSeat` does not implement it, so every HTTP client, and the browser,
is one rules-only refusal away from crashing its game.

**Fix.** (1) `Decision.Validate` rejects two chosen `attackers` options
with the same `Obj` (CR 508.1a: a creature attacks one player, planeswalker
or battle), so the answer is a `409` before it reaches the engine. (2)
`POST /intent` waits for the engine's verdict, and on a refusal the host
re-parks the same decision and answers the `POST` with a `409` carrying the
engine's reason (`server/host/match.go`), instead of crashing.

### D2: a duplicate POST was admitted twice, and a third hung (fixed)

**Observed.** Three concurrent identical `POST /intent` requests against
one pending decision, repeated six times:

```text
round 1 seq 18 post2: 204 0.000244s
round 1 seq 18 post3: 204 0.000245s
round 1 seq 18 post1: 000 3.006165s
round 2 seq 20 post2: 204 0.000250s
round 2 seq 20 post1: 204 0.000267s
round 2 seq 20 post3: 409 0.000422s
```

Two `204`s for one decision, and the third request hung until curl gave up
at 3 s.

**Cause.** `HumanSeat.submitAdmitted` (`server/host/humanseat.go`) validates
against the parked slot and sends on its capacity-1 channel, but leaves the
slot in place; the match goroutine clears it only when `await` returns. A
second submit in that window validates against the same slot and sends a
second answer; once the buffer is full, a third blocks forever on a channel
nobody will read, holding an HTTP handler goroutine. The same window lets a
single client's `GET /pending` return the decision it has just answered.

**Fix.** `HumanSeat.submitAdmittedWait` claims the slot under the mutex
before the send, so any later submit is a `409`. Rechecked 2026-10-21: three
concurrent identical POSTs got one `204` and two `409`s, none hung.

### D3: the default spectator visibility showed hidden cards (fixed)

`gorged -spectator` used to default to `omniscient`. On a `-humans` server,
every unauthenticated client could open `/api/stream` or `GET .../view`
without `?seat=` and read every hand, including the one an HTTP bot is playing
against. (Play-vs-bot tables default to `public` through
`-vsbot-spectator`.) **Decision (implemented):** `-spectator` defaults to
`public`; `omniscient` is an explicit opt-in. Cost XS.

### Smaller findings

- **No deadline** (P1-1): a dead HTTP bot leaves its game parked until the
  server stops. Abandoned on-demand tables also count against
  `-max-on-demand-tables`, so a crashed training loop can exhaust it
  (observed: `host: on-demand table limit 32 reached` after 32 games).
- **The SpellBench neutral translation costs strength** (P3-1): gorge's
  `bot` served through `cmd/spellbench-gorge-agent -world neutral` won 1 of
  20 games where the same bot natively won 17 of 20 (Death & Taxes against
  Dimir Tempo, seeds 0–19, `mtgsim -a sb-gorge` against `mtgsim -a bot`).
- **`game_over` is best effort**: `seat.Agent.EndMatch` writes it and stops
  the process immediately; in one three-game run only one `game_over` was
  acknowledged.
- **`-sb-agent` cannot use the exact world**: `spellbench-gorge-agent
  -world gorge` requires the `x_gorge_view_v1` extension, which the
  `-sb-agent` seat never offers (`agent requirement not offered:
  extensions`); only `-world neutral` works there.
- **SSE on a private table needs the token.** Since play-vs-bot tables
  became private, a subscribe from a stream opened without the seat's token
  is a `404`.
- Fixed since: `gorged -bot-policy`'s help now lists the registered
  policies, `mtgsim` no longer reports `reason: not_finished` for a game
  that ended naturally, and `examples/bots/python/measure_latency.py` opens
  its stream with the seat's token (it used to stop at `subscribe: HTTP
  404`; fixed after `513a2a0c3`).

## The gaps, and what to do about each

### Network transport for remote agents (P1-2)

Today an agent polls `GET /pending`, or opens the spectator SSE stream as a
wake-up and then calls `GET /pending`: two hops per decision either way,
plus back-off latency for a poller (the examples' 2–50 ms back-off makes a
whole-game average of 1.6–3.2 ms per decision against 0.25 ms of actual
HTTP). The options:

| Option | Round trips per decision | Client cost | Server cost |
|---|---|---|---|
| Polling (today) | 1 + idle polls | trivial | wasted requests |
| SSE wake-up + GET (today) | 2 | an SSE reader | none new |
| **Long-poll `GET /pending?wait_ms=`** | **1** | trivial, every stdlib | a wait on the seat's park signal |
| Seat-scoped SSE carrying the full decision | 1 (push) + 1 POST | an SSE reader | a per-seat projection on the stream |
| WebSocket | 1 each way | a client library in most languages | an RFC 6455 implementation in stdlib Go (no `x/net`): framing, masking, ping, close |
| gRPC | 1 | generated stubs | a third-party dependency: out |

**Decision: long-poll.** It reaches one round trip per decision with no new
client machinery (the examples change one URL), works through any proxy, and
needs only a channel the parked `HumanSeat` already has. A seat-scoped SSE
stream would push slightly sooner but makes every client parse SSE and
re-creates the visibility questions D3 raises. WebSocket buys nothing over
long-poll at one decision per exchange; revisit it only with P3-2.

### Authentication for remote seats (P1-3)

Per-seat tokens are the right primitive: unguessable capabilities bound to
one table and seat, never trusting a client's seat number. What is missing
is everything around them: anyone who can reach the server can create games
(`POST /api/games` takes no credential), tokens die with the process, and
`?token=` puts them in access logs. **Decision:** an optional `-api-key`
(header) that gates `POST /api/games` and future admin endpoints, the
bearer header as the documented form for bots (the query form stays for the
browser), and TLS left to a reverse proxy rather than built in. No user
accounts.

### Deadlines (P1-1)

**Decision:** a `think_ms` field on `POST /api/games` (and `-think-timeout`
for `-humans` tables), wired to the existing `host.Options.ThinkTimeout`
machinery so the caretaker answers on expiry and the game still replays; the
remaining budget published on the pending decision (`deadline_ms`), so an
agent can plan; and on-demand tables whose seat has not polled for a
configurable idle time are aborted and released. The caretaker path already
exists; this is plumbing and a reaper.

### A stable, versioned decision schema (P2-1)

The frame envelope has `protocol.Version`, but what an agent actually
parses, the `Decision`, `Option`, `Intent` and `View` JSON, comes straight
from Go structs that gain `omitempty` fields with every rules feature, and
the intent decoder refuses unknown fields, so a newer client sending a new
field to an older server fails. **Decision:** generate JSON Schema for those
types from the Go definitions (the generator that already emits the
TypeScript twins, `cmd/gentypes`, is the natural home), publish them with the
API reference, add a `schema_version` to `GET /api/bot-policies` or a new
`GET /api/version` alongside the corpus pin, and add a ratchet test that
fails when a field is removed or changes type without a version bump.
Additive fields stay free.

### Batching for training throughput (P3-2)

Measured: one Python process plays about 300 to 600 decisions per second
against a hosted bot, and sixteen in parallel about 3,300; the same games in
process run at 30,000 to 50,000 decisions per second per core. HTTP batching
(one request carrying answers for many seats, one long-poll returning every
pending decision across a client's tokens) could narrow that gap by perhaps
an order of magnitude, never close it. **Decision:** do not build it yet.
Training that needs engine-speed rollouts should run in Go (natively, or
by serving a model to a Go seat); build multi-seat endpoints when an
external training user shows a need above about 10,000 decisions per second.
P2-2 (client-chosen seeds and decks) is the cheap part of this that
reproducible experiments need now.

### Reference SDKs (P2-3)

**Decision:** the two example files are the reference clients. They are
standard-library only, about 400 (Python) and 570 (Java) lines, and cover
the whole surface including payment plans. CI runs
`scripts/examples-smoke.sh`, which starts `gorged -vsbot` and plays one
game with each, so a wire change that breaks them fails the build. Publishing packages (PyPI, Maven) waits for P2-1's
versioned schema; a package without a version contract would promise more
than gorge does.

### SpellBench v2 (P3-1)

Keep it exactly as the SpellBench specification defines it, stdio included:
its value is that agents written for the benchmark run unchanged. Two
fixes: find the fidelity loss in the neutral translation (compare the
decisions gorge's bot makes natively and through `-world neutral` on the
same seeds; the first divergence names the missing fact), and have
`EndMatch` wait briefly for `game_over`'s `ack` before stopping the
process.
