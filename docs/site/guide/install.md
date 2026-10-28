# Install and build

This page takes you from nothing to a checkout that can run games: the
prerequisites, the card corpus, the binaries and the browser client. It is
for anyone who wants to play against gorge's bots or run bot matches; a
contributor needs the same steps, and then [Testing](../contributing/testing.md).

*Verified 2026-10-27 on Linux (x86-64, Go 1.26.3) at commit `25d71cadc`.
Other platforms are not tested by the maintainers.*

## Prerequisites

| Tool | Version | Why |
|---|---|---|
| Go | the `go` line of [`go.mod`](https://github.com/adams-shaun/gorge/blob/main/go.mod), **1.25.8** today | builds everything. An older Go 1.21+ downloads the right toolchain by itself when `GOTOOLCHAIN=auto` (the default). |
| git | any recent | `make fetch-cards` uses a sparse, depth-1 `git fetch` |
| make | GNU make | the build targets (all of them are thin wrappers you can run by hand) |
| Node.js + npm | Node 20 or newer | **only** to build the browser client. Nothing else needs Node. |

gorge has no cgo and no third-party Go dependencies, so there is nothing else
to install. The Makefile builds with `CGO_ENABLED=0`.

## 1. Clone and fetch the card corpus

```sh
git clone https://github.com/adams-shaun/gorge && cd gorge
make fetch-cards
```

gorge does not ship card definitions. It plays the card scripts of the
[Forge](https://github.com/Card-Forge/forge) project, which are GPL-3.0 while
gorge is Apache-2.0, so they are downloaded into `.cards/` (gitignored) and
never committed. `make fetch-cards` builds `bin/forgec` and fetches the
scripts from gorge's fork of Forge at the exact commit pinned by `FORGE_REF`
in the [`Makefile`](https://github.com/adams-shaun/gorge/blob/main/Makefile).
The pin matters: every game, replay and coverage number is a function of it.

```text
fetched 34074 card scripts and 854 token scripts from https://github.com/adams-shaun/forge.git at 91402ac5 (91402ac511046c0b7ed0bb691a8a446ca7dfc873)
These scripts are licensed GPL-3.0. They are a build input only:
never commit them, never include them in a published image.
```

That fetch took 5 seconds and left 137 MB on disk (`.cards/cardsfolder` and
`.cards/tokenscripts`, plus `.cards/cards.lock` recording the commit).

## 2. Compile the corpus (optional but quick)

```sh
make compile-cards    # bin/forgec compile -dir .cards
make report           # coverage against the implemented primitives
```

`compile-cards` parses every script into gorge's intermediate representation
and caches it as `.cards/ir-<fingerprint>.gob.gz` (about 9 MB, plus a 17 MB
`.seg` file beside it). It took 1.6 s here. You can skip it: every tool opens
the corpus through the same loader, which compiles on first use when the cache
is missing or stale.

The fingerprint is a hash of the compiler that wrote the cache, so each engine
version you build gets its own `ir-*` files and old ones pile up. They are
safe to delete; the next run recompiles.

`make report` prints the corpus coverage at the pin, for example
`cards: 34074  playable: 32337 (94.9%)` (2026-10-27, `25d71cadc`). See
[Card sets](../support/sets.md) for what "playable" means.

## 3. Build the binaries

```sh
make build            # bin/forgec, bin/mtgsim, bin/gorged
```

| Binary | Source | What it is |
|---|---|---|
| `bin/gorged` | [`cmd/gorged`](https://github.com/adams-shaun/gorge/blob/main/cmd/gorged/main.go) | the game server: tables, the play-vs-bot lobby, the browser client ([Play in the browser](play.md)) |
| `bin/mtgsim` | [`cmd/mtgsim`](https://github.com/adams-shaun/gorge/blob/main/cmd/mtgsim/main.go) | headless bot-vs-bot games with replay verification ([Bot-vs-bot games](bot-games.md)) |
| `bin/forgec` | [`cmd/forgec`](https://github.com/adams-shaun/gorge/blob/main/cmd/forgec/main.go) | fetches, compiles and reports on the corpus |

Every tool also runs straight from source with `go run ./cmd/<name>`, which is
what the rest of this guide mostly shows. Run them from the repository root:
the defaults (`-cards .cards`, `-decks testkit/testutil/decks`) are relative
paths.

## 4. Build the browser client

`gorged` embeds the browser client (a Svelte app in `web/`) into its binary at
**compile time**, from `server/gorged/webdist/`. A fresh clone has only an
empty placeholder there, so a server built from a fresh clone serves the API
but answers the page itself with:

```text
HTTP 503: web build missing — run make web
```

To get the client:

```sh
make web              # npm ci, then vite build into server/gorged/webdist/
go build -o bin/gorged ./cmd/gorged   # rebuild: the bundle is embedded at build time
```

!!! note "Not run for this page"
    `make web` was not run while writing this guide (the docs build machine
    does not run npm). The target is `cd web && npm run build` after an
    `npm ci`; see [`web/README.md`](https://github.com/adams-shaun/gorge/blob/main/web/README.md)
    for the Node version notes. Build and lint work on Node 20; the client's
    unit tests need Node 22 or newer.

The order matters: build the client **before** `go build` or `go run`. A
binary built earlier keeps serving the 503 (or an old client) until you
rebuild it.

## Check the install

```sh
make sim
```

This builds `bin/mtgsim` and plays 20 four-seat games between the built-in
heuristic bots, replaying each one and comparing the rebuilt event chain with
the live one. Every game prints a result line and `replay OK`:

```text
seed 0: 4 seats,   1419 intents,   6941 events,  33 turns, winner=death-n-taxes, chain=c726ff7a143bef8d
  replay OK (chain c726ff7a143bef8d)
seed 1: 4 seats,   1717 intents,   8409 events,  36 turns, winner=death-n-taxes, chain=cdf023640deb1b71
  replay OK (chain cdf023640deb1b71)
```

All 20 games verified in 5 s here. If you see `replay diverged` or a
`FAILED` line, that is an engine bug worth [reporting](troubleshooting.md).

Next: [Play in the browser](play.md) or [Bot-vs-bot games](bot-games.md).
