# A2 target-selection rules: measured, kept opt-in, inert on the corpus

Date: 2026-10-19.
Ticket: A2 of four-yay's
`docs/design/2026-10-19-forge-heuristics-go-port-design.md`.
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
through `heuristic.NewBotWithTargetRules` / `Bot.EnableTargetRules`.

**Facts added to carry them** (all printed, all filled identically by both
adapters, pinned by the existing board-parity tests):

- `botpolicy.Card.Land`, `.Colour` (`cards.ColourMask`), `.TypeSig` (a
  word-set signature, because `Card` must stay comparable).
- `view.CardView.ColourIdentity` (json-tagged: it changes the frame
  encoding, so the searchprobe goldens were re-pinned with attribution).
- `deck.ManifestRow.Colour`, so the colour fold reads `Board.OwnDeck` alone.

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
number. It is not promoted, not hosted, and not referenced by any production
constructor.

The Board/view/manifest facts added to carry the rules are kept because they
are printed-card facts with independent value (a bot reading a deck's colour
posture), and both adapters are pinned to agree on them.

## Reproducing

four-yay's `cmd/botbench` has no arm for this policy (a bench arm for a
policy that failed its gate would be dead weight in the pinned dependency).
Add one to a scratch checkout of four-yay before measuring:

```go
// in cmd/botbench/main.go's policies map
"targets": func(seed uint64) seat.Seat {
	return heuristic.NewBotWithTargetRules(seed, botpolicy.TargetRules{
		LandPriority: true, ProminentColour: true, FilterFatalCopies: true,
	})
},
```

then point the build at this checkout:

```sh
cd /path/to/four-yay
printf '\nreplace github.com/adams-shaun/gorge => /path/to/gorge\n' >> go.mod
go build -o /tmp/botbench ./cmd/botbench
cd /path/to/gorge
/tmp/botbench -a bot -b targets -games 400 -seed 1000000 -dir .cards
/tmp/botbench -a bot -b bot     -games 400 -seed 1000000 -dir .cards  # bias control
```

The `replace` is a measurement scaffold only: it must not be committed, and
it is why this ticket's bench arm lives in the note rather than in four-yay.

## Pre-existing, not from this ticket

A `-pairs all` sweep aborts 1 game of ~87,000 with
`resolve: tape kernel divergence: the ask-free predicate missed an ask:
choose/etb` (seed 1020245). It reproduces with `bot -b bot` on the unmodified
policy, so it is an engine bug that predates this change and is recorded here
only so it is not misattributed to the targeting rules.
