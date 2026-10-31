# A1 removal-now rule: opt-in, with firing evidence

Date: 2026-10-20.
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

## The candidate rules (review round 2)

The r1 build held a spell on ANY opposing creature's threat; the r2 review
broke that three ways, and each break is now a candidate rule in
`removalNow`:

- **The mana half (`removalHeld`).** Holding the spell is worthless if the
  tap gate still floats its mana: the seat tapped toward the held 3-mana
  destroy, `chooseCast` then passed, and the pool emptied at step end (CR
  500.4) having bought nothing. `tapWants`, `bestUnpayable` (which
  `CastableNow`/`TapIntent` read through) and `AnyCastableNow` all skip a
  card `removalNow` refuses, so a held spell holds its mana too.
- **The lethal face (`lethalFace`).** The r1 predicate read only opposing
  creatures, so it sat on the winning bolt: an opponent at 2 life, a 1/1 on
  the table, Lightning Bolt in hand held the 3 damage because 11 < 24. Now
  a damage spell with a literal positive `NumDmg$` and a player-naming
  scope fires when some opponent's life is at or below the damage, whatever
  the creatures. An unknown figure (NumDmg$ X, an SVar) is NO candidate at
  all -- the ft1 contract's standing rule, an unknown is never lethal.
- **The target scope (`RemovalTargets`).** The r1 classifier accepted every
  `Destroy`/`DealDamage`, so a Shatter (destroy target ARTIFACT) was held
  because a 1/1 creature existed. The scope is read off the compiled
  `ValidTgts$` alternatives' base tokens (`effects.CastRemovalTargets`):
  "creature"/"player"/"any"/"" (neither). A spell with no creature scope is
  never held on creature threat, and its scope is carried only next to a
  held kind (a Raise Dead's creature-naming ChangeZone is not a bounce, so
  it reports no scope).
- **The targetability filter (`legalTargetOf`).** The r1 fold counted ANY
  opposing creature, so a hexproof 6/6 cleared the bar while the only legal
  target was the 1/1 beside it, and the removal was spent on the 1/1. The
  fold now skips a creature that is not a plausible legal target of an
  opponent's spell: Hexproof and Shroud (hard), any Protection keyword
  (over-matched; see below), and Indestructible against a destroy. The bar
  is about the best TARGET, not the biggest body.

## Firing evidence

The focused botpolicy tests pin both directions; the r2 review's five break
attempts are each one of these:

- `TestRemovalNowHoldsSmallThreat`: a castable Destroy spell plus a 3/3
  opponent casts with the rule off and passes with it on.
- `TestRemovalNowFiresAtLargeThreat`: a 6/6 opponent casts in both
  configurations.
- `TestRemovalNowLethalFaceDamageFires`: a 3-damage burn against an
  opponent at 2 life fires in BOTH configurations despite the 1/1;
  `TestRemovalNowHoldsNonLethalFaceDamage` holds the same burn against 20
  life, and `TestRemovalNowUnknownDamageGetsNoFaceCandidate` holds an
  X-damage spell at 1 life.
- `TestRemovalNowNonCreatureDestroyFires`: a Shatter-shaped destroy casts
  in both configurations beside a 1/1.
- `TestRemovalNowIgnoresUntargetableThreat`: a hexproof 6/6 beside a
  targetable 1/1 is held (the 6/6 is no candidate);
  `TestRemovalNowIndestructibleSurvivesDestroy` holds a destroy against an
  indestructible 6/6 and fires the same board's bounce.
- `TestRemovalNowHeldSpellGetsNoTap`: the held 3-mana destroy with an
  empty pool and one untapped land draws a pass, not a land activation;
  with the rule off the same window taps.
- `TestRemovalNowCastEntriesSkipHeld`: a tuned cast profile's
  `castEntries` table (the C11 `setEfficiency` census) counts no held
  spell, so follow-up value cannot credit a cast the policy would refuse.
- `TestRemovalKindAdaptersAgree`: destroy, damage, artifact destroy, bounce,
  a graveyard return and an X-damage spell parse to the same kind, scope and
  damage through `view.Project` (the real projection) and both Board
  adapters.

The board-table parity reference (`board_table_equiv_test.go`) was extended
with the new facts, so every whole-game parity test also holds the two
adapters to the same `RemovalKind`, `RemovalTargets` and `RemovalDamage`.

## What was not ported

- `useRemovalNow`'s `Pump` branch (outside this ticket).
- Forge's full target-legality read. The fold's filter is keyword-shaped
  (Hexproof/Shroud/Protection/Indestructible) at the grain the Board
  carries; it does NOT model a protection quality against the spell's
  colours (over-excluding, which can only fire a turn early -- a fired
  spell at worst spends mana, a held one wastes it), Ward (a cost gate,
  not a targeting illegality), or the engine's own per-target census. The
  rule holds or fires a spell; it never names the target, so an
  approximation here bends WHEN, never WHETHER a target was legal -- the
  engine's target ask still refuses an illegal one.
- Forge's `RemovalEffect` SVar hints. The facts here are the printed
  ability shape, not card-script annotations.
- Any deck-posture or win-rate claim. The Strength claim on
  `forge-heuristic` was not re-measured for this rule.

## Why it is opt-in

The A2 measurement showed that the 30-deck constructed corpus poses only a
handful of removal-shaped asks per game, and the ticket's gate is a win-rate
change. A1 can change those asks, but no measurement has been run, so the
rule stays on the experimental policy and the default path is untouched.
