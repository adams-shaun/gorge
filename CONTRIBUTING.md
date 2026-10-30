# Contributing to gorge

Thanks for helping. This guide is for a Go developer who does not yet know
Magic engine internals or Forge. If you want to fix a card that plays wrong,
read this page and then follow
[the first-card-fix walkthrough](docs/site/contributing/first-card-fix.md).
Unfamiliar terms (IR, primitive, `api:`, SVar, chain head, ratchet) are
defined in the [glossary](docs/site/contributing/glossary.md).

The full contributor's guide is published at
https://adams-shaun.github.io/gorge/contributing/ (source:
[docs/site/contributing/index.md](docs/site/contributing/index.md)). **Open
PRs against `develop`**, not `main`: work lands on `develop`, and `develop ->
main` is a maintainer-merged PR (`make pr PR_BASE=develop`).

## Prerequisites

- **Go**: the version in `go.mod` (`go 1.25.8` at the time of writing). With
  an older Go installed, `GOTOOLCHAIN=go1.25.8` fetches the right one.
- **git**, **make** and **bash**.
- **Node** only if you work on `web/` (`make web`, `make test-web`; Node 22 or
  newer, see `web/README.md`).

No cgo and no C toolchain is needed.

## Get the card corpus

```sh
make fetch-cards      # Forge card + token scripts into .cards/ (~240 MB)
make compile-cards    # parse them into the IR cache (.cards/ir-<fingerprint>.gob.gz)
make report           # optional: corpus coverage against the implemented primitives
```

