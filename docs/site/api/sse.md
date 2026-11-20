# The event stream (SSE)

`GET /api/stream` is one long-lived [Server-Sent Events](https://html.spec.whatwg.org/multipage/server-sent-events.html)
connection; two POSTs choose what it carries. Frame bodies are in [the wire protocol](protocol.md); schemas are in
[`openapi.json`](openapi.json) (`StreamFrame`). Behaviour is read from
[`httpapi/sse.go`](https://github.com/adams-shaun/gorge/blob/main/server/host/httpapi/sse.go),
[`host/session.go`](https://github.com/adams-shaun/gorge/blob/main/server/host/session.go) and
[`host/fanout.go`](https://github.com/adams-shaun/gorge/blob/main/server/host/fanout.go).

The stream is one-way because the engine never waits on a client: frames go into a bounded per-session buffer, and a
client that falls behind loses its session instead of slowing the game. One connection can watch every table in
overview and a few in full detail, and change its mind without reconnecting.

## Lifecycle

```mermaid
sequenceDiagram
  participant C as client
  participant S as gorged
  C->>S: GET /api/stream
  S-->>C: event: hello {session, tables}
  C->>S: POST /api/subscribe {session, table:"*", mode:"overview"}
  C->>S: POST /api/subscribe {session, table:"t2", mode:"focus"}
  S-->>C: id: s…:1 event: snapshot (the board now)
  loop every engine burst on t2
    S-->>C: id: s…:N event: event (one per engine event)
    S-->>C: id: s…:N+1 event: decision (who is asked what)
  end
  S-->>C: event: widget (every 250 ms, latest per overview table)
  S-->>C: : ping (every 15 s)
  Note over C,S: the connection drops
  C->>S: GET /api/stream, Last-Event-ID: s…:N+1
  S-->>C: the frames after N+1 (no hello)
```

1. **Open.** A connection without a usable `Last-Event-ID` gets a new session. Its first frame
   is `hello`, carrying the session id (an unguessable `s` + 32 hex digits) and the lobby's
   tables. A private play-vs-bot table is omitted unless the request's seat token claims it;
   event tables are public and stay watchable. The session id is the capability that
   subscribing needs, so do not share it.
2. **Subscribe.** `POST /api/subscribe {"session", "table", "mode"}` answers 204. Subscribing
   again to the same table replaces its mode. `POST /api/unsubscribe {"session", "table"}`
   removes it; unsubscribing a table you never subscribed is a 404.
3. **Receive** frames until you disconnect.
4. **Disconnect.** The session survives for **30 seconds** (the `ResumeGrace` option). A
   reconnect with `Last-Event-ID` within that window resumes it. After that it is closed and
   its subscriptions are gone.

## Wire format

Each frame is one SSE message. The `data:` line is one `protocol.Frame` serialised as a single
line of JSON:

```text
id: <session>:<frame id>
event: <frame type, the same as data.t>
data: {"v":1,"t":"…","id":…,"table":"…","match":…,"seq":…,"body":{…}}

```

- **`id:`** is `<session id>:<frame id>`. The frame id counts up from 1 per session. `hello`,
  `widget` and `overflow` frames carry no id. They are not resumable, so they have no `id:`
  line and no `id` field.
- **`event:`** equals the frame's `t`, so `EventSource.addEventListener("snapshot", …)` works.
- **`: ping`** is a comment line sent every **15 s** (the `KeepAlive` option) to keep proxies
  from timing the connection out.
- **Headers:** `Content-Type: text/event-stream`, `Cache-Control: no-cache, no-transform`,
  `Connection: keep-alive` and `X-Accel-Buffering: no` (this last one keeps nginx from
  buffering the stream).

A short session (long `data` cut at `...`):

```text
event: hello
data: {"v":1,"t":"hello","seq":0,"body":{"session":"s00cc29e2...","tables":[...

id: s00cc29e2...:2
event: snapshot
data: {"v":1,"t":"snapshot","id":2,"table":"t2","match":5,"seq":18,"body":{"view":{...

id: s00cc29e2...:3
event: event
data: {"v":1,"t":"event","id":3,"table":"t2","match":5,"seq":19,"body":{"event":{"seq":19,"kind":"decision_made","player":0,"text":"starting_player:[0]"},"line":"Player 1 answers starting_player:[0]"}}

id: s00cc29e2...:5
event: decision
data: {"v":1,"t":"decision","id":5,"table":"t2","match":5,"seq":20,"body":{"player":0,"kind":"mulligan","prompt":"..."}}

: ping
```

## What each mode sends

| Frame | `overview` | `focus` | When |
|---|---|---|---|
| `hello` | (once per session, before any subscription) | | A fresh connection |
| `widget` | yes | | The latest cell per table, coalesced and flushed every **250 ms**. Never buffered for resume. |
| `match_start` | yes | yes | A match begins on the table |
| `snapshot` | | yes | At once on a focus subscribe, when the table has a live match; at every match start |
| `event` | | yes | One per engine event, in seq order |
| `decision` | | yes | When a match first parks on a decision, then after each burst when a decision is pending, for any seat, bot or human. It names the seat, kind and prompt only. |
| `rewind` | | yes | An undo rolled the live match back (see below) |
| `match_end` | yes | yes | A match ends: `result`, `winner`, chain `head` |
| `table_state` | yes | yes | A **series** table's lifecycle state changed (`idle`, `live`, `between_games`); ordinary tables never send it |
| `table_halted` | yes | yes | The table crashed and stopped, with the reason |
| `overflow` | yes | yes | The last frame of a session whose buffer filled |

Draft events send their own frames (`event_state`, `draft_pack`, `pod_progress`, `series_update`) to a session after
`POST /api/events/{id}/subscribe`.

`table` `*` subscribes every public table, present and future, in overview mode. Private
play-vs-bot tables are included only for a session whose seat token claims them; a client
without a claim must subscribe to one explicitly (which the server refuses with a 404).
Focus on `*` is a 400: a client must pick which games it wants in full.

A focus subscriber joining mid-match never sees `match_start`, so its `snapshot` carries the seat list (`seats`).
Events up to the snapshot's `head` are inside it; the next `event` frame is `head + 1`.

### Following a match

Apply the `snapshot`, then each `event` in seq order. A `decision` frame says a seat is being asked, with kind and prompt
only: the stream is shared with spectators, so it never carries options or hidden cards. A seat reads its options from
`GET /api/tables/{t}/matches/{k}/pending?seat=N` and its private view from `GET .../view?seat=N`, both with its token.

### Rewind

When a seat undoes (`POST .../undo`), focus subscribers get a `rewind` frame: a full snapshot at the new head, with
the envelope's `seq` set to that head. Discard everything held past `body.head` (events, transcript lines, a pending
decision), then apply the body like a snapshot. Seq numbers are then reused for different events.

## Resuming

Reconnect with the last `id:` you saw:

```sh
curl -sN -H 'Last-Event-ID: s00cc29e285fc2d3aab2b299e97fab509:2085' $GORGE/api/stream
```

- If the session is still alive and its ring still holds every frame after that id, the
  server replays those frames and then continues live. There is **no** `hello`, and your
  subscriptions are intact.
- Otherwise the server starts a **fresh** session with a new `hello`, and you must subscribe
  again. That happens when the session is unknown or expired (the 30 s grace has passed), the
  header is malformed, or the id is older than the ring. A stale session that cannot be served
  is closed.

The ring holds the session's most recent frames. Its size is `host.Options.Ring`, 256 frames
by default. The ring and the outbound buffer have the same size.

## Overflow

When a session's outbound buffer is full, the server stops sending to it rather than wait. It
writes one final `overflow` frame with the number of frames dropped (`{"dropped": 17}`) and
closes the stream. A resume cannot help, because the frames are gone. Reconnect **without**
`Last-Event-ID`, subscribe again, and rebuild from the new snapshot.

## A minimal client

```js
const es = new EventSource("/api/stream");
es.addEventListener("hello", async (e) => {
  const { body } = JSON.parse(e.data);
  await fetch("/api/subscribe", { method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ session: body.session, table: "t1", mode: "focus" }) });
});
es.addEventListener("snapshot", (e) => render(JSON.parse(e.data).body.view));
es.addEventListener("event", (e) => apply(JSON.parse(e.data).body));
es.addEventListener("rewind", (e) => resetTo(JSON.parse(e.data).body));
```

`EventSource` sends `Last-Event-ID` itself on automatic reconnects. A resumed session gets no `hello`, so subscriptions stay as they were. `web/` holds the full client.
