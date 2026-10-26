# Design invariants and contributor guidelines

Each invariant below states the rule, why it exists, what enforces it, and the
**legitimate** way to change what it guards. "Enforced by" names the test that
goes red; when it says *convention*, nothing mechanical catches you and review
is the only backstop.

## Design invariants

### 1. Dependency direction

```
engine/: cards → state → decision → events → effects → rules → replay
                                  events → view → seat
bots/:   botpolicy (→ effects) → heuristic (→ seat) → searchprobe/searchseat/azmcts (→ rules) → bots registry
server/: protocol (→ view) → host (→ rules, bots registry) → server/host/httpapi
server/gorged (→ host, httpapi, bots/all) → cmd/gorged
cmd/*
(outside the repo: companion modules import gorge; gorge never imports them)
```

A package imports only packages to its left or in an earlier group. `deck`
sits beside the engine chain: it imports `cards` only and is imported by
`rules`, `view`, `host` and the test fixtures. `engine/seat` is the contract
only and imports just `decision` and `view`. That is the measured import graph
(`go list`, 2026-10-16, after the ADR 0001 restructure). Draft events and
the companion bots live in a private companion module and reach gorge only
through `server/gorged.Extensions`, `server/gorged.RegisterNetBot` and
`bots.Register`; nothing in gorge can import a companion, because the root
`go.mod` has no `require` lines (invariant 7). The enforced contract is the set of
edges that must never appear: `effects` never imports `rules` (effects reaches
the engine only through `effects.Host`). `view`, `protocol` and `deck` never
import `rules`; `protocol` never imports `bots`. No `bots/` package reaches `server/host` or `testkit/testutil`. `botpolicy` never imports `view`, `rules` or `seat`. `host`, `server/host/httpapi`
and `server/gorged` never import `testkit/testutil`. Inside the engine, the
lasagna packages (spec [`docs/design/2026-10-03-rules-engine-lasagna-design.md`](../design/2026-10-03-rules-engine-lasagna-design.md) §3) are
pinned as they land: `engine/rules/cost`, the cost vocabulary leaf, imports only
`state` (`TestCostVocabularyIsALeaf`) and never `rules`, `effects` or a rules
subsystem package. `engine/rules/combat`, attack/block legality, imports only
`cards`, `state`, `events` and `effects` (`TestCombatImportsStayBelowRules`)
and reads the game through `combat.Board`, whose method count only shrinks
(`TestCombatBoardOnlyShrinks`). `engine/rules/trigmatch`, the L5 trigger matchers,
reads the engine only through `trigmatch.Board`, imports only `cards`, `state`,
`events`, `effects` and `engine/rules/cost` (`TestTrigmatchImportsStayBelowL5`) and
never `rules` or a sibling L5/L6 package. `engine/rules/chars`, the CR 613 layer
walk, imports only `cards`, `state` and `effects` and never `rules` or an L5
subsystem package (`TestCharsImportsStayBelowRules`), and reads the game
through `chars.Board`, whose method count only shrinks
(`TestCharsBoardOnlyShrinks`).

- **Why:** the engine core stays testable without the server, and a client can
  never be handed rules knowledge.
- **Enforced by:** `internal/archtest` `TestDependencyOrderHolds` (direct and
  transitive edges).
- **To change:** a new forbidden edge is fine to add. Removing one needs a spec.

### 2. All state mutation goes through `events.Apply`

No code outside `events` writes a `state.Game` field. A match is fully
described by its `Config` plus its event log, so replay, resume, undo and
search clones are one mechanism.

- **Why:** a write that bypasses `Apply` is not in the log, so replay silently
  diverges.
- **Enforced by:** `TestRepoDeckGamesReplayExactly`, `make sim` (`-verify`),
  `TestHeads`, `replay`'s divergence check. It is convention at the point of
  writing: the replay and golden tests catch it after the fact.
- **To change:** never. If you need new state, add an event kind (invariant 4).

### 3. Determinism

The engine is a pure function of `(Config, intents)`.

