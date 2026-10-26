# AGENTS.md — gorge

Pure-Go Magic rules engine.

## What this is

The rules engine that will replace `mtgplay`'s XMage bridge. Card behaviour is
compiled from Forge card scripts into an IR; the engine implements the
primitives that IR references. See
[`docs/site/architecture/index.md`](docs/site/architecture/index.md).

## Repository layout

[ADR 0001](docs/decisions/0001-repo-layout.md) (2026-10-16) groups the tree:

- `engine/`: the rules core (`cards state decision events effects rules view
  replay deck`, and `seat`, the `Seat` contract only). Imports nothing outside.
- `server/`: `gorged` (the importable server; `cmd/gorged` is its thin main), `host` (+`httpapi`, `manabrewhttp`), `protocol`, `manabrew`, `divergence`.
- `bots/`: `botpolicy`, `heuristic` (the default bot), the registry, hosted bots and their inference runtime.
- `cmd/`: product and developer tools. `internal/`: ratchets, test support, generators.
- Draft events and the maintainers' extensions live in a private companion
  module, which imports gorge at a pinned commit; gorge never imports it, and
  nothing in gorge needs it to build or test (ADR 0001 amendment, 2026-10-16).
- The compliance oracles and the oracle-scenario corpora (hand Oracle-text audit, XMage ports)
  live in the public gorge-compliance (`github.com/adams-shaun/gorge-compliance`), which pins
  gorge and plays the corpora through `engine/rules`' exported oracle runner (ADR 0001
  amendment 3). gorge keeps the runner and `oracle_contract_test.go`'s residual scenarios.
- `scripts/`: contributor and CI scripts. gorge carries no maintainer automation or
  demo-deploy machinery.
- `docs/design/`: living designs; parked history is indexed in `docs/design/ARCHIVE.md`.

## Agent guide

The detail behind this file lives in [`docs/agents/`](docs/agents/README.md),
so it stays out of every turn's context:

- [repo-map.md](docs/agents/repo-map.md): what lives where, and where each
  kind of change goes.
- [invariants.md](docs/agents/invariants.md): the design invariants, what
  enforces each one, the legitimate way to change it, and the contributor
  workflow.
- [do-not.md](docs/agents/do-not.md): mistakes that have actually happened
  here.

Bot and policy work starts at
[`docs/site/bots/`](docs/site/bots/index.md) (native Go, HTTP seat, SpellBench
v2, and the training and adoption ladder).

## Hard rules

- **Never commit Forge card scripts.** They are GPL-3.0; gorge is Apache-2.0.
  `forgec fetch` pulls the card corpus and token scripts into `.cards/`,
  pinned to a commit SHA (`FORGE_REF` in the Makefile), which is gitignored.
  `engine/cards/boundary_test.go` fails the build if any are tracked.
- **No cgo, no third-party deps** in the card pipeline and rules core.
  `wazero` arrives with the plugin tier in M3.
- **All state mutation goes through `events.Apply`.** If you are writing to a
  `state.Game` field outside `events`, you are introducing a replay bug.
  `events.Kind` is append-only, and every kind M2r and M2d added was appended
  after all earlier kinds so ordinals, the hash chain and golden replays stay
  untouched: M2r appended `CastInfo`, `Choose`, `TokenCreate`, `StackCopy`,
  `Attach` and `AbilityPush`; M2d appended `ModeChosen` for mid-resolution
  modal answers.
- **No nondeterminism.** No wall clock, no ambient randomness, no `map` range
  where iteration order can reach an event.
- **This engine never imports anything from the mtgbld/mtgserve application.**

## Build / run / test

```sh
make fetch-cards          # one-time; ~140 MB, pinned commit from adams-shaun/forge (our fork of Card-Forge/forge; FORGE_REPO)
make compile-cards        # parse into the IR cache
make report               # card coverage against implemented primitives
make sim                  # build mtgsim and play 20 verified 4-seat games
make test lint
make conformance          # the focused CR conformance audit
```

