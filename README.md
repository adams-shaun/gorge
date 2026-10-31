# gorge

gorge is a *Magic: The Gathering* rules engine in pure Go, with a server and
browser client to play on it. It compiles [Forge](https://github.com/Card-Forge/forge)'s
card scripts instead of hand-coding cards, and records every game as a
deterministic, hash-chained event log that replays byte for byte.

**Documentation: [adams-shaun.github.io/gorge](https://adams-shaun.github.io/gorge/)**

| I want to… | Start at |
|---|---|
| try it in five minutes | [Getting started](https://adams-shaun.github.io/gorge/getting-started/) |
| host games, play in a browser, run a draft | [User's guide](https://adams-shaun.github.io/gorge/guide/) |
| connect a bot (Go, Python, Java) | [Bots](https://adams-shaun.github.io/gorge/bots/) and the [API reference](https://adams-shaun.github.io/gorge/api/) |
| fix a card or a rule | [CONTRIBUTING.md](CONTRIBUTING.md) and [Your first card fix](https://adams-shaun.github.io/gorge/contributing/first-card-fix/) |
| see how it is built | [Architecture](https://adams-shaun.github.io/gorge/architecture/) |
| compare speed or card coverage | [Performance](https://adams-shaun.github.io/gorge/performance/) and [Card support](https://adams-shaun.github.io/gorge/support/sets/) |

## What runs

- **The engine** (`engine/`) plays two- to eight-player games, Constructed and
  Commander. A card is playable when every primitive its compiled script names
  is implemented; `make report` prints the current share of Forge's corpus.
  gorge plays full games and replays them exactly, but it is not at rules parity
  with mature engines and is not ready for production use.
- **The server** (`gorged`) hosts perpetual bot tables, play-against-a-bot games
  and 4-seat draft events over HTTP and server-sent events. It speaks three
  client protocols: its native API, SpellBench v2 and ManaBrew.
- **Bots** (`bots/`): a default heuristic bot and several hosted policies. Any
  program can play a seat over HTTP.
- **Tools** (`cmd/`): `mtgsim` plays verified bot games, `repro` turns a player's
  bug report into a failing test, `forgec` fetches and compiles the cards.

A second, much faster two-player engine, dirac, is checked against gorge decision
by decision. Its source isn't in this repository yet;
[its pages](https://adams-shaun.github.io/gorge/architecture/dirac/) describe its design.

## Quickstart

You need Go (the version in `go.mod`), `git`, `make` and `bash`. There is no cgo
and there are no third-party Go modules.

```sh
make fetch-cards compile-cards    # one-time: Forge's scripts into .cards/ (about 140 MB), then the compiled cache
make sim                          # 20 verified 4-seat bot games; each prints "replay OK"
go build -trimpath -o bin/gorged ./cmd/gorged
bin/gorged -addr :8090 -dir /tmp/gorge-local -tables 2 -seats 4 -vsbot
```

`curl localhost:8090/api/tables` lists the running tables. The browser client is
embedded at build time, so run `make web` (needs Node) before `go build` to get it;
without it the web root answers `503` and the `/api/*` routes still work. Never
point two servers at one `-dir`. [Getting started](docs/site/getting-started.md)
walks through this and the tests.

## The one design rule

Every state change goes through `events.Apply`. Engine code never writes a
`state.Game` field; it emits an event into the log. A match is therefore its
configuration plus its log, and replay, undo, search clones and bug reproduction
all reuse that one mechanism. A write that bypasses `events.Apply` is a replay bug.

## The repository

| Directory | What it holds |
|---|---|
| `engine/` | The rules core: the card compiler, game state, events, effects, rules, views, replay, and `engine/game`, the seam a host runs an engine through |
| `server/` | The reference server `gorged`, its tables and persistence (`host`), and the wire adapters |
| `bots/` | The bot registry, the default bot and the hosted policies |
| `draft/` | The limited stack: boosters, pods, deck building, draft events |
| `web/` | The Svelte browser client |
| `cmd/` | Product and developer binaries |
| `testkit/`, `cli/` | Public support packages: test fixtures, game runner, wire conformance, command-line layer |
| `internal/` | Import-graph and determinism ratchets, generators |
| `scripts/`, `docs/` | Contributor and CI scripts; the site (`docs/site`) and decision records (`docs/decisions`) |

`engine/` imports nothing outside itself. [Repository layout](docs/site/contributing/repo-layout.md)
has the import rules and where each kind of change goes;
[The three repositories](docs/site/repos/index.md) covers the public compliance
repository and the private module that build on gorge. An AI coding agent should
start at [AGENTS.md](AGENTS.md).

## License

gorge is Apache-2.0 ([LICENSE](LICENSE)). Forge's card scripts are GPL-3.0, so gorge
ships a compiler for the format and never the scripts: `make fetch-cards` downloads
them into the gitignored `.cards/` at a commit pinned by `FORGE_REF` in the
`Makefile`, and `engine/cards/boundary_test.go` fails the build if one is tracked.
[NOTICE](NOTICE) names the third-party data sources.