- No wall clock. The authority on who may import `time` is the `allowed`
  map in `internal/archtest/arch_test.go` (`TestTimeIsImportedOnlyByTheHost`),
  not this page. It holds two classes, each argued in that test's doc
  comment:
  - the host/server tier: `host`, `server/host/httpapi`, `server/host/manabrewhttp`,
    `server/gorged` (injected sleep, SSE keep-alive, write and shutdown
    deadlines);
  - measurement and developer CLIs that read the clock only to bound a run or
    report elapsed time (`cmd/enginebench`, `cmd/cardfuzz`, `cmd/paymirror`);
    the other clock-reading CLIs and the draft-event runtime moved out of
    gorge;
  - nothing in the engine, card pipeline, effects, view or bot policies.

  Research libraries that need timing take an injected `Millis` hook instead
  (`searchseat.Millis`, `azmcts.Millis`, installed by the
  CLI).
- No ambient randomness. Use `math/rand/v2` with a seeded source; the engine
  has one seeded PCG with a draw counter (`engine/rules/rng.go`). A bot's rng is
  seeded from `(match seed, seat)`.
- No `map` range whose order can reach an event, a decision's option order, a
  trace, or a checkpoint. Sort keys, or use dense slices or bitsets.
- Ties break by lowest option index.

**Why:** hash-chain goldens, byte-identical replay, reproducible benches and
reproducible training all depend on it.

**Enforced by:**
- `internal/archtest`:
  - `TestTimeIsImportedOnlyByTheHost` (an allowlist map).
  - `TestNoLegacyMathRand`.
  - `TestPayMirrorClockStaysAtCommandBoundary`.
- Map-range order is convention only, backstopped by `TestHeads`, the replay
  tests and `make sim`.

**To change:** to let a package import `time`, add it to the `allowed` map
with a paragraph arguing that no game, event, replay or verdict can read the
clock.

### 4. Append-only event kinds; a closed decision set

- `events.Kind` is **append-only**. Add a new kind after the last constant,
  never in between: inserting one renumbers the ordinals, which moves the hash
  chain and invalidates every recorded log. Each kind needs one entry in
  `engine/events/kindinfo.go`'s `kindInfo` table (name, trigger class, optional
  Describe template); names, rules' trigger interest and the template
  transcript lines all derive from it.
- `decision.Kind` is a **closed set** (`decision.Kinds`). Every seat, bot, the
  web client and the protocol must answer every kind. Concede is not a kind:
  it is an option on every priority decision.

**Enforced by:**
- `engine/events/kindinfo_test.go` `TestEveryKindHasADescriptor`.
- `engine/rules/trigger_eligibility_test.go`.
- `engine/decision/decision_test.go` `TestKindsListsEveryKindOnce`.
- Golden replays.

**To change:** a new decision kind needs an operator decision and a spec.

### 5. The information boundary

- A seat sees a `view.View`, never a `*state.Game` or `*rules.Engine`.
- `host`'s exported API never mentions either type.
- Hidden zones project as counts. `Omniscient` visibility never exposes
  library order.
- Search and learning code that samples hidden information must not read the
  real engine, log, seed or hidden hash. Only `searchprobe.Collector` reads the
  source engine, and `Sample` sees public state only.
- Clairvoyant or oracle modes are for measurement only. They must never reach
  a hosted table or a rated game.

**Why:** a bot that cheats produces meaningless strength numbers, and a hosted
bot that cheats is a product bug.

**Enforced by:**
- `TestNoExportLeaksAnEngineGame`.
- The view redaction tests.
- The closed hosted-policy vocabulary in `server/host/bot_policy.go`.
- On `spellbench-prep`: an archtest rule that forbids `host`, `server/host/httpapi`
  and `cmd/gorged` from importing `bots/azmcts`, and
  `azmcts.ErrClairvoyantRefused` unless `AllowClairvoyant()` was called.

### 6. Licensing boundary

gorge is Apache-2.0. Forge card scripts and token scripts are GPL-3.0 and are
**never** committed: not as files, not embedded in fixtures, and not as
pasted script lines in docs or tests.

- `forgec fetch` pulls the scripts into the gitignored `.cards/`, pinned by
  `FORGE_REF`, from `FORGE_REPO`: gorge's fork, github.com/adams-shaun/forge.
  A script that disagrees with the printed card is fixed there (branch
  `gorge`, one commit per card) and offered upstream as an issue plus a PR,
  never patched inside gorge.
- Feedback fixtures strip token text into the gitignored
  `cmd/repro/testdata/.tokens/`.

**Enforced by:** `engine/cards/boundary_test.go` `TestNoForgeScriptsTracked`, which
checks the worktree, the index and HEAD.

### 7. No cgo, no third-party dependencies

