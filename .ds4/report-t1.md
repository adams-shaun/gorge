# Report — agent-20260922T193437Z-a964eea4

**Ticket:** finish the `EffectOwner$` identity half of the generic `Effect`
`Triggers$` registration, add `OneOff$` one-shot semantics, and pin the landed
generic registration.

**Status:** DONE. Commit `648ed4eb`.

## What changed and why (per file)

### `effects/context.go`
- New exported `EffectOwnerPlayers(h Host, c *Ctx, raw string) ([]state.PlayerID, bool)`
  — the ONE home for the `EffectOwner$` selector grammar used by both the
  generic `effEffect` path and the pregame path. `""`/`You` → the resolving
  controller; `Opponent`/`Other` → every other surviving seat; every other
  spelling is resolved through the shared `Defined$` referent grammar
  (`knownDefinedTargets` → `definedSpec`) and mapped to seats under Forge's
  `getDefinedPlayers` rule (so `TriggeredTarget`, `TriggeredDefendingPlayer`,
  `Targeted`, `Player.IsRemembered` work). `TargetedOwner` — which the brief
  believed `definedSpec` already implemented, but which it does **not** (see
  "Brief premises that did not hold") — is handled here from `ownersOf(g, c.Targets)`
  rather than by widening `definedSpec`, so unrelated cards that carry
  `Defined$ TargetedOwner` (Chaos Warp, Lodestone Bauble) are untouched.
  The bool distinguishes "unmodelled spelling" from "named nobody"; the caller
  fails closed on both.

### `effects/misc.go` (`effEffect`, the `Triggers$` loop)
- The owner is resolved **lazily** (`resolveOwners` closure) on the first arm
  that needs it. An `EffectOwner$` that cannot resolve emits a loud
  `"unresolvable EffectOwner$ <sel> (Triggers$ not registered)"` Note and
  registers nothing — it never silently defaults to the source controller.
- The three generic arms (`SpellCast`/`ChangesZone`, `Phase`, `default`)
  now emit one `DelayedRegister` **per owner** instead of one with
  `Player: c.Controller`. The firing-time matcher already prefers
  `dt.Controller` (`rules/trigger_delayed.go:297`), so matching follows for
  free.
- `OneOff$ True` bodies on the two event modes the delayed machinery can
  consume without minting a stack object (`SpellCast`, `ChangesZone`) drop the
  `|EF` marker (`efMarker`), so the registration is one-shot: `DelayedPush`
  removes a non-`EffectRepeat` registration on its first firing. Other modes
  keep the recurring form (their non-repeat dispatch is not wired; forcing
  one-shot would make them inert, not one-shot).
- The `BecomeMonarch` arm is **deliberately left with `Player: c.Controller`**
  (deviation from the brief — see below).

### `rules/opening_hand.go`
- `openingEffectOwners` now delegates to `effects.EffectOwnerPlayers`, so the
  pregame and generic paths cannot disagree about the owner set.
