# Replays

Every finished game in gorge can be replayed from its log, event by event.
The engine is deterministic, so a replay is not a recording of the screen:
it rebuilds the game from the seed, the decks and the list of answers each
player gave. This page covers watching a replay, moving through one, and,
on a **review server**, reading every decision the game posed: the options
on offer, the one that was chosen and, for a bot that searches, what it
considered and why it picked what it did.

<figure class="gg-annot" markdown>
<div class="gg-annot-frame" markdown>
![The review screen at decision 26 of a recorded game: the board on the left with both hands visible, the decision panel at the top of the right rail listing four options and a search table, the seat summaries and log below it, and the replay transport across the bottom](img/replay-review.jpg)
<svg class="gg-annot-ov" viewBox="0 0 1440 900" preserveAspectRatio="none" aria-hidden="true">
<rect class="gg-annot-region" x="4" y="4" width="1130" height="836" rx="6"/>
<rect class="gg-annot-region" x="1142" y="4" width="294" height="377" rx="6"/>
<rect class="gg-annot-region" x="1142" y="387" width="294" height="168" rx="6"/>
<rect class="gg-annot-region" x="1142" y="561" width="294" height="335" rx="6"/>
<rect class="gg-annot-region" x="4" y="846" width="1130" height="50" rx="6"/>
<g class="gg-annot-tag"><rect x="470" y="183" width="105" height="44" rx="5"/><text x="485" y="214">Board</text></g>
<line class="gg-annot-lead" x1="1102" y1="150" x2="1142" y2="150"/>
<circle class="gg-annot-dot" cx="1142" cy="150" r="6"/>
<g class="gg-annot-tag"><rect x="862" y="128" width="240" height="44" rx="5"/><text x="877" y="159">Decision panel</text></g>
<line class="gg-annot-lead" x1="1102" y1="470" x2="1142" y2="470"/>
<circle class="gg-annot-dot" cx="1142" cy="470" r="6"/>
<g class="gg-annot-tag"><rect x="847" y="448" width="255" height="44" rx="5"/><text x="862" y="479">Seats and stack</text></g>
<line class="gg-annot-lead" x1="1102" y1="680" x2="1142" y2="680"/>
<circle class="gg-annot-dot" cx="1142" cy="680" r="6"/>
<g class="gg-annot-tag"><rect x="1027" y="658" width="75" height="44" rx="5"/><text x="1042" y="689">Log</text></g>
<g class="gg-annot-tag"><rect x="16" y="768" width="165" height="44" rx="5"/><text x="31" y="799">Transport</text></g>
</svg>
</div>
<figcaption>A review server showing decision 26 of a recorded two-player game. The red seat is a search bot; it cast Lightning Bolt where its own heuristic would have cast Faithless Looting. Captured 2026-10-20.</figcaption>
</figure>

<div class="gg-regions" markdown>

**Board**
: The game as it stood at the cursor. A replay is a spectator's view, so the
  server decides whose hands you see: the operator's `-spectator` setting on
  an ordinary server, and every hand on a review server.

