# Testing

gorge has a large test suite. The full run happens in CI and after merge. Day to day you run
**focused** tests and learn to read the handful of ratchet and golden tests
that guard the whole engine. This page is for contributors writing or
debugging tests; setup and the worktree workflow are on
[Contributing](index.md), and the reasons behind each guard on
[Invariants](../architecture/invariants.md).

## Focused runs

```sh
go test -run 'TestOmniscienceInHandGrantsNothing' ./engine/rules/
go test -run 'TestOracleRunnerContract' ./engine/rules/
go test -v -run '^TestHeads$/^4seats$' ./engine/rules/
```

- One package per invocation, with a `-run` pattern. Do not use `go test ./...`
  (or `make test`, which is the same with `-p=2`) as an inner loop.
- On a shared machine, cap the run at the per-test budget:
  `systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400% env GOMAXPROCS=4 GOMEMLIMIT=3GiB go test -timeout 2m -run X ./pkg`.
- **Never `-count=1`.** It defeats the test cache. Go re-runs a test whose
  inputs changed. The one input it cannot see is the corpus (it is outside the
  module), so `make fetch-cards` clears the test cache whenever the pin moves.
- **Prove the corpus ran.** Tests load cards through
  `testutil.CorpusRegistry(t)` (`testkit/testutil/decks.go`), which calls
  `t.Skip` when `.cards/` is missing. A package that prints `ok` in about
  2 ms has run almost nothing. Use `-v` and look for `SKIP` if in doubt. In
  a git worktree, link the main checkout's `.cards/` into it.
- Load the corpus only through `testutil.CorpusRegistry`, never
  `cards.LoadRegistry`: the shared registry is opened once per test binary,
  while a fresh one costs hundreds of MB.

## The per-test budget

Every single test must fit **4 GB RSS, 4 vCPU and 1 minute of wall time**.
`cmd/testbudget` enforces it in the post-merge full suite
(`go test -exec "testbudget exec"`, then `testbudget check`). It names every test over 60 s and every
test binary whose peak RSS is over 4 GiB, and fails the batch on any not
already listed in the shrink-only files
`testkit/testutil/testdata/wall_exceptions.txt` and `rss_exceptions.txt`.
Fix an offender by splitting it into chunk tests that share fixtures. Never
cut its coverage, and never add a row.

## Writing a test

