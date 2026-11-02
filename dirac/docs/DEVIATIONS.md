# Deliberate differences from gorge

Lockstep plays a game on both engines and compares every decision and the state behind it
([verification.md](verification.md)). A divergence on a supported card is a failure: it is fixed, or the
card is refused. Two kinds of difference are not divergences, and each row below names its kind.

`C` is the canonical surface: the engines express the same game in different words, and the oracle's
adapter translates between them, so the games match. `capacity` is a declared limit: the game stops as a
named stall, counted and reported, and is not compared past that point.

Nothing else belongs in this file. `TestDeviationsRegisterHasNoRealDifference` holds it to those two
classes. Gorge behaviour dirac copies, and the differences still open, are in
[GORGE-FINDINGS.md](GORGE-FINDINGS.md).

- [C] The priority vocabulary is the autopay one: no mana-ability options and
  no concede. Casts are offered when sources plus pool can pay, unless
  GorgeAutopaySurface is set.
- [C] Single-option decisions are elided, and declarations are sequential picks
  closed by Done, where gorge makes one subset choice.
- [C] Ward and granted prowess are dirac-native triggers.
- [C] Every cost sacrifice happens after the mana is paid (gorge taps the
  planned sources at priority, CR 601.2g-h).
- [C] An as-enters choice is asked on the stack when the replacement program
  starts with it (a played land's while it is still in the hand), and
  otherwise after the permanent enters.
- [C] Spell copies (Thousand-Year Storm): a copy whose target declaration takes several objects (Run Away Together,
  Macabre Waltz) is asked as one pick per object closed by Done (gorge's
  one subset decision, as the oracle reposes it); dirac's path is
  tested in dirac/copytargets_test.go, but no lockstep game has reached
  it yet (0 in the 118 census pairs with Storm and such a spell).
- [capacity] A game that outgrows dirac's capacity (battlefield, objects, pending triggers, stack,
  override pools, a card zone holding tokens until the next state-based check (`zone`), a combat
  declaration with more pairs than MaxOptions (`options`)), or a turn that resolves more than 2000
  triggers, ends as a draw with `StallReason`; the lockstep reports it as a "kernel <reason>" stall.
- [capacity] Board: the battlefield holds `MaxPerm` permanents, and `nearCapacity` stops a token maker
  just short of them. Measured, a stall is an exponential token maker (Krenko, Mob Boss far more than any
  other card): played on in gorge past the stall, such games outgrow any fixed board, so a larger one only
  moves the stall later.
- [capacity] Endure Spirits: every live Endure Spirit (0/0 with a layer-7b N/N override) holds one of the
  `maxFx` effect records for as long as it lives, so a game that fills them ends the next Endure as a
  `StallTables` draw. `TestEndureLockstep` counts it.
- [capacity] The activation ring is gorge's `activationsThisTurn` for
  `Count$ThisTurnActivated_Activated.YouCtrl+inZoneGraveyard` (Laboratory Drudge) only: the first
  `actRingN` stack-pushed abilities per player per turn; the next stalls with `StallActRing`.
- [capacity] The cast ring is gorge's this-turn cast ledger, bounded to the first `castRingN` pushes per
  player per turn; the next stalls with `StallCastRing`. What it emulates of gorge's reads is in
  [GORGE-FINDINGS.md](GORGE-FINDINGS.md).
