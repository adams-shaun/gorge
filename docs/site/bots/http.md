# Playing a seat over HTTP

For bot authors in any language: the decision round trip with real requests and responses from a live `gorged` (2026-10-31 unless a
line says otherwise), how to answer each decision kind, errors and pitfalls. Field reference: [HTTP API](../api/http.md); working
agents: [Python and Java examples](examples.md).

The HTTP seat is what the browser plays through: a "human" seat parks each decision until someone posts an answer. A bot is a program
holding that seat's token.

## Start a server that has a seat for you

Build with `go build ./cmd/gorged` after `make fetch-cards`. Use a persistence directory of your own (`-dir`); two servers sharing one corrupt each other.

=== "On demand: POST /api/games"

    ```console
    $ ./gorged -addr 127.0.0.1:8080 -dir /tmp/gorge-bots -feedback /tmp/gorge-bots/feedback \
        -tables 0 -vsbot -max-on-demand-tables 0 -pace 0 -prewarm=false -bot-policy bot
    ```

    `-vsbot` arms `POST /api/games`, which creates a private two-seat table (you are seat 0, against a hosted bot) and returns your token.
    `-max-on-demand-tables 0` lifts the default cap of 32 retained tables; `-pace 0` removes the 50 ms pause after every decision.

=== "Fixed seat: -humans"

    ```console
    $ ./gorged -addr 127.0.0.1:8080 -dir /tmp/gorge-bots -feedback /tmp/gorge-bots/feedback \
        -tables 1 -seats 2 -humans 0 -seat-token docs-token -pace 0 -prewarm=false -bot-policy bot
    ```

    `-humans 0` makes seat 0 of table `t1` an HTTP seat; `-seat-token` fixes its token (omit it and the server mints one and prints the join URL).
    Keep `-spectator` at its `public` default, which hides every hand from an unauthenticated spectator; `omniscient` opens every hand.

Commands below use `GORGE=http://127.0.0.1:8080` and `TOKEN=<the seat's token>` (`docs-token` on the fixed seat).

## The round trip

```mermaid
sequenceDiagram
  autonumber
  participant B as Your bot
  participant H as gorged (httpapi)
  participant S as HumanSeat (your seat)
  participant E as Engine + opponent bot
  B->>H: POST /api/games {"format":"constructed","bot_policy":"bot"}
  H-->>B: 200 {"table":"g1","match":1,"seat":0,"token":"77885d…"}
  loop until the match is over
    E->>S: Decide(view, decision): parks it
    B->>H: GET /api/tables/g1/matches/1/pending?seat=0 (Bearer token)
    H->>S: copy of the parked decision
    H-->>B: 200 {"seq":38,"kind":"priority","options":[…]}
    opt the policy wants the board
      B->>H: GET …/view?seat=0
      H-->>B: 200 the seat's redacted view
    end
    B->>H: POST …/intent {"seq":38,"player":0,"choices":[0]}
    H->>S: Validate, claim the parked decision
    S->>E: Submit
    E-->>H: accepted (a refusal is a 409 and re-parks the decision)
    H-->>B: 204 No Content
    E->>E: the game advances, the bot opponent plays
    B->>H: GET …/pending?seat=0
    H-->>B: 409 nothing pending (yet): back off and retry
  end
  B->>H: GET /api/tables/g1/matches
  H-->>B: 200 [{"state":"finished","winner":1,…}]
```

