# A6 timing: counters and draw/token, opt-in, with firing evidence

Date: 2026-10-20.
Tickets: A6b and A6c of the A6 split
(`docs/design/2026-10-19-forge-heuristics-go-port-design.md`).
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

## Why it is opt-in

The rule changes when a counter is spent, and no win-rate measurement has
been run for A6. The policy stays experimental and the default path is
untouched.
