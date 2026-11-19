# ManaBrew wire

[ManaBrew](https://docs.manabrew.app/protocol/) is a published Magic client protocol. `gorged -manabrew` serves a seat over it, so a
ManaBrew client can play a gorge table with the same seat tokens as the native wire. This page is for client authors: the routes,
what gorge adds and what it leaves out. The message schemas are ManaBrew's own, and `server/protocol/manabrew` holds the Go types.
gorge's code is written independently from the published specification (CC BY 4.0); see that package's `ATTRIBUTION` file.

## Serve it

```sh
go run ./cmd/gorged -addr 127.0.0.1:8080 -dir /tmp/gorge-mb -vsbot -manabrew
```

```text
gorged: manabrew routes mounted at /api/manabrew/v0/tables/
```

`-manabrew` needs `-humans` or `-vsbot`, because the wire only serves seats. `-manabrew-addr` serves the routes on a separate listener
and implies `-manabrew`. `-manabrew-think` sets a think timeout for human seats while the wire is on, after which the seat's caretaker
bot answers; without it a seat waits indefinitely. Without the flags nothing is mounted, and these paths answer the JSON 404.

The browser client's **Protocol** switch plays a seat over this wire ([Play in the browser](../guide/play.md#the-table)). With the routes on
the main listener, gorged adds `<meta name="gorge-manabrew">` to the page it serves, and the client offers the switch only when it finds that
tag. A separate `-manabrew-addr` is another origin, so the page doesn't carry the tag there.

## Routes

All three are under `/api/manabrew/v0/tables/{t}/matches/{k}/`. Each needs the seat's token, as `Authorization: Bearer <token>` or
`?token=`; the seat comes from the token, never from the request. [The HTTP API](http.md#who-may-call-what) says how to get one, and the
same 401 and 403 rules apply.

| Method | Path | What |
|---|---|---|
| `GET` | `stream` | Server-Sent Events: the current state, the open prompt if there is one, then a live feed of engine messages. It never replays history |
| `POST` | `send` | One client message: a prompt response, a directive such as a concession, or `restoreSnapshot`, which asks for an undo |
| `GET` | `state` | The same state and open prompt, as a one-shot JSON array, for a client that never opens a stream |

`$GORGE` is the server's URL and `$TOKEN` the seat's token:

```console
$ curl -s -H "Authorization: Bearer $TOKEN" $GORGE/api/manabrew/v0/tables/g2/matches/1/state
[{"kind":"state","gameView":{"gameId":"g2/1","turn":0,"step":"untap", …}},
 {"kind":"prompt","promptId":22,"decidingPlayerId":"player-0","input":{"handCardIds":["o28", …],"mulliganCount":0,"type":"mulligan"}}]
```

Engine messages are `{"kind": …}` objects: gorge sends `state` (always a full `gameView`, never a patch), `prompt` and `error`. It sends no
`engineDirective`, and it refuses the recording and repeat directives. The census against upstream protocol 6.4.1 is `server/protocol/manabrew/testdata/upstream_census.tsv`. A rejected `send` comes back in its
HTTP reply and is also pushed on any open stream, followed by the prompt it answered when that prompt is still open, so a client
that drops a refused answer and waits for its next prompt gets one. Its code is one of the protocol's five:

| Code | Status |
|---|---|
| `stalePrompt`, `wrongPlayer` | 409 |
| `wrongPromptType`, `unknownActionId`, `invalidShape` | 400 |

## Skipping to your next turn

gorge stops at every priority window, so a seat gets several times the `chooseAction` prompts it gets from ManaBrew:
one for each step and each stack change, on every player's turn. gorge changes nothing about where it stops. A client skips the stops it
doesn't want with the protocol's own `pass` output, whose `until` clause installs an auto-pass rule on the connection. The rule is gorge's
alone: the server answers each of your later priority prompts with a pass, and sends you none of them, until your clause is met.

To pass through everything up to your next turn, answer the `chooseAction` prompt with `pass` and `until` your own player id at the
`upkeep` step. Your player id is the `decidingPlayerId` on the prompt. This is the whole `send` body, with `22` the prompt's `promptId`
and `player-0` your player id:

```json
{"kind":"response","promptId":22,"action":{"type":"pass","output":{"type":"pass","exhaustStack":false,"until":{"playerId":"player-0","phase":"upkeep"}}}}
```

The next prompt the seat sees is the first priority window in your own upkeep, where the active player always has priority, and the prompt after it comes at
your next stop as usual. Everything the rule passes through is an ordinary pass in the match log.

The rule stops, and the seat gets its prompt again, at the first of these:

- the active player is the `until` player and the step is the `until` phase. If that is already true at the next window (you pass in your
  own upkeep and a trigger is on the stack), the rule stops there;
- any prompt that isn't a priority prompt, such as a block, a target or a payment. That prompt is shown, and the rule ends;
- an undo (`restoreSnapshot`), which clears it.

`"exhaustStack": true` is the other rule: pass until the stack is empty, or stop early when an object appears that wasn't on the stack when
you passed. It can be combined with `until`. A plain `pass` (`"exhaustStack": false`, no `until`) installs no rule and passes once. A
`playerId` that isn't one gorge minted, or an empty `phase`, is `invalidShape`.

## What gorge adds and limits

- **`x_gorge_own_deck_v1`** on every state's `gameView` is the seat's own starting deck as name and count rows, the same data as the
  native view's `own_deck`. It isn't part of the published protocol; a client that doesn't know it ignores it. Prompts never carry it.
- **Declaring alone.** The native wire's `alone_restricted` has no field here. A combat prompt names a creature that can't attack or
  block alone in its presentation text, and a lone declaration is refused, as on the [native wire](http.md#decision-kinds).
- **Constraints on a set.** A rule over the whole answer has no field either, so its presentation text states it. Targets or cards that must
  share a controller or a property, or options that exclude each other, come with the first `min` candidates a legal answer. A `chooseCards`
  ask whose total must reach or stay within a value lists what each card counts for (`Card values: o39 1, o135 3.`), which needn't be anything
  on the card. Under a total to reach (Crew), the cards come largest value first and `min` is the fewest that can reach it.
  A combat prompt under an attack or block tax, or a cap on attackers, names creatures that can go together
  (`These can attack together: o10, o12.`, each at the first target it offers). The answer is still checked, and a broken constraint comes back
  `invalidShape`.
- **Auto-fill.** ManaBrew's payment actions have no auto-fill member. A `payManaCost` window that can auto-fill says so in its
  presentation `description` (the auto-fill's label, `Auto-fill: tap Forest, Island`), and `pay` with `auto: true` takes it.
- **No spectators.** ManaBrew has no spectator role, so spectating stays native. [Wires](../architecture/wires.md) lists what else isn't a
  supported combination, and the tests behind each one that is.
