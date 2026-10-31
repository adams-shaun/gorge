# SpellBench v2 as gorge's default two-player bot interface

Date: 2026-10-28. Parked; see `ARCHIVE.md`.
Outcome: **designed and measured, not switched.** The adapter agrees with
native on every comparable recorded decision. It costs about 3x native
ns/decision and about 6.7x the allocations, which fails the cost item of the gate below
(<= 2x native, with no allocation regression). Native `seat.Seat` stays the
default.

Hard constraint: the gorged web client and its HTTP/session API keep
working unchanged in both phases.

## Two layers

gorge keeps two decision models and changes which one is canonical:

1. **External, canonical: v2 `protocol.SeatDecision`.** This is the
   observation plus candidates, each with a typed `semantic`, as defined in
   `server/spellbench/protocol`. It is the contract offered to bots outside
   the engine, and the neutral world (`server/spellbench/neutral`) already
   implements it.
2. **Internal: `decision.Decision` + `view.View`.** These stay the engine's
   own model. The rules engine poses them, the native bots read them, and the
   gorged client sees them through `engine/view` and the host's JSON.

v2 sits above the engine as an input layer. It does not replace the engine's
model.

## Package `bots/v2api`

```go
type Agent interface { Decide(*protocol.SeatDecision) (candidateID int) }
```

`NativeAdapter` wraps any `seat.Seat` as an `Agent`, so existing bots keep
running unchanged. Its path is the `mapping.Pose` already built by
`server/spellbench/mapping`, then `Picked` back to an `Intent`. For a bot that
only sees the bare observation, it uses the neutral translation:
`neutral.Session.Payload`, then `agent.Rebuild`, then `seat.Seat.Decide`, then
`agent.Pick`, then `Session.Picked`. The benchmark's `v2Seat`
(`server/spellbench/neutral/v2_adapter_bench_test.go`) is exactly this shape.
The package does not exist: with no callers it would be a stub.

## Phases

- **Phase 1.** v2 is the canonical **external** contract, the one the
  neutral world already implements. Native bots stay native. The new seat
  kind is optional (`-seat-kind v2`, via `server/spellbench/seat`, which
  already exists), and v2 agents are registered in `bots/registry.go`.
- **Phase 2 (only after the gate).** Re-implement the inputs of
  `bots/heuristic` and `bots/botpolicy` on the candidate model. `seat.Seat`
  then becomes an adapter in the other direction
  (`Candidates -> decision.Decision` via the neutral world).

## Packages touched

| Phase | Packages that change |
|---|---|
| 1 | `server/spellbench/{neutral,mapping,env,observe}`, `bots/registry.go` (register v2 agents), `cmd/spellbench-gorge-*`, docs |
| 2 (adds) | `bots/heuristic`, `bots/botpolicy`, `bots/hosted`/az seats, `engine/seat` (only for the MatchSeat hook) |

**Not touched in either phase:** `engine/*` (the rules core),
`server/host` (including `server/host/httpapi` and
`server/host/humanseat.go`), `server/protocol` (`protocol.Version=1`), the
`server/gorged` routes (including `server/gorged/seats.go`), SSE, the
`engine/decision` JSON shapes, the web client, and replay hashes. The v2 path
is strictly additive.

## What stays native

- **Search bots** (az, sbsearch) need clones of the live engine
  (`MatchSeat` / `MatchContext.Engine`) or the honest redeal world
  (`x_gorge_view_v1` on gorge, kshadow/neutral elsewhere). The v2 observation
  hides that by design, so these bots keep their native path or the extension
  and are outside phase 1's "default interface". v2 is **not** the training
  interface, because it loses hidden information and is slower (see the
  numbers below).
- **N-seat and Commander.** A v2 game has exactly two players (`players`
  holds exactly two records, `spellbench-v2.md:793`), and gorge's headline
  modes (4-seat, Commander) can never be v2-only. The default is therefore
  "v2 for two-player constructed duels, native `seat.Seat` for N-seat",
  enforced by a hard rule in `bots/registry.go`. It is not a replacement.

## Gate (switch only after the numbers)

`bots` defaults to the v2 canonical path for 2-player constructed ONLY when
ALL of these hold, measured at the same head:

1. The twin and outcome thresholds of `scripts/sb-parity.sh` pass (N >= 200
   games, 3 bots).
2. 0 diffs over 1,000 games on dirac-supported decks, or dirac is explicitly
   out of scope.
3. The benchmark below: the v2 adapter is <= 2x native ns/decision, with no
   allocation regression on the heuristic hot path.
4. `go test ./server/host/... ./server/gorged/... ./view/...` and the
   existing web-client e2e suite pass unmodified, and the `engine/rules` golden heads (`TestHeads`) are unchanged.