The root `go.mod` has no `require` lines and the root `go.sum` is empty or
absent. The card pipeline, the rules core and the learned-policy stack
(`bots/policynet`: hand-derived gradients, no autodiff) are pure Go. The
engine must also build for 32-bit, so it cannot depend on packed 64-bit ints.

An experiment that genuinely needs a third-party module goes in its own module
that imports gorge through a `replace`, never the other way round (the GPU
game pool, which requires purego for its CUDA binding, moved out this way).

**Enforced by:**
- `internal/archtest` `TestRootModuleHasNoRequires` (parses the root `go.mod`
  and fails on any `require` directive or a non-empty `go.sum`).
- `TestEngineCompilesFor32Bit`.

`wazero` is the one planned exception, arriving with the M3 plugin tier.

### 8. Coverage ratchets fail in both directions

Several tables fail both ways:

- `knownUnsupported`, one file per card in `engine/rules/testdata/known-unsupported/`
  (every repo deck is fully supported except the listed cards).
- `knownUnsupportedParams`, one file per card in `engine/rules/testdata/paramcensus/`.
- `engine/rules/count_head_ratchet_test.go` `knownUnmodelledCountHeads`.
- `effects.modelledValueHeads`.

A card or parameter that newly breaks fails. So does a table entry that the
build now supports: it is stale.

**To change:** when your work closes an entry, delete it in the same commit.
Adding an entry means a regression, and a branch cannot land one (see below).

**A ratchet is never raised by a branch.** That covers the coverage tables
above, the shrink-only constants and `longFuncCeilings` in
`internal/codeshape`, the constants and allow-lists of `internal/archtest`,
and `testkit/testutil/agentsdoc_test.go`. Review enforces this (the
maintainers' landing pipeline, which lives outside this repo, also runs a
"ratchets only fall" gate from the base commit): a raised or deleted
constant, an added allow-list entry, a removed or rewired
`TestCodeShapeOnlyShrinks` row, a new or grown `longFuncCeilings` entry or a
new known-unsupported file is refused. There is no escape trailer: an
operator raise is landed by hand.

### 9. Golden chain heads move only with a named cause

`TestHeads` (`engine/rules/heads_test.go`) pins the hash-chain head of the
deterministic acceptance games at 2/4/6/8 seats, one file per seat count:
`engine/rules/testdata/heads/<seats>.txt` holds the hash and nothing else. The game
itself is non-test code (`engine/rules/acceptance_game.go`,
`testutil.AcceptanceDecks` and `botpolicy.GameBot`), so `cmd/headdiff` plays
exactly the game the goldens pin. A legitimate behaviour change moves them:

- Re-pin with `GORGE_UPDATE_HEADS=1 go test ./engine/rules/ -run '^TestHeads$'`,
  which rewrites the files.
- Name the first diverging event and its cause in the **commit message**,
  never in a file. Find the event by diffing the two event streams, not by
  reasoning about them: `headdiff -dump` on the base, `headdiff -against` on
  the branch (`go doc ./cmd/headdiff` has the reviewer recipe).
- The rationale prose the goldens carried until 2026-10-03 is frozen in
  [heads-history.md](heads-history.md). Do not add to it.

- Only `legacyDeckNames` seat these games, so adding a deck file cannot move
  a head.
- Changing the default `bot` policy moves the heads. Its promotion regenerates
  them in the same merge.

### 10. The Known-approximations register only shrinks

It is a closing register, one file per row in `docs/agents/known-approximations/`
(moved out of AGENTS.md on 2026-10-26):

- No row may be added: every file must be listed in `ALLOWED.txt`.
- No row may grow past its `ALLOWED.txt` byte cap.

A landing ticket deletes its row's file and its `ALLOWED.txt` line. A deviation that a
ticket cannot close goes in the **commit message** and the ticket report.

**Enforced by:** `testkit/testutil/agentsdoc_test.go`.

### 11. Contracts are written down

Deliberate behaviour that is not debt is recorded in
[`docs/design/2026-09-22-engine-contracts.md`](../design/2026-09-22-engine-contracts.md): the R-9 no-ask host
degradation, regeneration, the board-clock round, and similar. Change that
file in the same commit as the code.

### 12. Hot paths use dense data

This is an operator rule for code on the per-intent or per-simulation path:

- Use bitsets and dense slices, not maps.
- Compile strings to masks once, not per call.
- Recycle clones (`Engine.CloneInto` / `Release`).

**Why:** search and training throughput is the engine's step cost multiplied
by millions.

