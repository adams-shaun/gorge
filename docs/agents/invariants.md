# Design invariants and contributor guidelines

Each invariant states the rule, what enforces it, and the **legitimate** way to
change it. "Enforced by" names the test that goes red; *convention* means
nothing mechanical catches you and review is the only backstop. The long form,
with the reasoning and the classic mistake for each rule, is the docs site's
[Invariants](../site/architecture/invariants.md) page. Other code cites these
numbers ("invariant 7"), so they stay stable.

## Design invariants

### 1. Dependency direction

```
engine/: cards → state → decision → events → effects → rules → replay
                                  events → view → seat
bots/:   botpolicy (→ effects) → heuristic (→ seat) → searchprobe/searchseat/azmcts (→ rules) → bots registry
server/: protocol (→ view) → host (→ rules, bots registry) → server/host/httpapi
server/gorged (→ host, httpapi, bots/all) → cmd/gorged
draft/:  limited (→ engine/cards, engine/deck) → event → eventhost (→ bots, host) → gorgedevents (→ server/gorged) → cmd/gorged; draftlab (→ event, testkit/bench)
dirac/:  imports nothing outside dirac/ (its oracle, adapter and shadow read gorge's types)
(outside the repo: companion modules import gorge; gorge never imports them)
```

A package imports only packages to its left or in an earlier group. The edges
that must never appear are the contract:

- `effects` never imports `rules`; it reaches the engine only through `effects.Host`.
- `view`, `protocol` and `deck` never import `rules`; `protocol` never imports `bots`.
- No `bots/` package reaches `server/host` or `testkit/testutil`. `botpolicy`
  never imports `view`, `rules` or `seat`.
- `host`, `server/host/httpapi` and `server/gorged` never import `testkit/testutil`.
- `engine/` imports nothing outside `engine/` and never the draft stack or
  `dirac/`. Only `cmd/gorged`, `cmd/draftsim`, `cmd/labdemo` and `cmd/extagent`
  may depend on `draft/`; `cmd/gorged` hands it to the server through
  `server/gorged.Extensions`.
- A companion module's bots reach gorge only through `server/gorged.RegisterNetBot`
  and `bots.Register`. Nothing in gorge can import a companion: the root `go.mod`
  has no `require` lines (invariant 7).
- Inside `engine/rules`, each layered sub-package (`cost`, `chars`, `combat`,
  `trigmatch`, `scriptfacts`, `pay`, `resolve`) has a pinned import set and, where
  it reads the game, a narrow `Board` interface whose method count only shrinks
  (design: [`2026-10-03-rules-engine-layering-design.md`](../design/2026-10-03-rules-engine-layering-design.md) §3).

**Enforced by:** `internal/archtest` `TestDependencyOrderHolds` (direct and
transitive edges), `TestDraftTreeStaysAbove`, `TestEngineTierImportsNeitherDraftNorDirac`,
and per sub-package `TestCostVocabularyIsALeaf`, `TestCombatImportsStayBelowRules`,
`TestTrigmatchImportsStayBelowL5`, `TestCharsImportsStayBelowRules`,
`TestScriptFactsImportsStayBelowRules`, `TestResolveImportsStayBelowRules`,
`TestPayImportsStayBelowRules`, and the `*BoardOnlyShrinks` tests.

**To change:** a new forbidden edge is fine to add. Removing one needs a spec.

### 2. All state mutation goes through `events.Apply`

No code outside `events` writes a `state.Game` field. A match is its `Config`
plus its event log, so replay, resume, undo and search clones are one mechanism;
a write that bypasses `Apply` is not in the log and replay silently diverges.

**Enforced by:** after the fact, not at the line you write:
`TestRepoDeckGamesReplayExactly`, `make sim` (`mtgsim -verify`), `TestHeads`, and
`replay`'s divergence check.

**To change:** never. New state means a new event kind (invariant 4).

### 3. Determinism

The engine is a pure function of `(Config, intents)`.

- **No wall clock.** The `allowed` map in `internal/archtest/arch_test.go`
  (`TestTimeIsImportedOnlyByTheHost`) is the authority on who may import `time`,
  not this page; its doc comment argues each class: the host/server tier, the
  draft-event runtime, measurement CLIs and dirac's lockstep tooling. Nothing in
  the engine, card pipeline, effects, view or bot policies. Libraries that need
  timing take an injected `Millis` hook (`searchseat.Millis`, `azmcts.Millis`).
- **No ambient randomness.** Use `math/rand/v2` with a seeded source; the engine
  has one seeded PCG with a draw counter (`engine/rules/rng.go`). A bot's rng is
  seeded from `(match seed, seat)`.
