# Attacking move entries — review-fix round (sol5)

Fixes the two findings on the REQUEST_CHANGES verdict over `a0bac590`.

## [MAJOR] one loud Note per ChangeZone CALL, not per moved object

`applyAttackingEntry` classified the rider inside every mover loop, so a
defenderless `Attacking$ True` ChangeZone that moved two permanents emitted two
identical no-defender Notes (and a two-card `Attacking$ Remembered` move two
selector Notes) — contrary to the brief and to `effects/token.go`'s single-mint
read it mirrors.

The rider is now a value classified ONCE per move call, before the mover loop:

- `effects/zone.go` — `applyAttackingEntry` is replaced by the `attackingEntry`
  value (`attackingEntryNone/Attacks/NoDefender/Unsupported`),
  `classifyAttackingEntry(c, sa, to)` (pure — it emits nothing) and
  `(*attackingEntry).apply(h, c, id, player, to)`, which delivers the derived
  entry state per object and emits the single diagnostic at most once
  (`noted`). Classification is hoisted before the loop in `effChangeZone`
  (object path), `handMoveOwnersWalk` (hand movers), `moveDefinedLibraryObjects`,
  `effHiddenPick`, `applyLibrarySearch` and `effChangeZoneAll`;
  `settleChangeZoneMove`/`settleChangeZoneMoveAs` take the hoisted rider as a
  parameter (nil = no rider) instead of re-reading the SA per object.
- `effects/cardflow.go` — `effDig` classifies once before the target walk,
  `effDigUntil` once before the player walk (against `ZBattlefield`, since
  `apply` gates on the destination it is handed).

The diagnostic still rides the first entry the call actually settles, so a call
that moves nothing stays silent exactly as before — the change is strictly
"N Notes become 1", no new event on any path.

New regression tests (`effects/attacking_entry_test.go`), each of which fails
on the old per-object code with 2 Notes:

- `TestAttackingEntryMultiObjectNoDefenderNotesOnce` — two-object
  `effChangeZoneAll` sweep, no defender: both enter tapped and non-attacking,
  exactly ONE Note.
- `TestAttackingEntryMultiObjectSelectorNotesOnce` — same sweep with
  `Attacking$ Remembered`: exactly ONE Note, and it names the selector.
- `TestAttackingEntryDigNoDefenderNotesOnce` — a two-card Dig onto the
  battlefield: both taken cards enter tapped, exactly ONE Note.

The existing pins were rewritten onto the classify/apply pair unchanged in
substance.

## [MINOR] the named Alesha coverage

`TestAleshaReturnsCreatureTappedAndAttacking` still cannot exist, and the
substitution is now recorded where a reader meets it (the doc comment on
`TestYoreTillerNephilimReturnsCreatureTappedAndAttacking`) as well as here and
in the commit message. Re-measured at this commit rather than taken from the
previous round's report: with Alesha attacking and TWO untapped white sources
on her controller's battlefield, the triggered-cost window poses

```
CHOOSE player=0 prompt="Alesha, Who Smiles at Death - pay W/B W/B?" opts=[trigger_cost_decline/Do not pay]
```

— one option, decline. Her `AB$ ChangeZone ... Tapped$ True | Attacking$ True`
body therefore never resolves and no assertion about its entry rider is
possible. The gap is hybrid mana at triggered-cost windows, filed as
`agent-20260922T232740Z-cf0357bb`, and is out of this ticket's scope.
Thunderkin Awakener, the brief's other object-path carrier, is blocked the same
way by `agent-20260922T221917Z-aa93a144` (ValidTgts$ with an SVar-X comparison
resolves no target). `TestYoreTillerNephilimReturnsCreatureTappedAndAttacking`
covers the identical inlined object-loop move with a literal spec and no cost,
end to end through combat damage and replay.

Per AGENTS.md this remainder is recorded in the commit message and this report
only; the Known approximations register is untouched.

## Gates

See the commit message / handback for the exact commands and results.

## Issues

- The 13 non-literal `TokenAttacking$` selector lines remain a deliberate
  loud-Note degrade in `effects/token.go` / `effects/copypermanent.go`
  (unchanged, out of scope).
- `DB$ Meld` remains unregistered, so its one `Attacking$ True` line is inert.
- Alesha and Thunderkin remain unreachable through the two follow-up tickets
  above.
