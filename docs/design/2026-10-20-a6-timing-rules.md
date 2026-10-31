# A6 timing: counters, draw/token and pump-in-combat, opt-in, with firing evidence

Date: 2026-10-20.
Outcome: **implemented and armed only on the experimental `forge-heuristic`
policy.** The default `bot` is unchanged and no win rate is claimed.

## What was built

The shared A6 timing infrastructure plus its first rule, a clean-room
re-expression of Forge's counter timing in gorge's own vocabulary
(`bots/botpolicy/timing.go`):

- `TimingRules` (`CounterTop`) on `Board.Timing`, zero value off, armed
  through `heuristic.Bot.EnableTimingRules`.
- `timingNow(d, o)` -- the single cast gate. Each armed rule returns true for
  a card it does not govern.
- `castHeld(d, id)` -- the tap gate's half, composing A1's `removalHeld` with
  `timingNow`. It replaces the three `removalHeld` call sites in `tap.go`
  (`tapWants`, `bestUnpayable`, `AnyCastableNow`), so the main-phase tap gate
  never floats mana toward a held spell.
- `timedTap(d)` -- the out-of-main tap gate: among castable instant-speed
  cards the rule governs and the window can pay, pick the best `cardScore`
  (ties to lowest object id) and hand it to `tapToward`, the tail of
  `chooseTap` extracted verbatim.
- `stackTop(skip)` and `timingTargetPick(d)` -- the target half, so a counter
  targets the spell just below it rather than the highest-CMC foreign spell
  the ordinary removal ranking leads with.

## The counter rule

Forge counters a spell only when it is on top of the stack, refuses its own
team's spell, and counters CMC 1 30%, CMC 2 75% and CMC >= 3 always (CMC 0
is ungated). The probabilistic bands are dropped; the deterministic majority
reading is: **cast a counter only when the topmost stack entry is a foreign
spell whose mana value is not 1.** CMC 0 is countered (`CmcOf` counts {X} as
0).

The rule hooks the existing cast scorer after C8 and A1, so it runs only on
a decision that already offers a castable spell, and it holds or fires a
cast -- it never raises a score. The target half is gated by the same flag:
with the rule off, `chooseTargets` takes the ordinary removal ranking
unchanged.

Not ported: holding up mana for a counter in main 2 (Forge's main-2
prediction skips counters, so "hold up mana for a counter" is not Forge
behaviour), ability counters, and any probabilistic term.

## The draw/token rule (A6c)

`DrawTokenLate` is the second armed rule. `Card.Draw` and `Card.Token` are
the card's primary `SP$` API ("Draw"/"Token"), filled on the deciding seat's
own-zone entries by both adapters and left false on a foreign entry, exactly
like `Card.Counter`.

- A sorcery-speed draw or token is held in the seat's own first main; the
  second main re-offers it, so it fires there.
- An instant draw fires only at an opponent's end step, or when the seat's
  hand is at most one card.
- An instant token fires only on an opponent's turn from the declare
  attackers step through the end step.

The rule reads the same `Board.MyTurn`, `Board.Step`, `Board.FirstMain` and
`Board.HandSize` facts the cast scorer already carries, so no new engine or
view fact was needed. `timedCard` includes an instant draw/token so the
out-of-main `timedTap` can float mana toward it in the allowed window;
`sorcery-speed` draw/token is held by `timingNow` only.

Approximations, recorded here rather than in AGENTS.md: a sorcery token with
haste is deferred too, and in a multiplayer game any opponent's end step
counts as "the turn before the AI's".

## The pump-in-combat rule (A6a)

`PumpInCombat` is the third armed rule, a clean-room reading of Forge's
combat-trick timing. `Card.PumpPower`/`Card.PumpToughness` are the literal,
non-negative `NumAtt$`/`NumDef$` of the card's primary `SP$ Pump` ability when
its target scope names a creature (`effects.CastPumpBonus`); a curse, an {X}
or SVar amount, a mixed sign, a non-creature target and an opponent-only
target scope all read 0/0, so the rule never treats an unknown as a pump.
`Card.PumpOwn` (`effects.CastPumpOwnCombatants`) says whether that filter can
name both the caster's own attacking and blocking creatures: an attacking-only
(Run Amok), blocking-only, non-attacker, defending-player or otherwise
narrowed scope answers false, and the rule holds the cast rather than firing
it and falling through to the ordinary target ranking, which leads with the
opposing creature.

The cast gate fires only when all four hold:

- the filter can name the seat's own combatants (`Card.PumpOwn`);
- `Board.Step == state.StepDeclareBlockers` -- the one window the trick is
  for;
- no spell the deciding seat controls is already on the stack (a second pump
  cannot affect this combat before the first resolves);