- **No `map` range** whose order can reach an event, a decision's option order,
  a trace or a checkpoint. Sort keys, or use dense slices or bitsets.
- **Ties break by lowest option index.**

Hash-chain goldens, byte-identical replay and reproducible benches and training
depend on it.

**Enforced by:** `TestTimeIsImportedOnlyByTheHost`, `TestNoLegacyMathRand`,
`TestPayMirrorClockStaysAtCommandBoundary`. Map-range order is convention,
backstopped by `TestHeads`, the replay tests and `make sim`.

**To change:** to let a package import `time`, add it to the `allowed` map with a
paragraph arguing that no game, event, replay or verdict can read the clock.

### 4. Append-only event kinds; a closed decision set

- `events.Kind` is **append-only**. Add a kind after the last constant, never in
  between: inserting one renumbers the ordinals, which moves the hash chain and
  invalidates every recorded log. Each kind needs one entry in
  `engine/events/kindinfo.go`'s `kindInfo` table (name, trigger class, optional
  Describe template); names, rules' trigger interest and transcript lines derive from it.
- `decision.Kind` is a **closed set** (`decision.Kinds`). Every seat, bot, the web
  client and the protocol must answer every kind. Concede is not a kind: it is an
  option on every priority decision (it emits `PlayerLost` with Text "conceded").

**Enforced by:** `engine/events/kindinfo_test.go` `TestEveryKindHasADescriptor`,
`engine/rules/trigger_eligibility_test.go`, `engine/decision/decision_test.go`
`TestKindsListsEveryKindOnce`, golden replays.

| Kind | What it asks |
|---|---|
| `priority`, `target`, `attackers`, `blockers`, `trigger_order`, `trigger_optional` | The core turn decisions. |
| `choose` | An {X} value, delve exiles, cost sacrifices, an "as it enters" name, type or number, a miracle-style yes/no. |
| `mulligan` | The London keep/mulligan and bottoming round `Config.Mulligans` runs between the deal and turn 1. |
| `modes` | A modal pick or unless-pay ask mid-resolution; `ModeChosen` carries the answer into the log. |
| `commander_zone` | A commander's owner's CR 903.9 command-zone choice. |
| `replacement` | The CR 616.1 order choice over competing replacement effects. |
| `arrange` | The ordered-subset ask `Scry`, `Surveil` and `RearrangeTopOfLibrary` share. |
| `starting_player` | CR 103.1: the toss winner's choice of who goes first, posed by `NewStartingPlayerChoice`/`AskStartingPlayer`. Plain `New` is the deterministic fallback and never offers it. |

Engine-side defaults that stand in for a choice the engine cannot yet ask are in
[`known-approximations/`](known-approximations/).

**To change:** a new decision kind needs a maintainer decision and a spec.

### 5. The information boundary

- A seat sees a `view.View`, never a `*state.Game` or `*rules.Engine`; `host`'s
  exported API mentions neither.
- Hidden zones project as counts. `Omniscient` visibility never exposes library order.
- Search and learning code that samples hidden information must not read the
  real engine, log, seed or hidden hash. Only `searchprobe.Collector` reads the
  source engine, and `Sample` sees public state only.
- Clairvoyant or oracle modes are for measurement only; never a hosted table or a rated game.

**Enforced by:** `TestNoExportLeaksAnEngineGame`, the view redaction tests, the
closed hosted-policy vocabulary in `server/host/bot_policy.go`, and
`azmcts.NewSeat` refusing a clairvoyant world when `SeatConfig.Source` is nil
(`azredeal.Hosted()` leaves it nil).

### 6. Licensing boundary

gorge is Apache-2.0. Forge card and token scripts are GPL-3.0 and are **never**
committed: not as files, fixtures, or pasted script lines in docs or tests.

