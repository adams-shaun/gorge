# Serving dirac

`gorged -engine dirac` plays its tables on dirac alone: no rules engine is built
beside it. The web client and the native HTTP routes read the same view and decisions
as on gorge.

## Running it

```sh
make dirac-tables
gorged -engine dirac -vsbot -addr 127.0.0.1:8091 -dir /tmp/gorge-data
```

- `-dirac-sets` lists the card sets to try, in order; the default is `fdn`. A game
  plays on the first set holding both decks. A set whose tables are not built stops
  the server at startup, naming the command that builds them.
- `-decks` left at its default serves `dirac/testdata/bench/<first set>`. The lobby lists
  any other deck in a named directory, disabled, with the reason dirac refuses it.
  A directory with nothing playable stops the server at startup.

## What it plays

- Two seats, constructed. Every cast is paid by the server's plan, so the client
  shows no Auto Mana switch.
- A human against a bot, or bots against each other. A bot table skips, with a log
  line, a deck that the rotation cannot pair with its neighbour.
- Bots: `bot`, `lethal-pressure`, `forge-heuristic`, `forge-heuristic-hold`, and at a
  bot-only table `sb-tactical`, which plays without its payment planner.
- Undo, the game log and replay by event sequence. A past position is shown at the
  end of the burst holding that event.
- The ManaBrew wire (`-manabrew`) and the SpellBench v2 wire (`-sbv2`). `-sbv2` plays
  every table on the autopay surface, which is what that wire poses.
- Feedback reports. `go run ./cmd/repro <report>` replays one on dirac, from the tables
  its `engine_key` names; `-lockstep` plays the same answers against gorge and prints
  the first difference.

## Limits on the wires

- The derived log has no reveal or roll notes, so a ManaBrew client gets no reveal or
  dice prompt. Those prompts only acknowledge.
- A v2 observation reads no trigger amount, and a colour from a card's printing off
  the battlefield. An object that moves twice between two decisions counts as moved.
- A hosted bot reads a target's damage amount only when it is literal. An unless-pay
  option says "Pay the cost" and not which.

## What it refuses

Each of these says why, at startup or in the request's error.

| Asked for | Answer |
|---|---|
| `-seats` other than 2, `-format commander` | dirac plays two-seat constructed |
| `-auto-mana=false`, `-bot-auto-mana=false` | dirac has no manual mana |
| `-events`, a chosen starting player, sideboards | these need gorge |
| `-<policy>-net`, `az-redeal`, any search policy | a search needs a root, and dirac deals none |
| `sb-tactical` against a human | it declares it reads decklists, so it never seats with one |
| A deck with a card outside the set | the card is named |
| Paired bot, review decisions, shadow scans | these walk gorge's engine |
