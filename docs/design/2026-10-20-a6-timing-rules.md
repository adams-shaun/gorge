# A6b counter timing: opt-in, with firing evidence

Date: 2026-10-20.
Ticket: A6b of the A6 split
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

## Why it is opt-in

The rule changes when a counter is spent, and no win-rate measurement has
been run for A6. The policy stays experimental and the default path is
untouched.
