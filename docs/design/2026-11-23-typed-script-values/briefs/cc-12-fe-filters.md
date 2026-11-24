# feat(cardc): CC-12 front end, filters (Permanents, Players, Spells, Cards, CardsIn*) and target declarations

Kind: feature

Target branch: `cardc`. Complexity: medium. Seat: Sonnet. Review: Opus (it fixes the filter mapping every later
ticket reuses).
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, section 1 (the filter family).
Runs after CC-02 (`cc-02-mtgish-reader`).

## Goal

Register lowerings to `*ir.Filter` for mtgish's group enums, and to `*ir.Target` for its `Target` enum, covering
every variant the proof slice uses except the deferred list.

## Behaviour

- `c.Filter(x)` returns a Filter whose `Subject` and `Zones` come from the enum it was read from:

  | enum | Subject | Zones |
  |---|---|---|
  | `Permanents` | `SubjPermanent` | Battlefield |
  | `Spells` | `SubjSpell` | Stack |
  | `Players` | `SubjPlayer` | none |
  | `Cards` | `SubjCard` | none (any zone) |
  | `CardsInGraveyards` | `SubjCard` | Graveyard |
  | `CardsInExile` | `SubjCard` | Exile |
  | `CardsInLibrary` | `SubjCard` | Library |
  | `CardsInHand` | `SubjCard` | Hand |

  Every node of one enum's tree carries the same Subject and Zones.
- `c.Target(x)` declares the target with `c.DeclareTarget(<stem>)` before it returns, with the stem from the target
  table below, so the data's `Ref_<stem>` variants resolve (CC-02). The ability lowering (CC-16) appends targets to
  `Ability.Targets` in lowering order, which makes the slot equal the index.
- `CheckHasable.<V>` lowers to the keyword's rules-name string. Register exactly these 13 no-argument variants, the
  ones that are both a CC-10 keyword and a `CheckHasable` variant at `1553384`: `Flying`, `Deathtouch`, `Defender`,
  `Flash`, `Haste`, `Indestructible`, `Lifelink`, `Menace`, `Reach`, `Trample`, `Vigilance`, and
  `DoubleStrike` → "Double Strike", `FirstStrike` → "First Strike". The proof slice uses only `Flying`. The other
  `CheckHasable` variants (`AnyKicker`, `ThisAbility`, …) stay unregistered.
- Argument order and arity come from each variant's schema line in `oracle_cards.d.ts`.

## Design

Decided. These rows apply to **each** enum above that has the variant:
- `And(list)` → `FAll`; `Or(list)` → `FAny`.
- `IsCardtype(t)` → `FPred PCardType Word=t`; `IsNonCardtype(t)` → `FNot` of it.
- `IsCreatureType`, `IsLandType` and `IsArtifactType(w)` → `PSubtype Word=w`.
- `IsNonCreatureType` and `IsNonEnchantmentType(w)` → `FNot(PSubtype w)`.
- `IsSupertype(s)` → `PSupertype`; `IsNonSupertype` → `FNot`.
- Colours: `IsColor(c)` → `PColor Colors`; `IsNonColor` → `FNot`; `IsColorless` → `PColorless`.
  - The colour lowers through `Color.White`, `Blue`, `Black`, `Red` and `Green`, which CC-12 registers, each
    returning an `ir.ColorSet`. `Color.TheChosenColor` is deferred.
- `IsPermanent` → `PPermanentCard`.
- `ManaValueIs(cmp)`, `PowerIs(cmp)` and `ToughnessIs(cmp)` → `PManaValue`, `PPower` and `PToughness`, with Cmp and
  Value from `c.Compare(cmp)` (CC-13).
- `ControlledByAPlayer(ps)` → `PControlledBy, Ref: {RefAll, Filter: c.Filter(ps)}`. `OwnedByAPlayer(ps)` and
  `InAPlayersGraveyard(ps)` → `POwnedBy` with the same Ref.
