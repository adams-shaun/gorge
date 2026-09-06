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
cards → state → decision → events → effects → rules → cmd/*
```

`cards` depends on nothing in this module. Each later package may depend on
anything earlier in the chain, never the reverse — in particular, `effects`
must never import `rules`.

`deck` sits off that chain: it imports `cards` and nothing else, and its
consumers are `cmd/*` and the test fixtures. It is deliberately not a link in
the order above — no package in the chain may import it.

- `cards` — Forge card script parser, IR types, IR cache load/store
- `deck` — the bare {name, count} deck-list JSON, resolved against a
  `cards.Registry` into the repeated `[]*cards.Card` the rules core deals; the
  one parser the test fixtures and the match host share
- `state` — objects, zones, players, game state
- `decision` — the choice/option types the rules core presents to a seat
- `events` — the event union, `Apply`, the log, the hash chain
- `effects` — native primitive implementations (filter/count evaluators, etc.)
- `rules` — turn structure, priority, stack, combat, state-based actions
- `cmd/forgec` — fetches and compiles the Forge card corpus

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
make lint
```

## Running gorged

`gorged` is the M2a deliverable: perpetual bot tables served to a browser.
One command after the corpus is compiled:

```sh
make gorged
```

This builds and runs `bin/gorged` with 4 tables of 4 seats on `:8080`.
Open `http://localhost:8080/` to watch the tables. The Svelte client it
serves is built separately with `make web` (Needs Node); until that build
exists the server answers the web root with `503` but the `/api/*` REST
endpoints (e.g. `/api/tables`) work regardless. Match files accumulate in
`gorged-data/` (override with `-dir`). See `gorged -h` for all flags.

## Status

Milestone M0/M1 work — module skeleton, card fetch/compile pipeline, and
rules core (multi-seat priority, stack, combat, replay) are under active
development. Not yet ready for parity or production use.
