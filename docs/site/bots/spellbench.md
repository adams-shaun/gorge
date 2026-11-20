# SpellBench v2 agents

For people who have, or want, an agent for the SpellBench benchmark, or want to run a gorge bot inside it. It covers the four ways gorge
speaks the v2 protocol: seating an external agent at a gorge table, serving a gorge bot as an agent, serving gorge as a v2 environment,
and offering v2 seats over HTTP. Field reference: [SpellBench v2 reference](../api/spellbench-v2.md). Commands were run on 2026-10-31
(the `mtgsim` example again on 2026-11-08); the code is under `server/spellbench` and `cmd/spellbench-gorge-{agent,env}`.

## What SpellBench is

[SpellBench](https://github.com/jackmaiorino/spellbench) is an external, engine-neutral benchmark for Magic agents: one protocol between a
*host* that runs games and an *agent* that answers decisions, with an engine-neutral observation and candidate vocabulary, so an agent can
play on any engine that implements the environment side. gorge's implementation, ported from SpellBench's gorge adapter (MIT), is
[`server/spellbench`](https://github.com/adams-shaun/gorge/blob/main/server/spellbench).

## Transport and framing

v2 runs over a child process's **stdin and stdout**: one request per line, one response per line. There is no socket, so an agent runs on
the host's machine, one process per seat per match (for another machine, use the [HTTP seat](http.md)). Framing is in
[`server/spellbench/wire`](https://github.com/adams-shaun/gorge/blob/main/server/spellbench/wire):

- One JSON object per line, `\n`-terminated, at most 8 MiB; a longer line is drained and refused.
- Strict JSON: integers only within ±(2^53 − 1), no duplicate keys, nesting at most 64, NFC card names. gorge's engine role enforces this; its
  agent and seat adapters read more loosely.
- Every message carries `"protocol": "spellbench/v2"` and a `request_id`, echoed in the response.
- Deck ids, the card-name domain id and the per-game digest chain are SHA-256 over canonical JSON (RFC 8785, integer subset).

## The agent role: host asks, agent answers

Four requests, captured from `mtgsim` driving SpellBench's minimal agent (it
always answers candidate 0) through a relay that logged every line:

| Request | When | Response |
|---|---|---|
| `hello` | once, at process start | `hello_ok`, naming the bot and any observation or extension requirements |
| `game_start` | once per match | `ack` |
| `choose` | every decision | `choice` with a `selection.candidate_id` |
| `game_over` | at the end | `ack` |

```json
{"request_type":"hello","protocol":"spellbench/v2","protocol_minor":0,"request_id":"r-1"}
{"protocol":"spellbench/v2","request_id":"r-1","response_type":"hello_ok","bot":{"name":"minimal","version":"1.0.0"}}
```

`game_start` carries everything fixed for the game: the seat, its decklist (and the opponent's when the table reveals it), format and rules,
the engine's capability profile, the card-name domain, decision limits, time control (`max_decision_ms`, a bank and an increment) and an
`agent_seed`. A `choose` carries the acting seat, a step counter, the decision `context` (`"kind":"priority"`...), an engine-neutral
`observation` of what the seat can see, and the `candidates` (`candidate_id`, `semantic`, `display_text`). The agent answers
`{"response_type":"choice","selection":{"candidate_id":0}}`. Object ids (`o-…`) are keyed per game. A gorge decision that is several picks
(an attack declaration, a multi-target spell) becomes a *group* of substeps. A `choose` was 1.5 to 24 KB in the captured game.

SpellBench's minimal agent, which always answers candidate 0, is a dozen lines of Python
([`server/spellbench/seat/testdata/minimal_bot.py`](https://github.com/adams-shaun/gorge/blob/main/server/spellbench/seat/testdata/minimal_bot.py)).

## Seat an external v2 agent at a gorge table

`-sb-agent name=/path/to/agent` registers the agent as a bot policy named
`name`, in `mtgsim` and in `gorged`. The path is executed directly with no
arguments and no shell, so wrap any arguments in a script. The adapter
([`server/spellbench/seat`](https://github.com/adams-shaun/gorge/blob/main/server/spellbench/seat))
starts one process per seat per match, handshakes, translates every gorge decision
into v2 candidates and the answer back into an intent.

```console
$ go run ./cmd/mtgsim -seats 2 -games 1 -verify -a sb-min \
    -sb-agent sb-min=server/spellbench/seat/testdata/minimal_bot.py
seed 0: 2 seats,    691 intents,   3577 events,  30 turns, winner=dimir-tempo, chain=5ecd9040415c3eba
  replay OK (chain 5ecd9040415c3eba)
```

In `gorged` the agent becomes one more offered policy, so a play-vs-bot
game, or an [HTTP bot](http.md), can play against it:

```console
$ ./gorged -addr 127.0.0.1:8080 ... -vsbot -sb-agent sb-gorge-bot=/path/to/gorge-as-v2.sh
$ python3 examples/bots/python/gorge_agent.py --server http://127.0.0.1:8080 --bot-policy sb-gorge-bot
```

A v2 seat is two-player and constructed-only, and plays only on tables with
automatic mana payment (its registry entry sets `NeedsAutoPay`).

**Failure is a forfeit.** If the agent fails to start, times out, crashes,
answers malformed JSON or an out-of-range candidate, the seat records a
forfeit cause (`timeout`, `transport_error`, `agent_error`,
`invalid_selection`), concedes at its next priority decision, and answers
with the default bot until then. The decision budget is the host's: 2 s per
decision in `gorged`, unbounded in `mtgsim` and the benches. `game_over` is
written and the agent gets 1 s to answer it before its process is killed. An
agent that has died or timed out doesn't receive it.

## Serve a gorge bot as a v2 agent

`cmd/spellbench-gorge-agent` wraps any registered gorge policy as a v2
agent, so gorge's bots can enter SpellBench arenas on any engine.

```console
$ go build -o spellbench-gorge-agent ./cmd/spellbench-gorge-agent
$ ./spellbench-gorge-agent -list-policies    # name, label, tier, one per line
```

`-list-policies` prints every policy `bots/all` links, but only the four heuristics (`bot`, `forge-heuristic`, `forge-heuristic-hold`,
`lethal-pressure`) can serve a match: the agent refuses an `Env` or `Search` policy at `game_start`
(`checkPolicy` in `server/spellbench/agent/agent.go`).

It has two worlds:

- `-world gorge` (default) reads gorge's own view from the
  `x_gorge_view_v1` extension, which a gorge environment offers. It is the
  exact path, but only a gorge host has the extension; under `-sb-agent` it
  refuses the match (`agent requirement not offered: extensions`).
- `-world neutral` rebuilds the decision from the neutral observation and
  candidates of *any* v2 engine, and needs the card corpus (`-corpus .cards`)
  or a frozen registry (`-registry file -registry-sha256 digest`). It
  translates the candidate kinds gorge's own engine never poses
  (`choose_cast_method`, `choose_cost_option`, `distribute`, `choose_pile`)
  onto the nearest gorge decision, so only a genuinely unmappable kind reaches
  the agent's fallback.

Because `-sb-agent` passes no arguments, wrap the command:

```sh title="gorge-as-v2.sh"
#!/bin/sh
exec /path/to/spellbench-gorge-agent -policy bot -world neutral -corpus /path/to/gorge/.cards
```

```console
$ go run ./cmd/mtgsim -seats 2 -games 20 -a sb-gorge -sb-agent sb-gorge=./gorge-as-v2.sh
```

That runs gorge's `bot` through the full v2 round trip against gorge's own `bot`, which makes it a fidelity check: on the same seeds,
compare it with a native `-a bot` run.

### Latency

Timed by a relay between `mtgsim` and the agent that logged each request's round trip (3 games per agent, 2026-10-20, on the machine
named on [Latency](http.md#latency)):

| Agent | `hello` | `game_start` | `choose` median | `choose` p95 |
|---|---|---|---|---|
| SpellBench minimal (Python) | 8 ms | 2 ms | 0.05 ms | 0.10 ms |
| gorge `bot`, neutral world | 412 ms (opens the corpus) | 3.6 ms | 0.18 ms | 0.31 ms |

The 0.4 s start is paid per match, because the host starts a process per
match.

## Serve gorge as a v2 environment

`cmd/spellbench-gorge-env` is the other direction: gorge as the *engine* a
SpellBench harness drives. Its requests are `hello` (answered with
`hello_ok` and the deck catalog), `validate_deck` (`deck_ok` or an error),
`reset` (start a game; answered with the first `decision`), `step`
(submit a candidate, with `expected_step` and a `semantic_echo` the
environment checks; answered with the next `decision` or the `terminal`).
`probe_resample` is answered `unsupported_request`: gorge's environment has
no probe. `validate_deck` accepts only the `pauper-bo1` format and the decks
of the environment's own catalog.

```console
$ go build -o spellbench-gorge-env ./cmd/spellbench-gorge-env
$ printf '%s\n' '{"request_type":"hello","protocol":"spellbench/v2","request_id":"a","protocol_minor":0}' | ./spellbench-gorge-env -corpus .cards
```

Errors are `response_type: "error"` with a code from a closed list
(`malformed_json`, `protocol_mismatch`, `expected_step_mismatch`,
`candidate_id_out_of_range`, `semantic_echo_mismatch`, `unsupported_deck`,
…; [`server/spellbench/protocol/errors.go`](https://github.com/adams-shaun/gorge/blob/main/server/spellbench/protocol/errors.go)).
Mana is paid by the engine's planner by default (`-manual-mana` offers
interactive taps instead).

## Offer v2 seats over HTTP

`gorged -sbv2` (with `-vsbot` or `-humans`) mounts `/api/sbv2/v1/tables/{t}/matches/{k}/` with `state`, `stream` and `send` routes. These
give a seat the same `choose` content as the stdio protocol, without its `clock`, under the native wire's bearer token. It is two-player,
constructed and automatic-mana only; a table outside that answers 409 and a client falls back to native. [SpellBench v2 reference](../api/spellbench-v2.md) has the details.