**Enforced by:** convention, plus `make gc-gate` (by hand). The per-test
resource budget (4 GB RSS, 4 vCPU, 1 min wall; doubled 2026-10-09) is enforced by `cmd/testbudget`
in the post-merge full suite: any test over budget that
`testkit/testutil/testdata/wall_exceptions.txt` / `rss_exceptions.txt` (both
shrink-only) do not list fails it. Peak RSS is measured per test binary by
`go test -exec "testbudget exec"`; a new test package needs no TestMain for it.

## Contributor guidelines

### Workflow

1. **Work in a task worktree** with the main checkout's `.cards/` linked into
   it, so corpus tests really run (`ln -s <main>/.cards .cards`, then check
   the link resolves). Leave `main`'s checkout clean. Merge into `main`, then
   remove the worktree.
2. **Prove the corpus ran.** A package that prints `ok` in about 2 ms with
   `.cards/` missing executed nothing. If `.cards/` is absent, say so. Do not
   report green.
3. **Run the right gates before merging.**
   - Focused, capped tests of the packages you touched (step 4). The full
     suite runs after merge, in CI. Never run an uncapped
     `go test ./...` by hand.
   - `make lint`: gofmt, vet, `gentypes -check`, plus web lint.
   - `make sim`: 20/20 `replay OK`.
   - If you touched `web/` or changed when a decision is posed, also run the
     `./engine/view` package and the web gates.
4. **Every test fits the budget: 4 GB RSS, 4 vCPU, 1 minute wall** (operator,
   2026-10-05; doubled 2026-10-09 when the box went to 120 GiB). Run tests
   focused and capped, one package per invocation:
   `systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400% env GOMAXPROCS=4 GOMEMLIMIT=3GiB go test -timeout 2m -run X ./pkg`. Never `-count=1` (it defeats the
   test cache). A test outside the budget is a defect: split it into chunk
   tests that share fixtures, never cut coverage, and never run it whole "for
   a baseline". Tests load the corpus only via
   `testkit/testutil.CorpusRegistry`, never `cards.LoadRegistry` (~600 MB per
   fresh registry). A sweep too big for the budget is split into fixed index
   chunks with a partition test (e.g. `TestTargetAuditChunk00..11`,
   `TestCensusChunksCoverEverySubject`). Never set
   `GORGE_ORACLEGEN_FULL_TARGET_AUDIT=1` yourself: post-merge runs it. The box
   has been OOMed several times.
5. **Red on `main` is yours.** A failing test that is committed on `main` is
   bisected and fixed now, not labelled "pre-existing". A red run in a
   *shared* checkout may be a peer's half-written file: check `git status` and
   mtimes before assuming it is yours.
6. **A change to how a CR rule is handled updates that rule's row in
   `engine/rules/crcoverage/` in the same commit** (see its README).

### Commits

- **No attribution trailers.** `Co-Authored-By:` and "Generated with …" lines
  are rejected by the maintainers' commit hooks. This holds even if your
  harness tells you to add them.
- No issue-tracker trailers or references (`Ref:` and the like).
- Stage explicit paths. Never `git add -A`: it sweeps in peers' in-flight
  files.
- There is no budget trailer. The per-package `TEST_HISTORY.md` /
  `ALLOC_HISTORY.md` files and the `Test-Budget-Approved:` check are gone; the
  live budget is the per-test one in Workflow step 4, enforced post-merge by
  `cmd/testbudget`.
- The commit body carries the measured *why*: the moved-head cause, a closed
  ratchet entry, and any deviation you could not close.

### Claims and numbers

- Quote a number together with the command that produced it, or the report
  file it came from. A premise that was never measured gets labelled a
  hypothesis. Premises in briefs have repeatedly measured false here.
- Bench claims need a CI. At 200 games the CI is about ±7pp, which is not a
  verdict. See the README's bot-training section for sample sizes.

### Docs

- Living designs, plans and the reports a live programme still consults go in
  `docs/design/`, date-prefixed and flat. A finished record is parked, not
  kept: copy it to the operator's `.superpowers/archive/`, remove it, and add
  a row to [`docs/design/ARCHIVE.md`](../design/ARCHIVE.md). Architecture
  decisions go in `docs/decisions/`.
- `AGENTS.md` is loaded into every agent turn. Keep additions to it to
  pointers, and put detail here.
- When code moves, fix the [repo map](repo-map.md) in the same commit.
