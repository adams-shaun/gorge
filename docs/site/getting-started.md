# Getting started

This page takes you from a fresh clone to verified bot games, a local server with its API and a
focused test run. It needs Go and one download of about 140 MB; a browser board also needs Node
and `make web`. The
[User's guide](guide/index.md) goes further on hosting and playing;
[Contributing](contributing/index.md) on changing the code.

[Install and build](guide/install.md#prerequisites) lists the prerequisites.

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
`make compile-cards` prints a couple of diagnostics for rider text it doesn't support;
those are expected.

`make report` begins like this (2026-11-02, at `b78e4c8b1`):

```text
corpus: <FORGE_REF> @ <FORGE_REF> (GPL-3.0, 34074 files)
cards: 34074  playable: 32393 (95.1%)
tokens: 854
```

A card is *playable* when every primitive its script names is implemented, which doesn't
promise it behaves correctly; [Card support](support/sets.md) says what does. The report goes
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
seed 42: 2 seats,    611 intents,   3157 events,  18 turns, winner=dimir-tempo, chain=3baaa1a31c521ffd
  replay OK (chain 3baaa1a31c521ffd)
seed 43: 2 seats,    585 intents,   3144 events,  20 turns, winner=death-n-taxes, chain=ff2b8e4a6b5cc72c
  replay OK (chain ff2b8e4a6b5cc72c)
```

An *intent* is one answer to a decision, and `chain` is the hash of the game's whole event
log. The numbers move when a change alters what the games do; these are from 2026-11-01 at `5917ab1ae`.

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

To run the same server on dirac, see [Serving dirac](guide/dirac.md). `bin/gorged -h` lists every flag. A bare `gorged` uses `:8080`, `gorged-data/` and, for bug reports, `feedback/`. Two servers
must never share one `-dir`, because they silently corrupt each other's matches. At startup the
server fetches card art for the dealt decks into `<dir>/art`; pass `-prewarm=false` to skip that
when you are offline. [Play in the browser](guide/play.md) walks through the rest.

## 4. Run the tests

```sh
go test -run '^TestHeads$' ./engine/rules/
go test -run 'TestOracleRunnerContract' ./engine/rules/
```

`TestHeads` plays one golden bot game at each of 2, 4, 6 and 8 seats and compares each game's
chain head with a committed value. [Testing](contributing/testing.md#focused-runs) has how to run
tests without the whole suite, how to tell a skipped corpus test from a pass, and what each failure
means.

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
