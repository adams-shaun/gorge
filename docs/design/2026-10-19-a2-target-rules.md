# A2 target-selection rules: measured, kept opt-in, inert on the corpus

Date: 2026-10-19.
Outcome: **implemented, measured, gate NOT met, left opt-in with the numbers
below.** The default bot is unchanged.

## What was built

Three clean-room target-selection rules re-expressed from Forge's
`ComputerUtilCard` behaviour in gorge's own vocabulary
(`bots/botpolicy/targetrules.go`):

1. `LandPriority` -- a land in the opponent's dominant colour outranks an
   off-colour land (bonus 20 vs 8).
2. `ProminentColour` -- a creature of the dominant colour breaks near-ties
   (bonus 2).
3. `FilterFatalCopies` -- a strictly worse copy of a same-typed creature is
   demoted below the better copy.

Behind `botpolicy.TargetRules` (zero value = off) on `Board.Targets`, armed
through `heuristic.Bot.EnableTargetRules`. The registered `forge-heuristic`
policy (`bots/forgeheuristic`, experimental) is the constructor that arms all
three; the default `bot` leaves them off.

**Facts added to carry them** (filled identically by both adapters, pinned
by the existing board-parity tests). The type facts (`Land`, `TypeSig`) read
the effective type line on both halves: the layer-4 line when a type-changing
effect applies, the printed face otherwise:

- `botpolicy.Card.Land`, `.Colour` (`cards.ColourMask`), `.TypeSig` (a
  word-set signature, because `Card` must stay comparable).
- `view.CardView.ColourIdentity` (json-tagged: it changes the frame
  encoding, so the searchprobe goldens were re-pinned with attribution).
- `botpolicy.Board.PublicCards` (`PublicCard.Controller`, `.Colour`,
  `.FaceDown`), filled identically by both adapters from every player's
  public zones (battlefield, graveyard, exile, stack). The colour fold reads
  the TARGET controller's public cards, never the deciding seat's hidden
  deck list; a face-down card contributes nothing.

## The gate, run

`botbench -a bot -b targets`, base seed 1,000,000, seats traded each game,
Go-only, 400 games.

| deck pair | targets win rate | 95% CI | verdict |
|---|---|---|---|
| all rules, default pair | 53.2% B (46.8% A) | [41.9, 51.6] | no effect |
| `mono-white-equipment:mono-black-aggro` | 49.2% A | [44.4, 54.1] | no effect |
| `mono-black-aggro:mono-red-goblins` | 50.0% A | [45.1, 54.9] | no effect |
| `mono-black-aggro:mono-green-stompy` | 50.7% A | [45.9, 55.6] | no effect |

**The 53.2% is an artifact, not a gain.** `-a bot -b bot` on the same pair
and seed scores 46.5% A, the same split (one game apart), so that pair's
result is deck/seat bias, not policy strength. Read it as
`targets` = `bot` there.

## Why the rules are inert

Measured directly by instrumenting a whole game (removal asks, land options
offered, and how often each rule fires):

| decks | removal asks | land options | colour hits | copy hits |
|---|---|---|---|---|
| mono-black-aggro vs mono-green-stompy | 1 | 0 | 4 | 0 |
| mono-black-aggro vs mono-red-goblins | 2 | 0 | 2 | 0 |
| mono-white-equipment vs mono-black-aggro | 3 | 0 | 4 | 4 |

A whole game poses one to three removal-shaped target asks, and **no deck in
the corpus is ever offered a land as a removal target**. The land rule cannot
fire at all; the colour rule breaks ties among two or three creatures; the
copy rule sees four chances in one of three pairs. A rule with a handful of
trigger points across 400 games cannot move a win rate, and it does not.

This is a corpus fact, not a bug in the rules. Forge's AI, which the rules
were re-expressed from, plays a much larger card pool where land destruction
and duplicate creatures are common; gorge's 30-deck constructed corpus is not
that pool.

## Decision

Per the ticket's own acceptance rule -- "if it cannot beat the gate, the note
says so and the code is deleted or left opt-in with the number written down"
-- the code stays **opt-in, default path untouched**, and this note is the
number. It is registered as the experimental `forge-heuristic` policy so it
can be selected and benched, but it is not the default and no production
constructor arms it implicitly.

The Board/view facts added to carry the rules are kept because they are
printed-card facts with independent value (a bot reading a public colour
posture), and both adapters are pinned to agree on them.

## Reproducing

The measurements came from a bench runner outside this repository, with this
arm added to its policies map and gorge replaced by a local checkout:

```go
"targets": func(seed uint64) seat.Seat {
	b := heuristic.NewBot(seed).EnableTargetRules(botpolicy.TargetRules{
		LandPriority: true, ProminentColour: true, FilterFatalCopies: true,
	})
	return b
},
```

Then 400 games of `bot` against `targets`, and of `bot` against `bot` as the
bias control, both at seed 1000000.

## Pre-existing, not from this ticket

A `-pairs all` sweep aborts 1 game of ~87,000 with
`resolve: tape kernel divergence: the ask-free predicate missed an ask:
choose/etb` (seed 1020245). It reproduces with `bot -b bot` on the unmodified
policy, so it is an engine bug that predates this change and is recorded here
only so it is not misattributed to the targeting rules.