- New `openingEffectAlreadyRegistered(card, owner, mode, exec)` and a check
  before each pregame `DelayedRegister`: the pregame pass skips a body the
  generic path already minted for the same source/owner/Execute. This is the
  **OneOff home I chose**: make the pregame path not double-register. When the
  generic path withheld the body (e.g. the lifetime guard blocked a
  longer-lived Effect — Chancellor of the Annex's `Duration$ Permanent`), the
  check returns false and the pregame pass registers it, so the opening-hand
  case keeps working.

### `rules/effect_trigger_owner_test.go` (existing test adapted)
`TestEffectTriggerMatchesRegistrationOwner` pre-resolved `EffectOwner$ Opponent`
and passed `ctx.Controller = owner`; now that `effEffect` resolves the selector
itself, that double-applied it. The setup now enters resolution as the source's
controller (the behavior moved from the caller into `effEffect`); assertions are
unchanged and still prove the false/true firing halves.

### `rules/effect_trigger_owner_generic_test.go` (new — all new tests live here)
- `TestEffectTriggerOwnerTriggeredTarget` — Valiant Batrider's shape
  (`EffectOwner$ TriggeredTarget`, `TriggerContext.TriggerTarget` = seat 1):
  registration owned by seat 1, seat 0's damage does not fire, seat 1's does.
- `TestEffectTriggerOwnerTargetedOwner` — an object whose **owner differs from
  its controller** (owner 1, controller 0) resolves to seat 1.
- `TestEffectTriggerOwnerTargetedPlayer` — `EffectOwner$ Targeted` on a player.
- `TestEffectTriggerOwnerUnresolvableFailsClosed` — an unmodelled selector
  registers nothing AND the loud Note is present.
- `TestEffectOneOffDoesNotDoubleFireFromOpening` — an opening-hand `OneOff$ True`
  SpellCast Effect registers exactly ONCE, one-shot; the first cast fires it
  once and consumes it; a second cast does not re-fire.
- `TestEffectGenericRegistrationHoldsOnCorpus` — real corpus: Bonus Round's
  `A:SP$ Effect | Triggers$ TrigSpellCast` arms a recurring `SpellCast`
  registration.

## Deviation from the brief (with reasons)

**1. The `BecomeMonarch` arm is NOT owned by `EffectOwner$`.**

The brief says "in every registering arm". That is wrong for this arm. Its
registration `Player` is the **monarch relation's anchor**: `rules/trigger_delayed.go`
reads `ValidPlayer$ Player.OpponentOf Remembered` against `dt.Controller`. For
Palace Jailer the oracle is "until an opponent **of the Jailer's controller**
becomes the monarch", and the engine deliberately (and reviewed-ly) anchors on
the source's controller. Palace Jailer carries `EffectOwner$ TargetedOwner`,
but resolving it here would move the anchor to the exiled creature's owner and
return the creature when the Jailer's **own** controller takes the crown — the
opposite of the card text. Three existing tests pin this:
`TestPalaceJailerReturnsToThirdSeatOpponentMonarch`,
`TestPalaceJailerStaysExiledWhenJailerControllerTakesCrown` (the negative
control) and `TestPalaceJailerComeBackRegistersAndFiresOnOpponentMonarch`. My
first attempt did fan the arm out and those three tests failed; I reverted that
arm. `TargetedOwner` is still a supported selector for the generic event/phase
arms (proved by `TestEffectTriggerOwnerTargetedOwner`). The reason is recorded
in a code comment at the arm.

**2. `TargetedOwner` is handled inside `EffectOwnerPlayers`, not in `definedSpec`.**

The brief asserted `definedSpec` already implements `TargetedOwner`. It does
not. Adding it there would change 18 unrelated `Defined$ TargetedOwner` corpus
lines, including two repo-deck cards (Chaos Warp, Lodestone Bauble) — an
off-brief behavior change. Since the brief only asks for `EffectOwner$` support,
I mapped the one owner-suffix spelling inside the EffectOwner helper and left
`definedSpec` alone. (The general `Defined$ TargetedOwner` gap is filed under
`## Issues`.)

## Brief premises that did not hold (measured)

1. **`TargetedOwner` was not implemented.** `grep -n 'case "TargetedOwner"'
   effects/context.go` returned nothing before this change; the only readers of
   `EffectOwner$` were `rules/opening_hand.go`. The brief's "`TargetedOwner`
   family already implements" is false.
2. **The carrier ↔ deck intersection is NOT empty.** The brief said it was
   empty ("`TestConstructedDefaultIsByteIdentical` should NOT move"). Measured:
   5 carrier cards are in the deck universe —
   `Alchemist's Gambit`, `Hunter's Insight`, `Love on the Battlefield`,
   **`Palace Jailer`**, `Ugin, the Ineffable`. Palace Jailer is in
   `death-n-taxes`, which is the **first Legacy deck** the chain heads seat
   (`internal/testutil/decks.go` `legacyDeckNames`). This is exactly why I kept
   the `BecomeMonarch` arm byte-identical: changing it would have moved the
   chain heads through a deck card the brief believed was absent.
   (The other four are in non-Legacy decks and are unaffected anyway: the two
   `DB$ Effect` carriers without `EffectOwner$` keep `c.Controller`; `Love on
   the Battlefield` (`UntilEndOfCombat`) and `Ugin` (`ForgetOnMoved$`) are
   blocked by the guard the sibling ticket owns.)
3. **Deck-name count is 1022, not 1042** (measured
   `grep -hoE '"name": *"[^"]+"' internal/testutil/decks/*.json | sort -u`).
4. **"All six EffectOwner$ carriers are blocked by the lifetime guard"**
   glosses over Palace Jailer: its `BecomeMonarch` body is **exempt** from the
   guard, so it registers today. The six-selector census (6 lines) otherwise
   held.

## Head / ratchet / botbench movement

None observed, and none expected:
- `TestConstructedDefaultIsByteIdentical` — **PASS** (unchanged).
- No Legacy-deck card's registration changed: the only Legacy carrier is
  Palace Jailer, whose arm is unchanged; the remaining four are in non-Legacy
  decks and unchanged as argued above. No Legacy `OneOff$ True`
  SpellCast/ChangesZone carrier exists (measured).
- The `knownUnsupported` / `knownUnsupportedParams` ratchets cannot move: no
  new primitive/param is registered, only runtime owner/one-shot behavior.

`TestHeads` is a daemon gate (not named by the brief) and was not run; the
analysis above is why no seat-count golden should move.

## Known-approximations row

The `Effect` row at `AGENTS.md:217` is compound (it also covers
`StaticAbilities$` modes, the exotic `Duration$` grammar, and the other
Effect-delivered grant keywords). **This ticket does not close it; the row is
left as-is and `knownApproximationRows` is unchanged (9).** The `EffectOwner$`
selectors supported vs. still-failing-closed and the one-shot semantics are
reported here, not written into the table.

## Gates (real output)

All run from the worktree root with `.cards` present as a symlink to the real
corpus (verified: `.cards -> /home/sadams/projects/gorge/.cards`).

```
$ go build ./...
BUILD_OK

$ go test -run 'TestEffectTriggerMatchesRegistrationOwner|TestEffectTriggerOwner|TestEffectTriggerExpires|TestEffectEventModes|TestEffectTriggerBodySelfExile' ./rules/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/rules	0.050s

$ go test -run 'TestEffectTriggerMatchesRegistrationOwner|TestEffectTriggerOwner|TestEffectTriggerExpires|TestEffectEventModes|TestEffectTriggerBodySelfExile|TestEffectTriggerExpiryAndUnsupportedLifetime|TestChancellor|TestOpeningEffect|TestPalaceJailer|TestEffectOneOff' ./rules/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/rules	0.607s

$ go test -run 'TestEffectTriggerOwner|TestEffectOneOff|TestEffectGenericRegistration|TestPalaceJailer|TestKnightsOfTheBlackRose|TestCustodiLich|Monarch|TestEffectTriggerMatchesRegistrationOwner|TestEffectTriggerExpires|TestEffectEventModes|TestEffectTriggerBodySelfExile|TestEffectTriggerExpiryAndUnsupportedLifetime|TestChancellor|TestOpeningEffect' ./rules/ > .ds4/scratch/all3.log 2>&1; tail
ok  	github.com/adams-shaun/gorge/rules	0.531s

$ go test ./effects/
ok  	github.com/adams-shaun/gorge/effects	16.783s

$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.451s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	0.878s

$ gofmt -l effects/misc.go effects/context.go rules/opening_hand.go rules/effect_trigger_owner_generic_test.go rules/effect_trigger_owner_test.go
(no output)

$ go run ./cmd/gentypes -check
(no output, exit 0)
```

## Fails without the fix

I copied the three changed non-test files to `.ds4/scratch/`, reverted the four
behavioral hunks (owner selection → source controller; `TargetedOwner`
resolution removed; `|EF` always; pregame dedupe always false), ran the new
tests, then restored the files byte-identically (`cmp` clean on all three).

```
$ go test -run 'TestEffectTriggerOwner|TestEffectOneOffDoesNotDoubleFireFromOpening|TestEffectGenericRegistrationHoldsOnCorpus' ./rules/
--- FAIL: TestEffectTriggerOwnerTriggeredTarget (0.00s)
    effect_trigger_owner_generic_test.go:45: precondition: expected a recurring Effect registration owned by seat 1: [{ID:0 Phase:main1 Source:81 Controller:0 Execute:Pain ... EventMode:DamageDone Trigger:Hook EffectRepeat:true ...}]
--- FAIL: TestEffectTriggerOwnerTargetedOwner (0.00s)
    effect_trigger_owner_generic_test.go:79: precondition: TargetedOwner must resolve to seat 1: [] (ok=false)
--- FAIL: TestEffectTriggerOwnerTargetedPlayer (0.00s)
    effect_trigger_owner_generic_test.go:103: EffectOwner$ Targeted (player) registration = [{... Controller:0 ...}], want controller 1
--- FAIL: TestEffectTriggerOwnerUnresolvableFailsClosed (0.00s)
    effect_trigger_owner_generic_test.go:121: unresolvable EffectOwner$ must register nothing, got [{... Controller:0 ...}]
--- FAIL: TestEffectOneOffDoesNotDoubleFireFromOpening (0.00s)
    effect_trigger_owner_generic_test.go:145: opening OneOff$ effect registered 2 delayed triggers, want exactly 1: [{... EffectRepeat:true ...} {... EffectRepeat:false ...}]
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.438s
```

Restore verified:

```
$ cmp effects/misc.go .ds4/scratch/misc.go.fixed2 && echo misc OK
misc OK
$ cmp effects/context.go .ds4/scratch/context.go.fixed2 && echo context OK
context OK
$ cmp rules/opening_hand.go .ds4/scratch/opening_hand.go.fixed2 && echo opening OK
opening OK
```

## Landed generic registration — already pinned (cited)

The landed generic registration for matcher-backed modes is pinned by
`rules/effect_event_modes_test.go` (`TestEffectDamageDoneTriggerRepeatsWithinTurn`,
`TestEffectSpellCastTriggerRepeatsWithinTurn`,
`TestEffectChangesZoneTriggerRepeatsWithinTurn` — registration with
`EffectRepeat` plus repeat-within-turn firing) and by
`rules/effect_frame_trigger_test.go` (the one-shot self-exile frame). I did not
duplicate them; instead I added `TestEffectGenericRegistrationHoldsOnCorpus`,
which pins the **real corpus** carrier the report named (Bonus Round:
`A:SP$ Effect | Triggers$ TrigSpellCast`, `Mode$ SpellCast`, no `Duration`)
registering a recurring `SpellCast` Effect trigger.

## `EffectOwner$` selectors: supported vs failing closed

- **Supported (generic event/phase arms):** `""`/`You`, `Opponent`/`Other`,
  `TriggeredTarget`, `TriggeredDefendingPlayer`, `Targeted`,
  `Player.IsRemembered`, `TargetedOwner`.
- **Also resolve through the shared grammar** (not the six named, but no longer
  rejected): `TriggeredPlayer`, `TriggeredActivator`,
  `TriggeredSourceController`, `Remembered`, `RememberedController`,
  `RememberedOwner`, `CardOwner`, `ImprintedOwner`, … — whatever
  `knownDefinedTargets` recognises.
- **Fail closed loudly (register nothing + Note):** `Player.Opponent`,
  `RememberedOwner.Opponent` and any other spelling
  `knownDefinedTargets` does not recognise.
- The `BecomeMonarch` arm intentionally does not use the owner at all.

## Issues (found, not fixed)

1. **`Defined$ TargetedOwner` is not implemented in `definedSpec`** (18 raw
   corpus lines, e.g. `vanish_from_sight`, `oblation`, `blink`, and the
   `AlternativeDecider$ TargetedOwner` family). Today those fall through
   `Defined`'s source/chosen-target fallback to the wrong referent (the source).
   Reproduce: `/usr/bin/grep -rlE 'TargetedOwner' .cards/cardsfolder | wc -l`
   → 58 files. Two repo-deck cards carry it (`Chaos Warp`,
   `Lodestone Bauble`). This is a separate, real bug and should be its own
   ticket (add a `definedSpec` case returning `ownersOf(g, c.Targets)` and pin
   each consumer). It deserves a CR-lane test citing CR 108.3 (owner) and
   CR 701.x/61.x as applicable; I did not write it because it is outside this
   brief.
2. **`OneOff$ True` Effect trigger bodies on modes other than
   `SpellCast`/`ChangesZone` still register recurring** (`|EF`): `AbilityCast`,
   `LandPlayed`, `SpellAbilityCast`, `Untaps` (measured `Mode$` census over the
   48 `OneOff$ True` lines). Their non-repeat dispatch is not wired in
   `rules/trigger_delayed.go`, so forcing one-shot would make them inert.
   A follow-up should wire the non-repeat path for those modes and make every
   `OneOff$ True` body one-shot.
3. **Effect trigger modes with no registered matcher remain a printed-trigger
   gap** (the brief's own list: `PlaneswalkedTo`, `Blocks`,
   `AttackerBlockedByCreature`, `AttackerBlocked`, `Abandoned`,
   `ChangesController`, `VisitAttraction`, `LosesGame`, `TurnBegin`,
   `PlaneswalkedFrom`, `Shuffled`, `DiscardedAll`, `SacrificedOnce`). Each new
   matcher must join `rules/trigmatch_registry_test.go`'s `addedAfterTheSplit`.
   Not touched here.

## Commit

```
648ed4eb feat(effects): resolve EffectOwner$ and OneOff$ for the generic Effect Triggers$ path
```