**Test budget (operator, 2026-10-05, doubled 2026-10-09 with the box's DRAM):**
every test fits 4 GB RSS, 4 vCPU and 1 minute wall; one outside it is a defect
to split, never to trim. Agents run tests focused and capped, never
`go test ./...` or `-count=1`:
`systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400% env GOMAXPROCS=4 GOMEMLIMIT=3GiB go test -timeout 2m -run X ./pkg`.
The full suite runs after merge, in CI.
Rules and rationale: [invariants.md](docs/agents/invariants.md#workflow).

`make conformance` is the focused CR 601/733 audit. I-2 (mandatory-target
feasibility), I-7 (targets before payment), and the CR 733.1 illegal-cast
reversal are fixed and asserted in the ordinary suite; the historical
`requireCR601Audit` guard and `GORGE_CR_CONFORMANCE=1` switch were removed. The
Makefile target remains explicit so reviewers can run the focused audit without
running the full suite.

## Reproduce a feedback report

A player-submitted bug report that named a table carries a replayable
snapshot (`match.json`, `log.json`, the seat's `view.json`, captured by the
server's feedback capture). Turning it into a failing test is one command:

```sh
go run ./cmd/repro <feedback-dir>              # replay, verify the head, print the board
                                             # at the report point (turn, step, life,
                                             # battlefield, stack, last ~20 log lines)
                                             # (-at 0 also works flags-after-dir:
                                             #  `repro <dir> -at 0` is parsed same as
                                             #  `repro -at 0 <dir>`)
go run ./cmd/repro -at N <feedback-dir>        # replay to intent N instead
                                             # (-list prints the intent timeline to find N)
go run ./cmd/repro -omniscient <feedback-dir>  # show every hand in the summary
go run ./cmd/repro -emit-test <pkg> <feedback-dir>
                                             # copy the snapshot into
                                             # <pkg>/testdata/feedback/<id>/ and write a
                                             # failing test skeleton (feedback.EngineAt) as
                                             # the target package's EXTERNAL test package
                                             # (package <name>_test: feedback imports the
                                             # engine tier, so an internal-package skeleton
                                             # would be an import cycle for an engine-side
                                             # target); replace the TODO with the assertion

A committed fixture's `match.json` does not embed token script text — Forge
scripts are GPL-3.0 and must never be committed — so `-emit-test` and the
fixture generator strip `match.json`'s `tokens`/`tokens_unread` and sync the
exact text into the gitignored `cmd/repro/testdata/.tokens/<id>/` keyed by
the fixture id. `feedback.Load` resolves token scripts from that sync
directory first (exact historical text) and falls back to the live corpus
(`.cards/` at the current `FORGE_REF`) when it is missing (a fresh clone), so
a fixture still replays without ever re-embedding the text into committed
files.

A capture also records the name-card universe MODE (`name_universe`) and the
exact sorted label list that match offered (`name_universe_names`), because
a universe-backed match poses `NameCard` asks the legacy path never poses and
a replay rebuilt without the mode refuses the recorded name intent. The
committed fixture keeps only the mode bit -- the list is ~24k entries -- and
`feedback.config` re-derives it from the live corpus, so a `FORGE_REF` move
can renumber a recorded name choice and shows up as the same `DIVERGED`.
```

Exit 0 is a verified replay: the rebuilt event stream matches the recording
event for event and the final head equals log.json's `head`. A non-zero exit
prints `DIVERGED` naming the first mismatching event — **a corpus change since
the report was filed is a real, expected cause** (the snapshot replays against
the corpus as it was at the report's pin), and so is an engine change; read the
first diverging event before assuming either. In a test, load the same
snapshot with `feedback.EngineAt(t, dir, n)` (`testkit/testutil/feedback`;
`n < 0` = every recorded intent) and assert at the engine it returns.

The committed fixture `cmd/repro/testdata/feedback/20260914T120000Z-fb01/` is
a real capture produced by `host.SnapshotForFeedback`; regenerate it with
`REPRO_REGEN_FIXTURE=1 go test ./cmd/repro -run TestGenerateCommittedFixture`
after a corpus pin bump (`FORGE_REF`), or the fixture's own gate reports
DIVERGED exactly as a stale report would.

## Status

**The coverage ratchet.** `knownUnsupported`, one file per card in
`engine/rules/testdata/known-unsupported/` (Ruling P12/D2-a), names exactly the repo-deck cards the build does not fully
support. `TestEveryRepoDeckIsFullySupported` asserts that the measured gap
equals the table in both directions (Ruling R-20). A card the build newly
cannot support fails, named together with its missing primitives. A table
entry the build now supports is stale and fails too. **The test's log line is
the live count.**

Measured 2026-10-25 on `develop` 87ed64d3b (`go test ./engine/rules/ -run
'TestEveryRepoDeckIsFullySupported|TestEveryRepoDeckParamsAreRead' -v`,
`make report`, `engine/rules/testdata/heads/*.txt`). The live, regenerated numbers
are on the docs site, https://adams-shaun.github.io/gorge/; prefer it over
this snapshot.

- **Ratchet: 2 of 1123 distinct cards unsupported.** The cards come from the
  30 deck files in `testkit/testutil/decks/*.json`: 15 60-card constructed
  decks and 15 100-card Commander decks. The two are:
  - Incinerate (`stat:CantRegenerate`)
  - Vines of Vastwood (`stat:CantTarget`)
- **Param census: 15 of 1123.** `TestEveryRepoDeckParamsAreRead`
  (`engine/rules/paramcensus_test.go`) is the companion ratchet over the same decks'
  parameters. Its per-card files in `engine/rules/testdata/paramcensus/` hold 15 cards that carry an
  unread param or an unmodelled cost token: 11 distinct param labels and 1
  cost label.
- **Golden games.** The 12 pinned Legacy decks (`legacyDeckNames`, which
  deliberately excludes the Commander decks) round-robin across 2/4/6/8 seats
  (`TestRepoDecksPlayAtEverySeatCount`). They replay byte-identically
  (`TestRepoDeckGamesReplayExactly`), and `TestHeads` pins their chain heads
  in `engine/rules/testdata/heads/<seats>.txt`. Those files are the authority. At
  this date:

  | seats | 2 | 4 | 6 | 8 |
  |---|---|---|---|---|
  | chain head | `9a22e5122a3d436a` | `554f63c3593a585f` | `44753f92b24b32e5` | `c2c73be4093774fe` |

- **`make sim`:** 20/20 `replay OK` over 20 verified 4-seat games
  (re-run for this snapshot, 2026-10-25).
- **Corpus coverage** at the pin `76fd7691ab45ab1b42cef19ad04b41bf364501f4`
  (`FORGE_REF`): `make report` prints `cards: 34074  playable: 32265 (94.7%)`
  and `tokens: 854`. Since
  fuzz-cov3 (2026-09-24) the gate also counts the `count:<head>` value heads
  that a card's SVars read (`effects.modelledValueHeads`, held to the
  evaluator by `TestValueHeadRegistryMatchesEvaluator`). `repl:Untap` is out
  of scope and remains unsupported.

These figures go stale with every merge. Re-measure them before quoting;
don't copy them.

A seat's clients must be able to answer every `decision.Kind`, and the set is
closed: the M1 kinds (`priority`, `target`, `attackers`, `blockers`,
`trigger_order`, `trigger_optional`), M2r's `choose` (an {X} value, delve
exiles, cost sacrifices, "as it enters" name/type/number, miracle-style
yes/no), the two M2d closures -- `mulligan`, the London keep/mulligan
and bottoming round `Config.Mulligans` runs between the deal and turn 1, and
`modes`, the modal pick and unless-pay ask a mid-resolution answer serves
(the `ModeChosen` event carries the answer into the log) -- and the four
kinds the later card work added: `commander_zone` (a commander's OWNER's
CR 903.9 command-zone choice), `replacement` (the CR 616.1
order choice over competing replacement effects) and `arrange` (the ordered
subset ask `Scry`/`Surveil`/`RearrangeTopOfLibrary` share, Ruling J0), and
`starting_player` (CR 103.1's second half: the toss winner's choice of who
takes the first turn, posed by `NewStartingPlayerChoice`/`AskStartingPlayer`
with the pregame rounds deferred until it resolves -- plain `New` is the
deterministic R-9 no-host fallback and never offers it).
Concede (M2d-3) is not a kind: it is a `concede` option on every priority
decision that emits the existing `PlayerLost` with Text "conceded". The
engine-side defaults that still stand in for a choice the engine cannot yet
ask are listed in [`docs/agents/known-approximations/`](docs/agents/known-approximations/).

Acceptance commands:

```sh
go test ./engine/rules/ -run 'TestEveryRepoDeck|TestRepoDecks|TestRepoDeckGames' -v
make sim
```

## Known approximations

A closing register of stand-ins, one file per row in
[`docs/agents/known-approximations/`](docs/agents/known-approximations/): a ticket that closes one deletes
its file and its `ALLOWED.txt` line; nothing is ever added (`TestKnownApproximationsOnlyShrinks`).

## Runtime behavior details

Load-bearing filter, cast-provenance, embedder-hook and seat-privacy details
are maintained in [docs/agents/runtime-behavior.md](docs/agents/runtime-behavior.md).
Read that page when changing `effects.TriggerContext`, cast-provenance
matching, host observer hooks, or seat-authorised HTTP/session handling.

## Running a gorged server while you work

On the maintainers' shared box several sessions run servers at once, and a
live demo (deployed by hand by the maintainers; a redeploy aborts every
in-flight vs-bot game) holds 8080-8081. So ports are
allocated, not first-come:

| range | who |
|---|---|
| 8080-8081 | the demo. **Never bind these**, and never run the demo deploy |
| 8082-8089 | the bot-policy workstream |
| 8090-8099 | task agents on the engine side — pick one of these |

Put persistence under `/tmp/gorge-<something-unique>`, never the repo-root
default `gorged-data`: several servers sharing one persistence directory
silently corrupt each other, and a resumed directory written by an older binary
comes back with the fields that binary lacked set to their zero values
(`Format`'s zero is `constructed`, a real value, so `-format` is ignored with
nothing in the output to say so).

**Stopping a server: never `pkill -f` or a bare `pgrep -f`.** The pattern is
matched against every process's `/proc/<pid>/cmdline` including your own shell's,
and it has killed a session here. Find the server by its listening socket
(`ss -lptn`), confirm the pid, then signal that pid. Note that `/proc/<pid>/comm`
is the BINARY's name -- if you built `gorged-after`, its `comm` is
`gorged-after`, not `gorged` -- so match on the port rather than the name.


## Working in a worktree

Make each change in its own git worktree and keep the main checkout clean;
several sessions may share one checkout, so never `git checkout`, `git
switch`, `git reset` or bare `git stash` in it, and stage explicit paths
(never `git add -A`). A worktree carries only tracked files: link the
`.cards` corpus into it (`ln -s <main>/.cards .cards`, then check the link
resolves, e.g. `ls .cards/cardsfolder`), or every
corpus-dependent test SKIPS through `testutil.CorpusRegistry` and the package
still prints `ok` in about 2ms. If you find yourself in a worktree with no
`.cards`, stop and say so rather than reporting a green suite.