**Decision panel**
: Only on a review server. The decision at the cursor: who was asked, every
  option offered, which one they chose and, when the record carries notes,
  how they chose it. See [Reading a decision](#reading-a-decision).

**Seats and stack**
: Life, hand, library and graveyard counts for each player, the mana pool,
  and the stack with the last resolved card.

**Log**
: The game's event lines up to the cursor. *Show step lines* and *Show
  engine noise* add the lines the log hides by default.

**Transport**
: Play, step and scrub through the game. See
  [Moving through a game](#moving-through-a-game).

</div>

## Open a replay

There are two places a replay comes from. Any `gorged` server keeps one for
each match it finished, and `gorged review` serves games you saved.

| | A finished match on a server | `gorged review` |
|---|---|---|
| Where | `/t/<table>/m/<k>`, linked from the table's match list | `/t/<id>/m/1` for each game you pass it |
| Hands shown | as the server's `-spectator` flag says (hidden by default) | all of them |
| Decision panel, skip passes, jump keys | no | yes |
| Bot reasoning | no | when the game was recorded with notes |

### From a running server

Open the table in the lobby and pick a finished match, or go straight to
`/t/<table>/m/<k>`, where `k` counts the table's matches from 1. A live
table's spectator view has the same transport with one extra button: **Live**
while you follow the game as it happens, **Go live** (with how far behind you
are) after you step back. See
[Spectating](play.md#spectating) for the spectator
flags.

### Review a saved game

`gorged review` is a separate, read-only mode of the server. It takes one or
more saved games and serves each one as a table you can only watch. Like any
page of `gorged`, it needs the [browser client](install.md#2-build-the-browser-client-if-you-want-one)
built first:

```sh
go run ./cmd/gorged review cmd/repro/testdata/feedback/20260914T120000Z-fb01
```

```text
gorged review: 1 recorded games on http://127.0.0.1:8090/
gorged review: 20260914T120000Z-fb01 at http://127.0.0.1:8090/t/20260914T120000Z-fb01/m/1
```

That input is a bug report committed in the repository. A saved game is
either:

- **a feedback snapshot**: the directory a player's bug report writes
  (`match.json`, `log.json`), see [Reporting a bug from a game](play.md#reporting-a-bug-from-a-game);
- **a match record**: one `<id>.match.json.gz` file, written by a bot-game
  driver as [below](#record-games-for-review). Only a record can carry the
  bots' notes.

The table id is the input's file or directory name. Pass several inputs as
arguments or with `-records a,b,c`. The server replays every input when it
starts and refuses to start if any one fails to load or replay, so a game
that diverges is reported instead of quietly left out.

| Flag | Default | What it does |
|---|---|---|
| `-addr` | `127.0.0.1:8090` | listen address. It must be a loopback address unless you also pass `-review-public` |
| `-review-public` | off | allow any address. Every hidden card in every game becomes readable by anyone who can reach the port |
| `-art-dir` | none | fetch card images into this directory and serve them. Without it the board draws no art and nothing is written to disk |
| `-records` | | saved games, comma-separated; repeatable; positional arguments work too |

A review table has no seats: nothing can be played at it, and every seat
route answers `403`.

### Record games for review

A match record is written by whatever plays the games. In gorge that is
`testkit/bench`, the driver behind [bot-vs-bot games](bot-games.md): give
its `Hooks.Record` a sink and it writes one record per game. While a sink is
set, the driver also asks each seat that can explain its decisions to do so
(the search bots and the policy-net bot can) and stores the answers in the
record.

```go
hooks := bench.Hooks{EnvSeats: true}
hooks.Record = &matchrecord.Sink{
	Producer: matchrecord.Producer{Tool: "my-run", Policies: []string{"az-redeal", "heuristic"}},
	Put: func(r *matchrecord.Record) error {
		return matchrecord.Write(filepath.Join(dir, name+matchrecord.Ext), r)
	},
}
outcome, eng, err := bench.PlayGame(cfg, seats, 60, 6000, hooks)
```

`Put` decides where each record goes and which games to keep. With no sink
the driver records nothing and asks the seats for nothing. A two-player record
is about 10 to 20 KB gzipped, notes included.

To try it without writing a driver, one test writes three example records
(a search bot against the heuristic, twice, and a rollout search against
the heuristic):

```sh
GORGE_EXPLAIN_DEMO_DIR=/tmp/gorge-records \
  go test -run TestWriteExplainDemoRecords -v ./testkit/bench
go run ./cmd/gorged review -art-dir /tmp/gorge-art \
  /tmp/gorge-records/*.match.json.gz
```

It takes about 10 seconds and plays the same three games every time. The
screenshots on this page come from `az-redeal-vs-bot`.

## Moving through a game

The transport runs across the bottom of the board.

<figure markdown>
<svg class="gg-transport" viewBox="0 0 720 342" width="100%" role="img" aria-labelledby="tr-title">
<title id="tr-title">The replay transport, drawn in two rows. First row, left to right: the play and step buttons, the turn and active player, and the timeline with a band per turn and tick marks. Second row: the event position, the step unit switch, the Skip passes box and the speed menu.</title>
<rect class="gg-tr-bg" x="2" y="64" width="714" height="60"/>
<path class="dg-line" d="M2 64 H716 M2 124 H716"/>
<path class="dg-line" d="M2 64 V124"/>
<path class="dg-line-dashed" d="M716 64 V124"/>
<path class="dg-arrow" transform="translate(14 82.0) scale(1.5)" d="M3 3h2v10H3zM13 3v10L6 8z"/>
<path class="dg-arrow" transform="translate(48 82.0) scale(1.5)" d="M11 3v10L4 8z"/>
<path class="dg-arrow" transform="translate(132 82.0) scale(1.5)" d="M5 3v10l7-5z"/>
<path class="dg-arrow" transform="translate(166 82.0) scale(1.5)" d="M3 3v10l7-5zM11 3h2v10h-2z"/>
<circle class="st-zone" cx="102" cy="94.0" r="21"/>
<path class="gg-tr-on" transform="translate(91 82.0) scale(1.5)" d="M4 2.5v11L13 8z"/>
<text class="dg-text-bold" x="210" y="100.0" style="font-size:17px">Turn 1</text>
<rect class="st-combat" x="272" y="83.0" width="3" height="22"/>
<text class="dg-text" x="281" y="99.0" style="font-size:12.5px">mono-red-prowess</text>
<rect class="st-mana gg-tr-band" x="405.0" y="84.0" width="14.2" height="18"/>
<rect class="st-combat gg-tr-band" x="420.2" y="84.0" width="14.2" height="18"/>
<rect class="st-mana gg-tr-band" x="435.3" y="84.0" width="15.2" height="18"/>
<rect class="st-combat gg-tr-band" x="451.6" y="84.0" width="17.4" height="18"/>
<rect class="st-mana gg-tr-band" x="470.0" y="84.0" width="15.2" height="18"/>
<rect class="st-combat gg-tr-band" x="486.2" y="84.0" width="23.9" height="18"/>
<rect class="st-mana gg-tr-band" x="511.1" y="84.0" width="18.5" height="18"/>
<rect class="st-combat gg-tr-band" x="530.6" y="84.0" width="32.6" height="18"/>
<rect class="st-mana gg-tr-band" x="564.2" y="84.0" width="20.7" height="18"/>
<rect class="st-combat gg-tr-band" x="585.9" y="84.0" width="31.5" height="18"/>
<rect class="st-mana gg-tr-band" x="618.4" y="84.0" width="22.8" height="18"/>
<rect class="st-combat gg-tr-band" x="642.2" y="84.0" width="17.4" height="18"/>
<rect class="st-mana gg-tr-band" x="660.6" y="84.0" width="22.8" height="18"/>
<rect class="st-combat gg-tr-band" x="684.4" y="84.0" width="19.6" height="18"/>
<rect class="gg-tr-played" x="405" y="84.0" width="18.4" height="18"/>
<rect class="gg-tr-mark" x="423.4" y="76.0" width="3" height="26"/>
<rect class="gg-tr-mark" x="490.6" y="76.0" width="3" height="26"/>
<rect class="gg-tr-mark" x="538.2" y="76.0" width="3" height="26"/>
<rect class="gg-tr-mark" x="545.8" y="76.0" width="3" height="26"/>
<rect class="gg-tr-mark" x="593.4" y="76.0" width="3" height="26"/>
<rect class="dg-box-solid" x="422.4" y="81.0" width="3" height="24"/>
<text class="dg-text-muted" x="406.0" y="115.0" style="font-size:11px">1</text>
<text class="dg-text-muted" x="436.3" y="115.0" style="font-size:11px">2</text>
<text class="dg-text-muted" x="471.0" y="115.0" style="font-size:11px">3</text>
<text class="dg-text-muted" x="512.1" y="115.0" style="font-size:11px">4</text>
<text class="dg-text-muted" x="565.2" y="115.0" style="font-size:11px">5</text>
<text class="dg-text-muted" x="619.4" y="115.0" style="font-size:11px">6</text>
<text class="dg-text-muted" x="661.6" y="115.0" style="font-size:11px">7</text>
<path class="dg-line" d="M10 60 V54 H192 V60"/>
<text class="dg-text" x="101.0" y="42" text-anchor="middle" style="font-size:15px">Play and step</text>
<path class="dg-line" d="M405 60 V54 H705 V60"/>
<text class="dg-text" x="555.0" y="42" text-anchor="middle" style="font-size:15px">Timeline: one band per turn</text>
<path class="dg-line" d="M210 128 V134 H395 V128"/>
<text class="dg-text" x="302.5" y="158" text-anchor="middle" style="font-size:15px">Turn and active player</text>
<path class="dg-line" d="M405 128 V134 H705 V128"/>
<text class="dg-text" x="555.0" y="158" text-anchor="middle" style="font-size:15px">Override and fallback ticks</text>
<rect class="gg-tr-bg" x="2" y="236" width="468" height="60"/>
<path class="dg-line" d="M2 236 H470 M2 296 H470"/>
<path class="dg-line-dashed" d="M2 236 V296"/>
<path class="dg-line" d="M470 236 V296"/>
<text class="dg-text-muted" x="16" y="271.0" style="font-size:14px">event 159 of 2202</text>
<rect class="dg-box-muted" x="150" y="252.0" width="176" height="28" rx="4"/>
<text class="dg-text" x="175.0" y="271.0" text-anchor="middle" style="font-size:14px">Event</text>
<text class="dg-text-bold" x="237.0" y="271.0" text-anchor="middle" style="font-size:14px">Decision</text>
<rect class="dg-box-solid" x="206" y="276.0" width="62" height="2"/>
<text class="dg-text" x="300.0" y="271.0" text-anchor="middle" style="font-size:14px">Turn</text>
<rect class="st-zone" x="342" y="259.0" width="14" height="14" rx="2"/>
<path class="gg-tr-on" d="M345 266.0 l3 3 l5 -6" style="fill:none;stroke:var(--gg-riverbed);stroke-width:2"/>
<text class="dg-text" x="362" y="271.0" style="font-size:14px">Skip</text>
<rect class="dg-box-muted" x="412" y="252.0" width="48" height="28" rx="4"/>
<text class="dg-text" x="436" y="271.0" text-anchor="middle" style="font-size:14px">1×</text>
<path class="dg-line" d="M150 232 V226 H326 V232"/>
<text class="dg-text" x="238.0" y="214" text-anchor="middle" style="font-size:15px">Step by</text>
<path class="dg-line" d="M412 232 V226 H460 V232"/>
<text class="dg-text" x="436.0" y="214" text-anchor="middle" style="font-size:15px">Speed</text>
<path class="dg-line" d="M16 300 V306 H136 V300"/>
<text class="dg-text" x="76.0" y="330" text-anchor="middle" style="font-size:15px">Position</text>
<path class="dg-line" d="M342 300 V306 H400 V300"/>
<text class="dg-text" x="371.0" y="330" text-anchor="middle" style="font-size:15px">Skip passes</text>
</svg>
<figcaption>The transport of the game above, drawn larger and in two rows; on screen it is one strip. Decision is the step unit, Skip passes is on, and the cursor sits early in turn 1.</figcaption>
</figure>

From left to right:

- **Play and step.** The outer buttons jump a whole turn back or forward.
  The inner ones step one unit, and the round button plays and pauses.
  Pressing play at the end of the game starts it again from the beginning.
- **Turn and active player.** The turn number is the round, the same "Turn
  N" the phase bar shows, so in a two-player game turn 3 holds each
  player's third turn. Before the first turn it reads *Setup*.
- **Timeline.** One band per turn, tinted with the active player's colour,
  with the round number under each round's first turn. Click or drag to
  move, and hover to see the turn and the event under the pointer. On a
  review server, ticks mark the decisions worth a look: green where a bot
  overrode its own heuristic, ember where a fallback answered. Hover a tick
  for which decision it is.
- **Position.** The cursor as an event number, out of the game's total.
- **Step by.** What one step moves. See the table below.
- **Skip passes.** Only on a review server, and only while stepping by
  Decision. See [Skip passes](#skip-passes).
- **Speed.** ½×, 1×, 2×, 4× or 8×.

| Step by | One step moves to | One play step at 1× |
|---|---|---|
| Event | the next or previous logged event | 0.15 s |
| Decision | the next decision a player was asked. The board shows it **as they faced it**, before their answer | 0.7 s |
| Turn | the start of the next or previous turn | 1.8 s |

Decision is the default, because it is the unit a game is played in: each
stop is one moment where a player or a bot had to choose.

### Keys

| Key | Does |
|---|---|
| Space or K | play or pause |
| → and ← | step one unit forward or back |
| Shift+→ and Shift+← | step one turn |
| PageDown and PageUp, with the timeline focused | step one turn |
| Home and End, with the timeline focused | the start and the end of the game |
| O, F, D (review server) | the next override, fallback or searched decision. Shift goes to the previous one |

The keys do nothing while you are typing in a field.

### Skip passes

Most decisions in a game are a priority window with nothing to do: the only
options are *Pass priority* and *Concede*. In the example game 259 of its 395
decisions are like that. On a review server the transport knows every
decision's options, so with **Skip passes** on (the default), a Decision
step and Decision playback pass over those and stop only where a player had
a real choice.

<figure markdown>
<svg class="gg-skip" viewBox="0 0 760 210" width="100%" role="img" aria-labelledby="sk-title">
<title id="sk-title">Forty decisions in a row. With Skip passes on, a step jumps from one real decision to the next: decisions 2, 3, 4, 7, 24, 25, 26 and 27. With it off, a step visits every decision.</title>
<text class="dg-text-bold" x="32" y="40" style="font-size:13px">Skip passes on: 8 steps</text>
<line class="dg-line" x1="32" y1="92" x2="730.5" y2="92"/>
<rect class="dg-box-muted" x="37.5" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="55.0" y="87" width="5" height="10"/>
<rect class="dg-box-solid" x="72.0" y="83" width="6" height="18" rx="1"/>
<rect class="dg-box-solid" x="89.5" y="83" width="6" height="18" rx="1"/>
<rect class="dg-box-solid" x="107.0" y="83" width="6" height="18" rx="1"/>
<rect class="dg-box-muted" x="125.0" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="142.5" y="87" width="5" height="10"/>
<rect class="dg-box-solid" x="159.5" y="83" width="6" height="18" rx="1"/>
<rect class="dg-box-muted" x="177.5" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="195.0" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="212.5" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="230.0" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="247.5" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="265.0" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="282.5" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="300.0" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="317.5" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="335.0" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="352.5" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="370.0" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="387.5" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="405.0" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="422.5" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="440.0" y="87" width="5" height="10"/>
<rect class="dg-box-solid" x="457.0" y="83" width="6" height="18" rx="1"/>
<rect class="dg-box-solid" x="474.5" y="83" width="6" height="18" rx="1"/>
<rect class="dg-box-solid" x="492.0" y="83" width="6" height="18" rx="1"/>
<rect class="dg-box-solid" x="509.5" y="83" width="6" height="18" rx="1"/>
<rect class="dg-box-muted" x="527.5" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="545.0" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="562.5" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="580.0" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="597.5" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="615.0" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="632.5" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="650.0" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="667.5" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="685.0" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="702.5" y="87" width="5" height="10"/>
<rect class="dg-box-muted" x="720.0" y="87" width="5" height="10"/>
<circle class="dg-arrow" cx="18" cy="92" r="3.5"/>
<path class="dg-line-accent" d="M18.0 81 Q46.5 66.2 75.0 81"/>
<path class="dg-line-accent" d="M75.0 81 Q83.8 70.9 92.5 81"/>
<path class="dg-line-accent" d="M92.5 81 Q101.2 70.9 110.0 81"/>
<path class="dg-line-accent" d="M110.0 81 Q136.2 66.7 162.5 81"/>
<path class="dg-line-accent" d="M162.5 81 Q311.2 47.0 460.0 81"/>
<path class="dg-line-accent" d="M460.0 81 Q468.8 70.9 477.5 81"/>
<path class="dg-line-accent" d="M477.5 81 Q486.2 70.9 495.0 81"/>
<path class="dg-line-accent" d="M495.0 81 Q503.8 70.9 512.5 81"/>
<text class="dg-text-bold" x="32" y="126" style="font-size:13px">Skip passes off: 40 steps</text>
<line class="dg-line" x1="32" y1="178" x2="730.5" y2="178"/>
<rect class="dg-box-muted" x="37.5" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="55.0" y="173" width="5" height="10"/>
<rect class="dg-box-solid" x="72.0" y="169" width="6" height="18" rx="1"/>
<rect class="dg-box-solid" x="89.5" y="169" width="6" height="18" rx="1"/>
<rect class="dg-box-solid" x="107.0" y="169" width="6" height="18" rx="1"/>
<rect class="dg-box-muted" x="125.0" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="142.5" y="173" width="5" height="10"/>
<rect class="dg-box-solid" x="159.5" y="169" width="6" height="18" rx="1"/>
<rect class="dg-box-muted" x="177.5" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="195.0" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="212.5" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="230.0" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="247.5" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="265.0" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="282.5" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="300.0" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="317.5" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="335.0" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="352.5" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="370.0" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="387.5" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="405.0" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="422.5" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="440.0" y="173" width="5" height="10"/>
<rect class="dg-box-solid" x="457.0" y="169" width="6" height="18" rx="1"/>
<rect class="dg-box-solid" x="474.5" y="169" width="6" height="18" rx="1"/>
<rect class="dg-box-solid" x="492.0" y="169" width="6" height="18" rx="1"/>
<rect class="dg-box-solid" x="509.5" y="169" width="6" height="18" rx="1"/>
<rect class="dg-box-muted" x="527.5" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="545.0" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="562.5" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="580.0" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="597.5" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="615.0" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="632.5" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="650.0" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="667.5" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="685.0" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="702.5" y="173" width="5" height="10"/>
<rect class="dg-box-muted" x="720.0" y="173" width="5" height="10"/>
<circle class="dg-arrow" cx="18" cy="178" r="3.5"/>
<path class="dg-line" d="M18.0 167 Q29.0 156.4 40.0 167"/>
<path class="dg-line" d="M40.0 167 Q48.8 156.9 57.5 167"/>
<path class="dg-line" d="M57.5 167 Q66.2 156.9 75.0 167"/>
<path class="dg-line" d="M75.0 167 Q83.8 156.9 92.5 167"/>
<path class="dg-line" d="M92.5 167 Q101.2 156.9 110.0 167"/>
<path class="dg-line" d="M110.0 167 Q118.8 156.9 127.5 167"/>
<path class="dg-line" d="M127.5 167 Q136.2 156.9 145.0 167"/>
<path class="dg-line" d="M145.0 167 Q153.8 156.9 162.5 167"/>
<path class="dg-line" d="M162.5 167 Q171.2 156.9 180.0 167"/>
<path class="dg-line" d="M180.0 167 Q188.8 156.9 197.5 167"/>
<path class="dg-line" d="M197.5 167 Q206.2 156.9 215.0 167"/>
<path class="dg-line" d="M215.0 167 Q223.8 156.9 232.5 167"/>
<path class="dg-line" d="M232.5 167 Q241.2 156.9 250.0 167"/>
<path class="dg-line" d="M250.0 167 Q258.8 156.9 267.5 167"/>
<path class="dg-line" d="M267.5 167 Q276.2 156.9 285.0 167"/>
<path class="dg-line" d="M285.0 167 Q293.8 156.9 302.5 167"/>
<path class="dg-line" d="M302.5 167 Q311.2 156.9 320.0 167"/>
<path class="dg-line" d="M320.0 167 Q328.8 156.9 337.5 167"/>
<path class="dg-line" d="M337.5 167 Q346.2 156.9 355.0 167"/>
<path class="dg-line" d="M355.0 167 Q363.8 156.9 372.5 167"/>
<path class="dg-line" d="M372.5 167 Q381.2 156.9 390.0 167"/>
<path class="dg-line" d="M390.0 167 Q398.8 156.9 407.5 167"/>
<path class="dg-line" d="M407.5 167 Q416.2 156.9 425.0 167"/>
<path class="dg-line" d="M425.0 167 Q433.8 156.9 442.5 167"/>
<path class="dg-line" d="M442.5 167 Q451.2 156.9 460.0 167"/>
<path class="dg-line" d="M460.0 167 Q468.8 156.9 477.5 167"/>
<path class="dg-line" d="M477.5 167 Q486.2 156.9 495.0 167"/>
<path class="dg-line" d="M495.0 167 Q503.8 156.9 512.5 167"/>
<path class="dg-line" d="M512.5 167 Q521.2 156.9 530.0 167"/>
<path class="dg-line" d="M530.0 167 Q538.8 156.9 547.5 167"/>
<path class="dg-line" d="M547.5 167 Q556.2 156.9 565.0 167"/>
<path class="dg-line" d="M565.0 167 Q573.8 156.9 582.5 167"/>
<path class="dg-line" d="M582.5 167 Q591.2 156.9 600.0 167"/>
<path class="dg-line" d="M600.0 167 Q608.8 156.9 617.5 167"/>
<path class="dg-line" d="M617.5 167 Q626.2 156.9 635.0 167"/>
<path class="dg-line" d="M635.0 167 Q643.8 156.9 652.5 167"/>
<path class="dg-line" d="M652.5 167 Q661.2 156.9 670.0 167"/>
<path class="dg-line" d="M670.0 167 Q678.8 156.9 687.5 167"/>
<path class="dg-line" d="M687.5 167 Q696.2 156.9 705.0 167"/>
<path class="dg-line" d="M705.0 167 Q713.8 156.9 722.5 167"/>
<text class="dg-text-muted" x="40.0" y="202" text-anchor="middle" style="font-size:11px">0</text>
<text class="dg-text-muted" x="75.0" y="202" text-anchor="middle" style="font-size:11px">2</text>
<text class="dg-text-muted" x="162.5" y="202" text-anchor="middle" style="font-size:11px">7</text>
<text class="dg-text-muted" x="460.0" y="202" text-anchor="middle" style="font-size:11px">24</text>
<text class="dg-text-muted" x="512.5" y="202" text-anchor="middle" style="font-size:11px">27</text>
<text class="dg-text-muted" x="722.5" y="202" text-anchor="middle" style="font-size:11px">39</text>
</svg>
<figcaption>The example game's first 40 decisions. Tall marks offered a real choice; short ones offered only Pass priority and Concede.</figcaption>
</figure>

The setting is remembered by your browser. It changes only where a step
lands: Event and Turn steps, the timeline, the jump keys and a link to a
pass-only decision still reach every decision. A replay on an ordinary
server has no option lists, so it has no Skip passes box and stops at every
decision.

### Links to a moment

The address bar follows the cursor, so copying the URL shares the exact
moment, along with the step unit and speed when they are not the defaults.
You can also write a link by hand:

| Parameter | Opens at |
|---|---|
| `?seq=159` (or `?event=`) | event 159 |
| `?intent=26` (or `?decision=`) | decision 26, as the player faced it. This is the same point `go run ./cmd/repro -at 26` prints for a bug report |
| `?turn=4` | the start of turn 4, counted as the transport counts turns |
| `?unit=event`, `decision` or `turn` | with that step unit |
| `?speed=0.5`, `1`, `2`, `4` or `8` | at that speed |
| `?play=1` | already playing |

If a link names more than one position, `seq` wins, then `intent`, then
`turn`. A link with no position opens at the end of the game.

## Reading a decision

On a review server the top of the rail is the decision panel. It always
shows the decision at the cursor or the last one before it. Between two
decisions (while stepping by Event, say) it keeps the last one, dimmed.

<div class="gg-panel" markdown>
<figure markdown>
![The decision panel for decision 26: a Policy badge, the red seat's priority decision marked overrode the bot, four options with Cast Faithless Looting marked bot and Cast Lightning Bolt marked chosen, then a search table with three candidates, and the override, fallback and searched jump buttons](img/replay-decision-panel.png)
<figcaption>Decision 26, the moment in the screenshot above.</figcaption>
</figure>
<div markdown>

**Heading.** *Decision 26 of 394* is its index from 0, the number `?intent=`
and `repro -at` take. The badge at the right says who answered, as
described [below](#who-answered). It is there only when the record
carries notes.

**Who and what.** The player, with their colour, and the kind of decision
(*priority*, *attackers*, *target*, *choose* and so on), then the engine's
prompt. *overrode the bot* appears when a bot chose differently from its
own heuristic.

**Options.** Everything the player could have answered, numbered as the
engine numbered them. The answer has a gold edge and *chosen*. On an
override, the heuristic's answer has a green edge and *bot*. A decision with
more than 64 options lists the first 64 and says how many there were.
When the answer is not a single option (several attackers, a cast with an
{X} value) an *Answer* line spells it out.

**Search.** Shown when the bot searched. See
[Reading the search](#reading-the-search).

**Jumps.** How many overrides, fallbacks and searched decisions the game
has, and buttons (or O, F and D) to go to the next one.

</div>
</div>

A record saved without notes, and every feedback snapshot, still shows the
options and the answer for every decision. The panel then says *No explain
recorded* and has no badges, ticks or jumps.

### Who answered

A seat is not always the one that picked the answer. A search bot hands
some decisions to its heuristic, and when something goes wrong a fallback
answers instead. The badge tells you which happened.

| Badge | Meaning |
|---|---|
| Policy | the bot's search or network chose the answer |
| Heuristic | the seat's heuristic answered: a plain heuristic seat, or a kind of decision the bot does not search. A *Reason* line says why when a search bot fell back to it at a kind it does search |
| Forced | not searched, because there was only one answer the search could express. Often a pass-only window, but not always: a land drop or a mana tap the search cannot represent counts here too |
| Prior skip | too many options for the network to rank, so the search ran over the first candidates with equal priors. It still searched, and the panel still shows the table |
| Feed fallback | the bot's view of the game had stopped updating, so the driver answered for it |
| Root fallback | the search could not start from this position (the *Reason* line says why), so the driver answered for it |
| Refusal retry | the engine refused the bot's first answer and accepted its second |
| Refusal fallback | the engine refused the bot's answer and a shared fallback answered. The tooltip says which rung of the fallback ladder |

The last four are **fallbacks**: the F key and the ember ticks find them.
A fallback keeps the search the bot ran, so you can see what it wanted
even though that is not what was played.

### Reading the search

The table lists the candidates the search considered at the root, most
visited first: up to 16, plus the chosen and the heuristic's answer if they
fall outside the top 16 (*N more not listed* counts the rest).

| Column | Meaning |
|---|---|
| Candidate | the answer, in the options' words. Two options with the same name get their number added |
| Visits | how many simulations went through this candidate. The search spends its time on the answers that look best, so visits are its real preference. The bar is scaled to the most-visited row |
| Prior | how promising the network rated the candidate before searching |
| Q | the average result of those simulations, for the player deciding, from −1 (a sure loss) through 0 (even) to +1 (a sure win) |

Under the table: **Root** is the search's value of the position as a whole on
the same scale, then the number of simulations, the time it took when the
driver measured it, and the search engine. A bot that only scores the
options with a network, without searching, shows *Net value* instead.

In the example, all three candidates started with the same prior of 0.33,
so the search had no preference going in. After 100 simulations Lightning
Bolt had 43 visits and a value of −0.06, close to even. Passing scored
−0.15, and Faithless Looting, the heuristic's choice, scored −0.25 and got
the fewest visits. The root value of −0.13 says the search thought the red
player was slightly behind either way; it preferred the Bolt because that
lost the least.

### Finding the interesting decisions

A game of a few hundred decisions has only a handful worth reading closely.
The quickest way through one is:

1. Press **O** to go to the next override, a decision where the bot
   disagreed with its own heuristic. These are where the search changed
   the game. Compare the *chosen* and *bot* rows of the table.
2. Press **F** for the next fallback. Many fallbacks in one game usually
   mean the bot stopped working properly there, rather than that it played
   badly.
3. Press **D** to walk every searched decision, including the ones where
   search and heuristic agreed.

The ticks on the timeline show the same overrides and fallbacks at a
glance, so a cluster of them stands out before you step anywhere.

## What the notes can and cannot tell you

- **An override is measured against the bot's own heuristic**, not against
  the best move. It shows where the search changed the answer, not whether
  the change was right. A set answer (attackers, blockers) counts as the
  same if it has the same members; an ordered one (trigger order, arranging
  cards) only in the same order.
- **Values are the deciding player's**, so in a two-player game the same
  position reads +0.4 for one player and about −0.4 for the other. They are
  rounded to three decimals and shown to two.
- **Visits matter more than Q.** A candidate the search barely visited can
  have a high Q from a few lucky simulations. The search picks by visits.
- **The time column is wall-clock time** on the machine that played the
  game. It is never part of the replay, and two runs of the same game agree
  on everything else.
- **Notes do not change the game.** They are stored beside the log, are
  never replayed and never hashed. A record whose notes don't line up with
  its decisions is still served, without notes, with a warning in the
  server's output.

## Limits

- Only a review server has the decision panel, Skip passes, ticks and jump
  keys. An ordinary server's replays have the board, log and transport.
- The decision index loads in one piece, which is fine for a game of a few
  thousand decisions.
- `gorged review` serves only games played by the gorge engine. A record
  from another engine that carries its own random draws is refused at
  startup.
