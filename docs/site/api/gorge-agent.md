# gorge-agent/1

`gorge-agent/1` connects an out-of-process drafter, deck builder or player to gorge's limited tools. gorge runs every
draft and game and calls **out** to your program with batches of questions; your program only answers. It is built for
training and benchmark programs that want many games answered at once.

Use it under `cmd/labdemo` or the `draftlab` library. To sit at a table on a running server use the
[HTTP decision loop](http.md#the-decision-loop) or [SpellBench v2](spellbench-v2.md); `gorged` does not speak `gorge-agent/1`.
The transcripts below came from piping the golden messages through `cmd/extagent` and `examples/extagent/random_agent.py`
(2026-10-30, commit `834ac7d9a`).

## Try it

```sh
go run ./cmd/labdemo -player ext:exec:examples/extagent/random_agent.py -seed 1
```

[`random_agent.py`](https://github.com/adams-shaun/gorge/blob/main/examples/extagent/random_agent.py) is a complete
standard-library agent (about 200 lines). It answers at random, so some answers are refused; the run logs a line like
`extern: ext:py-random@1 decide refused (...): answered by fallback` and carries on.

### Where agents plug in

`labdemo`'s `-player`, `-opponent-players`, `-drafter` and `-builder` flags, and the
`draftlab` library through `draft/draftlab/extern`, accept these specs:

| Spec | Transport |
|---|---|
| `ext:exec:<path>[@<version>]` | gorge starts the program and talks over its stdin and stdout. The program's stderr is free-form logging. There is no way to pass arguments, so use a wrapper script to choose a checkpoint. |
| `ext:unix:<path>[@<version>]` | gorge connects to a Unix socket you already serve. |

`@<version>` pins a generation: the run refuses an agent whose `model_version`
differs. The agent is recorded as `ext:<agent>@<model_version>`, and the path or
socket never reaches a result or a manifest.

`cmd/extagent` is the reference agent as a binary:

```sh
go run ./cmd/extagent -listen unix:/tmp/gorge-agent.sock
go run ./cmd/labdemo -player ext:unix:/tmp/gorge-agent.sock -seed 1
```

## Framing

- One JSON object per line, UTF-8, ending in `\n`, at most 64 MiB. A longer line is a
  protocol violation, and the reader refuses it without buffering it whole.
- Every message carries `"v": 1` and a `"type"`.
- Both sides ignore unknown **fields**, so adding a field never bumps `v`. An unknown
  `type`, a wrong `v` or malformed JSON is a protocol violation, and the session ends.
- gorge writes `hello`, `batch`, `stream_end` and `bye`. The agent writes `hello`,
  `result` and `error`. gorge starts every exchange.
- Seeds are integers below 2^53, so every JSON implementation reads them exactly.

## A session

`>` lines are what gorge sends; the rest is what `cmd/extagent -policy first -agent demo` wrote back:

```text
> {"v":1,"type":"hello","protocol":"gorge-agent/1","gorge_revision":"...","kinds":["pick","build","decide"]}
{"v":1,"type":"hello","protocol":"gorge-agent/1","agent":"demo","model_version":"1","kinds":["pick","build","decide"],"view_omit":[...]}
> {"v":1,"type":"batch","batch":17,"items":[{"id":41,"kind":"pick", ... }]}
{"v":1,"type":"result","batch":17,"answers":[{"id":41,"pick":"c1"}]}
> {"v":1,"type":"stream_end","stream":"g12.1"}
> {"v":1,"type":"bye"}
```

Exact bytes of every message type are golden files in
[`draft/draftlab/extern/wire/testdata/`](https://github.com/adams-shaun/gorge/tree/main/draft/draftlab/extern/wire/testdata),
held by the tests of [`draft/draftlab/extern/wire`](https://pkg.go.dev/github.com/adams-shaun/gorge/draft/draftlab/extern/wire).

## Handshake

gorge speaks first. `gorge_revision` is informational: a VCS revision, a module
version or `devel`. The agent answers with its own `hello`:

| Field | Rule |
|---|---|
| `protocol` | must be `gorge-agent/1` |
| `agent` | your name, matching `^[a-z0-9][a-z0-9._-]{0,63}$` |
| `model_version` | identifies the weights, matching `^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`. It must change whenever the weights change |
| `kinds` | the roles served: a non-empty subset of `pick`, `build`, `decide` with no repeats. Using an agent in a role it did not declare is an error |
| `max_batch` | the most items you want per batch; 0 or absent means no preference. gorge's own default is 256 and its cap is 1024 |
| `skip_forced` | true: gorge answers a forced decision itself and never sends it. A decision is forced when it has one option, `min` and `max` of 1, and no payment choice |
| `payment_actions` | true: decisions carry `payment_actions`, and you may answer with `intent.payment`. False: casts pay through ordinary decisions |
| `target_hints` | true: priority options carry `target_hints` |
| `view_omit` | parts of the view you never read, so gorge need not project or send them: `library`, `available`, `archetype`, `ability_costs`, `effective_cost`, `own_deck`, `potential`, `card_lists`, `derived_chars`. An unknown name fails the handshake. The decision itself cannot be omitted |

A bad field fails the handshake with an error naming that field.

## Batches

```json
{"v":1,"type":"batch","batch":17,"items":[ITEM, ...]}
```

`batch` numbers start at 1 and increase within a session. Item ids are unique within a
session. One batch per session is in flight at a time, so a batch waits for your
`result`. Every item has `id`, `kind` and `seed`, and then:

| `kind` | Other fields | Question |
|---|---|---|
| `pick` | `set`, `round`, `pick_no`, `pack`, `picks`, `seen` | which card to take from `pack`. `picks` is your own pool so far; `seen` is the packs this seat saw earlier |
| `build` | `set`, `deck_size`, `pool` | which deck to play from `pool`. `deck_size` is at least 40 |
| `decide` | `stream`, `view` | how to answer `view.decision` |

A card in `pack`, `picks`, `seen` or `pool` looks like this; `mana_value`, `colors` and
the other facts are absent when gorge's corpus does not know the card:

```json
{"id":"c1","name":"Boreal Gravewalker","set":"FDN","number":"1","rarity":"C","mana_cost":"1W","mana_value":1,"colors":"B","type_line":"Creature — Zombie"}
```

A `view` is the same JSON a human seat reads from
[`GET …/view`](http.md): the deciding seat's redacted projection, with `view.decision`
always present and `view.viewer` the seat. A decision lists its legal `options`:

```json
{"seq":7,"player":1,"kind":"priority","prompt":"Your priority.","min":1,"max":1,"options":[
  {"index":0,"kind":"pass","label":"Pass priority"},
  {"index":1,"kind":"cast","label":"Cast Shock","obj":42},
  {"index":2,"kind":"concede","label":"Concede"}]}
```

`stream` names one seat of one game, `g<n>.<seat>`, and stays the same for that game.
A seat used outside a `draftlab` game is `u<n>.<seat>`.
`n` depends on scheduling, so never let it change an answer. It exists so you can keep
per-game state and drop it on `stream_end`.

## Answers

```json
{"v":1,"type":"result","batch":17,"answers":[ANSWER, ...]}
```

Answer every item id of the batch exactly once, in any order. Each answer has `id` and
exactly one of:

| Answer field | For | Shape |
|---|---|---|
| `pick` | `pick` | a card `id` from `pack` |
| `deck` | `build` | `{"main":[ids from pool],"basics":[W,U,B,R,G]}` |
| `intent` | `decide` | `{"choices":[option index values]}`, plus optional `rest`, `payment` and `announce`. `seq` and `player` may be left out; gorge fills them in from the decision |
| `error` | any | `{"code":"refused","message":"…"}`: you decline this one item |

### Refused and violating answers

A **refusal** costs one answer and the game goes on. These are refusals:

- an `error` answer with code `refused`;
- a pick that is not in the pack;
- a deck that fails validation;
- an intent the decision does not accept.

gorge counts it, logs one line and answers for you: with the `rating-colour` drafter,
the `best23` builder or the default bot's decision. The fallback is recorded like any
other answer. To debug a model, make refusals fatal with `labdemo -ext-strict` or
`Options.StrictRefusals`.

A **violation** ends the session: a wrong batch number, a missing, duplicate or unknown
id, an answer field that does not fit the item's kind, or more than one answer field.
After that, or after the agent exits, closes its connection or sends
`{"v":1,"type":"error","code":"bad_request"|"unsupported"|"internal","message":"…"}`,
every pending request fails. The run fails too, unless you asked for
`labdemo -ext-fail fallback` (`FailFallback`), which answers all later requests with the
same fallbacks. Failing is the default because a run that silently changed policy would
mislabel its data.

### No timeouts

gorge reads no clock here. A hung agent hangs its games until the caller cancels (Ctrl-C, or a context deadline).

### Other messages

| Message | Direction | Meaning |
|---|---|---|
| `{"v":1,"type":"stream_end","stream":"g12.1"}` | gorge to agent, no reply | that seat's game ended |
| `{"v":1,"type":"bye"}` | gorge to agent, no reply | the session ends: exit, or close the connection |

## The agent contract

An answer must be a **pure function of its item and the model version**: batch composition, order and stream numbers depend
on scheduling and must not change an answer. Then results do not depend on worker count, flush mode or batch size. Seed a
fresh generator from `item["seed"]` per item, as the example does. A broken contract is still recorded exactly and
replayable from logs and tapes, but a re-run may differ.

## How batches form

Batches come from games waiting at the same time, so an agent that wants batches of 64
should run with about 64 workers (`labdemo -workers`, `Options.Workers`), whatever the
core count. There are two flush modes (`labdemo -ext-flush`, `Options.Flush`):

| Mode | A batch is sent when |
|---|---|
| `barrier` (the default) | every live game in the process is waiting on an external agent, or a queue holds `max_batch` items |
| `ready` | the session is idle and anything is queued |

## Hidden information

An agent gets only the deciding seat's view, one seat's pack and picks, or one seat's pool, never the engine or another
seat's view. One process serving several seats could combine their views, as one program running two built-in bots could;
use one endpoint per seat to keep them isolated.

## Replay without the model

Your answers are recorded. Picks and decks are in the event log, and a game's accepted
intents are in its tape (`Options.RecordTape`). `event.ReplayLog`, `Lab.ReplayTape` and
`Lab.VerifyEventTapes` replay them with the model gone; `labdemo -verify-tapes` does this
for every pod game.

## Versioning

`wire.Protocol` is `gorge-agent/1` and `wire.Version` is 1. The version bumps only on a
breaking change. Additive fields do not bump it, the same rule as
[gorge's HTTP wire](index.md#versioning-and-compatibility). The golden files named above
are the check that a change is additive.
