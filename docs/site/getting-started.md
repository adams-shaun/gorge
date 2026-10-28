# Getting started

The shortest path from a fresh clone to verified games, a local server and a
focused test run. It is for anyone trying gorge for the first time; the
[User's guide](guide/index.md) goes further on running and playing, and
[Contributing](contributing/index.md) on changing the code.

gorge needs Go (the version in `go.mod`, `go 1.25.8`), `make`, `git` and
`bash`. Node is needed only to build the browser client.

*Commands checked on `develop` @ `25d71cadc`, 2026-10-27.*

## 1. Fetch and compile the card corpus

```sh
git clone https://github.com/adams-shaun/gorge && cd gorge
make fetch-cards      # one-time: Forge card + token scripts at FORGE_REF into .cards/ (about 140 MB on disk)
make compile-cards    # parse the scripts into the IR cache (.cards/ir-<fingerprint>.gob.gz)
make report           # corpus coverage against the implemented primitives
```

`make fetch-cards` builds `bin/forgec` and does a sparse checkout of the Forge
card scripts at the commit pinned by `FORGE_REF` in the `Makefile`
(`FORGE_REPO` points at gorge's fork of Card-Forge/forge). Coverage numbers
are always relative to that pin. `make report` begins:

```text
corpus: 76fd7691ab45ab1b42cef19ad04b41bf364501f4 @ 76fd7691ab45ab1b42cef19ad04b41bf364501f4 (GPL-3.0, 34074 files)
cards: 34074  playable: 32337 (94.9%)
tokens: 854
```

and continues with the missing primitives ranked by how many cards each
would unlock. `make coverage` writes the same measurement as markdown tables
into the gitignored `.coverage/`, broken down by card type, colour and mana
value.

## 2. Play verified games

```sh
make sim
```

This builds `bin/mtgsim` and plays 20 four-seat games between the built-in
bot, replaying each one and checking that the rebuilt event chain matches the
live run. Every game prints `replay OK`. `mtgsim` also takes its flags
directly (`-seats`, `-games`, `-seed`, `-decks`, `-a`/`-b` for the two seats'
bot policies, `-verify`, `-v`):

```sh
bin/mtgsim -seats 2 -games 3 -seed 42 -verify
```

```text
seed 42: 2 seats,    661 intents,   3423 events,  19 turns, winner=death-n-taxes, chain=99228a9168f4832f
  replay OK (chain 99228a9168f4832f)
...
```

## 3. Run a local server

`gorged` hosts perpetual bot tables and a play-against-a-bot flow over HTTP
(JSON plus server-sent events) and serves the browser client.

```sh
make web                                   # optional: build the Svelte client (needs Node)
go build -o bin/gorged ./cmd/gorged
bin/gorged -addr :8090 -dir /tmp/gorge-local -tables 2 -seats 4 -vsbot
```

Then open `http://localhost:8090/`. Without the web build the root answers
`503`, but the REST endpoints such as `/api/tables` still work. `gorged -h`
lists every flag (`-tables`, `-seats`, `-format`, `-pace`, `-humans`,
`-bot-policy`, ...). The [User's guide](guide/index.md) walks through them.

!!! tip "Pick your own port and data directory"
    `make gorged` and a bare `gorged` use `:8080` and `gorged-data/`. Pass
    your own `-addr` and `-dir` if anything else might use those. Two servers
    must never share one `-dir`: they silently corrupt each other's persisted
    matches.

## 4. Run the tests

Run tests focused: one package and a `-run` pattern at a time.

```sh
go test -run 'TestHeads$' ./engine/rules/
go test -run 'TestOracleRunnerContract' ./engine/rules/
```

Avoid `make test` (`go test ./...`) as an everyday command: the full suite
has over 12,000 top-level tests and runs in CI and after merge. Never add
`-count=1`; it only defeats the test cache. Tests that need the corpus
**skip** when `.cards/` is missing, so a fast green run without the corpus
has tested very little. Every test must fit 4 GB RSS, 4 vCPU and one minute
of wall time.

[Testing](contributing/testing.md) explains the helpers, the oracle scenarios
and what each ratchet failure means. `make conformance` runs the focused
CR 601/733 conformance audit on its own.

## 5. Reproduce a reported game

Every game is replayable from its configuration plus the recorded answers. A
feedback snapshot (`match.json`, `log.json`, the seat's `view.json`) replays
exactly and turns into a failing test:

```sh
go run ./cmd/repro cmd/repro/testdata/feedback/20260914T120000Z-fb01   # a committed example
go run ./cmd/repro -list <feedback-dir>             # the timeline of recorded answers
go run ./cmd/repro -at N <feedback-dir>             # stop at answer N
go run ./cmd/repro -emit-test <pkg> <feedback-dir>  # copy the snapshot and write a failing test skeleton
```

Exit status 0 means the rebuilt event stream matched the recording event for
event. A `DIVERGED` result names the first mismatching event; a corpus pin
change since the snapshot was taken is an expected cause.

## Licensing

gorge is Apache-2.0. Forge's card scripts are GPL-3.0, so they are **never
committed** to this repository: they live in the gitignored `.cards/`, and
`engine/cards/boundary_test.go` fails the build if a script is tracked in the
working tree, the index or `HEAD`. Replay fixtures strip embedded token-script
text before they are committed, and published coverage tables contain only
counts. The full story, including how the companion repositories handle it,
is on [The three repositories](repos/index.md#licensing-and-the-forge-card-scripts).
