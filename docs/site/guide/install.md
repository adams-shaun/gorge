# Install and build

This page takes you from nothing to a checkout that can run games: the prerequisites, the
card corpus, the binaries and the browser client. Only the corpus step is required for
bot games. A contributor needs the same steps, then [Testing](../contributing/testing.md).

The output below is from 2026-10-31 on Linux x86-64 with Go 1.26.3. The maintainers test
on Linux only.

## Prerequisites

| Tool | Version | Why |
|---|---|---|
| Go | the `go` line of [`go.mod`](https://github.com/adams-shaun/gorge/blob/main/go.mod). An older Go 1.21 or later downloads the right toolchain itself under the default `GOTOOLCHAIN=auto` | builds everything |
| git | any recent | `make fetch-cards` runs a sparse, depth-1 `git fetch` |
| GNU make | any recent | the build targets |
| Node.js and npm | Node 20 or later | only the browser client. Its unit tests need Node 22 or later ([`web/README.md`](https://github.com/adams-shaun/gorge/blob/main/web/README.md)) |

gorge has no cgo and no third-party Go dependencies.

## 1. Fetch the card corpus

```sh
git clone https://github.com/adams-shaun/gorge && cd gorge
make fetch-cards
```

gorge doesn't ship card definitions. It plays the card scripts of the
[Forge](https://github.com/Card-Forge/forge) project, which are GPL-3.0 where gorge is
Apache-2.0, so they are downloaded into the gitignored `.cards/` and never committed.
`make fetch-cards` builds `bin/forgec` and fetches the scripts from gorge's fork of Forge at
the commit pinned by `FORGE_REF` in the
[`Makefile`](https://github.com/adams-shaun/gorge/blob/main/Makefile). Every game and replay
is a function of that pin. The fetch leaves about 140 MB on disk.

```text
fetched 34074 card scripts and 854 token scripts from https://github.com/adams-shaun/forge.git at 91402ac5 (91402ac511046c0b7ed0bb691a8a446ca7dfc873)
These scripts are licensed GPL-3.0. They are a build input only:
never commit them, never include them in a published image.
```

Every tool opens the corpus through one loader, which parses the scripts into gorge's
intermediate representation on first use and caches it as `.cards/ir-<fingerprint>.gob.gz`
with a `.seg` file beside it (about 26 MB together). `make compile-cards` does that step
ahead of time, and `make report` prints the corpus coverage
([Getting started](../getting-started.md#1-fetch-and-compile-the-card-corpus) explains its
output). The fingerprint is a hash of the compiler, so each engine version you build writes
its own files. Old ones are safe to delete.

## 2. Build the browser client, if you want one

`gorged` embeds the browser client, a Svelte app in `web/`, at **compile time** from
`server/gorged/webdist/`. A fresh clone has only an empty placeholder there, so the client
has to be built before `gorged` is:

```sh
make web              # npm ci, then vite build into server/gorged/webdist/
```

A `gorged` compiled before that step serves the whole API, but answers every page with:

```text
HTTP 503: web build missing — run make web
```

Rebuild `gorged` after `make web`, or after any change under `web/`. `go run` rebuilds for
you.

## 3. Build the binaries

```sh
make build            # bin/forgec, bin/mtgsim, bin/gorged
```

| Binary | What it is |
|---|---|
| `bin/gorged` | the game server: tables, play-vs-bot, draft events, the browser client ([Play in the browser](play.md)) |
| `bin/mtgsim` | headless bot games with replay verification ([Bot-vs-bot games](bot-games.md)) |
| `bin/forgec` | fetches, compiles and reports on the corpus |

Every tool also runs from source with `go run ./cmd/<name>`, as the rest of this guide
shows. Run them from the repository root: the defaults (`-cards .cards`,
`-decks testkit/testutil/decks`) are relative paths.

## Check the install

```sh
make sim
```

This plays 20 four-seat games between the built-in heuristic bots, replays each one and
compares the rebuilt event chain with the live one. Each game prints a result line and
`replay OK`:

```text
seed 0: 4 seats,   1419 intents,   6941 events,  33 turns, winner=death-n-taxes, chain=c726ff7a143bef8d
  replay OK (chain c726ff7a143bef8d)
seed 1: 4 seats,   1717 intents,   8409 events,  36 turns, winner=death-n-taxes, chain=cdf023640deb1b71
  replay OK (chain cdf023640deb1b71)
```

The 20 games took about 5 seconds. A `replay diverged` or `FAILED` line is an engine bug
worth [reporting](troubleshooting.md#a-game-prints-livelock-did-not-terminate-or-replay-diverged).

Next: [Play in the browser](play.md) or [Bot-vs-bot games](bot-games.md).