`make fetch-cards` builds `bin/forgec` and checks out the scripts at the
commit pinned by `FORGE_REF` in the `Makefile`, from gorge's fork
[adams-shaun/forge](https://github.com/adams-shaun/forge) (`FORGE_REPO`).
Every coverage number and every golden test is relative to that pin.

**The GPL boundary.** gorge is Apache-2.0; Forge's scripts are GPL-3.0. They
live only in the gitignored `.cards/` and are **never committed**, not even
a fragment pasted into a test, a fixture or a doc. Tests name cards and read
the corpus at run time. `engine/cards/boundary_test.go` fails the build if a
script is ever tracked. If a card is wrong because its *script* is wrong,
the fix goes to the fork and upstream, not here (see the walkthrough).
`internal/archtest`'s `TestNoVerbatimForgeLines` enforces the no-fragment rule
for `dirac/` and `draft/`, and holds a shrinking baseline elsewhere. kgen's
generated tables are never committed either.

## The six rules that matter

These are the design invariants a change most often breaks. The full list,
with what enforces each one, is in
[docs/agents/invariants.md](docs/agents/invariants.md).

1. **All state mutation goes through `events.Apply`.** Engine code never
   writes a `state.Game` field directly; it emits an event (`e.emit(...)` in
   `engine/rules`). A write that bypasses the log silently breaks replay.
   (Test fixtures may set up state directly; engine code may not.)
2. **No nondeterminism.** No wall clock, no ambient randomness (only the
   game's seeded RNG), and no `map` range whose iteration order can reach an
   event. The engine is a pure function of `(Config, intents)`.
3. **No cgo and no third-party dependencies** in the card pipeline and the
   rules core. The root `go.mod` has no `require` lines.
4. **`events.Kind` is append-only.** A new event kind goes after the last
   constant in `engine/events/event.go`, so ordinals, the hash chain and
   every golden replay stay valid.
5. **Never commit Forge card scripts or kgen's generated tables** (the GPL boundary above).
6. **The engine never imports the server or the application.** `engine/`
   packages do not import `server/`, `bots/` or `cmd/`; within the
   engine, `effects` never imports `rules`. `internal/archtest` enforces the
   forbidden edges (`TestDependencyOrderHolds`).

## Running tests

Run tests **focused**: one package, one `-run` pattern.

```sh
go test -run 'TestOmniscienceInHandGrantsNothing' ./engine/rules/
go test -run 'TestOracleRunnerContract' ./engine/rules/
```

- **Do not run `go test ./...`** as your inner loop. The suite is large and
  heavy; the full run happens after merge.
- **Never use `-count=1`.** It defeats the test cache and buys nothing:
  Go already re-runs a test whose inputs changed.
- **The `.cards`-missing trap.** Corpus-dependent tests call
  `testutil.CorpusRegistry`, which **skips** when `.cards/` is absent. The
  package then prints `ok` in about 2 ms having tested almost nothing. If
  a run looks suspiciously fast, re-run with `-v` and look for `SKIP`. A
  git worktree does not carry `.cards/`: link the main checkout's into it
  (`ln -s <main checkout>/.cards .cards`).
- **Per-test budget.** Every single test must fit 4 GB RSS, 4 vCPU and one
  minute of wall time. `cmd/testbudget` enforces it in the post-merge full
  suite. A test over budget is a defect: split it into chunk tests, never trim
  its coverage. (`AGENTS.md` wraps test commands in `systemd-run` with these
  caps; that is the maintainers' shared box's rule, not something you need
  locally.)

What each ratchet and golden test means when it fails is in
[docs/site/contributing/testing.md](docs/site/contributing/testing.md).

## Git hooks

```sh
git config core.hooksPath .githooks
```

- `pre-commit` (and `pre-merge-commit`) runs `go run ./cmd/pkgmap -check`
  when you add, delete or rename a Go file. A **new package** needs a tier and
  status entry in `docs/site/architecture/packages.tiers.json`, or the hook
  fails.
- `pre-push` runs the race detector one package at a time when you push
  `main` with `RACE=1` set; otherwise it does nothing.

Write plain commit messages: no attribution trailers (`Co-Authored-By:`,
"Generated with …") and no issue-tracker references.

## PR checklist

Modelled on the first outside PR (`git show 786510b3f`), which is a good
example of the shape we like.

- [ ] **The commit message says why**, with the CR (Comprehensive Rules)
      rule it implements, what was wrong, and what you measured. If you
      checked how many corpus cards share the shape, say so with numbers.
- [ ] **A test that fails without the change**, in the package you changed,
      named for the behaviour (`TestOmniscienceInHandGrantsNothing`).
- [ ] `gofmt -l` is empty for your files.
- [ ] `go vet` on the packages you touched (`go vet ./engine/rules/`).
- [ ] `go run ./cmd/gentypes -check` if you changed anything in
      `server/protocol` (it regenerates `web/src/protocol.ts`; `make gentypes`
      rewrites it).
- [ ] `go test ./server/host/httpapi -run TestOpenAPI` if you changed
      `server/protocol` or an HTTP route: the published API spec
      (`docs/site/api/openapi.json`) must match the code. Regenerate its
      generated parts with `-update` and commit the result.
- [ ] Focused `go test` of the packages you touched, **with `.cards/`
      present**.
- [ ] `make sim`: 20/20 `replay OK`.
- [ ] `TestHeads` (`go test -run '^TestHeads$' ./engine/rules/`). It pins the
      hash-chain head of deterministic bot games at 2/4/6/8 seats. If a head
      moves, your change altered what those games do. That is fine **if it
      is intended**: re-pin with
      `GORGE_UPDATE_HEADS=1 go test ./engine/rules/ -run '^TestHeads$'` and
      name the first diverging event and its cause in the commit message
      (`go doc ./cmd/headdiff` shows how to find it). An unexplained moved
      head is a bug.
- [ ] If your change makes a listed card or parameter supported, delete its
      ratchet file in the same commit (see testing.md).

## Where do I change…

| I want to… | Go to |
|---|---|
| Fix how a card plays | Start with [first-card-fix.md](docs/site/contributing/first-card-fix.md). |
| Implement an `api:` effect | `engine/effects/`: `effects.Register("Name", fn)` in an `init()`. If it needs more than `effects.Host` offers, it is rules work. |
| Implement a keyword, trigger, static or replacement | `engine/rules/`, declared with `effects.RegisterNonAPI("kw:X", …)`. A keyword that expands into ordinary script lines is an `engine/cards/kw_*.go` expander instead. |
| Add a `count:` value head | The evaluator in `engine/effects/` (`count*.go`) **and** `modelledValueHeads` in `engine/effects/value_heads.go` (`TestValueHeadRegistryMatchesEvaluator` holds them together). |
| Change how scripts parse | `engine/cards/` (this changes the IR cache fingerprint; `make compile-cards` again). |
| Add an event kind | Append to `engine/events/event.go` after the last constant and add its `kindInfo` entry in `engine/events/kindinfo.go`; the state change itself goes in `events.Apply`. |
| Add a decision kind | Don't, without a maintainer decision: the set is closed and every seat, bot, client and the wire must answer it. |
| Change a seat's projected view | `engine/view/card.go` (cards, battlefield), `engine/view/stack.go` (stack); `engine/view/view.go` only for the assembler. |
| Change the wire | `server/protocol/`, then `make gentypes`. |
| Add a bot policy | Bench-only: a `seat.Seat` driven through `testkit/bench`. Hosted: `bots.Register` in an `init()` (`bots/registry.go`), linked from `bots/all`. Read [docs/site/bots/native-go.md](docs/site/bots/native-go.md) and [training-and-adoption.md](docs/site/bots/training-and-adoption.md) first. |
| Add a repo deck | `testkit/testutil/decks/*.json` via `cmd/deckimport`; every card in it must be fully supported. |
| Turn a player's feedback report into a test | `go run ./cmd/repro -emit-test <pkg> <feedback-dir>` (see `AGENTS.md`, "Reproduce a feedback report"). |
| Add a package | Anywhere it fits the tiers, plus an entry in `docs/site/architecture/packages.tiers.json`. |

The per-package map is [docs/agents/repo-map.md](docs/agents/repo-map.md)
and `go run ./cmd/pkgmap` prints every package with its purpose and tier.
Draft events and the maintainers' extensions are not in this repository:
they live in a private companion module that imports gorge (README,
"The private companion module"). The compliance oracles and the
oracle-scenario corpora (the hand Oracle-text audit, the XMage ports) live in
the public [gorge-compliance](https://github.com/adams-shaun/gorge-compliance),
which measures a pinned gorge revision; gorge keeps the oracle runner
(`engine/rules/oracle_*.go`) and a small residual scenario set as its
contract check (`engine/rules/oracle_contract_test.go`). A change in
gorge never needs it, but a change to a seam it uses (`bots.Register`,
`server/gorged.Extensions`, `server/gorged.RegisterNetBot`, `testkit/`, `cli/`)
is an API change for it.
Behaviour that looks like a bug but is deliberate is recorded in
`docs/design/2026-09-22-engine-contracts.md` and in `AGENTS.md`'s
"Known approximations" table; read both before filing.
