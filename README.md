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

## Package layout

Packages have a strict, one-directional dependency order:

```
cards → state → decision → events → effects → rules → cmd/*
```

`cards` depends on nothing in this module. Each later package may depend on
anything earlier in the chain, never the reverse — in particular, `effects`
must never import `rules`.

- `cards` — Forge card script parser, IR types, IR cache load/store
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
scripts themselves. `forgec fetch` pulls the corpus into `.cards/`, which is
gitignored and never committed. `cards/boundary_test.go` fails the build if
any Forge `.txt` script is ever tracked in git, working tree, index, or HEAD.

## Getting started

```sh
make fetch-cards          # one-time; ~25 MB from Card-Forge/forge
make compile-cards        # parse into the IR cache
make report               # print card coverage against implemented primitives
make test
make lint
```

## Status

Milestone M0/M1 work — module skeleton, card fetch/compile pipeline, and
rules core (multi-seat priority, stack, combat, replay) are under active
development. Not yet ready for parity or production use.
