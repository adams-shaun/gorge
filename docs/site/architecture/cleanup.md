# Cleanup roadmap

Code in gorge that is redundant, parked or optional, with the evidence and
the recommended direction. It is for contributors looking for safe cleanup
work and for reviewers asking "is this still used?". It is a roadmap, not a
commitment: each item is decided on its own.

*Re-checked 2026-10-27 on `develop` @ `25d71cadc` against `go run ./cmd/pkgmap`
(100 packages, none classified SUSP or RED) and the Makefile, scripts and
workflows. Closed items are summarised at the end. Confidence: **H** high,
**M** medium, **L** low.*

## Open items

| Item | Evidence | Direction |
|---|---|---|
| Four overlapping search stacks are down to two | gorge keeps the PIMC harness (`bots/searchprobe` + `bots/searchseat`, ~10.8k non-test lines) and the PUCT search (`bots/azmcts`, ~6.2k); `searchprobe` and `searchseat` are also linked into `server/host`. | Choose the survivor on benchmark evidence. (L) |
| ManaBrew wire stack (~11.2k non-test lines across `server/manabrew`, `server/protocol/manabrew`, `server/host/manabrewhttp`) | Deprioritised; its tests build only with `-tags manabrew` (`make test-manabrew`) and are outside CI, but the code is compiled into `gorged` (`-manabrew`) and `cmd/cardfuzz` uses its mock client. | Keep; optionally tag the non-test code too. (L) |
| SpellBench v2 adapters (`server/spellbench/*`, ~12.9k non-test lines; `cmd/spellbench-gorge-env`, `cmd/spellbench-gorge-agent`) | Ported back into gorge on 2026-10-19/20 as the environment and agent roles of the SpellBench v2 protocol; classified EXT. The earlier v1/v2 bridges left gorge on 2026-10-16. | Keep while the benchmark is in use; it imports `engine/`, `bots/` and `server/` packages, plus the one `testkit/` edge below. (M) |
| `cmd/kcensus` | A one-off mechanic census for the dirty-dirac kernel's card-set planning; no Makefile, script or workflow references it. | Keep while kernel set planning uses it; a candidate to retire afterwards. (L) |
| One tier back-edge | `server/spellbench/internal/testcorpus` (test support) imports `testkit/testutil`, drawn red on [Packages](packages.md). | Harmless (test-only support), but could move under `testkit/`. (L) |
| `codeshape` ratchets skipped | `TestCodeShapeOnlyShrinks` and `TestLongFunctionsOnlyShrink` have been skipped by the maintainers since 2026-10-06. | Re-enable by deleting the `t.Skip` lines once the ceilings are re-measured. (M) |

## Legitimately unimported

These have no importer inside gorge by design and are **not** cleanup
candidates:

- `internal/archtest` and `internal/corpusheap`: test-only ratchets.
- `server/protocol/manabrew/internal/schemagen`: a generator.
- `bots/gonet`, `bots/netsimd`, `bots/spellbench`: libraries imported by
  tests here and by companion modules (the network forward pass, its SIMD
  variant, the SpellBench deck catalogs).
- `bots/gonet/gonettest`, `server/spellbench/internal/testcorpus`,
  `server/spellbench/internal/testgame` and `server/spellbench/minihost`:
  test support, imported only by tests.
- `engine/cards/oracletext`: the printed-card renderer the Oracle-text
  audit in gorge-compliance uses; a subpackage so it stays out of the IR
  cache fingerprint.
- `cli`: the command-line layer, for companion modules' commands.
- Every `cmd/` package: nothing imports a command.

## Commands with no Makefile, script or workflow reference

Measured by searching `Makefile`, `scripts/` and `.github/` for `cmd/<name>`
or `bin/<name>`: `codeshape`, `deckimport`, `headdiff`, `kcensus`,
`keywordbench`, `paymirror`, `repro`, `spellbench-gorge-agent`,
`spellbench-gorge-env`, `testbudget`. All but `kcensus` and `keywordbench`
are documented developer tools run by hand (`repro`, `headdiff`, `deckimport`,
`codeshape`) or by the maintainers' post-merge suite (`testbudget`); they are
not candidates. `keywordbench` (keyword presence statistics) is the one with
no current documented use. (L)

## Closed

- **Two heuristic policies** (`bots/botpolicy` and `bots/spellbench/builtins`):
  `builtins`' package doc now says why it exists (a port of the SpellBench
  builtin bots for a native SpellBench-style workup); both stay.
- **Flat root, dead commands, stray root files** (`deck.list`,
  `benchmark.md`, `patches/`, `cmd/autopayaudit`, `cmd/manamodelstats`,
  `internal/hindsight`, `cmd/hindsight`): restructured or deleted on
  2026-10-16 ([ADR 0001](https://github.com/adams-shaun/gorge/blob/main/docs/decisions/0001-repo-layout.md)).
- **Out-of-scope code in gorge**: the bots that are not hosted by gorge moved
  to the private companion module, and the compliance oracles and scenario
  corpora to
  [gorge-compliance](https://github.com/adams-shaun/gorge-compliance)
  ([The three repositories](../repos/index.md)). Draft events left with them on
  2026-10-16 and returned as the public `draft/` tree on 2026-10-30
  ([ADR 0002](https://github.com/adams-shaun/gorge/blob/main/docs/decisions/0002-public-engines-and-draft.md)).
- **A third-party dependency**: the GPU game pool that required
  `github.com/ebitengine/purego` moved out; `TestRootModuleHasNoRequires`
  keeps `go.mod` empty.
- **`seat` linking concrete bots**: `engine/seat` is the `Seat` contract
  only; the default bot is `bots/heuristic`.
- **Duplicate registries and wrappers**: the second policy registry and the
  three identical `bots/sb*` wrappers left with the bots that moved out; gorge has
  one registry, `bots`.
- **Documentation drift**: the package map is generated by `cmd/pkgmap` at
  every site build; `cmd/testbudget`'s doc states the current 4 GB / 4 vCPU
  budget; the `time` allowlist is down to seven packages and
  [Invariants](invariants.md#no-nondeterminism) names them.
- **Historical design records**: parked out of the tree, indexed in
  [`docs/design/ARCHIVE.md`](https://github.com/adams-shaun/gorge/blob/main/docs/design/ARCHIVE.md).
