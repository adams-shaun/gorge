# SpellBench v2 agents

This page is for people who have, or want, an agent for the SpellBench
benchmark, and for anyone who wants to run a gorge bot inside it. It explains
what SpellBench is, the messages its v2 protocol exchanges, and the three ways
gorge speaks it: seating an external v2 agent at a gorge table, serving a
gorge bot as a v2 agent, and serving gorge as a v2 environment. Message and
field reference: [SpellBench v2 reference](../api/spellbench-v2.md).
Everything shown was run on 2026-10-20 at `ad3777023`; the `mtgsim` and
environment examples, the policy listing and the fidelity check below were
run again on 2026-10-21 at `513a2a0c3` with the same results. The two
policy listings were updated on 2026-10-27 at `25d71cadc` for
`forge-heuristic-hold`, registered since.

## What SpellBench is

[SpellBench](https://github.com/jackmaiorino/spellbench) is an external,
engine-neutral benchmark for Magic-playing agents. It defines one protocol
between a *host* that runs games and an *agent* that answers decisions, and
an engine-neutral observation and candidate vocabulary, so that the same
agent can play on any engine that implements the environment side, and
agents can be rated against each other on fixed deck pools. gorge's
implementation, ported from the SpellBench repository's gorge adapter (MIT),
lives in
[`server/spellbench`](https://github.com/adams-shaun/gorge/blob/main/server/spellbench).

## Transport and framing

v2 runs over a child process's **stdin and stdout**: the host starts the
agent (or environment) as a process, writes one request per line, and reads
one response per line. There is no network socket, so an agent always runs
on the same machine as its host, one process per seat per match. (For a bot
on another machine, use the [HTTP seat](http.md).)

The framing rules
([`server/spellbench/wire`](https://github.com/adams-shaun/gorge/blob/main/server/spellbench/wire)):

- **One JSON object per line**, `\n`-terminated (a trailing `\r` is
  tolerated), at most 8 MiB per line. A longer line is drained and refused,
  so framing resynchronises at the next line.
- **Strict JSON**: integers only (no floats), within ±(2^53 − 1); no
  duplicate keys; nesting at most 64 deep; precomposed (NFC) card names.
- **Every message** carries `"protocol": "spellbench/v2"` and a
  `request_id`, echoed in the response.
- **Canonical JSON** (RFC 8785, for this integer-only subset) is what the
  protocol hashes: deck ids, the card-name domain id and the per-game digest
  chain are SHA-256 over canonical bytes, so two implementations agree on
  them byte for byte.

## The agent role: host asks, agent answers

Four requests, captured from `mtgsim` driving SpellBench's minimal agent (it
always answers candidate 0) through a relay that logged every line:

| Request | When | Response |
|---|---|---|
| `hello` | once, at process start | `hello_ok`, naming the bot and any observation or extension requirements |
| `game_start` | once per match | `ack` |
| `choose` | every decision | `choice` with a `selection.candidate_id` |
| `game_over` | at the end, best effort | `ack` |

```json
{"request_type":"hello","protocol":"spellbench/v2","protocol_minor":0,"request_id":"r-1"}
{"protocol":"spellbench/v2","request_id":"r-1","response_type":"hello_ok","bot":{"name":"minimal","version":"1.0.0"}}
```

`game_start` tells the agent everything fixed for the game: its seat, its own
decklist (and the opponent's when the table reveals it), the format and
rules, the engine's capability profile (`decision_kinds`, what the
observation includes), the card-name domain, the decision limits and the
time control (`max_decision_ms`, a bank and an increment), and an
`agent_seed` for the agent's own randomness.

A `choose` carries the decision: the acting seat, a step counter, the
decision's `context` (`"kind":"priority"`…), an engine-neutral
`observation` of what the seat can see, and the candidates:

```json
{"request_type":"choose","protocol":"spellbench/v2","request_id":"r-5","game_id":"g-0000000000000000",
 "clock":{"max_decision_ms":0,"remaining_ms":600000},
 "decision":{"acting_seat":"p0","seat_step":2,
   "group":{"group_id":2,"substep_index":0,"substep_count":1},
   "context":{"kind":"priority","source":null,"purpose":null,"text":null,"rewind":false},
   "observation":{"viewer":"p0","turn":1,"phase_step":"precombat_main","active_seat":"p0","priority_seat":"p0",
     "players":[{"seat":"p0","life":20,"hand_count":7,"library_count":53,
       "hand":[{"object_id":"o-4d0777330094f7aa","card_name":"Stoneforge Mystic","zone":"hand",
                "characteristics":{"types":["creature"],"subtypes":["kor","artificer"],"colors":["white"],
                                   "mana_value":2,"power":1,"toughness":2,"keywords":[]}}, "…"]}, "…"],
     "stack":[], "pending_triggers":[], "known":[]},
   "candidates":[
     {"candidate_id":0,"semantic":{"kind":"pass"},"display_text":null},
     {"candidate_id":1,"semantic":{"kind":"play_land","face":0,
       "source":{"object_id":"o-de201734884be0b8","card_name":"Plains","owner_seat":"p0","controller_seat":"p0","zone":"hand"}},
      "display_text":null}],
   "extensions":{}}}
```

```json
{"protocol":"spellbench/v2","request_id":"r-5","response_type":"choice","selection":{"candidate_id":0}}
```

Object ids (`o-…`) are keyed per game, so an agent cannot correlate them
across games. A gorge decision that is several picks (an attack declaration,
a multi-target spell) becomes a *group* of substeps, one pick each. A
`choose` request in that game was 1.5 to 24 KB.

The whole of SpellBench's minimal agent is a dozen lines of Python
([`server/spellbench/seat/testdata/minimal_bot.py`](https://github.com/adams-shaun/gorge/blob/main/server/spellbench/seat/testdata/minimal_bot.py)):

```python
for line in sys.stdin.buffer:
    request = json.loads(line)
    reply = {"protocol": "spellbench/v2", "request_id": request["request_id"]}
    if request["request_type"] == "hello":
        reply.update(response_type="hello_ok", bot={"name": "minimal", "version": "1.0.0"})
    elif request["request_type"] == "choose":
        reply.update(response_type="choice", selection={"candidate_id": 0})
    else:
        reply.update(response_type="ack")
    sys.stdout.write(json.dumps(reply) + "\n")
    sys.stdout.flush()
```

## Seat an external v2 agent at a gorge table

`-sb-agent name=/path/to/agent` registers the agent as a bot policy named
`name`, in `mtgsim` and in `gorged`. The path is executed directly with no
arguments and no shell, so wrap any arguments in a script. The adapter
([`server/spellbench/seat`](https://github.com/adams-shaun/gorge/blob/main/server/spellbench/seat))
starts one process per match, handshakes, translates every gorge decision
into v2 candidates and the answer back into an intent.

```console
$ go run ./cmd/mtgsim -seats 2 -games 1 -verify -a sb-min \
    -sb-agent sb-min=server/spellbench/seat/testdata/minimal_bot.py
seed 0: 2 seats,    691 intents,   3577 events,  30 turns, winner=dimir-tempo, chain=6766ab2087f96884
  replay OK (chain 6766ab2087f96884)
```

In `gorged` the agent becomes one more offered policy, so a play-vs-bot
game, or an [HTTP bot](http.md), can play against it:

```console
$ ./gorged -addr 127.0.0.1:8093 ... -vsbot -sb-agent sb-gorge-bot=/path/to/gorge-as-v2.sh
$ curl -s http://127.0.0.1:8093/api/bot-policies | jq -c '[.policies[] | [.name, .label]]'
[["bot","Bot"],["forge-heuristic","Forge heuristic"],["forge-heuristic-hold","Forge heuristic + hold"],["lethal-pressure","Lethal Pressure"],["sb-gorge-bot","SpellBench v2 agent"]]
$ python3 gorge_agent.py --server http://127.0.0.1:8093 --bot-policy sb-gorge-bot
...
match finished after 21 turns in 1.5 s: winner us (seat 0, tron)
```

A v2 seat is two-player and constructed-only, and plays only on tables with
automatic mana payment (its registry entry sets `NeedsAutoPay`).

**Failure is a forfeit.** If the agent fails to start, times out, crashes,
answers malformed JSON or an out-of-range candidate, the seat records a
forfeit cause (`timeout`, `transport_error`, `agent_error`,
`invalid_selection`), concedes at its next priority decision, and answers
with the default bot until then. The decision budget is the host's: 2 s per
decision in `gorged`, unbounded in `mtgsim` and the benches. `game_over` is
written and the process stopped immediately, so an agent must not rely on
receiving it.

## Serve a gorge bot as a v2 agent

`cmd/spellbench-gorge-agent` wraps any registered gorge policy as a v2
agent, so gorge's bots can enter SpellBench arenas on any engine.

```console
$ go build -o spellbench-gorge-agent ./cmd/spellbench-gorge-agent
$ ./spellbench-gorge-agent -list-policies
az-redeal	az-redeal (AlphaZero search)	experimental
bot	Bot	production
forge-heuristic	Forge heuristic	experimental
forge-heuristic-hold	Forge heuristic + hold	experimental
lethal-pressure	Lethal Pressure	experimental
sb-tactical	sb-tactical (SpellBench tactical heuristic)	experimental
```

It has two worlds:

- `-world gorge` (default) reads gorge's own view from the
  `x_gorge_view_v1` extension, which a gorge environment offers. It is the
  exact path, but only a gorge host has the extension; under `-sb-agent` it
  refuses the match (`agent requirement not offered: extensions`).
- `-world neutral` rebuilds the decision from the neutral observation and
  candidates of *any* v2 engine, and needs the card corpus (`-corpus .cards`)
  or a frozen registry (`-registry file -registry-sha256 digest`). It
  translates the candidate kinds gorge's own engine never poses
  (`choose_cast_method`, `choose_option`, `choose_cost_option`, `distribute`,
  `choose_pile`) onto the nearest gorge decision, so only a genuinely
  unmappable kind reaches the agent's fallback.

Because `-sb-agent` passes no arguments, wrap the command:

```sh title="gorge-as-v2.sh"
#!/bin/sh
exec /path/to/spellbench-gorge-agent -policy bot -world neutral -corpus /path/to/gorge/.cards
```

```console
$ go run ./cmd/mtgsim -seats 2 -games 20 -a sb-gorge -sb-agent sb-gorge=./gorge-as-v2.sh
```

That runs gorge's `bot` through the full v2 round trip against gorge's own
`bot`. It is a useful fidelity check: over seeds 0–19 with Death & Taxes
against Dimir Tempo, the bot wins **17 of 20 games natively** (`-a bot`) and
**17 of 20 through the neutral v2 translation** (2026-10-28 at `315962169`
with the `cmd/mtgsim` payment-plan guard; 20 games took 17 s). Before that
guard the neutral seat received no payment plans and won 1 of 20: it could
still pick the engine's manual cast options, but could never apply a planned
payment.

### Latency

Timed by a relay between `mtgsim` and the agent that logged each request's
round trip (3 games per agent, 2026-10-20):

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
$ printf '%s\n' '{"request_type":"hello","protocol":"spellbench/v2","request_id":"a","protocol_minor":0}' \
    '{"request_type":"validate_deck","protocol":"spellbench/v2","request_id":"v","format":"pauper-bo1","deck":{"catalog_id":"Burn"}}' \
  | ./spellbench-gorge-env -corpus .cards
{"catalog":[{"catalog_id":"Wildfire","decklist":[{"count":4,"name":"Twisted Landscape"}, …
{"protocol":"spellbench/v2","request_id":"v","response_type":"deck_ok"}
```

Errors are `response_type: "error"` with a code from a closed list
(`malformed_json`, `protocol_mismatch`, `expected_step_mismatch`,
`candidate_id_out_of_range`, `semantic_echo_mismatch`, `unsupported_deck`,
…; [`server/spellbench/protocol/errors.go`](https://github.com/adams-shaun/gorge/blob/main/server/spellbench/protocol/errors.go)).
Mana is paid by the engine's planner by default (`-manual-mana` offers
interactive taps instead).
