# Getting started

From a fresh clone to verified bot games, a local server with its API, and a focused test run.
You need Go and one download of about 140 MB; a browser board also needs Node and `make web`
([prerequisites](guide/install.md#prerequisites)). The [User's guide](guide/index.md) covers
hosting and playing; [Contributing](contributing/index.md) covers changing the code.

## 1. Fetch and compile the card corpus

```sh
git clone https://github.com/adams-shaun/gorge && cd gorge
make fetch-cards      # one-time: Forge's card and token scripts into .cards/ (about 140 MB)
make compile-cards    # parse them into the IR cache, .cards/ir-<fingerprint>.gob.gz
make report           # coverage against the implemented primitives
```

`make fetch-cards` builds `bin/forgec` and checks out the scripts at the commit
`FORGE_REF` pins in the `Makefile`, so every number is relative to that pin. The scripts
are GPL-3.0 and gorge is Apache-2.0, so they stay in the gitignored `.cards/` and are never
committed ([why](repos/index.md#licensing-and-the-forge-card-scripts)).
`make compile-cards` prints one non-fatal diagnostic line per rider text it doesn't support.

`make report` begins like this (the values depend on the `FORGE_REF` pin):

```text
corpus: <FORGE_REF> @ <FORGE_REF> (GPL-3.0, <n> files)
cards: <n>  playable: <n> (<pct>)
tokens: <n>
```

A card is *playable* when every primitive its script names is implemented; that does not
promise it behaves correctly ([Card support](support/sets.md) says what does). The report goes
on to rank the missing primitives by how many cards each would unlock.

## 2. Play verified games

```sh
make sim
```

This builds `bin/mtgsim` and plays 20 four-seat games between the built-in bots, replaying
each and checking that the rebuilt event chain matches the live run. Every game prints
`replay OK`. To choose the game yourself, call `mtgsim` directly (`bin/mtgsim -h` lists its
flags):

```sh
bin/mtgsim -seats 2 -games 2 -seed 42 -verify
```

```text
seed 42: 2 seats,    611 intents,   3157 events,  18 turns, winner=dimir-tempo, chain=347aee8d651c2a2b
  replay OK (chain 347aee8d651c2a2b)
seed 43: 2 seats,    590 intents,   3154 events,  20 turns, winner=death-n-taxes, chain=cf8d3caf6565f0da
  replay OK (chain cf8d3caf6565f0da)
```

An *intent* is one answer to a decision, and `chain` is the hash of the game's whole event
log. The numbers move whenever a change alters what the games do (the committed golden heads
are in `engine/rules/testdata/heads/`); these were printed at `fc2be1aabe` on 2026-11-08.

## 3. Run a local server

```sh
make web                                     # optional: the browser client, which needs Node 20.19+
go build -trimpath -o bin/gorged ./cmd/gorged
bin/gorged -dir /tmp/gorge-local -tables 2 -seats 4 -vsbot
```

This runs two perpetual four-seat bot tables and arms the play-against-a-bot flow. Open
`http://localhost:8080/`. The client is embedded when `gorged` is built, so build it first;
without it the web root answers `503 web build missing`, and the API still works:

```sh
curl localhost:8080/api/tables
```

To skip the build, run the published image instead. It needs only Docker and serves on
`http://127.0.0.1:8090/` ([Run the demo in Docker](web/docker.md#run-the-published-image)):

```sh
docker run -d --name gorge -p 127.0.0.1:8090:8080 -v gorge-cards:/cards -v gorge-data:/data \
  damadama22/gorged-demo:latest
```

To serve dirac, see [Serving dirac](guide/dirac.md). `bin/gorged -h` lists every flag. A bare `gorged`
uses `:8080`, `gorged-data/` and `feedback/`; two servers must never share one `-dir` (they corrupt
each other's matches). At startup it fetches card art for dealt decks into `<dir>/art`
(`-prewarm=false` skips it offline). [Play in the browser](guide/play.md) has the rest.

## 4. Run the tests

```sh
make test-one PKG=./engine/rules RUN='^TestHeads$'
make test-one PKG=./engine/rules RUN='^TestOracleRunnerContract$'
```

`TestHeads` plays golden bot games at 2, 4, 6 and 8 seats and a four-seat Commander game, and compares
each chain head with a committed value. [Testing](contributing/testing.md#focused-runs) explains the
make targets, how a skipped corpus test shows up (`ENV`, not a pass), and what each failure means.

## 5. Reproduce a reported game

A player's bug report from a server carries a replayable snapshot. This one is committed:

```sh
go run ./cmd/repro cmd/repro/testdata/feedback/20260914T120000Z-fb01
```

```text
== feedback report 20260914T120000Z-fb01 (table t1, match 1) ==
snapshot: captured: match.json log.json view.json
replayed 26 of 26 recorded intents (151 events); recorded head 67b55d2eb519629d
turn 2, round 1, main1 (step main1) — active: seat 0, priority: seat 0
...
```

Exit status 0 means the rebuilt event stream matched the recording, event for event. A
`DIVERGED` result names the first mismatching event. [Contributing](contributing/index.md#reproduce-a-feedback-report)
has the other flags, including the one that writes a failing test from a snapshot.
