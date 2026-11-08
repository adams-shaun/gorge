# The champion draft persona

*A tuned FDN drafter and builder, selectable beside the default*

`champion` is a bot persona for Foundations (FDN) drafts. It drafts with the [knobbed drafter](drafting.md) under a tuned set of knobs, and both its drafting and its deck building read a table of card values fitted from simulated games instead of the 17Lands ratings. In a measured comparison it built decks that won 64.5% of their games against 17Lands decks, where the default persona's decks won 43.9%. That comparison has limits, listed below, and it is not a claim about human opponents.

## What it is

| Part | What it is |
|---|---|
| Drafter | `pickbot.Knobbed` with colour-fit weight 8, signal weight 2 and a colour bias of W 0, U 0, B -2, R -2, G +3 (the knob string `cfm=8;bias=G3/B-2/R-2;sig=2`) |
| Value table | `draft/limited/pickbot/values/FDN-champion.json`: 270 FDN cards, field `dv`, rescaled at load onto the 17Lands FDN mean and spread. A card absent from the table counts as the 17Lands mean |
| Builder | `pickbot.Best23` with its default tuning, reading the same value table |

The persona is FDN only and is not the default: the default persona is still `rating-colour` with `best23`. `GET /api/events/personas` lists `champion` with the other personas.

## How to pick it

- **Event create screen.** Each bot seat has a drafter list filled from `GET /api/events/personas`; choose `champion`. The seat builds with the same persona.
- **Create body.** Set `"drafter": "champion"` on a bot seat of the `POST /api/events` body, for example `{"kind":"bot","drafter":"champion"}`. The event's set must be `FDN`.
- **`draftsim`.** Pass `-drafter champion -builder champion`:

```text
go run ./cmd/draftsim -set FDN -seed 1 -drafter champion -builder champion -v
```

Any other set is refused. A create request for another set answers 400 with `persona "champion" is tuned for FDN only, not <set>`, and `draftsim` exits with the same message before it opens the card corpus.

## What was measured

Measured on 2026-11-06 on the dirac engine with the heuristic bot piloting both seats. 500 eight-seat pods were drafted with a seed block never used for tuning. Each kept deck then played eight of 3,150 held-out 17Lands FDN decks, which human players drafted, in both seat orders.

| Persona | Win rate | Games |
|---|---:|---:|
| `champion` drafter and `champion` builder | 64.5% [63.7, 65.2] | 18,055 |
| default: `rating-colour` + `best23` | 43.9% [43.0, 44.9] | 19,851 |
| `balanced`: committed drafter + balanced builder | 42.7% [41.8, 43.7] | 19,863 |

The gap between the champion and the default is +20.6 points, and the intervals do not overlap.

For context, in self-play between bot-drafted pods on the same engine the same pair scored 56.7% [55.9, 57.3], against 39.5% [38.7, 40.1] for the committed drafter on 17Lands ratings (1,500 pods, 60,640 games, 2026-11-05). The human-deck figure above is the one that shows the advantage carries over to human-drafted decks.

This is not yet a gorge-engine number: every game above ran on dirac.

Read the figure as "against 17Lands decks played by the heuristic bot", not as a win rate against human players.

## Caveats

1. The games ran on the dirac engine with the heuristic bot as pilot, not on the gorge engine, so the figure is not yet a gorge-engine human-deck number.
2. A deck holding a card dirac refuses to play is dropped from the match-ups; the most common refusals were Genesis Wave, Ajani, Caller of the Pride and Fiend Artisan, so the sign of that bias is unknown.
3. The measured pods had 8 seats and hosted events have 4.
4. The opponents are 17Lands decks of mixed quality, and the same bot pilots both sides.
5. The values are conditional on that pilot: some cards that 17Lands rates highly rate low here and the reverse (Blasphemous Edict and Day of Judgment are far lower than their 17Lands standing; Juggernaut and Shivan Dragon far higher), so "beats 17Lands ratings" means "drafts better for this bot".
6. The champion packs more cards of mana value 6 or more into its decks than human drafters do (2.02 per deck against 1.21 for 17Lands top-player decks), a shape the heuristic pilot tolerates and a human opponent may not.

## The value table

The table is a ridge-logistic fit of game result on deck membership over 227,000 dirac self-play games of bot-drafted FDN decks (generation 2 of an iterated refit; ridge strength 1e-4 times the game count), played by the heuristic bot on both seats. The fitted coefficients are mapped to `dv = 0.55 + 0.25 * coefficient`.

It holds 270 cards. The drafter and the builder rescale `dv` at load onto the 17Lands FDN mean and spread, so a card the table lacks takes the 17Lands mean. The file, [`FDN-champion.json`](https://github.com/adams-shaun/gorge/blob/main/draft/limited/pickbot/values/FDN-champion.json), records its source, its licence, its attribution and this measured result in its own fields.

The drafted decks came from bots whose picks used 17Lands ratings, so the table is derived from 17Lands data and carries the 17Lands credit.

!!! info "17Lands data"
    The champion value table, [`draft/limited/pickbot/values/FDN-champion.json`](https://github.com/adams-shaun/gorge/blob/main/draft/limited/pickbot/values/FDN-champion.json), is derived from 17Lands data: data from 17Lands (<https://www.17lands.com>), public datasets, CC BY 4.0. Changes: values fitted from simulated games on decks drafted with 17Lands ratings, rescaled at load.

## What is not known yet

- The gorge engine against human decks: the figure above is a dirac figure.
- Four-seat pods, the size of a hosted event.
- Any set other than FDN: the persona is refused for them.
