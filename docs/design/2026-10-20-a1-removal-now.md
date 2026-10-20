# A1 removal-now rule: opt-in, with firing evidence

Date: 2026-10-20.
Ticket: A1 of four-yay's
`docs/design/2026-10-19-forge-heuristics-go-port-design.md`.
Outcome: **implemented and armed only on the experimental `forge-heuristic`
policy.** The default `bot` is unchanged and no win rate is claimed.

## What was built

One clean-room cast-time rule re-expressed from Forge's
`ComputerUtilCard.useRemovalNow` behaviour in gorge's own vocabulary
(`bots/botpolicy/removalnow.go`):

- `HoldForBigger` -- hold a one-shot removal spell while the best opposing
  creature is below a threat bar, unless the spell is needed now.
- The bar is `faceScore() + 24`. Untapped vanilla creatures score
  `3*power + 2*toughness + 6`: a 3/3 scores 21 and is held, a 4/4 scores 26
  and is removed, and a 6/6 scores 36 and is removed.
- "One-shot removal" is the printed spell shape `destroy`, `damage` or
  `bounce`. Pump, sacrifice, exile and every other zone change are outside
  the rule.

Behind `botpolicy.RemovalRules` (zero value = off) on `Board.Removal`, armed
through `heuristic.Bot.EnableRemovalRules`. The registered `forge-heuristic`
policy (`bots/forgeheuristic`, experimental) arms it; the default `bot`
leaves it off.

The rule hooks the existing cast scorer, so it runs only on a decision that
already offers a castable spell. It never rewrites target choice and never
asks the engine for a target hint.

**The fact added to carry it** is `botpolicy.Card.RemovalKind`, filled
identically by the game adapter (`BoardFromGameInto`/`BoardFromInc`) and the
view adapter (`BoardFromView`) from the printed spell ability. It is exposed
on the wire as `view.CardView.RemovalKind` (`removal_kind,omitempty`) and is
derived from the same printed ability as `SpellAPI`, so it is a visible card
fact, never hidden information. The classifier is
`engine/effects.CastRemovalKind`:

- `APIDestroy` -> `destroy`;
- `APIDealDamage` -> `damage`;
- `APIChangeZone` to hand with a battlefield origin -> `bounce`.

This is deliberately narrower than the rules-side `targetRemoval` classifier:
it must not hold a reanimation spell, a graveyard return, or a pump spell
that merely mentions a zone change.

## Firing evidence

The focused botpolicy tests pin both directions:

- `TestRemovalNowHoldsSmallThreat`: a castable Destroy spell plus a 3/3
  opponent casts with the rule off and passes with it on.
- `TestRemovalNowFiresAtLargeThreat`: a 6/6 opponent casts in both
  configurations.
- `TestRemovalKindAdaptersAgree`: destroy, damage and bounce parse to the
  same kind through the game-shaped and view-shaped adapters.

The board-table parity reference was extended with the new fact, so every
whole-game parity test also holds the two adapters to the same
`RemovalKind`.

## What was not ported

- `useRemovalNow`'s `Pump` branch and its `CantBeTargeted`/protection
  checks. The cast scorer's own target feasibility already rejects an
  untargetable creature.
- Forge's `RemovalEffect` SVar hints. The fact here is the printed ability
  shape, not a card-script annotation.
- Any deck-posture or win-rate claim. The Strength claim on
  `forge-heuristic` was not re-measured for this rule.

## Why it is opt-in

The A2 measurement showed that the 30-deck constructed corpus poses only a
handful of removal-shaped asks per game, and the ticket's gate is a win-rate
change. A1 can change those asks, but no measurement has been run, so the
rule stays on the experimental policy and the default path is untouched.
