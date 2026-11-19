# manabrew-bridge

Hosts a [ManaBrew](https://docs.manabrew.app/) relay room whose engine is a `gorged` server, so
ManaBrew's own client (the web app, the desktop app, its bots and its `loadgen` seat client) plays
gorge. The guide is [Play gorge in ManaBrew's client](../../docs/site/guide/manabrew.md).

ManaBrew's client has no setting for a remote engine. Every multiplayer seat talks to the room's
host through ManaBrew's relay (`manabrew-server`). This bridge signs in to the relay as a hosted-room
host; when the room's controller presses **Start game**, it creates the game on `gorged` with
`POST /api/games` (one human seat per relay seat, each with the deck chosen in ManaBrew), opens one
`/api/manabrew/v0/.../stream` per seat and carries the envelopes both ways. It rewrites nothing
inside a frame.

```sh
RELAY=ws://127.0.0.1:9443 GORGE=http://127.0.0.1:8081 node tools/manabrew-bridge/bridge.mjs
```

Node 22 or later; no dependencies. It is original code that speaks ManaBrew's published relay and
seat protocols; it contains no ManaBrew source.

| Variable | Default | What |
|---|---|---|
| `RELAY` | `ws://127.0.0.1:9443` | The relay's WebSocket URL |
| `RELAY_PASSWORD` | `forge` | The relay's password (`MANABREW_SERVER_KEY` on the relay) |
| `GORGE` | `http://127.0.0.1:8081` | The `gorged -manabrew` server |
| `ROOM_NAME` | `gorge` | The room's name in ManaBrew's lobby |
| `SEATS` | `4` | Seats in the room |
| `FORMAT` | `Commander` | A ManaBrew format; `Commander` makes a gorge Commander game, anything else a constructed one |
| `MULLIGANS` | `1` | Passed to `POST /api/games` |
| `POOL_FALLBACK` | on | `0` fails the start when `gorged` refuses a seat's deck, instead of dealing gorge's own pool decks |
| `COMMANDERS` | none | A JSON file `{deckName: commanderName}`, for seats that send no commander name (ManaBrew's `loadgen`) |
| `GATE` | off | `1` holds the room with a non-ready placeholder until all `SEATS` seats are ready, for pods of `loadgen` seats, which start the game at two |
| `ONCE` | off | `1` exits after one game; otherwise the room is reused |
| `TRANSCRIPT` | none | A JSONL file that gets every frame both ways |

One bridge hosts one room, one game at a time. Run several, with different `ROOM_NAME`s, for more.