- `forgec fetch` pulls them into the gitignored `.cards/`, pinned by `FORGE_REF`,
  from `FORGE_REPO` (gorge's fork, github.com/adams-shaun/forge). A script that
  disagrees with the printed card is fixed there (branch `gorge`, one commit per
  card) and offered upstream, never patched inside gorge.
- Feedback fixtures strip token text into the gitignored `cmd/repro/testdata/.tokens/`.

**Enforced by:** `engine/cards/boundary_test.go` `TestNoForgeScriptsTracked`
(worktree, index and HEAD); in `internal/archtest`, `TestNoVerbatimForgeLines`
and `TestPublicBoundary` (generated tables, private paths, the private module).

### 7. No cgo, no third-party dependencies

The root `go.mod` has no `require` lines and `go.sum` is empty or absent. The
card pipeline, the rules core and the learned-policy stack (`bots/policynet`)
are pure Go. The engine must also build for 32-bit, so it cannot depend on
packed 64-bit ints. An experiment that needs a third-party module goes in its
own module that imports gorge, never the other way round.

**Enforced by:** `internal/archtest` `TestRootModuleHasNoRequires`,
`TestEngineCompilesFor32Bit`.

### 8. Coverage ratchets fail in both directions

| Table | Test |
|---|---|
| `engine/rules/testdata/known-unsupported/` (one file per repo-deck card) | `TestEveryRepoDeckIsFullySupported` |
| `engine/rules/testdata/paramcensus/` | `TestEveryRepoDeckParamsAreRead` |
| `knownUnmodelledCountHeads` | `TestEveryRepoDeckCountHeadResolves` (`engine/rules/count_head_ratchet_test.go`) |
| `effects.modelledValueHeads` | `TestValueHeadRegistryMatchesEvaluator` |

A card or parameter that newly breaks fails; so does a table entry the build now
supports (it is stale). When your work closes an entry, delete it in the same
commit. Adding an entry is a regression, and a branch cannot land one.

**Code-shape ratchets fail on growth only.** `internal/codeshape` holds two kinds,
run by every precheck (`scripts/test_targets.py` always selects the package):

- `TestCodeShapeOnlyShrinks`: the constants in `ratchet_test.go` (interface
  widths, `*Engine` surface, string-keyed dispatch and parameter leaks in
  `engine/rules` and `engine/effects`).
- `TestLongFunctionsOnlyShrink`, `TestComplexFunctionsOnlyShrink`: every non-test,
  non-generated function in `engine/rules`, `engine/effects` and `dirac/` (kgen
  included) over 300 lines or cyclomatic complexity 50 has a row in
  `internal/codeshape/testdata/func_ceilings.tsv`. A listed function fails when
  it grows past its row; an unlisted one fails when it crosses a threshold.

Each check has a slack (`internal/codeshape/slack.go`): a count may exceed its
ceiling by max(2, 1%) (`Slack`; a zero ceiling stays exact), a function its row
or threshold by max(5, 3%) (`FuncSlack`). Growth within the slack passes with a
logged WARNING; never raise a ceiling to absorb it.

Shrinks and deletions only log, so concurrent refactors never fail on each other.
`CODESHAPE_LOWER=1 go test -run '^TestFuncCeilingsLowerOnly$' ./internal/codeshape`
lowers the rows to the measurement and drops stale ones; it never raises or adds
one. A renamed or moved function takes its row under the new key in the same
commit at no higher value.

A pull request that lowers a row or a `ratchet_test.go` constant fails the
lint job with "lowering is post-merge only": `cmd/checkratchet` compares the
branch with its merge base. Lowering is post-merge only, so a lowering cannot
race a branch that grows the same function into one queue batch. The daily
`ratchet-lower` workflow runs the generator on main and opens a bot PR (rebuilt
over main when it goes red or conflicting).

**A ratchet is never raised by a branch.** That covers the coverage tables above,
the constants and `func_ceilings.tsv` in `internal/codeshape`, the constants and
allow-lists of `internal/archtest`, and `testkit/testutil/agentsdoc_test.go`.
Review refuses a raised or deleted constant, an added allow-list entry, a removed
or rewired `TestCodeShapeOnlyShrinks` row, a new or raised `func_ceilings.tsv`
row, or a new known-unsupported file. A raise is a maintainer's decision.

### 9. Golden chain heads move only with a named cause

`TestHeads` (`engine/rules/heads_test.go`) pins the hash-chain head of the
deterministic acceptance games at 2/4/6/8 seats and the 4-seat Commander game
(`cmd4`), one file per game: `engine/rules/testdata/heads/<seats>.txt` (`cmd4.txt`
for Commander), the hash and nothing else. The game is non-test code
(`engine/rules/acceptance_game.go`, `testutil.AcceptanceDecks`,
`botpolicy.GameBot`), so `cmd/headdiff` plays exactly the game the goldens pin.
A legitimate behaviour change moves them:

- Re-pin with `GORGE_UPDATE_HEADS=1 go test ./engine/rules/ -run '^TestHeads$'`,
  which rewrites the files.
- Name the first diverging event and its cause in the **commit message**, never
  in a file. Find it by diffing event streams, not by reasoning: `headdiff -dump`
  on the base, `headdiff -against` on the branch (`go doc ./cmd/headdiff`).
- Only `legacyDeckNames` (`testkit/testutil/decks.go`) seat these games, so
  adding a deck file cannot move a head.
- `TestSetHeads` (`engine/rules/setheads_test.go`) is the wider family under the
  same rule: pinned two-seat games over decks from the Standard program's sets.
  `testdata/setheads/games.txt` holds each game's seed and deck lists (a corpus
  bump never changes what a golden plays); `testdata/setheads/heads/<game>.txt`
  holds one head per game. Re-pin with `GORGE_UPDATE_SETHEADS=1`; find the first
  diverging event by diffing the `GORGE_DUMP_SETHEADS=<dir>` files of base and
  branch. `TestSetHeadsGenerate` (`GORGE_GENERATE_SETHEADS=1`) chooses the game
  list; never edit it by hand.