Engine tests live in the package they test (`package rules`), so they can use
unexported helpers. The common ones (`handEngine`, `card`, `corpusCard`,
`onBoard`, `onBoardCard`, `castAndResolve`, `passUntil`) are listed with
their locations in
[Your first card fix](first-card-fix.md#option-b-a-go-test-in-enginerules).

- **Name cards; do not embed scripts.** Use corpus cards by name, or a
  minimal synthetic script you wrote (`card(t, "Name:Mountain\nTypes:Basic
  Land Mountain\nOracle:x\n")`). Forge's script text is GPL-3.0 and never
  enters the repository.
- **Fixtures may set state directly** (`e.G.AddObject`, `e.G.SetZone`) to
  build a position. Engine code may not. Once the game is running, move
  things with `e.emit(events.Event{...})` so the log stays truthful.
- **Assert a control.** Show the card still works where it should.
- **`t.Parallel()`** unless the test mutates package state.

## Oracle scenarios

An oracle scenario is a JSON file that states what a card does according to
its printed text and the CR; gorge's runner (`engine/rules/oracle_run.go`)
sets up a real game, drives it through player actions only, and asserts
observable state. The scenario corpora live in
[gorge-compliance](https://github.com/adams-shaun/gorge-compliance)
(`scenarios/oracle/`, `scenarios/xmageport/`), whose `audit/` drivers
`TestOracleAudit` and `TestXMagePortScenarios` play them against a pinned
gorge. Run one card there with `make oracle-audit ORACLE_RUN=<Card_Name>`.

gorge keeps the runner and the few scenario files its own card tests load
(`engine/rules/testdata/oracle/<family>/`); `TestOracleRunnerContract`
(`engine/rules/oracle_contract_test.go`) plays them through the exported API
gorge-compliance uses, so a runner change that would break it fails here.

Two per-card ratchets sit beside the scenarios in gorge-compliance:

- `<family>/known-divergent/<slug>.json`: scenarios known to fail. A listed
  scenario that now passes is stale and fails the build until you delete its
  row. An *unlisted* failure fails the build.
- `<family>/known-unconsumed/<slug>.json`: scenarios with a scripted answer
  no decision used. Rows are only ever deleted.

gorge-compliance's `TestOracleScenarioFilesWellFormed` checks the schema and
needs no corpus; in gorge, `TestOracleScenarioShapeCheck` does the same for
the residual files.

## Ratchets and goldens: what a failure means

A **ratchet** is a test whose expected value is a committed table, and which
fails in *both* directions. A new gap is a regression. A listed gap that your
change closed is a stale entry, and you delete it in the same commit. A
branch never adds an entry or raises a ceiling.

| Test | Where | When it fails |
|---|---|---|
| `TestHeads` | `engine/rules/heads_test.go`; goldens in `engine/rules/testdata/heads/<seats>.txt` | The deterministic bot games over the 12 Legacy repo decks, at 2/4/6/8 seats, finished with a different hash-chain head. Your change altered what happens in those games. If intended, re-pin with `GORGE_UPDATE_HEADS=1 go test ./engine/rules/ -run '^TestHeads$'` and name the first diverging event and its cause in the commit message (`go doc ./cmd/headdiff` shows how to find it). If not intended, you have a bug, often nondeterminism. |
| `TestRepoDeckGamesReplayExactly` | `engine/rules/acceptance_test.go` | A game replayed from its log diverged from the live run: something mutated state outside `events.Apply`, or something nondeterministic reached an event. |
| `TestEveryRepoDeckIsFullySupported` | `engine/rules/acceptance_test.go`; table in `engine/rules/testdata/known-unsupported/` (one JSON file per card) | A card in the repo decks (`testkit/testutil/decks/*.json`) needs a primitive this build does not register (a regression: fix it), or a listed card is now fully supported (delete its file). |
| `TestEveryRepoDeckParamsAreRead` | `engine/rules/paramcensus_test.go`; table in `engine/rules/testdata/paramcensus/` | The same, for script parameters no code reads and cost tokens no code models. |
| `TestValueHeadRegistryMatchesEvaluator` | `engine/rules/value_heads_test.go` | A `count:` value head was added to the evaluator but not to `modelledValueHeads` in `engine/effects/value_heads.go`, or the reverse. |
| `internal/archtest` (`TestDependencyOrderHolds`, `TestTimeIsImportedOnlyByTheHost`, `TestRootModuleHasNoRequires`, the `layering_*` tests, …) | `internal/archtest/` | You added a forbidden import edge (for example `effects` → `rules`, `view` → `rules`), imported `time` outside the host tier and its allowlist, or added a `require` to `go.mod`. Restructure the code; do not edit the allowlists. |
| `TestClassificationIsComplete` | `cmd/pkgmap/main_test.go` | A package has no tier entry in `docs/site/architecture/packages.tiers.json`. Also run by the `pre-commit` hook (`go run ./cmd/pkgmap -check`). |
| `TestKnownApproximationsOnlyShrinks` | `testkit/testutil/agentsdoc_test.go` | The register in `docs/agents/known-approximations/` grew. It only shrinks. |
| `internal/codeshape` (`TestCodeShapeOnlyShrinks`, `TestLongFunctionsOnlyShrink`) | `internal/codeshape/ratchet_test.go` | Shrink-only ceilings on function length, interface width and string-keyed dispatch in `engine/rules` and `engine/effects` (`go run ./cmd/codeshape -table` measures them). Both tests are currently skipped, but the ceilings still describe the direction: do not grow a function past 300 lines. |
| `TestNoForgeScriptsTracked` | `engine/cards/boundary_test.go` | A Forge script is tracked in git. Remove it. |
| `TestPublicBoundary`, `TestNoVerbatimForgeLines` | `internal/archtest/` | A generated card table, a binary, a private path or a fragment of a Forge script is tracked. Remove it. |
| `TestConformance*`, `TestManaBrew*`, `TestFoundPairs` | `testkit/conformance/` | A client wire says something different between two sessions of one game. `scripts/wire-matrix.sh commit` runs every [wire cell](../architecture/wires.md). |

## Before you open a PR

The checklist is on
[Contributing](index.md#commits-and-pull-requests):
gofmt, `go vet` on the touched packages, `go run ./cmd/gentypes -check` if
protocol types moved, focused tests with the corpus present, `make sim`
(20/20 `replay OK`), and `TestHeads`.
