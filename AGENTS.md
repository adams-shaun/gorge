# AGENTS.md — gorge

Pure-Go Magic rules engine. Card behaviour is compiled from Forge card scripts
into an IR; the engine implements the primitives that IR references. See
[`docs/site/architecture/index.md`](docs/site/architecture/index.md).

## Repository layout

[ADR 0001](docs/decisions/0001-repo-layout.md) and [ADR 0002](docs/decisions/0002-public-engines-and-draft.md) group the tree.
[`docs/agents/repo-map.md`](docs/agents/repo-map.md) lists every package.

- `engine/`: the rules core (`cards state decision events effects rules view
  replay eventdiff deck`, `game` and its `gorgegame`, the seam a host runs an
  engine through, and `seat`, the `Seat` contract only). Imports nothing outside.
- `dirac/`: the second engine, in lockstep with `engine/`, built for machine
  learning (kernel, `kop`, `kgen`, `oracle`, `adapter`, `lab`, `manifest` and more).
  The core imports nothing outside `dirac/`; nothing in `engine/` imports it.
  See [`dirac/README.md`](dirac/README.md).
- `server/`: `gorged` (importable server; `cmd/gorged` is its thin main), `host`
  (+`httpapi`, `manabrewhttp`, `sbv2http`), `protocol`, `manabrew`, `divergence`,
  `matchrecord`, `spellbench`.
- `bots/`: `botpolicy`, `heuristic` (the default bot), the registry, hosted bots
  and their inference runtime.
- `draft/`: the limited stack; `cmd/gorged` links it.
- `cmd/`: product and developer tools. `internal/`: ratchets and generators.
  `testkit/`: public test support. `scripts/`: contributor and CI scripts.
  `docs/design/`: living designs (parked records: `docs/design/ARCHIVE.md`).
- Outside this repo: the public
  [gorge-compliance](https://github.com/adams-shaun/gorge-compliance) (oracles and
  scenario corpora; it pins gorge and plays them through `engine/rules`' exported
  oracle runner), and a private companion module (machine learning, private
  experiments; it imports gorge at a pinned commit). gorge never imports either.

## Agent guide

Detail lives in [`docs/agents/`](docs/agents/README.md), out of every turn's context:

- [repo-map.md](docs/agents/repo-map.md): what lives where, where each kind of change goes.
- [invariants.md](docs/agents/invariants.md): the design invariants, what enforces
  each, and the contributor workflow.
- [do-not.md](docs/agents/do-not.md): mistakes that have actually happened here.
- [runtime-behavior.md](docs/agents/runtime-behavior.md): read it when changing
  `effects.TriggerContext`, cast-provenance matching, host observer hooks, or
  seat-authorised HTTP/session handling.
- [repro.md](docs/agents/repro.md): a player's bug report into a failing test.
- [known-approximations/](docs/agents/known-approximations/): engine-side
  stand-ins; a register that only shrinks (`TestKnownApproximationsOnlyShrinks`).

Bot and policy work starts at [`docs/site/bots/`](docs/site/bots/index.md).

## Hard rules

- **Never commit Forge card scripts.** They are GPL-3.0; gorge is Apache-2.0.
  `forgec fetch` pulls them into the gitignored `.cards/`, pinned by `FORGE_REF`
  in the Makefile. `engine/cards/boundary_test.go` fails the build if any are tracked.
- **Never commit kgen's generated card tables, or anything private.** Tables are
  mechanical translations of GPL-3.0 scripts and live in a gitignored directory.
  `internal/archtest`'s `TestPublicBoundary` fails on them, on private paths, and
  on any import of the private module.
- **No cgo, no third-party deps** in the card pipeline and rules core.
- **A card dirac supports plays identically to gorge, decision by decision.** gorge
  leads. A gorge change that alters behaviour on a supported card fixes dirac in the
  same change, or makes kgen refuse the card by name (`kgen -refuse`). There is no
  list of tolerated divergences; capacity stalls are declared limits and always
  counted. `scripts/dirac-gate.sh` holds this.
- **`dirac.State` holds no references** beyond its views of one pointer-free buffer
  and its fork pool, and the transition loop allocates nothing
  (`TestColdArraysHoldNoPointers`, `TestStateSize`, the gate's allocation and
  instruction steps).
- **All state mutation goes through `events.Apply`.** A write to a `state.Game`
  field outside `events` is a replay bug. `events.Kind` is append-only: append
  after all earlier kinds, so ordinals, the hash chain and golden replays stay untouched.
- **No nondeterminism.** No wall clock, no ambient randomness, no `map` range
  where iteration order can reach an event.
- **The engine never imports anything from an application built on it.**

## Build / run / test