- Changing the default `bot` policy moves the heads; its promotion regenerates
  them in the same merge.

### 10. The Known-approximations register only shrinks

One file per row in `docs/agents/known-approximations/`. Every file must be
listed in `ALLOWED.txt`, and none may grow past its byte cap. A change that
closes a row deletes its file and its `ALLOWED.txt` line. A deviation a change
cannot close goes in the **commit message**. A reviewer treats any added or grown
row as a MAJOR finding.

**Enforced by:** `testkit/testutil/agentsdoc_test.go` `TestKnownApproximationsOnlyShrinks`.

### 11. Contracts are written down

Deliberate behaviour that is not debt (the no-ask host degradation, regeneration,
the board-clock round, and similar) is recorded in
[`docs/design/2026-09-22-engine-contracts.md`](../design/2026-09-22-engine-contracts.md).
Change that file in the same commit as the code.

### 12. Hot paths use dense data

On the per-intent or per-simulation path: bitsets and dense slices, not maps;
compile strings to masks once, not per call; recycle clones
(`Engine.CloneInto` / `Release`). Search and training throughput is the
engine's step cost times millions.

**Enforced by:** convention, plus `make gc-gate` (by hand).

### 13. A card dirac supports plays identically to gorge

gorge leads. A gorge change that alters behaviour on a supported card fixes
dirac in the same change, or refuses the card by name (`kgen -refuse`). There is
no list of tolerated divergences; a capacity stall is a declared limit and is
always counted. `dirac.State` holds no references beyond its views of one
pointer-free buffer and its fork pool, and the transition loop allocates nothing.

**Enforced by:** `scripts/dirac-gate.sh <base>` (lockstep on the committed FDN
and BLB slices, the size bar, allocations and instructions per transition),
`make dirac-slices` on every merge-queue batch, `TestColdArraysHoldNoPointers`,
`TestStateSize`. The whole of `dirac/oracle` runs about every 2 hours on `main`
([`oracle.yml`](../../.github/workflows/oracle.yml)).

## Contributor guidelines

### Workflow

1. **Work in a task worktree** with the main checkout's `.cards/` linked into it
   (`ln -s <main>/.cards .cards`, then check the link resolves), so corpus tests
   really run. Leave `main`'s checkout clean. Open a pull request against `main`,
   then remove the worktree.
2. **Prove the corpus ran.** A package that prints `ok` in about 2 ms with
   `.cards/` missing executed nothing. If `.cards/` is absent, say so; do not report green.
3. **Run the right gates before opening the PR.**
   - `make test-targeted` on each iteration: the packages your diff touches plus
     every package whose tests import them; capped, one verdict line.
   - `make test-smart` once on the committed tree (`make precheck` is an alias):
     fail-fast, the affected packages (`./engine/rules` whole when the change
     reaches it) plus the pin, census, layout and lint groups. By default it runs
     the narrow set; `GORGE_TEST_SMART_GATES=1` adds the affected-package and
     dirac gates side by side, `0` turns them off (header of
     `scripts/test-targets.sh`). Quote its verdict line.
   - `make lint`: gofmt, vet, `gentypes -check`, plus web lint (`lint-web`, which
     runs npm).
   - `make sim`: 20/20 `replay OK`.
   - If you touched `web/` or changed when a decision is posed, also run the
     `./engine/view` package and `make test-web`.
4. **How a PR lands.** Every non-draft PR to `main` is queued by Mergify once its
   own checks, `fast` (`scripts/ci-fast.sh`: the diff's packages and their test
   importers) and `lint`, pass. Mergify batches up to 10 queued PRs on a
   `mergify/merge-queue/*` branch, where `ci.yml` runs the full suite once per
   batch: the six test shards (`scripts/ci_shard.sh`), `examples`, `wire-matrix`
   `dirac-slices` and `oracle-sentinels`. The batch merges when `ci-ok` is green; a red batch is
   split. A fix for a red `main` goes on a `hotfix/*` branch (or takes the
   `hotfix` label) and jumps the queue. The whole `dirac/oracle` runs on `main`
   about every 2 hours (`oracle.yml`); `ci.yml` also has a nightly run. A test that
   reddens main's oracle is appended to `dirac/oracle/testdata/sentinels.txt` (run in every batch) in its fix or
   revert PR. The old
   landing train (`land` and `develop` branches, promote PRs) is retired. Never
   run an uncapped `go test ./...` or the whole oracle by hand.
