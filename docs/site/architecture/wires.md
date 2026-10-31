# Wires

gorge serves three client protocols: its native HTTP API, ManaBrew (`gorged -manabrew`)
and SpellBench v2 (`gorged -sbv2`). This page is for anyone writing a client or changing
a wire. A wire counts as supported only where a test says so, in a gate that fails on it.
This table is every such cell. A cell is an engine, a wire and the consumer reading it.
A number missing from the first column has no test in this repository.
`testkit/conformance` is the harness: it plays one game through a wire and diffs what two
sessions read, after a normaliser named per wire removes what that wire documents as
representation. `TestWireMatrixPageMatchesScript` holds this table to the script.

`scripts/wire-matrix.sh commit|nightly|pr` runs the cells of a tier, one line per
cell, and exits non-zero if any fails. A skipped test counts as a failure, so it needs
the corpus. `make wire-matrix` is the `commit` tier, and CI runs that as the job
`wire-matrix`.

`scripts/wire-nightly.sh` is the nightly run, on a seed that moves each day. It
plays chunks of games on each wire (`TestConformanceChunk`), fuzzed decks through
ManaBrew, and SpellBench v2 parity. v2 parity is reported and does not fail the
run. A game that diffs is written to
`wire-found-<date>.tsv`; once added to `testkit/conformance/testdata/found.tsv` it
replays in `TestFoundPairs` for good.

| Cell | Engine / wire / consumer | Asserts | Test or command | Tiers |
|---|---|---|---|---|
| 1 | gorge / native / harness | Two sessions of one game agree on every frame, with at least 100 frames per game | `TestNativeConformanceGorgeSelf` | commit nightly pr |
| 3 | gorge / native / hosted-bots | Chain heads hold, games replay, registered bots consume payment plans | `TestHeads`, `make sim`, `TestRegisteredBotsConsumePaymentPlans` | commit nightly pr |
| 4 | gorge / native / http-agent | The Python and Java agents each play a complete game | `scripts/examples-smoke.sh` | commit nightly pr |
| 5 | gorge / manabrew / harness | Two sessions agree through `/state` and `/send`; a moved life total and a missing action are caught; the normaliser removes only its documented fields | `TestManaBrewConformanceGorgeSelf`, `TestManaBrewNormaliserLines`, `TestManaBrewCatchesAWrongLife`, `TestManaBrewCatchesAMissingAction` | commit nightly pr |
| 6 | gorge / manabrew / bot | A bot through the wire reaches the native chain head, in process and over HTTP; state parity; no leak; every option reachable; nothing unmapped | `server/manabrew/mbtest`, `server/host/manabrewhttp` | commit nightly pr |
| 7 | gorge / manabrew / fuzz | No failure over 2,000 fuzzed-deck games on a rolling seed | `cardfuzz -manabrew` | nightly |
| 9 | gorge / v2 / harness | Goldens, chain heads over `/api/sbv2`, an empty native-only census, privacy | `server/spellbench/...`, `server/host/sbv2http` | commit nightly pr |
| 11 | gorge / v2 / stdio-agent | An agent plays to the end over the frozen wire | `TestV2AgentSeatPlaysGorgeAgent`, `TestSpellbenchSeatAppliesAgentPick`, `TestSpellbenchSeatAnswersUnposableDecisionWithFallback` | commit nightly pr |

Not every combination is a cell:

- More than two seats, or Commander, on SpellBench v2. The protocol is two players:
  `/api/sbv2` answers 409 and a client falls back to native.
- Spectators on ManaBrew or SpellBench v2. Neither wire has one, so spectating stays
  native.
