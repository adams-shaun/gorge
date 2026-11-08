# Serving dirac

`gorged -engine dirac` runs a server's tables on [dirac](../architecture/dirac/index.md) instead of gorge, with
no rules engine built beside it. This page is for a host who wants those tables: how to start them, what they
play and what they refuse. [`dirac/docs/serving.md`](https://github.com/adams-shaun/gorge/blob/main/dirac/docs/serving.md)
has the complete list.

## Running it

```sh
make dirac-tables
go build -trimpath -o bin/gorged ./cmd/gorged
bin/gorged -engine dirac -seats 2 -vsbot -dir /tmp/gorge-dirac
```

The tables come from the card corpus ([Install and build](install.md#build-diracs-card-tables-if-you-want-them)).
`-seats 2` is required, because `-seats` defaults to 4. The startup line reads `gorged: 4 tables of 2 on :8080`, and
every entry in `curl localhost:8080/api/tables` says `"engine":"dirac"`. The web client and the native HTTP routes
read the same view and decisions as on gorge, and `make web` applies the same way.

## What it plays

Two-seat Constructed games on limited decks from two card sets, FDN and BLB, and two-seat Commander games on the five Foundations precons (`-format commander -dirac-sets cmd5`). `-dirac-sets` lists the sets to try,
in order, defaulting to `fdn`, and a game plays on the first set holding both decks; `-dirac-sets blb` serves
Bloomburrow. `-decks` left at its default serves the first set's decks in `dirac/testdata/bench/`. dirac plays only
its [supported cards](../architecture/dirac/index.md#what-it-supports), so a deck holding any other card is logged at
startup and listed in the lobby, disabled, with the reason, and the same goes for a directory you name with `-decks`.
None of the decks in `testkit/testutil/decks` fits one table, so the repository's Constructed decks don't play here.

A human can play a bot, or bots can play each other. `bot`, `lethal-pressure`, `forge-heuristic` and
`forge-heuristic-hold` take a seat, and `sb-tactical` plays at a bot-only table. Undo, the game log, replay by event
sequence, feedback reports, the ManaBrew wire and the SpellBench v2 wire all work.

## What differs from a gorge server

- **Every cast is paid by the server's plan.** dirac has no manual mana, so the client shows no Auto Mana switch.
- **No "reveals" or "rolled" lines.** dirac keeps no event log, so the game log is derived from its positions, and it
  has no such notes. Two games of the same cards read differently on the two engines. A ManaBrew client gets no reveal
  or dice prompt; those prompts only acknowledge.
- **A past position is shown at the end of the burst holding its event**, not between that burst's events.

## What it refuses

Each of these says why, at startup or in the request's error.

| Asked for | Answer |
|---|---|
| `-seats` other than 2 | dirac plays two seats; Commander is two-seat, one commander per seat, from a set that holds the decks (`-dirac-sets cmd5`) |
| `-auto-mana=false`, `-bot-auto-mana=false` | dirac has no manual mana |
| `-events`, a chosen starting player, sideboards | these need gorge |
| `-<policy>-net`, `az-redeal`, any search policy | a search needs a root, and dirac deals none |
| `sb-tactical` against a human | it declares it reads decklists, so it never seats with one |
| A deck with a card outside the set, or one dirac doesn't support | the card is named |
| Paired bot, review decisions, shadow scans | these walk gorge's engine |