5. **Every test fits the budget: 4 GB RSS, 4 vCPU, 1 minute wall.** Run tests
   through the make targets (`scripts/test-targets.sh`), never a raw `go test`.
   To debug one test: `make test-one PKG=./pkg RUN='^TestX$'`, with optional
   `SKIP=`, `V=1`, `TIMEOUT=` (default and maximum 5m), `SHORT=1`, `TAGS=`,
   `UPDATE=1`, and `TESTENV='KGEN_REPIN=1'` for the allow-listed regeneration
   variables. Each target caps itself (test-one: 4G, 400% CPU, `GOMAXPROCS=4`,
   `GOMEMLIMIT=3GiB`), never passes `-count` (the cache stays on), and ends with
   one line `test-<mode>: PASS|FAIL|ENV|EMPTY|BUDGET|BUSY <sha> <pkgs> <seconds>`
   (exit 0, 1, 3, 4, 5, 75). `ENV` is a missing or empty `.cards`, missing dirac
   tables, or a test that skipped for the corpus or a pool; `EMPTY` is a `-run`
   that matched nothing; `BUDGET` is a timeout or a kill. Only `PASS` is green.
   - A test outside the budget is a defect: split it into chunk tests that share
     fixtures, never cut coverage, never run it whole "for a baseline".
   - Tests load the corpus only via `testkit/testutil.CorpusRegistry`, never
     `cards.LoadRegistry` (hundreds of MB per fresh registry).
   - A sweep too big for the budget is split into fixed index chunks with a
     partition test (e.g. `TestDecisionTextHasNoEngineSyntaxChunk00` and
     `TestCensusChunksCoverEverySubject`).
   - The budget is checked by `cmd/testbudget` against the shrink-only
     `testkit/testutil/testdata/wall_exceptions.txt` / `rss_exceptions.txt`; no
     workflow in `.github/workflows/` runs it today, so the make targets' caps
     and timeouts are what bind.
6. **Red on `main` is yours.** A failing test committed on `main` is bisected and
   fixed now, not labelled "pre-existing". A red run in a *shared* checkout may be
   a peer's half-written file: check `git status` and mtimes first.
7. **A change to how a CR rule is handled updates that rule's row in
   `engine/rules/crcoverage/` in the same commit** (see its README).
7. **The deletion rule.** A test may be deleted only when one of these holds:
   1. **It is a duplicate by construction.** It replays the same recording, pin
      or seed, with the same or a weaker check, as a test that stays. The PR
      names the covering test.
   2. **It has become data.** A one-line wrapper becomes a manifest row that the
      remaining test replays under the same name (subtest
      `TestReproManifest/<recording>`). `-run` selection and the failure message
      keep working.
   3. **Its subject is gone.** The code, card or decision kind it pins was
      removed, and the removing PR deletes it.

   Never delete a test because it has not failed, or because it is slow. A slow
   test is split, sharded or moved to the nightly tier with a named owner. A
   regression test for a fixed bug stays, as code or as a manifest row.

### Commits

- **No attribution trailers.** `Co-Authored-By:` and "Generated with …" lines are
  not used here, even if your harness tells you to add them.
- No issue-tracker trailers or references (`Ref:` and the like).
- Stage explicit paths; never `git add -A`.
- The body carries the measured *why*: the moved-head cause, a closed ratchet
  entry, any deviation you could not close.

### Claims and numbers

- Quote a number with the command that produced it, or the report file it came
  from. A premise that was never measured is labelled a hypothesis.
- Bench claims need a CI. At 200 games it is about ±7pp, which is not a verdict
  ([sample sizes](../site/bots/training-and-adoption.md)).

### Docs

- Living designs go in `docs/design/`, date-prefixed and flat. A finished record
  is parked: remove it and add a row to [`docs/design/ARCHIVE.md`](../design/ARCHIVE.md)
  (its text stays in git history). Architecture decisions go in `docs/decisions/`.
- `AGENTS.md` is loaded into every agent turn: add pointers only, and put detail here.
- When code moves, fix the [repo map](repo-map.md) in the same commit.