- `HasAbility(k)` → `PHasKeyword Word=k`; `DoesntHaveAbility` → `FNot`.
- `IsToken` → `PToken`; `IsNonToken` → `FNot(PToken)`.
- `IsTapped` → `PTapped`; `IsUntapped` → `FNot(PTapped)`.
- `IsAttacking` → `PAttacking`; `IsBlocking` → `PBlocking`.
- `Other(ref)` → `FNot(PIs Ref=c.Ref(ref))` (for Players, `FNot(PPlayerIs)`).
- `SinglePermanent`, `SingleSpell` and `SinglePlayer(ref)` → `PIs` (for players, `PPlayerIs`).
- `Ref_TargetPermanents` and `Ref_TargetGraveyardCards` → `PIs Ref={RefTarget, Slot: c.TargetSlot(v)}`.
- `AnyPermanent`, `AnySpell`, `AnyCard`, `AnyCardInAnyGraveyard` and `AnyPlayer` → `PAny`.
- `IsNamed(nf)` → `PName Word=c.lower(nf)`. Its argument is a `NameFilter`, not a string. CC-12 registers
  `NameFilter.NamedCard(s)`, which returns the string `s`; `NameFilter.TheChosenName` is deferred.
- Counters (the counter word from `c.lower(ct)`, CC-13's `CounterType` string):
  - `HasACounterOfType(ct)` → `PCounters Word, Cmp: CmpGE, Value: const 1`;
  - `HasNoCountersOfType(ct)` → `FNot` of that;
  - `HasNumberCountersOfType(cmp, ct)` → `PCounters` with the compare.
- Receipts: `TheCardsExiledThisWay` → `PIs Ref={RefReceipt, RcExiled}`; `TheTokensCreatedThisWay` →
  `PIs Ref={RefReceipt, RcCreated}`.

Players only:
- `Opponent` → `POpponentOf Ref={RefYou}`.
- `AttackedThisTurn` → `PAttackedThisTurn`.
- `ControlsA(ps)` → `PControls, Args: [c.Filter(ps)]`.
- `ControlsMorePermanentThanPlayer(pl, ps)` → `PControlsMoreThan, Ref, Args`.
- `LifeTotalIs(cmp)` → `PLifeTotal`.
- `LostLifeThisTurn` → `PLostLifeThisTurn`.
- `NumCardsInGraveyardIs(cmp)` → `PCardsInGraveyard`.
- `NumCardsInHandIs(cmp)` → `PCardsInHand`.

`Target.<V>` → `*ir.Target`, where `filter(...)` is the argument's `c.Filter`:

| variant | stem | Min | Max | Filter |
|---|---|---:|---:|---|
| `TargetPermanent(ps)` | `TargetPermanent` | 1 | 1 | filter(ps) |
| `UptoOneTargetPermanent(ps)` | `TargetPermanent` | 0 | 1 | filter(ps) |
| `TargetPlayer(ps)` | `TargetPlayer` | 1 | 1 | filter(ps) |
| `TargetSpell(ss)` | `TargetSpell` | 1 | 1 | filter(ss) |
| `TargetGraveyardCard(gs)` | `TargetGraveyardCard` | 1 | 1 | filter(gs) |
| `UptoOneTargetGraveyardCard(gs)` | `TargetGraveyardCard` | 0 | 1 | filter(gs) |
| `NumberTargetPermanents(n, ps)` | `TargetPermanents` | n | n | filter(ps) |
| `UptoNumberTargetPermanents(n, ps)` | `TargetPermanents` | 0 | n | filter(ps) |
| `AnyNumberOfTargetPermanents(ps)` | `TargetPermanents` | 0 | -1 (no limit) | filter(ps) |
| `UptoNumberTargetGraveyardCards(n, gs)` | `TargetGraveyardCards` | 0 | n | filter(gs) |
| `AnyTarget` | `AnyTarget` | 1 | 1 | any-target |
| `TargetPlayerOrPermanent(pls, ps)` | `TargetPlayerOrPermanent` | 1 | 1 | `FAny(filter(pls), filter(ps))` |

The stems come from the data at `1553384`: Fiery Annihilation declares `TargetPermanent` then
`UptoOneTargetPermanent` and refers to them as `Ref_TargetPermanent1` and `Ref_TargetPermanent2`; Felidar Savior
declares `UptoNumberTargetPermanents` and refers to `Ref_TargetPermanents`. Check a stem against the data before
changing it.

- When `n` lowers to a `VConst`, Min and Max take its N. Otherwise `MaxV` is set and Min/Max are 0/0.
- Any-target is `FAny` of four filters: `{SubjPermanent, Battlefield, PCardType "Creature"}`, the same for
  `"Planeswalker"` and `"Battle"`, and `{SubjPlayer, PAny}` (CR 115.4).

Deferred, with their reasons in the coverage test:
- `Permanents.AttachedToPermanent`, `CastByPlayer`, `ExceptFor`, `HasTheMostVotesOrTiedForTheMostVotes`,
  `IsCreatureTypeVariable`, `OnTheBattlefield`, `SharesANameWithPermanent`, `SurgeCostWasPaid`, `WasKicked`,
  `ThePermanentsNotChosenThisWay`;
- `Spells.IsCreatureTypeVariable`;
- `CardsInGraveyards.WasPutIntoGraveyardFromAnywhereThisTurn`;
- `CardsInExile.IsFaceUp`, `WasExiledWithPermanent`;
- `CardsInLibrary.SharesANameWithTheCardRevealedThisWay`;
- `Color.TheChosenColor`;
- `NameFilter.TheChosenName` (Cabal Therapy, Pithing Needle: a name chosen at resolution);
- `Target.NumberTargetGroupPermanents`, `UptoNumberTargetGroupGraveyardCards`, `UptoNumberAnyTargetsExcept`.

## Implementation steps

1. Add `fe_filter.go`: a helper that registers the shared rows for one enum with its Subject and Zones, called once
   per enum in `init()`. Add the Players-only rows, and `CheckHasable`/`Color` lowerings.
2. Add `fe_target.go` with the Target table.
3. Write the tests.

## Tests

In `engine/cardc/mtgish`:
- `TestFilterLowering`: synthetic JSON per shared row, on at least `Permanents` and `CardsInGraveyards`. It checks
  Subject and Zones too, and uses `withStubs` for `Comparison`, `CounterType` and the reference variants.
- `TestPlayerFilterLowering`: every Players row.
- `TestTargetLowering`: every Target row, including the declaration side effect (`c.TargetSlot("Ref_<stem>")` finds
  it) and the any-target filter.
- `TestFilterSliceVariantsRegistered`: `checkFamilyCoverage` over `Permanents`, `Players`, `Spells`, `Cards`,
  `CardsInGraveyards`, `CardsInExile`, `CardsInLibrary`, `CardsInHand`, `Target`, `CheckHasable`, `Color` and
  `NameFilter`, with the deferred list above.

## Done means

- [ ] `make test-one PKG=./engine/cardc/mtgish RUN='^(TestFilterLowering|TestPlayerFilterLowering|TestTargetLowering|TestFilterSliceVariantsRegistered)$'` passes (not ENV).
- [ ] `make test-targeted BASE=origin/cardc` passes.
- [ ] `make test-smart BASE=origin/cardc` passes.

## Out of scope

- `Comparison` and `CounterType` (CC-13).
- References (CC-11).
- Adding Pred values to `engine/cardc/ir`: every Pred used above is already in CC-01. A deferred variant needing a
  new Pred stays deferred.
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

Depends on CC-02. Its only files are `fe_filter.go`, `fe_target.go` and their tests. It runs in parallel with CC-10,
CC-11, CC-13 and CC-14.
