# Testing

Day to day you run **focused** tests and learn to read the ratchet and golden
tests that guard the engine. The full suite runs in CI, once per merge-queue
batch ([Contributing](index.md#commits-and-pull-requests)). Setup and
worktrees are on [Contributing](index.md); the reasons behind each guard are on
[Invariants](../architecture/invariants.md).

## Focused runs

Run tests through the make targets. Each caps memory and CPU, checks that the
corpus is present, and ends with one verdict line
`test-<mode>: PASS|FAIL|ENV|EMPTY|BUDGET|BUSY <sha> <pkgs> <seconds>`
(`scripts/test-targets.sh` documents the modes).

```sh
make test-one PKG=./engine/rules RUN='^TestHeads$/^4seats$'   # debugging: one package, named tests
make test-targeted    # each iteration: the diff's packages and every package whose tests import them
make test-smart       # once, before you open the PR (`make precheck` is an alias)
```

- **Never raw `go test ./...`** (or `make test`) as an inner loop, and **never
  `-count=1`**: it defeats Go's test cache. Go re-runs a test whose inputs
  changed. A `(cached)` pass is a pass. The one input Go cannot see is the
  corpus, so `make fetch-cards` clears the test cache when the pin moves.
- **`ENV` is not green.** Tests load cards through
  `testutil.CorpusRegistry(t)` (`testkit/testutil/decks.go`), which calls
  `t.Skip` when `.cards/` is missing. A raw run of a package that prints `ok`
  in about 2 ms ran almost nothing; the make targets report that as `ENV`. In
  a git worktree, link the main checkout's `.cards/` into it.
- Load the corpus only through `testutil.CorpusRegistry`, never
  `cards.LoadRegistry`: the shared registry opens once per test binary, while a
  fresh one costs hundreds of MB.

## The per-test budget

Every test must fit **4 GB RSS, 4 vCPU and 1 minute of wall time**. The make
targets cap a run at that budget and report a timeout or kill as `BUDGET`.
`cmd/testbudget` (`go test -exec "testbudget exec"`, then `testbudget check`)
names every test over 60 s and every test binary over 4 GiB peak RSS, except
those in the shrink-only files `testkit/testutil/testdata/wall_exceptions.txt`
and `rss_exceptions.txt`. Fix an offender by splitting it into chunk tests that
share fixtures. Never cut its coverage, never add a row.

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

An oracle scenario is a JSON file stating what a card does by its printed text and the CR.
gorge's runner (`engine/rules/oracle_run.go`) plays it in a real game through player actions only and
asserts observable state. The corpora (`scenarios/oracle/`, `scenarios/xmageport/`) live in
[gorge-compliance](https://github.com/adams-shaun/gorge-compliance), whose `audit/` drivers
(`TestOracleAudit`, `TestXMagePortScenarios`) play them against a pinned gorge; run one card there
with `make oracle-audit ORACLE_RUN=<Card_Name>`.

gorge keeps the runner and the scenario files its own card tests load
(`engine/rules/testdata/oracle/<family>/`). `TestOracleRunnerContract`
(`engine/rules/oracle_contract_test.go`) plays them through the exported API gorge-compliance uses,
so a runner change that would break it fails here; `TestOracleScenarioShapeCheck` checks the schema.

gorge-compliance holds two per-card ratchets beside the scenarios: `<family>/known-divergent/<slug>.json`
(scenarios known to fail; a listed one that now passes is stale, an unlisted failure fails) and
`<family>/known-unconsumed/<slug>.json` (a scripted answer no decision used; rows are only deleted).

## Ratchets and goldens: what a failure means

A **ratchet** is a test whose expected value is a committed table, and which
fails in *both* directions. A new gap is a regression. A listed gap that your
change closed is a stale entry, and you delete it in the same commit. A
branch never adds an entry or raises a ceiling.

| Test | Where | When it fails |
|---|---|---|
| `TestHeads` | `engine/rules/heads_test.go`; goldens in `engine/rules/testdata/heads/<seats>.txt` | The deterministic bot games over the Legacy repo decks (`testutil.LegacyDeckNames`), at the seat counts in `AcceptanceSeatCounts()` (2, 4, 6, 8) and the 4-seat Commander game (`cmd4`), finished with a different hash-chain head. Your change altered what happens in those games. If intended, re-pin with `GORGE_UPDATE_HEADS=1 go test ./engine/rules/ -run '^TestHeads$'` and name the first diverging event and its cause in the commit message (`go doc ./cmd/headdiff` shows how to find it). If not intended, you have a bug, often nondeterminism. |
| `TestSetHeads` | `engine/rules/setheads_test.go`; games in `engine/rules/testdata/setheads/games.txt`, goldens in `engine/rules/testdata/setheads/heads/<game>.txt` | The same kind of golden as `TestHeads`, over a wider spread: small two-seat games whose decks are drawn from the Standard program's sets and chosen so they play keywords, ability APIs, trigger, static and replacement modes the Legacy games leave unplayed. The failure names the game, its decks and seed. If intended, re-pin with `GORGE_UPDATE_SETHEADS=1 go test ./engine/rules/ -run '^TestSetHeads$'`; find the first diverging event by running the test with `GORGE_DUMP_SETHEADS=<dir>` on the base and on your tree and diffing the two `<game>.events` files (one JSON event per line); name it and its cause in the commit message. `GORGE_REACH=1` on `TestSetHeadsReach` logs what both families play; `GORGE_GENERATE_SETHEADS=1` on `TestSetHeadsGenerate` re-chooses the games by greedy cover. |
| `TestRepoDeckGamesReplayExactly` | `engine/rules/acceptance_test.go` | A game replayed from its log diverged from the live run: something mutated state outside `events.Apply`, or something nondeterministic reached an event. |
| `TestEveryRepoDeckIsFullySupported` | `engine/rules/acceptance_test.go`; table in `engine/rules/testdata/known-unsupported/` (one JSON file per card) | A card in the repo decks (`testkit/testutil/decks/*.json`) needs a primitive this build does not register (a regression: fix it), or a listed card is now fully supported (delete its file). |
| `TestEveryRepoDeckParamsAreRead` | `engine/rules/paramcensus_test.go`; table in `engine/rules/testdata/paramcensus/` | The same, for script parameters no code reads and cost tokens no code models. |
| `TestValueHeadRegistryMatchesEvaluator` | `engine/rules/value_heads_test.go` | A `count:` value head was added to the evaluator but not to `modelledValueHeads` in `engine/effects/value_heads.go`, or the reverse. |
| `internal/archtest` (`TestDependencyOrderHolds`, `TestTimeIsImportedOnlyByTheHost`, `TestRootModuleHasNoRequires`, the `layering_*` tests, …) | `internal/archtest/` | You added a forbidden import edge (for example `effects` → `rules`, `view` → `rules`), imported `time` outside the host tier and its allowlist, or added a `require` to `go.mod`. Restructure the code; do not edit the allowlists. |
| `TestClassificationIsComplete` | `cmd/pkgmap/main_test.go` | A package has no tier entry in `docs/site/architecture/packages.tiers.json`. Also run by the `pre-commit` hook (`go run ./cmd/pkgmap -check`). |
| `TestKnownApproximationsOnlyShrinks` | `testkit/testutil/agentsdoc_test.go` | The register in `docs/agents/known-approximations/` grew. It only shrinks. |
| `internal/codeshape` (`TestCodeShapeOnlyShrinks`, `TestLongFunctionsOnlyShrink`, `TestComplexFunctionsOnlyShrink`) | `internal/codeshape/ratchet_test.go`, `funcshape_test.go`; per-function ceilings in `internal/codeshape/testdata/func_ceilings.tsv` | A shape metric grew: interface width, `*Engine` surface or string-keyed dispatch in `engine/rules` and `engine/effects` (`go run ./cmd/codeshape -table` measures them), or a function in `engine/rules`, `engine/effects` or `dirac/` (kgen included) grew in lines or cyclomatic complexity past its row, or a new one crossed 300 lines or complexity 50. Extract the new logic into a helper or a registered handler; never raise a ceiling. Shrinking only logs; `CODESHAPE_LOWER=1 go test -run '^TestFuncCeilingsLowerOnly$' ./internal/codeshape` locks it in. |
| `TestNoForgeScriptsTracked` | `engine/cards/boundary_test.go` | A Forge script is tracked in git. Remove it. |
| `TestPublicBoundary`, `TestNoVerbatimForgeLines` | `internal/archtest/` | A generated card table, a binary, a private path or a fragment of a Forge script is tracked. Remove it. |
| `TestSupportDocIsCurrent` | `dirac/kgen/`; page `dirac/docs/support.md` | A change made dirac's counts fall: a list's lowered, supported or 4-seat count dropped, or its refused count rose past the cards refused by name. A rise only logs, since queued PRs each carry a correct page that no longer matches their union; `DIRAC_SUPPORT_STRICT=1` (the nightly run) compares it byte for byte. `DIRAC_UPDATE_SUPPORT=1` rewrites the page; the docs build reads it ([how](../support/regenerating.md#dirac-refreshing-the-support-page)). |
| `scripts/dirac-gate.sh <base>` | `dirac/oracle`, `./dirac` | A [supported card](../architecture/dirac/index.md#what-it-supports) plays differently on the two engines, or dirac grew or allocates ([the invariant](../architecture/invariants.md#a-card-dirac-supports-plays-identically-to-gorge)). |
| `TestWireMatrixPageMatchesScript` | `testkit/conformance/` | A cell in `scripts/wire-matrix.sh` and [Wires](../architecture/wires.md) disagree. Change both. |
| `TestConformance*`, `TestManaBrew*`, `TestFoundPairs` | `testkit/conformance/` | A client wire says something different between two sessions of one game. `scripts/wire-matrix.sh commit` runs every [wire cell](../architecture/wires.md). |

## Before you open a PR

Run the [checklist](index.md#commits-and-pull-requests).