`POST .../intent` waits for the engine's verdict. `204` means accepted and the game has moved on. `409` means refused (wire checks or the
rules engine); the decision stays pending, unchanged, and you can answer again. The first `POST` for a decision claims it, so a duplicate is a
`409`. After a `204`, `GET .../pending` answers `409` until the game asks your seat something new. Instead of polling, wait for a `decision`
frame on [SSE](#push-instead-of-poll-sse).

## Authentication

Every seat-scoped request carries the seat's token, as
`Authorization: Bearer <token>` (preferred) or `?token=<token>`. The token
is the credential: the server derives the table and seat from it and refuses
any request whose table or `?seat=` disagrees.

```console
$ curl -s -w ' [%{http_code}]\n' "$GORGE/api/tables/t1/matches/1/pending?seat=0"
{"code":"unauthorized","message":"request does not hold a seat claim"} [401]
$ curl -s -w ' [%{http_code}]\n' -H 'Authorization: Bearer docs-token' "$GORGE/api/tables/t1/matches/1/pending?seat=1"
{"code":"forbidden","message":"claim holds seat 0, not the requested seat 1"} [403]
```

Tokens are opaque, 16 random bytes in hex, held in server memory only (lost on restart), and are not general authentication. Run the
server on a network you trust, or behind TLS.

## The endpoints a bot uses

| Endpoint | Auth | Purpose |
|---|---|---|
| `POST /api/games` | none | create a game against a hosted bot (needs `-vsbot`) |
| `GET /api/decks`, `GET /api/bot-policies` | none | what the server can deal and seat |
| `GET /api/tables/{t}/matches/{k}/pending?seat=N` | seat | the decision asked of your seat, or 409 |
| `GET /api/tables/{t}/matches/{k}/view?seat=N` | seat | your seat's redacted view (`&seq=` for a past point) |
| `POST /api/tables/{t}/matches/{k}/intent` | seat | answer the pending decision |
| `GET /api/tables/{t}/matches` | none | match state, winner, turns |
| `GET /api/tables/{t}/matches/{k}/events?seat=N&since=S` | seat | your seat's redacted event log |
| `GET /api/stream`, `POST /api/subscribe` | seat token for a private table | server-sent events: push notifications |

### Create a game

```console
$ curl -s -X POST -d '{"format":"constructed","bot_policy":"bot","human_deck":"mono-green-stompy","bot_deck":"mono-red-goblins"}' $GORGE/api/games
{"table":"g7","match":1,"seed":6673408969535470007,"seat":0,"token":"b63edb2dcd4ea366426123ebd99fe424","bot_policy":"bot","join":"/t/g7?seat=0&token=b63edb2dcd4ea366426123ebd99fe424"}
```

Every field is optional: `format` defaults to `constructed`, a missing deck is dealt at random, a missing `bot_policy` takes
the server's `-bot-policy`. `mulligans` (0 to 6) overrides the server's London mulligan allowance. Deck ids come from
`GET /api/decks`; an unknown one is a `400`. The seed is drawn by the server and returned; a client can't choose it. Your seat
is always seat 0, and the `join` path is the same game in a browser, which is a good way to watch your bot play. The table
appears a moment after the response, so retry a `404` on its first poll briefly.

### Read the pending decision

```console
$ curl -s -H "Authorization: Bearer $TOKEN" "$GORGE/api/tables/g2/matches/1/pending?seat=0"
{"seq":18,"player":0,"kind":"starting_player","prompt":"You won the toss. Choose who takes the first turn.","min":1,"max":1,"options":[{"index":0,"kind":"player","label":"You","player":0},{"index":1,"kind":"player","label":"Bot","player":1}]}
```

Every decision has the same shape: `seq` (which decision this is), `player`
(your seat), `kind`, a human-readable `prompt`, `min`/`max` (how many options
to pick) and `options`, each with an `index` and an option `kind`. `obj` names
the card or permanent the option is about (look it up in the view by `id`),
`player` a seat. Some decisions carry constraints a valid answer must meet
(`group`, `required`, `maxSum`, `minSum`, `groupLimit`, `payer_life`…); the
[reference](../api/http.md) lists them all.

When nothing is pending the answer is `409` with the server's reason:
`"host: no decision pending for player 0"` while the opponent thinks,
`"host: match 1 is finished, nothing pending"` once it is over.

### Read the view

```console
$ curl -s -H "Authorization: Bearer $TOKEN" "$GORGE/api/tables/g208/matches/1/view?seat=0" | jq '{viewer, visibility, turn, step, over, winner, them: .players[1] | {hand_size, hand}}'
{"viewer": 0, "visibility": "seat", "turn": 0, "step": "untap", "over": false, "winner": null, "them": {"hand_size": 7, "hand": null}}
```

The opponent's `hand` is `null` where yours lists cards (`id`, `name`, `types`, `mana_cost`, `power`, `toughness`, `tapped`, ...). Your
`library` is in a canonical order that never reveals the real one. The view embeds the pending `decision`, so a policy that always wants
the board can read both from one `GET /view`, at about four times the cost ([Latency](#latency)).

### Answer

```console
$ curl -s -i -X POST -H 'Authorization: Bearer docs-token' -d '{"seq":22,"player":0,"choices":[0]}' "$GORGE/api/tables/t1/matches/1/intent"
HTTP/1.1 204 No Content
```

An intent is `{"seq", "player", "choices"}`: echo the decision's `seq` and
`player`, and list the option indices you pick. Unknown fields are refused,
so a typo is a `400`, not a silent default. Two other answer shapes exist for
casting spells, covered next: `payment` and `announce`.

### Casting a spell: payment actions

On a table with automatic mana payment (the default for `-vsbot` games), a spell you cannot pay from floating mana is not a `cast` option.
It arrives as a **payment action** on the priority decision, with complete plans saying which permanents the engine will tap:

```json
"payment_actions":[{"id":"a1ef0fac…","cast":{"object":28,"face":0,"origin":"hand"},"label":"Cast Elvish Mystic",
  "plans":[{"version":1,"id":"79b02ab3…","cost":{"generic":0,"mana":[0,0,0,0,1,0]},"activations":[…],"pool_spend":[…],"pool_after":[…]}]}]
```

Answer with the action's id and one plan **echoed verbatim** instead of `choices`:

```json
{"seq":44,"player":0,"payment":{"action_id":"a1ef0fac…","plan":{"version":1,"id":"79b02ab3…", "…": "exactly as received"}}}
```

The server re-checks the plan against the live game. A bot may instead `activate` its lands by hand; once the mana floats, the spell is an
ordinary `cast` option. `{"announce":{"action_id":…}}` starts the cast without a plan and pays in a follow-up mana window, as the browser's
manual payment does.

## Answering each decision kind

Every kind uses the same answer shape; what differs is how many options to pick and what the order means. The option kinds are
examples, not a closed list.

| Decision `kind` | What it asks | How to answer | Example option kinds |
|---|---|---|---|
| `priority` | What to do now | exactly one option, or a `payment` answer. Never pick `concede` by accident: it ends the game. | `pass`, `play_land`, `cast`, `activate` (mana abilities), `ability`, `concede` |
| `mulligan` | Keep or mulligan; then which cards to bottom | `keep`/`mulligan` phase: one option. Bottoming phase: exactly `min` distinct `bottom` options | `keep`, `mulligan`, `bottom` |
| `starting_player` | You won the toss: who plays first? | one option; `player` is the seat that would start | `player` |
| `target` | Targets for a spell or ability | `min`..`max` options; `target_effect` says what will happen to them (`{"api":"Destroy","removal":{"kind":"destroy"}}`) so you can avoid your own things | `permanent`, `player`, `spell`, `trigger` |
| `attackers` | Declare attackers | any subset, **at most one option per creature** (`obj`) and per `group`; include every `required` option | `attacker` |
| `blockers` | Declare blockers | any subset, at most one option per `group` (each blocker's group is `blocker:<id>`); `attacker` names who it blocks | `block` |
| `choose` | A list pick (X values, costs, searches, names, colours…) | `min`..`max` options; the shared option kind says what is chosen | `x`, `sacrifice`, `discard`, `search`, `dig`, `name`, `color`, `yes`, `pay_life`… |
| `modes` | A modal spell's modes, or "pay or decline" | `min`..`max` options; `mode` is `unless_pay` / `unless_decline` on a pay-or-decline ask | `mode` |
| `trigger_optional` | Put an optional trigger on the stack? | one of `yes` / `no` | `yes`, `no` |
| `trigger_order` | Order your simultaneous triggers | **all** options, as a permutation. The first you list goes on the stack first and so resolves **last** | `trigger` |
| `arrange` | Scry, surveil, look-at-the-top orders | an ordered subset, pile A: the ones you pick stay on top, the first you list closest to the top. The rest go where the shared option kind says; `"rest":[…]` orders them when `restable` is true | `bottom`, `graveyard`, `exile`, `hand` |
| `replacement` | Several replacement effects apply: which first? | one option | `replacement`, `apply`, `decline` |
| `commander_zone` | Your commander died or was exiled (or would go to hand/library): to the command zone? | one option; on an exile ask, a `leave` whose `mode` is `adventure_recast` / `foretell_cast` / `plot_cast` keeps that cast-from-exile permission | `command_zone`, `leave` |

A rules-ignorant fallback that is accepted almost always: at `priority` the
`pass` option; anywhere else the first `min` options. The example agents
try their policy's answer, then a few random answers that respect `min`,
`max`, `required` and the exclusivity rules, then that fallback.

## Errors

Every error body is `{"code": …, "message": …}`. Captured:

| Status | `code` | When | Example message |
|---|---|---|---|
| 400 | `bad_request` | malformed JSON, an unknown field, a missing `?seat=` on `/pending` | `malformed JSON body: json: unknown field "choice"` |
| 401 | `unauthorized` | no token, or an unknown one | `request does not hold a seat claim` |
| 403 | `forbidden` | the token is for another seat or table | `claim holds seat 0, not the requested seat 1` · `claim does not hold a seat at this table` |
| 404 | `not_found` | no such match (yet) | `host: not found` |
| 409 | `conflict` | nothing pending; a stale `seq`; an answer the decision does not allow; an answer the rules engine refuses | `intent seq 1, pending decision seq 22` · `choice 7 out of range (2 options)` · `attacker 38 declared against more than one defender` |

A refused intent leaves the decision pending and the game exactly where it was: read `/pending` again and send a different
answer. That holds for a refusal from the rules engine too.

## Timeouts and disconnects

`gorged` gives an HTTP seat **no deadline**: a parked decision waits until answered. If your bot dies, its game waits, and the seat's
*caretaker* answers only when the table shuts down. An abandoned table still counts against `-max-on-demand-tables` (`host: on-demand table
limit 32 reached`); `0` lifts the cap. `-manabrew-think` is the one think-time knob, and applies only with the ManaBrew wire on.

## Push instead of poll (SSE)

`GET /api/stream` opens a server-sent-event stream. The first frame is `hello` with a session id; post it to `/api/subscribe` with your table
and `"mode":"focus"`, and the stream carries a `snapshot`, every public `event`, and a `decision` frame naming the seat whenever a decision is
posed. A play-vs-bot table is private: open the stream with your seat's token (`Authorization: Bearer` or `?token=`) or the subscribe is a
`404` (`host: not found`). A `-humans` table is public and needs no token.

```console
$ curl -sN -H "Authorization: Bearer $TOKEN" $GORGE/api/stream      # keep this open
$ curl -s -X POST -d '{"session":"sc4947…","table":"g2","mode":"focus"}' $GORGE/api/subscribe
```

It is a *spectator* stream: it never shows your seat's options. Use it as a wake-up: on a `decision` frame for your seat, `GET /pending`;
on `match_end`, stop. It sends a `: ping` comment every 15 s, resumes from `Last-Event-ID` within 30 s of a disconnect, and closes with an
`overflow` frame if your reader falls 256 frames behind.

## Latency

`examples/bots/python/measure_latency.py` plays ten games answering every decision minimally and times each call (2026-10-20, loopback,
Ryzen 9 9950X, `gorged -pace 0`):

| Call | Poll median / p95 | SSE median / p95 |
|---|---|---|
| `GET pending` | 0.12 / 0.19 ms | 0.20 / 0.29 ms |
| `GET view` | 0.50 / 1.09 ms | 0.53 / 1.52 ms |
| `POST intent` | 0.11 / 0.16 ms | 0.12 / 0.18 ms |
| answer to next own decision visible | 0.13 / 0.49 ms | 0.24 / 0.67 ms |

One decision costs about 0.25 ms of HTTP, or 0.75 ms if the bot also reads the view. Polling flat out is fastest but burns a core (the
examples back off from 2 ms to 50 ms); SSE costs one wake-up hop and nothing while idle. There is no long-poll or batching, so training
that needs engine-speed rollouts should run in Go.

## Pitfalls

- **One creature, one defender.** An `attackers` decision has one option per (creature, defender) pair. Pick at most one per `obj`, or get a
  `409` (`attacker 38 declared against more than one defender`).
- **Private tables need the token on the stream** ([above](#push-instead-of-poll-sse)).
- **Spectator visibility.** On a `-humans` server the default is `public`, so `/api/stream` and a seat-less `/view` hide every hand. Don't pass
  `-spectator omniscient` on a table with people at it.
- **Concede is an option.** A random policy that samples every priority option concedes within a few turns. Filter it out.
- **The first poll can 404.** A fresh table goes live a moment after `POST /api/games` returns.
