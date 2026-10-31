# AGENTS.md — gorge

Pure-Go Magic rules engine. Card behaviour is compiled from Forge card scripts
into an IR; the engine implements the primitives that IR references. See
[`docs/site/architecture/index.md`](docs/site/architecture/index.md).

## Repository layout

[ADR 0001](docs/decisions/0001-repo-layout.md) and [ADR 0002](docs/decisions/0002-public-engines-and-draft.md) group the tree:

- `engine/`: the rules core (`cards state decision events effects rules view
  replay eventdiff deck`, `game` and its `gorgegame`, the seam a host runs an
  engine through, and `seat`, the `Seat` contract only). Imports nothing outside.
- `server/`: `gorged` (the importable server; `cmd/gorged` is its thin main), `host` (+`httpapi`, `manabrewhttp`), `protocol`, `manabrew`, `divergence`.
- `bots/`: `botpolicy`, `heuristic` (the default bot), the registry, hosted bots and their inference runtime.
- `cmd/`: product and developer tools. `internal/`: ratchets, test support, generators.
- `draft/`: the limited stack (`limited`, `event`, `eventhost`, `gorgedevents`, `draftlab`); `cmd/gorged` links it.
- `scripts/`: contributor and CI scripts. `docs/design/`: living designs and historical notes; parked records are indexed in `docs/design/ARCHIVE.md`.
- The compliance oracles and the oracle-scenario corpora live in the public
  [gorge-compliance](https://github.com/adams-shaun/gorge-compliance), which pins
  gorge and plays the corpora through `engine/rules`' exported oracle runner.
- Machine learning and private experiments live in a private companion module,
  which imports gorge at a pinned commit; gorge never imports it, and nothing in
  gorge needs it to build or test.

## Agent guide

The detail lives in [`docs/agents/`](docs/agents/README.md), so it stays out of
every turn's context:

- [repo-map.md](docs/agents/repo-map.md): what lives where, and where each kind
  of change goes.
- [invariants.md](docs/agents/invariants.md): the design invariants, what
  enforces each one, the legitimate way to change it, and the contributor
  workflow.
- [do-not.md](docs/agents/do-not.md): mistakes that have actually happened here.
- [runtime-behavior.md](docs/agents/runtime-behavior.md): read it when changing
  `effects.TriggerContext`, cast-provenance matching, host observer hooks, or
  seat-authorised HTTP/session handling.
- [repro.md](docs/agents/repro.md): turning a player's bug report into a failing
  test (`go run ./cmd/repro`).
- [known-approximations/](docs/agents/known-approximations/): the engine-side
  stand-ins, a register that only shrinks (`TestKnownApproximationsOnlyShrinks`).

Bot and policy work starts at [`docs/site/bots/`](docs/site/bots/index.md).

## Hard rules

- **Never commit Forge card scripts.** They are GPL-3.0; gorge is Apache-2.0.
  `forgec fetch` pulls the card corpus and token scripts into `.cards/`,
  pinned to a commit SHA (`FORGE_REF` in the Makefile), which is gitignored.
  `engine/cards/boundary_test.go` fails the build if any are tracked.
- **Never commit kgen's generated card tables, or anything private.** Tables are mechanical translations of GPL-3.0 scripts and live in a gitignored directory. `internal/archtest`'s `TestPublicBoundary` fails on them, on private paths, and on any import of the private module.
- **No cgo, no third-party deps** in the card pipeline and rules core.
- **All state mutation goes through `events.Apply`.** If you are writing to a
  `state.Game` field outside `events`, you are introducing a replay bug.
  `events.Kind` is append-only: every kind is appended after all earlier kinds,
  so ordinals, the hash chain and golden replays stay untouched.
- **No nondeterminism.** No wall clock, no ambient randomness, no `map` range
  where iteration order can reach an event.
- **The engine never imports anything from an application built on it.**

## Build / run / test

```sh
make fetch-cards          # one-time; ~140 MB, pinned commit from adams-shaun/forge (our fork of Card-Forge/forge; FORGE_REPO)
make compile-cards        # parse into the IR cache
make report               # card coverage against implemented primitives
make sim                  # build mtgsim and play 20 verified 4-seat games
make test lint
make conformance          # the focused CR 601/733 conformance audit
```

**Test budget:** every test fits 4 GB RSS, 4 vCPU and 1 minute wall; one outside
it is a defect to split, never to trim. Agents run tests focused and capped,
never `go test ./...` or `-count=1`:
`systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400% env GOMAXPROCS=4 GOMEMLIMIT=3GiB go test -timeout 2m -run X ./pkg`.
The full suite runs after merge, in CI.
Rules and rationale: [invariants.md](docs/agents/invariants.md#workflow).

Acceptance commands:

```sh
go test ./engine/rules/ -run 'TestEveryRepoDeck|TestRepoDecks|TestRepoDeckGames' -v
make sim
```

## Where the numbers live

Coverage and golden figures change with every merge, so none are written here.

- The [docs site](https://adams-shaun.github.io/gorge/) publishes the live,
  regenerated numbers.
- `TestEveryRepoDeckIsFullySupported` (`knownUnsupported`, one file per card in
  `engine/rules/testdata/known-unsupported/`) names exactly the repo-deck cards
  the build does not fully support, and its log line is the live count. It fails
  in both directions: on a card that newly breaks, and on a stale entry.
- `TestEveryRepoDeckParamsAreRead` and `TestEveryStandardSetParamsAreRead` are the
  parameter ratchets, over `engine/rules/testdata/paramcensus/` and
  `testdata/paramcensus-sets/`.
- `engine/rules/testdata/heads/<seats>.txt` hold the golden chain heads
  (`TestHeads`, invariant 9).
- `make report` prints corpus coverage at the `FORGE_REF` pin.

Re-measure before quoting.

## Running a gorged server while you work

Put persistence under `/tmp/gorge-<something-unique>`, never the repo-root
default `gorged-data`: several servers sharing one persistence directory
silently corrupt each other. Pick a free port with `-addr`; `:8080` is the
default, and `scripts/smoke.sh` takes 8090-8099.

**Stopping a server: never `pkill -f` or a bare `pgrep -f`.** The pattern is
matched against every process's `/proc/<pid>/cmdline` including your own shell's,
and it has killed a session here. Find the server by its listening socket
(`ss -lptn`), confirm the pid, then signal that pid.

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
