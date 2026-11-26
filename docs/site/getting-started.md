# Getting started

From a fresh clone to verified bot games, a local server with its API, and a focused test run.
To host and play, read the [User's guide](guide/index.md); to change the code, [Contributing](contributing/index.md).

<div class="gg-needs" markdown>

| You need | For |
|---|---|
| Go | everything below ([prerequisites](guide/install.md#prerequisites)) |
| One download of about 140 MB | Forge's card and token scripts, fetched once into `.cards/` |
| Node 20.19+ and `make web` | the browser board only |
| Docker | only to run the published demo image instead of building |

</div>

<div class="gg-steps" markdown>

## Fetch and compile the card corpus { #1-fetch-and-compile-the-card-corpus }

```sh title="From a fresh clone"
git clone https://github.com/adams-shaun/gorge && cd gorge
make fetch-cards      # one-time, about 140 MB into .cards/
make compile-cards    # parse them into the IR cache
make report           # coverage against the implemented primitives
```

- `make fetch-cards` builds `bin/forgec` and checks out the scripts at the commit `FORGE_REF`
  pins in the `Makefile`, so every number is relative to that pin.
- `make compile-cards` writes `.cards/ir-<fingerprint>.gob.gz` and prints one non-fatal
  diagnostic line per rider text it doesn't support.
- `make report` begins like this; the values depend on the `FORGE_REF` pin:

```text
corpus: <FORGE_REF> @ <FORGE_REF> (GPL-3.0, <n> files)
cards: <n>  playable: <n> (<pct>)
tokens: <n>
```

A card is *playable* when every primitive its script names is implemented; that does not
promise it behaves correctly ([Card support](support/sets.md) says what does). The report goes
on to rank the missing primitives by how many cards each would unlock.

!!! warning "The scripts are never committed"
    They are GPL-3.0 and gorge is Apache-2.0, so they stay in the gitignored `.cards/`
    ([why](repos/index.md#licensing-and-the-forge-card-scripts)).

## Play verified games { #2-play-verified-games }

```sh
make sim
```

This builds `bin/mtgsim` and plays 20 four-seat games between the built-in bots, replaying
each and checking that the rebuilt event chain matches the live run. Every game prints
`replay OK`. To choose the game yourself, call `mtgsim` directly (`bin/mtgsim -h` lists its flags):

```sh title="Two two-seat games from seed 42"
bin/mtgsim -seats 2 -games 2 -seed 42 -verify
```

```text
seed 42: 2 seats,    611 intents,   3157 events,  18 turns, winner=dimir-tempo, chain=347aee8d651c2a2b
  replay OK (chain 347aee8d651c2a2b)
seed 43: 2 seats,    590 intents,   3154 events,  20 turns, winner=death-n-taxes, chain=cf8d3caf6565f0da
  replay OK (chain cf8d3caf6565f0da)
```

An *intent* is one answer to a decision, and `chain` is the hash of the game's whole event
log. The output above was printed at `fc2be1aabe` on 2026-11-08; it moves whenever a change
alters what the games do (the committed golden heads are in `engine/rules/testdata/heads/`).

## Run a local server { #3-run-a-local-server }

=== "Build it"

    ```sh
    make web              # optional: the browser client
    go build -trimpath -o bin/gorged ./cmd/gorged
    bin/gorged -dir /tmp/gorge-local -tables 2 -seats 4 -vsbot
    ```

    This runs two perpetual four-seat bot tables and arms the play-against-a-bot flow. Open
    `http://localhost:8080/`. The client is embedded when `gorged` is built, so build it first;
    without it the web root answers `503 web build missing`, and the API still works:

    ```sh
    curl localhost:8080/api/tables
    ```

=== "Run the published image"

    Needs only Docker and serves on `http://127.0.0.1:8090/`
    ([Run the demo in Docker](web/docker.md#run-the-published-image)):

    ```sh
    docker run -d --name gorge -p 127.0.0.1:8090:8080 -v gorge-cards:/cards -v gorge-data:/data \
      damadama22/gorged-demo:latest
    ```

!!! warning "Two servers must never share one `-dir`"
    They corrupt each other's matches. A bare `gorged` uses `:8080`, `gorged-data/` and `feedback/`.

At startup `gorged` fetches card art for dealt decks into `<dir>/art` (`-prewarm=false` skips it
offline). `bin/gorged -h` lists every flag; [Serving dirac](guide/dirac.md) and
[Play in the browser](guide/play.md) have the rest.

## Run the tests { #4-run-the-tests }

```sh title="Two focused runs"
make test-one PKG=./engine/rules RUN='^TestHeads$'
make test-one PKG=./engine/rules RUN='^TestOracleRunnerContract$'
```

`TestHeads` plays golden bot games at 2, 4, 6 and 8 seats and a four-seat Commander game, and
compares each chain head with a committed value. [Testing](contributing/testing.md#focused-runs)
explains the make targets, how a skipped corpus test shows up (`ENV`, not a pass), and what each
failure means.

## Reproduce a reported game { #5-reproduce-a-reported-game }

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
`DIVERGED` result names the first mismatching event.
[Contributing](contributing/index.md#reproduce-a-feedback-report) has the other flags, including
the one that writes a failing test from a snapshot.

</div>
