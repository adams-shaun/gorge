# Contributing to gorge

This guide is for a Go developer who does not yet know Magic engine internals or
Forge. To fix a card that plays wrong, read this page and follow
[Your first card fix](docs/site/contributing/first-card-fix.md). Unfamiliar terms
(IR, primitive, SVar, chain head, ratchet) are in the
[glossary](docs/site/contributing/glossary.md). The fuller guide, including
worktrees and what CI runs, is [docs/site/contributing/index.md](docs/site/contributing/index.md).

**Open pull requests against `develop`,** not `main`. The maintainers merge
`develop` into `main`.

## Set up

You need Go (the version on the `go` line of `go.mod`; Go 1.21 or later downloads it
itself under the default `GOTOOLCHAIN=auto`), `git`, `make` and `bash`. Node 22.12 or
newer is needed only for `web/`. There is no cgo and no C toolchain.

```sh
make fetch-cards      # Forge card and token scripts into .cards/ (about 140 MB)
make compile-cards    # parse them into the IR cache, .cards/ir-<fingerprint>.gob.gz
make report           # optional: corpus coverage against the implemented primitives
git config core.hooksPath .githooks   # optional: pkgmap check on commit
```

Every coverage number and golden test is relative to the commit `FORGE_REF` pins in
the `Makefile`.

**The GPL boundary.** gorge is Apache-2.0; Forge's scripts are GPL-3.0. They live only
in the gitignored `.cards/` and are never committed, not even a fragment in a test, a
fixture or a doc. Tests name cards and read the corpus at run time.
`engine/cards/boundary_test.go` fails the build if a script is tracked, and
`internal/archtest`'s `TestPublicBoundary` and `TestNoVerbatimForgeLines` guard the
rest. If a card is wrong because its *script* is wrong, the fix goes to the fork
[adams-shaun/forge](https://github.com/adams-shaun/forge) and upstream.

## The rules a change most often breaks

The full list, with what enforces each, is
[Invariants](docs/site/architecture/invariants.md).

1. **All state mutation goes through `events.Apply`.** Engine code emits an event
   (`e.emit(...)` in `engine/rules`) and never writes a `state.Game` field. Test
   fixtures may set up state directly.
2. **No nondeterminism.** No wall clock, no randomness but the game's seeded RNG, no
   `map` range whose order can reach an event.
3. **No cgo and no third-party dependencies.** The root `go.mod` has no `require` lines.
4. **`events.Kind` is append-only.** A new kind goes after the last constant in
   `engine/events/event.go`.
5. **Never commit Forge scripts** or anything generated from them.
6. **Imports run one way.** `engine/` packages import nothing outside `engine/`, and
   `effects` never imports `rules`.

## Run tests focused

```sh
go test -run 'TestOmniscienceInHandGrantsNothing' ./engine/rules/
go test -run '^TestHeads$' ./engine/rules/
```

- One package and one `-run` pattern at a time. `go test ./...` is the full suite,
  which CI runs.
- Never add `-count=1`. Go already re-runs a test whose inputs changed.
- Corpus tests **skip** without `.cards/`, so a package can print `ok` in 2 ms having
  run almost nothing. If a run looks fast, re-run with `-v` and look for `SKIP`. A git
  worktree has no `.cards/` until you link it: `ln -s <main checkout>/.cards .cards`.
- Every test must fit 4 GB RSS, 4 vCPU and one minute. A test over budget gets split
  into chunks, never trimmed.

What each ratchet and golden failure means is in
[Testing](docs/site/contributing/testing.md).

## Pull request checklist

- [ ] The commit message says why: the Comprehensive Rules (CR) number, what was
      wrong, what you measured. If you counted how many corpus cards share the bug's
      shape, give the numbers. No attribution trailers (`Co-Authored-By:`) and no
      issue-tracker references.
- [ ] A test that fails without the change, named for the behaviour, in the package
      you changed.
- [ ] `gofmt -l` prints nothing for your files; `go vet` on the packages you touched.
- [ ] If you changed `server/protocol`: `make gentypes`, and
      `go test ./server/host/httpapi -run TestOpenAPI` (`-update` regenerates
      `docs/site/api/openapi.json`'s generated parts).
- [ ] Focused `go test` of the packages you touched, with `.cards/` present.
- [ ] `make sim`: 20/20 `replay OK`.
- [ ] `go test -run '^TestHeads$' ./engine/rules/`. If a chain head moved, your change
      altered what those games do. If that is intended, re-pin with
      `GORGE_UPDATE_HEADS=1 go test ./engine/rules/ -run '^TestHeads$'` and name the
      first diverging event in the commit message (`go doc ./cmd/headdiff` shows how to
      find it). An unexplained moved head is a bug.
- [ ] If your change makes a listed card supported, delete its ratchet file in the same
      commit.

The first outside contribution, `git show 786510b3f`, is a good example of the shape.

## Where does my change go?

[Repository layout](docs/site/contributing/repo-layout.md#where-does-my-change-go) has
the table, and `go run ./cmd/pkgmap` prints every package with its purpose and tier.
Behaviour that looks like a bug but is deliberate is recorded in
[`docs/design/2026-09-22-engine-contracts.md`](docs/design/2026-09-22-engine-contracts.md)
and [`docs/agents/known-approximations/`](docs/agents/known-approximations/); read both
before filing.