- `bestPumpTarget` finds a combatant the bonus actually helps.

`bestPumpTarget` simulates the combat with and without the bonus through the
same `blockCombat` the combat heuristic uses, and scores each own creature
that is attacking or blocking (shrouded ones excluded):

- **save** (3): it dies unpumped and survives pumped, or a blocking creature
  dies unpumped and survives pumped;
- **kill** (2): an opposing creature survives unpumped and dies pumped (for
  an attacker, one of its blockers; for a blocker, the attacker);
- **lethal** (1): an unblocked attacker's team total is below the defender's
  life and the bonus takes it to at least that life.

Indestructible creatures are never saved or killed by damage, so those classes
skip them. The winner is the highest class, then the highest `threat()`, then
the lowest object id -- a deterministic max fold with no map-order input. The
target half (`timingTargetPick`) reads the pump bonus from the resolving stack
entry (`d.Source`) and picks that combatant from the offered options.

Approximations, recorded here rather than in AGENTS.md: the combat simulation
orders blockers and damage assignment by object id; it models neither trample
nor infect; and an attack against a planeswalker is counted against that
planeswalker's controller's life.

## Firing evidence

The focused botpolicy tests pin both directions:

- `TestCounterTopSkipsOneDrop`: a CMC-1 foreign top is held with the rule on
  and countered with it off; CMC 3 and CMC 0 are countered in both.
- `TestCounterTopNeedsForeignTop`: the seat's own spell on top is not
  countered (the foreign spell below it does not count).
- `TestCounterTopTargetsTop`: with the rule on the counter targets the top
  spell below it; with the rule off the same ask leads with the
  highest-CMC foreign spell.
- `TestCounterTopTimedTap`: outside a main phase the armed rule activates a
  mana source toward the counter; a CMC-1 top draws a pass.
- `TestCounterTopDefaultUntouched`: the zero `TimingRules` is the rule-off
  board.
- `TestCounterTopHeldGetsNoMainTap`: a counter with no foreign spell on the
  stack draws a pass instead of a main-phase tap (the `castHeld` half).

The A6c rule's focused tests pin both directions:

- `TestDrawSorceryWaitsForMain2` and `TestTokenSorceryWaitsForMain2`: the
  first main holds the spell, the second main casts it, and the rule-off
  board casts it in the first main.
- `TestInstantDrawWaitsForEndStep`: an opponent's upkeep holds the draw, the
  end step fires it, and a one-card hand fires it early.
- `TestInstantTokenWaitsForAttack`: an opponent's upkeep holds the token
  spell, the declare-attackers step fires it.
- `TestDrawTokenAdaptersAgree`: a parsed Divination and Raise the Alarm
  classify as Draw and Token on the projected `CardView` and on both Board
  halves, and Lightning Bolt as neither.

The A6a rule's focused tests pin both directions:

- `TestPumpInCombatHoldsWithoutTarget`: with no worthwhile combatant the
  armed rule passes and the zero rules cast.
- `TestPumpInCombatFiresSavingAttacker` and
  `TestPumpInCombatFiresSavingBlocker`: the bonus saving a 2/2 that would die
  fires the cast.
- `TestPumpInCombatFiresKillingBlocker`: an attacker that survives either way
  still fires the cast when the bonus kills the blocker.
- `TestPumpInCombatFiresLethal`: a 2-power unblocked attacker against 4 life
  fires; an attack already at lethal is held.
- `TestPumpInCombatHoldsOutsideDeclareBlockers`,
  `TestPumpInCombatHoldsWhenOwnSpellOnStack` and
  `TestPumpInCombatHoldsForIndestructible`: the window, own-stack and
  indestructible boundaries hold the pump.
- `TestPumpInCombatTargetsBestClass` and
  `TestPumpInCombatTargetPicksLowestIDOnTie`: the target half picks the save
  over the kill and breaks a full tie on the lowest id.
- `TestPumpInCombatDefaultUntouched`: the zero `TimingRules` is the rule-off
  board.
- `TestPumpAdaptersAgree`: a parsed Giant Growth projects +3/+3 on the
  `CardView` and on both Board halves, while a curse, a dynamic amount and a
  non-creature target project 0/0, and an attacking-only or blocking-only
  scope projects the literal bonus with `PumpOwn` false.
- `TestPumpHoldsRestrictedTargetScope`: a `Creature.attacking` pump that could
  only "save" a blocker the card cannot name is held at declare blockers.
- `TestCastPumpBonusReadsTheCompiledPump` and
  `TestCastPumpOwnCombatantsReadsTheTargetScope`: the reader's
  unknown-is-zero and target-scope boundaries.

## Why it is opt-in

The rule changes when a counter is spent, and no win-rate measurement has
been run for A6. The policy stays experimental and the default path is
untouched.
