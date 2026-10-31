# User guide

This guide is for people who want to use gorge rather than change it: to play Magic against
its bots or friends in a browser, host draft events, or run bot-vs-bot games. To change the
engine, start at [Contributing](../contributing/index.md). To connect a bot written in
another language, see [Bots](../bots/index.md).

## From clone to playing

```sh
git clone https://github.com/adams-shaun/gorge && cd gorge
make fetch-cards   # the pinned Forge card scripts, into .cards/
make web           # the browser client; needs Node 20+ and must come before the next line
go run ./cmd/gorged -addr 127.0.0.1:8090 -dir /tmp/gorge-play \
  -feedback /tmp/gorge-play-feedback -vsbot
# open http://127.0.0.1:8090/ and press "Start game"
```

You need Go, git, make and bash; Node only for `make web`. [Install and build](install.md) explains
each step. For bot games alone, skip the client and run `make sim` after `make fetch-cards`:
it plays 20 verified bot games in a few seconds.

## What runs where

When you play, one `gorged` process does everything except draw the board.
The browser holds no rules: it shows what the server sends and posts back the
option you picked. Bots are not separate programs; they are Go code running
inside the same process, answering their seats' decisions directly.

<figure markdown>
<svg viewBox="0 0 800 420" width="100%" role="img" aria-label="Browsers talk HTTP JSON and server-sent events to one gorged process. Inside it the HTTP API sits on the host registry, which runs one rules engine per match. Each engine's seats are either human seats, which wait for an intent from a browser, or bot seats running a registered policy in-process. The process reads the corpus and deck files at startup, writes match logs and feedback reports to disk, and caches card art fetched from Scryfall.">
  <defs>
    <marker id="gd-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
      <path d="M0,0 L10,5 L0,10 z" class="dg-arrow"/>
    </marker>
  </defs>
  <!-- browsers -->
  <rect x="20" y="70" width="170" height="56" rx="6" class="dg-box"/>
  <text x="105" y="93" text-anchor="middle" class="dg-text-bold">Your browser</text>
  <text x="105" y="112" text-anchor="middle" class="dg-text-muted">seat 0 + its token</text>
  <rect x="20" y="150" width="170" height="56" rx="6" class="dg-box"/>
  <text x="105" y="173" text-anchor="middle" class="dg-text-bold">A friend's browser</text>
  <text x="105" y="192" text-anchor="middle" class="dg-text-muted">seat 1 + its token</text>
  <rect x="20" y="230" width="170" height="56" rx="6" class="dg-box"/>
  <text x="105" y="253" text-anchor="middle" class="dg-text-bold">Spectator</text>
  <text x="105" y="272" text-anchor="middle" class="dg-text-muted">no token: watch only</text>
  <text x="105" y="48" text-anchor="middle" class="dg-text-muted">HTTP JSON + SSE stream</text>
  <line x1="190" y1="98" x2="258" y2="82" class="dg-line-accent" marker-start="url(#gd-arrow)" marker-end="url(#gd-arrow)"/>
  <line x1="190" y1="178" x2="258" y2="90" class="dg-line-accent" marker-start="url(#gd-arrow)" marker-end="url(#gd-arrow)"/>
  <line x1="190" y1="258" x2="258" y2="98" class="dg-line-dashed" marker-end="url(#gd-arrow)"/>
  <!-- gorged process -->
  <rect x="240" y="20" width="360" height="380" rx="10" class="dg-box-muted"/>
  <text x="256" y="42" class="dg-text-bold">gorged (one Go process)</text>
  <rect x="260" y="56" width="320" height="52" rx="6" class="dg-box"/>
  <text x="420" y="78" text-anchor="middle" class="dg-text-bold">HTTP API (server/host/httpapi)</text>
  <text x="420" y="97" text-anchor="middle" class="dg-text-mono">/api/tables  /api/games  …/intent  /api/stream</text>
  <rect x="260" y="128" width="320" height="52" rx="6" class="dg-box"/>
  <text x="420" y="150" text-anchor="middle" class="dg-text-bold">host registry (server/host)</text>
  <text x="420" y="169" text-anchor="middle" class="dg-text-muted">tables, matches, seat tokens, one view per seat</text>
  <rect x="260" y="200" width="320" height="52" rx="6" class="dg-box-accent"/>
  <text x="420" y="222" text-anchor="middle" class="dg-text-bold">rules engine, one per match (engine/)</text>
  <text x="420" y="241" text-anchor="middle" class="dg-text-muted">every change is a logged event; seeded, replayable</text>
  <rect x="260" y="272" width="150" height="62" rx="6" class="dg-box"/>
  <text x="335" y="296" text-anchor="middle" class="dg-text-bold">Human seat</text>
  <text x="335" y="315" text-anchor="middle" class="dg-text-muted">waits for your answer</text>
  <rect x="430" y="272" width="150" height="62" rx="6" class="dg-box"/>
  <text x="505" y="296" text-anchor="middle" class="dg-text-bold">Bot seat</text>
  <text x="505" y="315" text-anchor="middle" class="dg-text-muted">a policy, in-process</text>
  <line x1="420" y1="108" x2="420" y2="126" class="dg-line" marker-start="url(#gd-arrow)" marker-end="url(#gd-arrow)"/>
  <line x1="420" y1="180" x2="420" y2="198" class="dg-line" marker-start="url(#gd-arrow)" marker-end="url(#gd-arrow)"/>
  <line x1="335" y1="252" x2="335" y2="270" class="dg-line" marker-start="url(#gd-arrow)" marker-end="url(#gd-arrow)"/>
  <line x1="505" y1="252" x2="505" y2="270" class="dg-line" marker-start="url(#gd-arrow)" marker-end="url(#gd-arrow)"/>
  <text x="420" y="364" text-anchor="middle" class="dg-text-muted">the engine asks a seat for a decision;</text>
  <text x="420" y="380" text-anchor="middle" class="dg-text-muted">the seat answers with an intent</text>
  <!-- outside world -->
  <rect x="630" y="56" width="150" height="52" rx="6" class="dg-box-muted"/>
  <text x="705" y="78" text-anchor="middle" class="dg-text-bold">Scryfall</text>
  <text x="705" y="97" text-anchor="middle" class="dg-text-muted">card art, cached</text>
  <rect x="630" y="128" width="150" height="52" rx="6" class="dg-box"/>
  <text x="705" y="150" text-anchor="middle" class="dg-text-mono">-dir</text>
  <text x="705" y="169" text-anchor="middle" class="dg-text-muted">tables, event logs</text>
  <rect x="630" y="200" width="150" height="52" rx="6" class="dg-box"/>
  <text x="705" y="222" text-anchor="middle" class="dg-text-mono">-feedback</text>
  <text x="705" y="241" text-anchor="middle" class="dg-text-muted">bug reports + snapshots</text>
  <rect x="630" y="272" width="150" height="62" rx="6" class="dg-box"/>
  <text x="705" y="296" text-anchor="middle" class="dg-text-mono">.cards  -decks</text>
  <text x="705" y="315" text-anchor="middle" class="dg-text-muted">corpus and decks,</text>
  <text x="705" y="330" text-anchor="middle" class="dg-text-muted">read at startup</text>
  <line x1="580" y1="82" x2="628" y2="82" class="dg-line-dashed" marker-end="url(#gd-arrow)"/>
  <line x1="580" y1="154" x2="628" y2="154" class="dg-line" marker-end="url(#gd-arrow)"/>
  <line x1="580" y1="226" x2="628" y2="226" class="dg-line" marker-end="url(#gd-arrow)"/>
  <line x1="628" y1="303" x2="582" y2="303" class="dg-line" marker-end="url(#gd-arrow)"/>
</svg>
<figcaption>One <code>gorged</code> process serves every browser. Human seats wait for a browser's answer; bot seats answer in-process.</figcaption>
</figure>

Two properties of this design come up again in the guide:

- **The server decides what each viewer sees.** A seat's view is redacted on the server, and a
  spectator gets the public view or the omniscient one, as the operator chose. The client
  can't ask for more.
- **Every game replays.** The engine is deterministic: a match is its seed, its decks and the
  list of answers. That is what makes undo, the replay pages, bug-report snapshots and
  bot-match verification work.

## Where to go next

| You want to | Read |
|---|---|
| install the tools, the corpus and the client | [Install and build](install.md) |
| play against a bot, or against friends on a LAN | [Play in the browser](play.md) |
| host or simulate a four-seat draft | [Draft events](draft.md) |
| compare two bot strategies, or watch bots play | [Bot-vs-bot games](bot-games.md) |
| replay a game, or read why a bot made each choice | [Replays](replays.md) |
| connect a bot written in Python, Java or over HTTP | [Bots](../bots/index.md) |
| fix something that went wrong | [Troubleshooting](troubleshooting.md) |
| look up a term | [Glossary](../contributing/glossary.md) |
