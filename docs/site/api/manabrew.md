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

The browser client's **Protocol** switch plays a seat over this wire ([Play in the browser](../guide/play.md#the-table)).

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

Engine messages are `{"kind": …}` objects: `state`, `stateDelta`, `display`, `prompt` and `error`. A rejected `send` comes back in its
HTTP reply and is also pushed on any open stream. Its code is one of the protocol's five:

| Code | Status |
|---|---|
| `stalePrompt`, `wrongPlayer` | 409 |
| `wrongPromptType`, `unknownActionId`, `invalidShape` | 400 |

## What gorge adds and limits

- **`x_gorge_own_deck_v1`** on every state's `gameView` is the seat's own starting deck as name and count rows, the same data as the
  native view's `own_deck`. It isn't part of the published protocol; a client that doesn't know it ignores it. Prompts never carry it.
- **Declaring alone.** The native wire's `alone_restricted` has no field here. A combat prompt names a creature that can't attack or
  block alone in its presentation text, and a lone declaration is refused, as on the [native wire](http.md#decision-kinds).
- **No spectators.** ManaBrew has no spectator role, so spectating stays native. [Wires](../architecture/wires.md) lists what else isn't a
  supported combination, and the tests behind each one that is.
