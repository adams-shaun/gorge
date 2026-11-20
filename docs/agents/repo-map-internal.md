# Repo map — `internal/` infrastructure and shared tooling

Part of the [repo map](repo-map.md) index. Bot runtime packages are in
[repo-map-engine.md](repo-map-engine.md#bots). `testkit/` and `cli/` are public so outside modules can import them.

## `internal/`, `testkit/`, `cli/`

| Package | What it owns |
|---|---|
| `internal/precheckgate` | Tests only: the `heads_gate` function `scripts/pr-precheck.sh` defines, read back out of the script. |
| `internal/archtest` | Import-graph and structural tests (dependency order, `time` allowlist, no `math/rand`, 32-bit build, root `go.mod` has no `require`, layering budgets for the `engine/rules` sub-packages, no engine leaks from host). |
| `testkit/testutil` | Corpus registry (`CorpusRegistry` — **skips when `.cards/` is absent**), repo decks (`testkit/testutil/decks/*.json`), shared invariants, the per-test budget exception inventories (`testkit/testutil/testdata/`), and the Known-approximations ratchet (`agentsdoc_test.go`). |
| `testkit/testutil/feedback` | Loads player feedback snapshots; `feedback.EngineAt`. `feedback/diracfb` replays one on whichever engine played it, dirac included (kept apart so engine tests do not link dirac). |
| `testkit/conformance` | Plays one game on two engines through a client wire and reports the first thing the wire says differently (`make wire-matrix`). |
| `testkit/cardcov` | Per-card coverage inventory and use probe shared by `cardfuzz` and `mtgsim`. |
| `testkit/bench` | Policy-agnostic game runner: `PlayGame`, `RunPairs`, watchdogs, livelock recovery. Shared with outside modules' bench tools. |
| `internal/codeshape` | Code-shape metrics and ratchets: interface width and string dispatch for `engine/rules` and `engine/effects`, per-function length and cyclomatic complexity for those and `dirac/` (`testdata/func_ceilings.tsv`); `cmd/codeshape` prints them. |
| `internal/corpusheap` | Heap-footprint ratchet for the opened corpus (tests only). |
| `testkit/recstream` | Condensed JSON Lines record streams (delta frames against the previous record, keyframes, gzip members) and the reader that opens plain, gzipped and condensed `.jsonl` alike; the format spec is its `doc.go`. Public so outside modules import it; `cmd/recstream` converts and verifies, `scripts/recstream.py` is the stdlib Python reader. |
| `testkit/paymirror` | A/B check of planned vs manual cast payment; `cmd/paymirror` drives it. |
| `cli` | Cobra-style commands on the standard library: GNU flags, environment variables and JSON `--config` files ([`docs/cli.md`](../cli.md)). |
| `internal/schemacheck` | Validates JSON documents against a directory of JSON Schema files that refer to each other by relative name. |
| `testkit/hiddeninfo` | Test oracle for what a seat may read: follows a game and says whether a piece of text names a card that seat cannot see. |
| `internal/testbudget` | Per-test-binary peak-RSS records for `cmd/testbudget`. |
| `internal/tsgen` | Go→TypeScript for `web/src/protocol.ts` (`cmd/gentypes`). |