Until then v2 remains the compatibility and benchmark surface, and the native
seat stays the default.

## Measurement

Benchmark: `server/spellbench/neutral/v2_adapter_bench_test.go`.

- **Corpus.** gorge bot mirrors played on gorge's own engine through
  `minihost`, with `x_gorge_view_v1` on and no engine auto-pay (as in
  `TestDecisionsMatchGorgeOnGorgeGames`). Every `choose` request and the
  candidate the seat answered are recorded per seat. The JSON is decoded once,
  outside the timer.
- **Bot.** `test-bot-auto-pay` (the heuristic with seat auto-pay), built with
  the recording seat's agent seed.
- **native.** The bot on gorge's own view and decision (the
  `x_gorge_view_v1` payload, pre-decoded): `agent.Rebuild`, then
  `Decide` once per native decision and per folded follow-up, then
  `agent.Pick`.
- **adapter.** The same bot and plan/Pick loop, but its view and decision
  come from `neutral.Session.Payload` over the typed `SeatDecision`
  (observation and candidates only, no extension). This is the in-process
  `NativeAdapter`, with no JSON.
- **adapter+json.** adapter, plus decoding each request line the way the wire
  agent does (the typed `SeatDecision`, and the semantics with
  `UseNumber`).

### Agreement: `TestV2AdapterAgreesWithNative`

Over one game, 645 recorded decisions were replayed. First, as a
precondition, the native path reproduces the recording seat's pick on every
one of them. Then 578 decisions were compared, 241 of them with more than one
candidate; the 67 picks that float mana are excluded, because the neutral
world never floats mana under engine autopay. **The adapter picks the same
candidate as native on all of them.** The test needs at least 20 comparable
multi-candidate decisions. A mutation that blinds the adapter's view to the
battlefield makes it fail with 18 disagreements.

### Cost: `BenchmarkV2Adapter`

The command:

```
systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400% env GOMAXPROCS=4 GOMEMLIMIT=3GiB \
  go test -timeout 2m -run TestV2AdapterAgreesWithNative -bench BenchmarkV2Adapter -benchtime 3x ./server/spellbench/neutral/
```

It ran on an AMD Ryzen 9 9950X with GOMAXPROCS=4. The corpus is 2 games and
1205 recorded decisions. One op is a full pass over the corpus with a fresh
bot (and, for the adapter, a fresh session).

| path | ns/op (corpus) | ns/decision | B/op | allocs/op | allocs/decision | Decide calls/decision |
|---|---:|---:|---:|---:|---:|---:|
| native | 17,268,203 | 14,330 | 19,276,741 | 115,325 | 96 | 0.989 |
| adapter | 51,575,111 | 42,801 | 74,181,469 | 767,036 | 637 | 0.968 |
| adapter+json | 392,614,056 | 325,821 | 222,520,053 | 1,581,060 | 1,312 | 0.968 |
| adapter (record compare) | 119,789,166 | 62,914 | 330,210,028 | 598,703 | 314 | 0.981 |

Ratios to native:

- adapter is **2.99x ns/decision** and **6.65x allocs**.
- adapter+json is **22.7x ns/decision** and **13.7x allocs**.

The adapter (record compare) row is a 2026-10-29 run of the same command, when
the recorded corpus had grown to 2 games and 1904 decisions. Its same-run
native path was 17,693 ns/decision and 134.5 allocs/decision, so the
record-compare adapter is **3.56x ns/decision** and **2.34x allocs**. The gate's
allocation clause passes (314 <= 318); its 2x ns clause does not, so the
outcome is unchanged. The cache removes the per-object rebuild (`Builder.View`
reuses a card whose record is unchanged), deciding reuse by comparing a deep
copy of the record field by field. The remaining adapter cost is
`Session.Payload`'s candidate translation and the bot's own `Decide`.

A second adapter run (5x, with a CPU profile) gave 44,212 ns/decision and
the same allocation count. In that profile, about 60% of the adapter's own
time is in `Session.Payload`, almost all of it `Builder.View`, which rebuilds
a `view.View` (card faces, printed characteristics) from the observation on
every decision. About 36% is the bot's own `Decide`.

### Reading the numbers

- **The in-process adapter is about 3x native and over the 2x budget.**
  Allocations grow 6.65x where the gate asks for no regression. Getting under
  2x needs `Builder.View` to reuse the previous decision's card views and diff
  the observation, instead of rebuilding the view.
- **JSON is the dominant external cost.** With JSON decoding, the path is
  22.7x native, so training loops stay on the native path.
- **Decide calls differ slightly** (0.968 vs 0.989 per decision). The extra
  native calls are most likely follow-ups on the decisions that float mana (not
  traced), which the neutral world never poses. This is not a disagreement:
  those decisions are excluded from the comparison.
