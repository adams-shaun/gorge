# Troubleshooting

The failures people actually hit when running gorge's server and bot tools,
each with the message you see, the cause and the fix. Most of them were
reproduced while writing this guide (2026-10-20), and every message was
re-checked against the code at commit `513a2a0c3` (2026-10-21).

## Starting up

### `opening corpus at .cards: lstat .cards/cardsfolder: no such file or directory`

```text
gorged: opening corpus at .cards: lstat .cards/cardsfolder: no such file or directory (run make fetch-cards compile-cards)
```

Either the corpus was never fetched, or you started the tool from a directory
other than the repository root: `-cards` (in `mtgsim`, `-dir`) defaults to the
relative path `.cards`. Run `make fetch-cards` once, run tools from the
checkout root, or pass the corpus path explicitly
([Install](install.md#1-clone-and-fetch-the-card-corpus)).

### The page says `web build missing — run make web`

The server answers every non-API path with HTTP 503 when no browser client was
embedded at build time. The API still works. Build the client with
`make web`, then **rebuild** `gorged` (`go run` rebuilds for you): the bundle
is compiled into the binary, so a binary built before `make web` keeps
answering 503 ([Install](install.md#4-build-the-browser-client)).

### `bind: address already in use`

```text
gorged: listen tcp 127.0.0.1:8090: bind: address already in use
```

Another process holds the port, often a `gorged` you started earlier. Find it
by its port, then stop that process id:

```sh
ss -ltnp | grep ':8090 '      # or: lsof -iTCP:8090 -sTCP:LISTEN
kill <pid>
```

Prefer this over `pkill -f gorged`: a `-f` pattern matches every process
whose command line contains the word, including your own shell or editor
sessions. Or simply pick another `-addr`.

### A flag is refused at startup

| Message | Cause |
|---|---|
| `-bot-policies: bot policy "sb-tactical" reads opponents' decklists and cannot be offered` | `sb-tactical` and `az-redeal` are never seated against a person; leave them out of `-bot-policies` |
| `-bot-opt "bot.depth=3": bot policy "bot" declares no options` | `bot` declares no options; only `forge-heuristic` has one today, `hold-instant` ([Bot-vs-bot games](bot-games.md#the-registered-policies)) |
| `-bot-policy "az-redeal" reads opponents' decklists and cannot be the default` | the default policy must be one a person can play against: `bot`, `forge-heuristic`, `forge-heuristic-hold` or `lethal-pressure` |
| `-pprof requires a loopback listener (got [::]:8091)` | `-pprof` needs `-addr 127.0.0.1:…` |
| `-manabrew requires a seat gate: pass -humans or -vsbot` | the ManaBrew adapter only serves seats |

## Persistence

### My flags are ignored after a restart

A server resumes whatever its `-dir` holds. If `tables.json` there already
lists tables, `-tables`, `-seats`, `-format` and the other table flags are
not applied, even though the startup line prints the new seat count. Reproduced:
a directory created with `-tables 2 -seats 2` came back as the same two
2-seat constructed tables when restarted with `-tables 3 -seats 4 -format
commander`. Start from an empty directory whenever you change the table
setup.

### Tables come back wrong after an upgrade

A directory written by an older binary resumes with every field that binary
did not know at its zero value. `Format`'s zero value is `constructed`, so a
commander table can silently come back as constructed. Start a new binary on
an empty `-dir`.

### Two servers, strange matches

Two servers sharing one `-dir` silently corrupt each other's persisted
matches. Give each server its own directory (and its own `-feedback`).

### My play-vs-bot game is gone after a restart

Expected. A play-vs-bot game's seat token lives only in the process, so the
table is dropped at restart. Its event log stays under `<dir>/g<N>` and the
id is not reused.

## In the browser

### `play-vs-bot games are not enabled on this server`

`POST /api/games` (the lobby's **Start game**) answers 404 unless the server
was started with `-vsbot`.

### A seat request answers 401 or 403

`{"code":"unauthorized","message":"request does not hold a seat claim"}` means
no valid token reached the server: open the full join URL, including
`?seat=N&token=…`, or send `Authorization: Bearer <token>`. A 403 says
either `claim does not hold a seat at this table` (the token belongs to
another table or seat) or `this server is spectator-only` (started without
`-vsbot` or `-humans`). Tokens are minted per process, so links from before a
restart no longer work.

### Spectators can see the players' hands

The startup tables, including a `-humans` table, use `-spectator public` by
default, so a seat-less `/view` or `/api/stream` hides every hand. An
operator who passed `-spectator omniscient` explicitly opens every hand to
every unauthenticated spectator; do that only on a table with no people at it
([Play](play.md#play-against-other-people)).

### Undo is greyed out

Undo works only when you are the table's only human. With two or more people
at a table it is refused, because the other player cannot be asked.

### The Draft button does nothing useful

Draft pods (`-events`) need an extension that the public `gorged` binary does
not include, so the lobby's *Draft a 4-player pod* entry fails on a plain
build.

### Card images are missing, and the log shows `429, retrying`

Card art comes from Scryfall through the server's cache. On a fresh cache the
startup prewarm fetches every deck card at a limited rate, and Scryfall may
answer 429 (slow down); the server backs off and retries. Art appears as the
cache fills and never affects play. `-art-dir` keeps the cache across a wiped
`-dir`; `-prewarm=false` turns the startup fill off.

## Bot runs

### `mtgsim: ... Deps.Cards is nil`

`sb-tactical` needs the card registry and an environment-aware driver, which
`mtgsim` does not provide. Use the
[`testkit/bench` matchup program](bot-games.md#a-fair-matchup-with-testkitbench).
`az-redeal` in `mtgsim` does not fail, but it never searches.

### A game prints `LIVELOCK`, `DID NOT TERMINATE` or `replay diverged`

All three are engine bugs, not slow games: the engine's livelock watcher
fired, a game ran past `mtgsim`'s 400,000-intent bound, or a replay of a
finished game produced a different event chain. Report the exact command and
seed; the seed reproduces the game.

### The win rate moved and nothing changed

Check the build and the corpus first. A different gorge commit or a different
`FORGE_REF` (`.cards/cards.lock`) can change every game, and the same command
on the same build prints the same output. Then check the sample size against
the [table](bot-games.md#reading-the-result): ±5 points is ordinary noise at
400 games.

## `repro` prints `DIVERGED`

`cmd/repro` rebuilds a feedback snapshot with the engine and corpus you have
now. `DIVERGED` names the first event where the rebuilt game differs from the
recording. The two usual causes are both expected:

- **The corpus moved.** The report was captured at an older `FORGE_REF`, and a
  card script it used has changed since.
- **The engine changed.** A fix or a new rule since the report.

To read the report as it was, check out the gorge commit the server was built
from (and the matching corpus pin via `make fetch-cards`) and replay there.
A snapshot does not record the server's commit, so whoever runs a server
that collects reports should note it.

## Tests

### Tests pass in milliseconds

Tests that need the card corpus **skip** when `.cards/` is missing, so a
package can report `ok` in a couple of milliseconds having tested almost
nothing. In a fresh clone or a new git worktree, fetch or link `.cards` first.
See [Testing](../contributing/testing.md).

### `.cards` keeps growing

Each engine build that compiles the corpus writes its own
`.cards/ir-<fingerprint>.gob.gz` and `.seg` (about 26 MB together). Old ones
are safe to delete; the next run recompiles in a couple of seconds.