```sh
make fetch-cards          # one-time; the FORGE_REF pin, from FORGE_REPO (our fork of Card-Forge/forge)
make compile-cards        # parse into the IR cache
make report               # card coverage against implemented primitives
make sim                  # build mtgsim and play 20 verified 4-seat games
make lint                 # gofmt, vet, gentypes -check, and web lint (lint-web runs npm)
make conformance          # the focused CR 601/733 conformance audit
make dirac-tables         # dirac's binary card tables, under .cards; needed before any dirac test plays a game
make fresh-clone-check    # dirac with no corpus: builds, tests skip, none fail
```

**Test budget:** every test fits 4 GB RSS, 4 vCPU and 1 minute wall; one outside
it is a defect to split, never to trim. Run tests through the make targets, never
a raw `go test`, `go test ./...` or `-count=1`. Each is capped, checks the corpus
(a skip for a missing `.cards` is `ENV`, never green) and ends with one line
`test-<mode>: PASS|FAIL|ENV|EMPTY|BUDGET|BUSY <sha> <pkgs> <seconds>`
(exit 0, 1, 3, 4, 5, 75):

```sh
make test-one PKG=./engine/rules RUN='^TestX$'   # debugging: one package, named tests
make test-targeted                               # each iteration: the diff's packages + all their test reverse deps
make test-smart                                  # once, before DONE: the precheck (affected packages + test groups)
```

A `(cached)` ok, or a verdict with `cached=K` in its `tests ran=` line, is a pass:
go re-runs a test by itself when its code, flags, env or input files change. Never
re-run it, add `-count=1` or `go clean -testcache` to "make sure", or call a fast
PASS suspicious. `-timeout`, `-v`, `-json` and `-failfast` are part of the cache
key, so the make targets keep them fixed.

`make precheck` is an alias for `test-smart`. The full suite runs in CI, never
locally. A pull request's own checks are `fast` (`scripts/ci-fast.sh`) and `lint`;
the Mergify merge queue then batches up to 10 PRs and runs the six test shards,
`examples`, `wire-matrix`, `dirac-slices` and `oracle-sentinels` once per batch
([`.mergify.yml`](.mergify.yml), [`ci.yml`](.github/workflows/ci.yml)). The whole
`dirac/oracle` runs on `main` about every 2 hours
([`oracle.yml`](.github/workflows/oracle.yml); a red run bisects itself into a draft revert PR), and every batch
runs the `oracle-sentinels` set (`dirac/oracle/testdata/sentinels.txt`).
Rules and rationale: [invariants.md](docs/agents/invariants.md#workflow).

Acceptance (each prints its verdict line):

```sh
make test-one PKG=./engine/rules RUN='^(TestEveryRepoDeckIsFullySupported|TestRepoDecksPlayAtEverySeatCount|TestRepoDeckGamesReplayExactly)$'
make sim
```

## Where the numbers live

None are written here; they change with every merge. Re-measure before quoting.

- The [docs site](https://adams-shaun.github.io/gorge/) publishes the live, regenerated numbers.
- `TestEveryRepoDeckIsFullySupported`: `engine/rules/testdata/known-unsupported/`
  (one file per repo-deck card the build does not fully support; the directory is
  absent while there are none); its log line is
  the live count. It fails both ways: a card that newly breaks, a stale entry.
- `TestEveryRepoDeckParamsAreRead`, `TestEveryStandardSetParamsAreRead`: the
  parameter ratchets over `engine/rules/testdata/paramcensus/` and `paramcensus-sets/`.
- `engine/rules/testdata/heads/<seats>.txt`: golden chain heads (`TestHeads`,
  [invariant 9](docs/agents/invariants.md#9-golden-chain-heads-move-only-with-a-named-cause)).
- `make report`: corpus coverage at the `FORGE_REF` pin.
- [`dirac/docs/support.md`](dirac/docs/support.md), generated by
  `TestSupportDocIsCurrent`: dirac's lowered, refused, unverified and supported cards per set.

## Running a gorged server while you work

Put persistence under `/tmp/gorge-<something-unique>`, never the repo-root default
`gorged-data`: servers sharing one directory silently corrupt each other. Pick a
free port with `-addr` (`:8080` is the default; `scripts/smoke.sh` takes 8090-8099).

**Stopping a server: never `pkill -f` or a bare `pgrep -f`.** The pattern matches
every process's `/proc/<pid>/cmdline`, your own shell's included, and has killed a
session here. Find the server by its listening socket (`ss -lptn`), confirm the
pid, then signal that pid.

## Working in a worktree

Make each change in its own git worktree and keep the main checkout clean. Several
sessions may share one checkout: never `git checkout`, `git switch`, `git reset`
or bare `git stash` in it, and stage explicit paths (never `git add -A`).

A worktree carries only tracked files. Link the corpus (`ln -s <main>/.cards
.cards`) and check it resolves (`ls .cards/cardsfolder`), or every
corpus-dependent test SKIPS through `testutil.CorpusRegistry` and the package
still prints `ok` in about 2ms. In a worktree with no `.cards`, stop and say so
rather than reporting a green suite.
