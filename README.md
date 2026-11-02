# gorge

This repository holds two *Magic: The Gathering* rules engines in pure Go, a server
and browser client to play on them, and the bots and draft stack around them.

**gorge** plays two to eight seats, Constructed and Commander. It compiles
[Forge](https://github.com/Card-Forge/forge)'s card scripts instead of hand-coding
cards, and records every game as a deterministic, hash-chained event log that
replays byte for byte.

**dirac** plays two seats, built for search and self-play: a game is one flat value
and one buffer sized to its card table, copied in place. It plays only cards it has
shown it agrees with gorge on, decision by decision, and refuses the rest by name.

**Documentation: [adams-shaun.github.io/gorge](https://adams-shaun.github.io/gorge/)**

| I want to… | Start at |
|---|---|
| play verified bot games and reach the API | [Getting started](https://adams-shaun.github.io/gorge/getting-started/) |
| play in a browser (needs Node 20.19+ and `make web`) | [Install and build](https://adams-shaun.github.io/gorge/guide/install/) |
| host games, run a draft | [User's guide](https://adams-shaun.github.io/gorge/guide/) |
| connect a bot (Go, Python, Java) | [Bots](https://adams-shaun.github.io/gorge/bots/) and the [API reference](https://adams-shaun.github.io/gorge/api/) |
| fix a card or a rule | [CONTRIBUTING.md](CONTRIBUTING.md) and [Your first card fix](https://adams-shaun.github.io/gorge/contributing/first-card-fix/) |
| see how the engines are built | [Architecture](https://adams-shaun.github.io/gorge/architecture/) and [dirac](https://adams-shaun.github.io/gorge/architecture/dirac/) |
| compare speed or card coverage | [Performance](https://adams-shaun.github.io/gorge/performance/) and [Card support](https://adams-shaun.github.io/gorge/support/sets/) |

## What runs

| | gorge | dirac |
|---|---|---|
| Seats and formats | 2 to 8, Constructed and Commander | 2, Constructed |
| Cards | `<measure:gorge-playable>` of `<measure:corpus-files>` scripts playable | `<measure:dirac-supported>` of `<measure:corpus-names>` names supported |
| What the count means | every primitive the script names is implemented; [per set](https://adams-shaun.github.io/gorge/support/sets/) also checked against XMage | lowered, not refused by name, and clean in the lockstep games of the evidence corpus |
| Random games per core-second, FDN decks | `<measure:gorge-games>` | `<measure:dirac-games>` |
| Record of a game | hash-chained event log | the answers given; the log is derived |
| Checked by | gorge-compliance: `<measure:level-a>` sets at level A, `<measure:level-b>` at level B, against XMage | gorge, at every decision, on every change |

gorge plays full games and replays them exactly, but it is not at rules parity
with mature engines and is not ready for production use. dirac agrees with gorge
on the cards it supports, so it shares gorge's bugs: lockstep can't see a mistake
both engines make. Measured `<measure:date>`
at `<measure:rev>`; [Performance](https://adams-shaun.github.io/gorge/performance/) has the method.

- **The server** (`gorged`) hosts perpetual bot tables, play-against-a-bot games
  and 4-seat draft events over HTTP and server-sent events. It speaks three
  client protocols, its native API, SpellBench v2 and ManaBrew, and
  `gorged -engine dirac` serves two-seat games on FDN and BLB limited decks
  ([Serving dirac](https://adams-shaun.github.io/gorge/guide/dirac/)).
- **Bots** (`bots/`): a default heuristic bot and several hosted policies. Any
  program can play a seat over HTTP.
- **Tools** (`cmd/`): `mtgsim` plays verified bot games, `repro` turns a player's
  bug report into a failing test, `forgec` fetches and compiles the cards.

## Quickstart

[Install and build](docs/site/guide/install.md#prerequisites) lists the prerequisites.

```sh
make fetch-cards compile-cards    # one-time: Forge's scripts into .cards/ (about 140 MB), then the compiled cache
make sim                          # 20 verified 4-seat bot games; each prints "replay OK"
make web                          # optional: the browser client, which needs Node 20.19+
go build -trimpath -o bin/gorged ./cmd/gorged
bin/gorged -dir /tmp/gorge-local -tables 2 -seats 4 -vsbot
```

`curl localhost:8080/api/tables` lists the running tables. The browser client is
embedded at build time; without `make web` the web root answers `503` and the
`/api/*` routes still work. Never point two servers at one `-dir`.
[Getting started](docs/site/getting-started.md) walks through this and the tests.

To play the same server on dirac, build its card tables from the same corpus:

```sh
make dirac-tables
bin/gorged -engine dirac -seats 2 -vsbot -dir /tmp/gorge-dirac
```

## The one design rule

Every state change in gorge goes through `events.Apply`. Engine code never writes a
`state.Game` field; it emits an event into the log. A match is therefore its
configuration plus its log, and replay, undo, search clones and bug reproduction
all reuse that one mechanism. A write that bypasses `events.Apply` is a replay bug.

## The repository

| Directory | What it holds |
|---|---|
| `engine/` | The rules core: the card compiler, game state, events, effects, rules, views, replay, and `engine/game`, the seam a host runs an engine through |
| `dirac/` | The second engine, its card generator (`kgen`), and the oracle that plays it against gorge |
| `server/` | The reference server `gorged`, its tables and persistence (`host`), and the wire adapters |
| `bots/` | The bot registry, the default bot and the hosted policies |
| `draft/` | The limited stack: boosters, pods, deck building, draft events |
| `web/` | The Svelte browser client |
| `cmd/` | Product and developer binaries |
| `testkit/`, `cli/` | Public support packages: test fixtures, game runner, wire conformance, command-line layer |
| `internal/` | Import-graph and determinism ratchets, generators |
| `scripts/`, `docs/` | Contributor and CI scripts; the site (`docs/site`) and decision records (`docs/decisions`) |

`engine/` imports nothing outside itself, and dirac's core imports nothing outside
`dirac/`. [Repository layout](docs/site/contributing/repo-layout.md)
has the import rules and where each kind of change goes;
[The three repositories](docs/site/repos/index.md) covers the public compliance
repository and the maintainers' private companion module that build on gorge. An AI
coding agent should start at [AGENTS.md](AGENTS.md).

## License

gorge is Apache-2.0 ([LICENSE](LICENSE)). Forge's card scripts are GPL-3.0, so gorge
ships a compiler for the format and never the scripts: `make fetch-cards` downloads
them into the gitignored `.cards/` at a commit pinned by `FORGE_REF` in the
`Makefile`, and `engine/cards/boundary_test.go` fails the build if one is tracked.
dirac's card tables are generated from those scripts, so they are built locally with
`make dirac-tables` into the same gitignored directory and never committed.
[NOTICE](NOTICE) names the third-party data sources.
