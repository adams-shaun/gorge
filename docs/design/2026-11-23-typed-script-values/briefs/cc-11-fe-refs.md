# feat(cardc): CC-11 front end, references (single objects and players)

Kind: feature

Target branch: `cardc`. Complexity: medium. Seat: Sonnet. Review: Opus (it fixes the reference mapping every later
ticket reuses).
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, section 1 (the reference family).
Runs after CC-02 (`cc-02-mtgish-reader`).

## Goal

Register lowerings from mtgish's single-object and single-player enums to `*ir.Ref`, for every variant the proof slice
(FDN pool plus the 12 Legacy decks) uses, except the deferred ones listed below.

## Behaviour

- `c.Ref(x)` returns the `*ir.Ref` below for each listed variant.
- Nested operands go through the typed helpers, `c.Ref`, `c.Filter` and `c.Value`. A nested variant another ticket
  has not registered yet surfaces as that variant's `*RefusalError`, which is expected until CC-12/13 land.
- `Ref_*` variants resolve through `c.TargetSlot(variant)` (CC-02). An unresolved one returns
  `&RefusalError{Key: "<Enum>.<Variant>:unbound"}`.
- Argument order and arity come from the schema line of each variant in `oracle_cards.d.ts`. Read it; do not guess.

## Design

Decided. "Event object" is `RefEventObject` and "target" is `{RefTarget, Slot: c.TargetSlot(v)}`.

| enum | variant | lowers to |
|---|---|---|
| Permanent | `ThisPermanent` | `{RefSelf}` |
| Permanent | `HostPermanent` | `{RefAttachedTo, Of: {RefSelf}}` (what this is attached to) |
| Permanent | `Ref_TargetPermanent`, `Ref_TargetPermanent1`, `Ref_TargetPermanent2` | target |
| Permanent | `ThatEnteringPermanent`, `Trigger_ThatCreature`, `Trigger_ThatPermanent` | event object |
| Player | `You` | `{RefYou}` |
| Player | `ControllerOfPermanent(p)`, `ControllerOfSpell(s)` | `{RefControllerOf, Of: c.Ref(arg)}` |
| Player | `ControllerOfTargetPermanent` | `{RefControllerOf, Of: target("Ref_TargetPermanent")}` |
| Player | `ControllerOfTargetSpell` | `{RefControllerOf, Of: target("Ref_TargetSpell")}` |
| Player | `OwnerOfPermanent(p)` | `{RefOwnerOf, Of}` |
| Player | `PlayerOrControllerOfPermanent(p)` | `{RefPlayerOrControllerOf, Of}` |
| Player | `Ref_TargetPlayer` | target |
| Player | `TheActivePlayer` | `{RefActivePlayer}` |
| Player | `Trigger_DefendingPlayer` | `{RefDefendingPlayer}` |
| Player | `Trigger_ThatPlayer` | `{RefEventPlayer}` |
| Player | `ActionPlayer` | `{RefIterPlayer}` |
| Spell | `ThisSpell` | `{RefSelf}` |
| Spell | `Ref_TargetSpell` | target |
| Spell | `Trigger_ThatSpell` | event object |
| Card | `ThisCard` | `{RefSelf}` |
| CardInGraveyards | `ThisGraveyardCard` | `{RefSelf}` |
| CardInGraveyards | `Ref_TargetGraveyardCard` | target |
| CardInGraveyards | `Trigger_ThatGraveyardCard` | event object |
| CardInExile | `TheCardExiledThisWay` | `{RefReceipt, Receipt: RcExiled}` |
| CardInExile | `TheChosenExiledCard`, `TheExiledCardChosenThisWay` | `{RefReceipt, Receipt: RcChosen}` |
| DamageRecipient | `EachPermanent(ps)` | `{RefAll, Filter: c.Filter(ps)}` |
| DamageRecipient | `EachPlayer(ps)` | `{RefAll, Filter: c.Filter(ps)}` |
| DamageRecipient | `Permanent(p)`, `Player(p)` | `c.Ref(arg)` |
| DamageRecipient | `Ref_AnyTarget`, `Ref_AnyTargets`, `Ref_TargetPlayerOrPermanent` | target |
| Exilable | `Permanent(p)`, `Spell(s)`, `CardInGraveyards(g)` | `c.Ref(arg)` |
| Exilable | `EachPermanent(ps)`, `EachCardInGraveyards(f)` | `{RefAll, Filter: c.Filter(arg)}` |
| Exilable | `ACardFromHand`, `ACardInGraveyards` | `{RefChoose, Filter, N: {VConst, N: 1}}` (see below) |
| Exilable | `TheTopCardOfLibrary(pl)` | `{RefTopOfLibrary, Of: c.Ref(pl), N: const 1}` |
| Exilable | `TheTopNumberCardsOfLibrary(n, pl)` | `{RefTopOfLibrary, Of, N: c.Value(n)}` |
| Exilable | `TheTopCardOfEachPlayersLibraries(ps)` | `{RefTopOfLibrary, Of: {RefAll, Filter}, N: const 1}` |
| Exilable | `AllCardsFromPlayersLibrary(pl)` | `{RefAll, Filter}` (see below) |

The two Filters the table leaves out:
- `ACardFromHand` and `ACardInGraveyards`: the Filter is the argument's filter conjoined with
  `{SubjCard, Zones: Hand}` or `{Zones: Graveyard}`, as `FAll`. With no argument it is that zone filter with `PAny`.
- `AllCardsFromPlayersLibrary(pl)`: the Filter is
  `{SubjCard, Zones: Library, Op: FPred, Pred: POwnedBy, Ref: c.Ref(pl)}`.

Deferred (not registered; the coverage test lists them):
- `Permanent.ThisGuest`, one card in the slice: its meaning needs the Guest mechanic.

## Implementation steps

1. Add `fe_ref.go` with one `init()` registering every row, plus small constructors (`target(c, v)`, `event()`).
2. Write the tests.

## Tests

In `engine/cardc/mtgish`:
- `TestRefLowering`: a table of synthetic JSON nodes, one per row above, built from variant names only. Each is
  lowered after the needed `DeclareTarget` calls and compared with `ir.Equal`. Nested filters and values are given as
  variants that CC-12/13 own. The test overlays stub lowerings for them with CC-02's `withStubs`, so this ticket needs
  no other family ticket.
- `TestRefUnboundTarget`: `Ref_TargetPermanent` with no declaration returns the `:unbound` refusal.
- `TestRefSliceVariantsRegistered`: `checkFamilyCoverage` over enums `Permanent`, `Player`, `Spell`, `Card`,
  `CardInGraveyards`, `CardInExile`, `DamageRecipient` and `Exilable`, with deferred `{"Permanent.ThisGuest": "Guest
  mechanic"}`.

## Done means

- [ ] `make test-one PKG=./engine/cardc/mtgish RUN='^(TestRefLowering|TestRefUnboundTarget|TestRefSliceVariantsRegistered)$'` passes (not ENV).
- [ ] `make test-targeted BASE=origin/cardc` passes.

## Out of scope

- Target declarations (`Target.*`, CC-12).
- Filters and values themselves (CC-12, CC-13).
- Any edit to `engine/cardc/ir`: every RefKind and ReceiptKind needed is already in CC-01.

## Collisions

Depends on CC-02. Its only files are `fe_ref.go` and `fe_ref_test.go`. It runs in parallel with CC-10, CC-12, CC-13
and CC-14; they register other enums.
