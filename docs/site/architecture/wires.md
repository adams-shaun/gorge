# Wires

gorge serves three client protocols: its native HTTP API, ManaBrew (`gorged -manabrew`)
and SpellBench v2 (`gorged -sbv2`). This page is for anyone writing a client or changing
a wire. A wire counts as supported only where a test says so, in a gate that fails on it.
This table is every such cell. A cell is an engine, a wire and the consumer reading it.
`testkit/conformance` is the harness: it plays one game through a wire and diffs what two
sessions read, after a normaliser named per wire removes what that wire documents as
representation. `TestWireMatrixPageMatchesScript` holds this table to the script.

`scripts/wire-matrix.sh commit|nightly|pr` runs the cells of a tier, one line per
cell, and exits non-zero if any fails. A skipped test counts as a failure, so it needs
the corpus. `make wire-matrix` is the `commit` tier, and CI runs that as the job
`wire-matrix`. The cells over the SpellBench and ManaBrew packages, whose tests sit behind
the build tags `spellbench` and `manabrew`, run nightly only, so they do not gate a merge.

`scripts/wire-nightly.sh` is the nightly run, on a seed that moves each day. It
plays chunks of games on each wire (`TestConformanceChunk`), fuzzed decks through
ManaBrew, and SpellBench v2 parity. v2 parity is reported and does not fail the
run. A game that diffs is written to
`wire-found-<date>.tsv`; once added to `testkit/conformance/testdata/found.tsv` it
replays in `TestFoundPairs` for good.

| Engine / wire / consumer | Asserts | Test or command | Tiers |
|---|---|---|---|
| gorge / native / harness | Two sessions of one game agree on every frame, with at least 100 frames per game | `TestNativeConformanceGorgeSelf` | commit nightly pr |
| gorge / native / hosted-bots | Chain heads hold, games replay, registered bots consume payment plans | `TestHeads`, `make sim`, `TestRegisteredBotsConsumePaymentPlans` | commit nightly pr |
| gorge / native / http-agent | The Python and Java agents each play a complete game | `scripts/examples-smoke.sh` | commit nightly pr |
| gorge / manabrew / harness | Two sessions agree through `/state` and `/send`; a moved life total and a missing action are caught; the normaliser removes only its documented fields | `TestManaBrewConformanceGorgeSelf`, `TestManaBrewNormaliserLines`, `TestManaBrewCatchesAWrongLife`, `TestManaBrewCatchesAMissingAction` | commit nightly pr |
| gorge / manabrew / bot | A bot through the wire reaches the native chain head, in process and over HTTP; state parity; no leak; every option reachable; nothing unmapped | `server/manabrew/mbtest`, `server/host/manabrewhttp` | nightly |
| gorge / manabrew / fuzz | No failure over 2,000 fuzzed-deck games on a rolling seed | `cardfuzz -manabrew` | nightly |
| gorge / v2 / harness | Goldens, chain heads over `/api/sbv2`, an empty native-only census, privacy | `server/spellbench/...`, `server/host/sbv2http` | nightly |
| gorge / v2 / stdio-agent | An agent plays to the end over the frozen wire | `TestV2AgentSeatPlaysGorgeAgent`, `TestSpellbenchSeatAppliesAgentPick`, `TestSpellbenchSeatAnswersUnposableDecisionWithFallback` | nightly |
| dirac / native / harness | Gorge and dirac agree frame by frame on one game, same seed and answers, less what the test names | `TestNativeWireMatchesGorge` | commit nightly pr |
| dirac / native / hosted-bots | Each policy dirac seats finishes 40 pairs without the refusal ladder | `TestBotsPlayDiracGames` | commit nightly pr |
| dirac / native / http-agent | A scripted client plays a complete game over HTTP, undo included | `TestServesADiracGame` | commit nightly pr |
| dirac / manabrew / harness | Every seat a seeded client, a complete game through `/state` and `/send` | `TestManaBrewServesADiracGame` | commit nightly pr |
| dirac / manabrew / bot | A bot through the translator plays the game it plays with no wire | `TestBotThroughManaBrewOnDirac` | commit nightly pr |
| dirac / v2 / harness | A complete game over `/api/sbv2` with no rules engine built and no decision left to the native wire | `TestSBV2ServesADiracGame` | nightly |
| dirac / v2 / stdio-agent | An agent plays to the end over the frozen wire | `TestV2AgentSeatPlaysDiracGame` | nightly |

Not every combination is a cell:

- More than two seats, or Commander, on SpellBench v2. The protocol is two players:
  `/api/sbv2` answers 409 and a client falls back to native. The web client does so without a notice.
- Spectators on ManaBrew or SpellBench v2. Neither wire has one, so spectating stays
  native.

The web client plays all three, chosen per browser by the Protocol setting ([Play](../guide/play.md)). It offers
ManaBrew and SpellBench v2 where the server's page carries `<meta name="gorge-manabrew">` or
`<meta name="gorge-sbv2">`. Its adapters are in `web/src/lib/manabrew` and `web/src/lib/sbv2`, with what they
share in `web/src/lib/wire`. Vitest covers them, and `make smoke`, the CI job `browser`, plays a seated game in a
browser over each wire on each engine. That job is outside this table: it needs Node and a browser, which the tiers don't.

`-sbv2` costs a native seat nothing: a seat is played over v2 only once a v2 client
[attaches to it](../api/spellbench-v2.md#attaching-a-seat), and goes back on its next native answer.
