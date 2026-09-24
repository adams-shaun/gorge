# Report — agent-20260923T144746Z-b284de7f

Dotted `AttachedTo <spec>` Defined-selector — Cass, Rhuk, Fumble, Murderous
Spoils.

Status: DONE. Commit `397ba4d6`.

## What changed and why (per file)

- **`state/object.go`** — new `Object.LastBearer ObjID` next to `AttachedTo`,
  documented as written ONLY inside `events.Apply` (the `LastNotedMana`
  contract). It backs the "objects that WERE attached to it" reads the
  trigger carriers resolve after the CR 704.5 sweep has cleared `AttachedTo`.
- **`events/apply.go`** — the three fold writes: `Unattached` sets
  `LastBearer = e.IDs[0]` (the former bearer, already carried on the event);
  `Attach` with `len(e.IDs) > 0` clears it on a re-attach; the
  Move-leaves-battlefield fold copies the pre-clear `AttachedTo` into
  `LastBearer` when it is non-zero. No `events.Event` field changed; the fold
  is a pure read of existing event bytes, so replay rebuilds it identically.
- **`effects/context.go`** — `attachedToDefinedSelector`, dispatched as a
  prefix at the top of `definedSpec` (so `knownDefinedTargets` and `Defined`
  reach it automatically). It reuses the predicate's canonical referent set
  and cardinality rule by calling the SAME function the predicate uses
  (`attachedToReferentObjects` on a `SpecContext` built from the `*Ctx`):
  absent binding, stale id, or a PLURAL bearer binding → unknown, fail closed.
  The attachment read is live `AttachedTo == bearer` PLUS the
  `AttachedTo == 0 && LastBearer == bearer` were-attached fallback, not
  zone-restricted (Cass's swept Auras are graveyard cards). The qualifier list
  is comma-OR over each attachment's own type/class words via
  `MatchesObjectCtx`.
- **`effects/attach.go`** — `Object$` now keeps the WHOLE resolved list
  (`objs`); `attachTo` takes the object as an argument; `attachAll` /
  `attachableBy` attach every resolved object a destination legally admits.
  Single-object carriers reduce to the exact previous reads (verified by the
  diff: `objs == []{obj}` on every single-object path).
- **`effects/zone.go`** — `effHiddenPick` consumes `ChooseFromDefined$`:
  intersect the origin-zone sweep with `knownDefinedTargets(value)`, fail
  closed with one Note on an unresolvable value, `OptionalPrompt$` becomes the
  prompt and an absent `ChangeNum$` with the param present means "any number"
  (pool-bounded). It also now applies `changeZoneAttachedTo` on a battlefield
  destination — the rider every other mover already applies, which the
  hidden-pick mover was missing. Required for Cass's returned
  `AttachedTo$ Targeted` Aura to sit on the target instead of being swept.

Which rule I reused for the referent: `attachedToReferentObjects`
(`effects/filter.go`) directly, not a re-implementation. It is the one home of
the 82db540a cardinality rule (exactly one bearer; empty Targeted stays bound;
plural fails closed) and of the referent→object reads, so the selector and the
filter predicate cannot drift. The function reads a `SpecContext`; `definedSpec`
has a `*Ctx`, so it builds one with `c.SpecContext(c.Controller)` — the same
conversion every other resolution-time filter read makes.

## Gates (exact commands, real output)

Targeted test run (brief's pattern):
```
$ go test -run 'TestAttachedToDefined|TestAttachedToSelector|TestRhuk|TestCassHand|TestFumble|TestMurderousSpoils' ./effects/ ./rules/
ok  	github.com/adams-shaun/gorge/effects	(cached)
ok  	github.com/adams-shaun/gorge/rules	0.439s
```

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	1.342s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	(cached)

$ gofmt -l state/object.go events/apply.go effects/context.go effects/attach.go effects/zone.go effects/attachedto_defined_selector_test.go rules/attachedto_defined_card_test.go
(empty)

$ go run ./cmd/gentypes -check
gentypes OK

$ go test -run TestHeads ./rules/
ok  	github.com/adams-shaun/gorge/rules	1.578s
```

TestHeads did NOT move (no seat count, no behaviour) — expected, since no
event bytes changed.

`.cards` was present as a symlink at `.cards -> /home/sadams/projects/gorge/.cards`
before the first run; the corpus-backed tests ran (rules package ~0.44 s with
the corpus, not the ~2 ms vacuous skip).

No `events.Event` field changed. No TEST_HISTORY budget change was needed (the
commit hook did not flag either package).

## Fails without the fix

Each reversion was made in the real non-test file, the test run, then the file
restored and `cmp`'d byte-identically against a `.ds4/scratch/fixcopy` copy
(all five non-test files reported `match`).

**1. Selector dispatch reverted** (`effects/context.go`, the
`attachedToDefinedSelector` call removed; `LastBearer` left in so the tests
compile):
```
--- FAIL: TestAttachedToDefinedSelectorTargetedLiveAndWereAttached
    AttachedTo Targeted classified unknown
--- FAIL: TestAttachedToDefinedSelectorQualifierList
    AttachedTo Targeted.Aura,Equipment classified unknown
--- FAIL: TestAttachedToDefinedSelectorTriggerReferents
    AttachedTo TriggeredCardLKICopy.Equipment classified unknown
--- FAIL: TestAttachedToDefinedSelectorBareFormUnchanged
    dotted AttachedTo Targeted.Equipment = [], false; want the Equipment
--- FAIL: TestMurderousSpoilsStealsEquipmentAttachedToDestroyedCreature
    Equipment controller = 1, want 0 (control gained)
--- FAIL: TestFumbleGainsBothAndAttachesThemToAnotherCreature
    GainControl left aura=1 equip=1, want both controlled by 0
--- FAIL: TestRhukAttachesWereAttachedEquipment
    Attacks half: Equipment AttachedTo = 2, want Rhuk 1
--- FAIL: TestCassHandOfVengeanceReturnsOnlyTheWereAttachedAuras
    the were-attached Aura 43 was never offered by the hidden pick
FAIL
```

**2. effAttach plural reverted to first-take** (`objs = append(objs, os[0].Obj)`):
```
--- FAIL: TestRhukAttachesWereAttachedEquipment
    death half: first Equipment AttachedTo = 0, want Rhuk 1
FAIL
```

**3. `changeZoneAttachedTo` rider removed from the hidden-pick mover**:
```
--- FAIL: TestCassHandOfVengeanceReturnsOnlyTheWereAttachedAuras
    returned Aura AttachedTo = 0, want the target 2
FAIL
```

**4. ChooseFromDefined fail-closed Note removed**:
```
--- FAIL: TestAttachedToSelectorChooseFromDefinedFailsClosed
    no ChooseFromDefined$ fail-closed Note was emitted
FAIL
```
(With the Note suppressed the decoy-Aura assertion still held, so the Note is
its own observable half of the fail-closed contract.)

Every test asserts its own precondition: the object is on the battlefield /
in the zone the rule reads, the compared values differ, the referent really is
bound, the Equipment really is attached before the read, and the decoy really
has `LastBearer == 0`.

## Deviations from the brief

- **`effHiddenPick` now applies `changeZoneAttachedTo`** (4 lines + comment).
  The brief said `changeZoneAttachedTo` "already works; do not touch it" — the
  function is fine, but the hidden-pick mover never CALLED it, so Cass's
  returned Aura entered unattached and the CR 704.5m SBA would sweep it. This
  is the one gap the Cass carrier needs closed; the change is the same rider
  at the same point as every other mover (zone.go:3709, 3765). Measured blast
  radius: 8 corpus files carry `Hidden$ True` + `AttachedTo$` on one line and
  were silently entering unattached — a fix, not a regression.
- **`ChooseFromDefined` now fails closed for the values with no resolver.**
  Before the ticket the param was read NOWHERE, so all 17 carriers offered the
  whole origin zone; now the resolvable ones (`AttachedTo …`, and the
  `Remembered`/`Remembered.<qual>` family, which `knownDefinedTargets` already
  recognises) are intersected and the rest offer nothing with the Note the
  brief prescribes. 15 occurrences / 12 files are now loud-unimplemented
  rather than quietly-wrong; filed as a follow-up ticket (see Issues).
- **The brief's Cass test expected the destination ask to be driven by the
  engine's suspension.** A direct `effects.Resolve` outside a stack frame does
  not resume a `hidden_pick` ask in this test harness, so the Cass test drives
  the answered re-entry directly (`Ctx.HiddenPick`/`HiddenPickDone`) — the same
  "effects.Resolve with the binding" shape the Arna source-filter test uses.
  Both the pool (asking pass) and the move (answered pass) are real; the
  engine's own suspension/resume is covered by
  `rules/changezone_hidden_reveal_test.go`.

## Issues

- **The other `ChooseFromDefined$` value spellings have no consumer.**
  `TriggeredCards` (3), `TriggeredSources` (1), `TopThirdOfLibrary` (1),
  `Targeted.cmcLE4` (1), `ReplacedCards.Land` (1), `ExiledWith.Creature` (1) —
  15 occurrences / 12 files besides Cass's implemented `AttachedTo`. They are
  loud-unimplemented (empty offer + Note) rather than quietly-wrong. Filed as
  `.ds4/new-tickets/choosefromdefined-remaining-values.md`.
- **The `ChooseFromDefined$ Remembered`/`Remembered.<qual>` family now
  resolves** via `knownDefinedTargets` (6 occurrences) because the resolver is
  shared — correct per the fail-closed direction, but unpinned by a real-card
  test. Covered by the same follow-up ticket.
- **`effAttach`'s `Choices$`/`ChoiceZone$` known limitation is unchanged**
  (pre-existing, documented in `effects/attach.go`): a `ChoiceZone$`/`Chooser$`
  attach takes the wrong pool and refuses. Not this ticket.
- No ledger entry id was closed by this ticket (the work was invisible to the
  CR lane — it is a `Defined$` resolution gap, not a CR-rule audit item). A
  forward CR-lane test would cite CR 608.2 (resolution-time referent binding);
  I did not write one, per the brief.

STATUS=DONE
COMMITS=397ba4d6
TESTS=go test -run 'TestAttachedToDefined|TestAttachedToSelector|TestRhuk|TestCassHand|TestFumble|TestMurderousSpoils' ./effects/ ./rules/ → ok (both packages); archtest, botbench pin, gofmt, gentypes -check, TestHeads all pass
