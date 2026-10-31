# External agents: the Python example

`random_agent.py` is a ~200-line, standard-library-only Python agent that
speaks gorge's external-agent wire, `gorge-agent/1`. It is the starting point
for a training team: it handshakes, answers draft picks, builds a deck from a
pool, and answers game decisions — all uniformly at random, so it plays badly
on purpose. It is the Python twin of `draft/draftlab/extern/refagent` and need not
agree with it bit for bit.

The wire protocol of record is
[`docs/site/api/gorge-agent.md`](../../docs/site/api/gorge-agent.md):
one JSON object per line, `{"v":1,"type":...}`, gorge writing `hello`,
`batch`, `stream_end` and `bye`, and the agent writing `hello`, `result` and
`error`.

## Run the example

Through the lab:

```sh
go run ./cmd/labdemo -player ext:exec:examples/extagent/random_agent.py -seed 1
```

The script has a `#!/usr/bin/env python3` shebang and mode 755, so
`ext:exec:` runs it directly — by design, `ext:exec:` takes no arguments; a
wrapper script selects a checkpoint.

## Serve it yourself

Over stdin/stdout:

```sh
examples/extagent/random_agent.py
```

Or run gorge's reference agent on a Unix socket and point `ext:unix:` at it:

```sh
go run ./cmd/extagent -listen unix:/tmp/gorge-agent.sock
go run ./cmd/labdemo -player ext:unix:/tmp/gorge-agent.sock -seed 1
```

## The agent contract

An answer must be a **pure function of its item and the model version**.
Batch composition, batch order and stream ids are scheduling-dependent, so an
agent must never let them change an answer; this example seeds a fresh
`random.Random(item["seed"])` per item for exactly that reason. `model_version`
identifies the weights and must change whenever they do; a spec's
`@<version>` half (`ext:exec:<path>@gen7`) pins a generation, and a run
refuses an agent that does not match.

## Hidden information

An agent receives only the deciding seat's own Seat-visibility view (one
seat's PickView at a pick, one seat's pool at a build) and nothing else — never
the engine, a botpolicy Board, a feed, a redeal or another seat's view. One
process serving several seats sees each such redacted view on its one
connection; isolate seats with one endpoint per seat.

## A hung agent hangs the run

There is no per-request timeout: a deadline comes only from the caller's
context. An agent that stops answering stalls its games until the program
running gorge is cancelled. Cancel it (Ctrl-C) to recover.
