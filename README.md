# gorge

A deterministic, event-sourced Magic: the Gathering rules engine, written in
pure Go. No cgo, no third-party dependencies in the card pipeline or rules
core.

## Design

Every state mutation goes through `events.Apply` — there is no path that
writes to a `state.Game` field except by emitting and applying an event. That
one rule is what makes the rest of the engine work: a match is fully
described by its config plus its event log, so replaying a log and resuming a
match from any point in it are the same operation, not separate features to
maintain.

## Test-time budgets

Every Go package with tests carries a `TEST_HISTORY.md` recording how long its
tests take and a hard `budget_s` that a commit cannot exceed. `make test-time`
(`go run ./cmd/testtime -all`) measures every package and appends a row; the
pre-commit hook runs `go run ./cmd/testtime -changed` on staged `.go` files and
blocks the commit when a package exceeds its budget. To raise a budget, edit
`budget_s` in the package's `TEST_HISTORY.md` and add a
`Test-Budget-Approved: <who> — <why>` trailer to the commit message.

## Package layout

Packages have a strict, one-directional dependency order:

```
cards → state → decision → botpolicy → events → effects → rules → view →
seat → replay → protocol → host → host/httpapi → cmd/*
```

`cards` depends on nothing in this module. Each later package may depend on
anything earlier in the chain, never the reverse — in particular, `effects`
must never import `rules`, the wire-facing packages (`view`, `protocol`,
`botpolicy`) must never import the engine, and `host` must never leak a
`*state.Game` or `*rules.Engine` to a client. The order is enforced by
`internal/archtest`'s `TestDependencyOrderHolds`, which walks the module's
import graph and fails when a forbidden edge (direct or transitive) appears;
`internal/archtest` also pins the determinism rule that only `host`,
`host/httpapi`, `cmd/gorged` and `cmd/testtime` may import `time`.

`deck` sits off that chain: it imports `cards` and nothing else, and its
consumers are the test fixtures and the match host. It is deliberately not a
link in the order above — no package in the chain may import it.

- `cards` — compiles Forge card scripts into the engine's card IR
- `deck` — the bare {name, count} deck-list JSON, resolved against a
  `cards.Registry` into the repeated `[]*cards.Card` the rules core deals;
  the one parser the test fixtures and the match host share
- `state` — objects, zones, players, game state
- `decision` — the choice/option types the engine presents to a seat
- `botpolicy` — the one bot decision policy, in one copy, shared by `seat`
  and the engine's own fuzz-driver
- `events` — the event union, `Apply`, the log, the hash chain
- `effects` — native primitive implementations (filter/count evaluators, etc.)
- `rules` — turn structure, priority, stack, combat, state-based actions
- `view` — projects one seat's view of a game (hidden zones and other seats'
  decisions redacted); the only package a client reads state through
- `seat` — who answers the engine's decisions; a `Seat` is handed a
  `view.View`, never a `*state.Game` or `*rules.Engine`
- `replay` — re-executes a match from its recorded log and verifies the
  rebuilt event stream matches
- `protocol` — the versioned wire between a match host and its clients; types
  only, never imports the engine
- `host` — keeps tables running (one goroutine per table), sessions,
  snapshots, append-only persistence, crash handling
- `host/httpapi` — serves a `host.Registry` over plain net/http: JSON, one
  SSE stream per client, and the embedded web client
- `internal/` — `archtest` (import-graph/determinism enforcement), `testutil`
  (fixtures shared by tests), `tsgen` (Go→TypeScript for the web client)

`cmd/` holds the commands, each a `main.go` program:

- `cmd/forgec` — fetches and compiles the Forge corpus, reports coverage
- `cmd/gorged` — runs perpetual bot tables and serves them to a browser
- `cmd/gentypes` — regenerates `web/src/protocol.ts` from package `protocol`
- `cmd/mtgsim` — headless self-play over the repo decks, verifying replay
- `cmd/botbench` — plays N matches between two bot policies, reports the split
- `cmd/keywordbench` — measures keyword presence separately from event-log use
- `cmd/testtime` — measures package test wall time and enforces its budget
- `cmd/gcgate` — budgets the CPU share a package's tests spend on GC
- `cmd/allocgate` — budgets a package's total allocation and peak RSS

## Licensing boundary

gorge itself is Apache-2.0. Card *behaviour* is compiled from
[Card-Forge/forge](https://github.com/Card-Forge/forge)'s card scripts, which
are GPL-3.0 — this repo ships a compiler for that script format, never the
scripts themselves. `forgec fetch` pulls the card corpus and its token
scripts into `.cards/`, which is gitignored and never committed.
`cards/boundary_test.go` fails the build if any Forge `.txt` script is ever
tracked in git, working tree, index, or HEAD.

## Getting started

```sh
make fetch-cards          # one-time; ~25 MB from Card-Forge/forge
make compile-cards        # parse into the IR cache
make report               # print card coverage against implemented primitives
make test
```

`make lint` also runs the web half (`lint-web`), which needs `npm ci`; the
Go-only subset is `gofmt -l . && go vet ./... && go run ./cmd/gentypes -check`.

## Running gorged

`gorged` is the M2a deliverable: perpetual bot tables served to a browser.
One command after the corpus is compiled:

```sh
make gorged
```

This builds and runs `bin/gorged`. `make gorged` starts 4 tables of 4 seats
(constructed and Commander) on `:8080`; `gorged -h` lists the flags and their
defaults (`-tables`, `-seats`, `-addr`, `-pace`, `-format`, `-spectator`,
`-mulligans`, `-dir`, ...). Open `http://localhost:8080/` to watch. The
Svelte client it serves is built separately with `make web` (needs Node);
until that build exists `webFS` serves nil for the client and the web root
answers `503`, but the `/api/*` REST endpoints (e.g. `/api/tables`) work
regardless. Match files accumulate in `gorged-data/` (override with `-dir`).

## Status

The engine now plays the repo's 19 bundled decks at every seat count and
replays those games byte-identically, and `gorged` serves perpetual bot tables
to a browser. But it is **not** ready for parity or production use.

There is an opt-in, known-red Comprehensive Rules conformance lane:

```sh
make conformance        # GORGE_CR_CONFORMANCE=1 go test ./rules -run TestCR
```

It is red **on purpose**: each leaf pins a specific documented divergence from
the Comprehensive Rules, so a failure is a catalogued defect rather than
breakage, and a pass on a leaf is the signal that defect was fixed. The
divergences are listed in `AGENTS.md`'s "Known approximations" table — read
that before assuming any odd behaviour is a bug.
