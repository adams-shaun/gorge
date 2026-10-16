# Contributing to gorge

gorge is a pure-Go Magic: The Gathering rules engine, licensed Apache-2.0.
Outside contributions are welcome. This page covers what a pull request needs
in order to land. The engine's design, its invariants and the internal
agent workflow are documented in [`docs/agents/`](docs/agents/README.md).
That material is not repeated here.

## Workflow

1. Fork `adams-shaun/gorge` and branch from `main`.
2. Open a pull request against `main`. Fill in the template.
3. A first-time contributor's workflow runs wait for the owner to approve
   them. This is GitHub's fork-PR policy, not a judgement on the PR.
4. `main` requires one approving review from a code owner (see
   [`.github/CODEOWNERS`](.github/CODEOWNERS)). The `ci-ok` and `contracts`
   checks must be green before the PR is merged.

Keep each PR to one change. A refactor and a fix are reviewed better apart.

## What CI runs, and how to reproduce it

`.github/workflows/ci.yml`:

- **lint**: `gofmt -l`, `go vet ./...` and `go run ./cmd/gentypes -check`.
- **test**: six shards of `go test`, plus `go test -run '^TestHeads$'
  ./rules` (the golden replay chain heads). Pull requests skip the
  experimental and benchmark packages. The nightly run takes every package.
- **ci-ok**: one aggregate check over the jobs above.

`.github/workflows/contracts.yml`:

- **changelog**: a PR that changes `contracts/registry.json` or
  `docs/contracts.md` must also change `CHANGELOG.md`.
- **apidiff**: the exported API of each stable package must stay compatible
  with the newest release tag. See [Contracts and stability](#contracts-and-stability).

### The card corpus

Most tests run against the Forge card corpus. The corpus is GPL-3.0 and is
never committed. It is fetched into the gitignored `.cards/`, pinned to the
commit named by `FORGE_REF` in the `Makefile`:

```sh
make fetch-cards compile-cards   # once, and again whenever FORGE_REF moves
```

Without `.cards/`, every corpus-dependent test calls `t.Skip`. The package
still prints `ok`, in a couple of milliseconds, having tested almost nothing.
**A run without the corpus is not a green run.** Do not report it as one.
CI refuses to test without the corpus at the pinned ref.

### Running tests locally

Run tests focused on the packages you touched, rather than the whole tree:

```sh
gofmt -l . && go vet ./... && go run ./cmd/gentypes -check   # Go half of make lint
go test -timeout 2m ./rules -run TestYourThing
go test -timeout 10m -run '^TestHeads$' ./rules              # if you touched the engine
bash scripts/ci_shard.sh 0 6                                 # list CI shard 0's packages
bash scripts/apidiff.sh                                      # the API-compat check
```

`make lint` also lints the web client, which needs `npm ci` in `web/`.

## Hard rules

Breaking any of these is a blocking review finding. Most are also enforced by
a test.

- **Never commit Forge card scripts**, or anything derived from their text.
  They are GPL-3.0 and gorge is Apache-2.0. `cards/boundary_test.go` fails the
  build if any are tracked.
- **No cgo and no third-party dependencies** in the engine. The root `go.mod`
  has no `require` lines (`TestRootModuleHasNoRequires`).
- **All game-state mutation goes through `events.Apply`.** Writing a
  `state.Game` field anywhere else breaks replay. `events.Kind` is
  append-only: new kinds go at the end.
- **No nondeterminism.** This means no wall clock, no ambient randomness, and
  no `map` iteration whose order can reach an event. Games replay byte for
  byte, and `rules/testdata/heads/` pins their chain heads. A PR that moves a
  head must explain why in its description.
- **Every test fits the per-test budget:** at most 1 minute wall, 4 GB RSS and
  4 CPUs. A test over budget is split into chunks. Its coverage is never cut.
- **The engine never imports the application** (mtgbld/mtgserve). The
  internal dependency direction is listed in
  [invariants.md](docs/agents/invariants.md).

## Contracts and stability

gorge is pre-1.0. Module versions are `v0.x.y` tags. For the full policy, see
[docs/releasing.md](docs/releasing.md).

- **Stable Go packages** are listed in
  [`.github/stable-packages.txt`](.github/stable-packages.txt). They are
  currently `decision` and `draftlab/extern/wire`. An incompatible change to
  one of them fails the `apidiff` check. If the break is intended, label the
  PR `breaking-change` and describe it under `### Breaking` in
  `CHANGELOG.md`. Every other package is unstable and may change without
  notice.
- **Wire and file contracts**, such as the JSON a client or a training
  pipeline reads, are listed in `contracts/registry.json` and described in
  [docs/contracts.md](docs/contracts.md). Changing a contract means updating
  its registry entry (bump its major version on a break, and refresh its
  golden hashes). It also needs a `CHANGELOG.md` entry under `### Contracts`.
- Any user-visible change gets a line under `## [Unreleased]` in
  [CHANGELOG.md](CHANGELOG.md).

## Commits and pull requests

- Write a short imperative subject with a scope prefix, for example
  `fix(rules): …`, `feat(view): …` or `ci: …`. The body explains *why*, and
  quotes any number together with the command that measured it.
- Do not add attribution trailers such as `Co-Authored-By:` or "Generated
  with …" lines, and do not add issue-tracker trailers. The hooks in
  `.githooks/` reject them. Enable the hooks with
  `git config core.hooksPath .githooks`.
- A rules fix cites the Comprehensive Rules section it implements (for
  example `CR 113.6`). It comes with a test that fails without the fix.
- Card-behaviour bugs are fixed for the whole mechanism, not for one card.
  [do-not.md](docs/agents/do-not.md) lists the mistakes that have actually
  happened here.
- A bug from a played game can usually be turned into a failing test with
  `go run ./cmd/repro -emit-test <pkg> <feedback-dir>` (see `AGENTS.md`).
- Rebasing onto `main` and merging `main` in are both fine. PRs land as merge
  commits.

## Licensing

By contributing, you agree that your contribution is licensed under the
Apache License 2.0, the repository's [LICENSE](LICENSE).
