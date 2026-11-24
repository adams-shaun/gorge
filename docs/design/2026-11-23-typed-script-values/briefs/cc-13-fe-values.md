# feat(cardc): CC-13 front end, values (GameNumber), comparisons and counter kinds

Kind: feature

Target branch: `cardc`. Complexity: medium. Seat: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, section 1 (the value family).
Runs after CC-02 (`cc-02-mtgish-reader`).

## Goal

Register lowerings for:
- `GameNumber` to `*ir.Value`;
- `Comparison` to `*ir.Compare`;
- `CounterType` to gorge's counter-kind word.

Cover every variant the proof slice uses except the deferred list.

## Behaviour

- `c.Value(x)` and `c.Compare(x)` return the nodes below.
- `c.lower(ct)` on a `CounterType` returns a `string`.
- Nested references and filters go through `c.Ref` and `c.Filter`.
- Argument order comes from each variant's schema line in `oracle_cards.d.ts`.

## Design

Decided.

**GameNumber**

| variant | `ir.Value` |
|---|---|
| `Integer(n)` | `{VConst, N: n}` |
| `ValueX` | `{VX}` |
| `Plus(a, b)` | `{VAdd, Args: [a, b]}` |
| `Minus(a, b)` | `{VSub, Args: [a, b]}` |
| `Twice(a)` | `{VMul, Args: [{VConst, N: 2}, a]}` |
| `TheNumberOfPermanentsOnTheBattlefield(ps)`, `TheNumberOfGraveyardCards(gs)` | `{VCount, Filter: c.Filter(arg)}` |
| `PowerOfPermanent(p)` | `{VPower, Ref: c.Ref(p)}` |
| `PowerOfDeadPermanent` (no args) | `{VPower, Ref: {RefEventObject}}` (the dying permanent, last known) |
| `ToughnessOfPermanent(p)` | `{VToughness, Ref}` |
| `ManaValueOfPermanent(p)` | `{VManaValue, Ref}` |
| `ManaValueOfTheCardRevealedThisWay` | `{VManaValue, Ref: {RefReceipt, RcRevealed}}` |
| `LifeTotalOfPlayer(pl)` | `{VLife, Ref}` |
| `StartingLifeTotalOfPlayer(pl)` | `{VStartingLife, Ref}` |
| `LifeGainedByPlayerThisTurn(pl)` | `{VLifeGainedThisTurn, Ref}` |
| `PlayerDevotionTo(pl, colour)` | `{VDevotion, Ref, Colors}` |
| `TheTotalNumberOfCountersOfTypeAmongPermanents(ct, ps)` | `{VCountersAmong, Word: counter word, Filter}` |
| `TotalPowerOfPermanents(ps)` | `{VTotalPower, Filter}` |
| `NumDifferentManaValuesAmongPermanents(ps)` | `{VDistinctManaValues, Filter}` |
| `NumSpellsCastByPlayerBeforeSpellThisTurn(ss, pl, sp)` | `{VSpellsCastBeforeThisTurn, Filter: c.Filter(ss), Ref: c.Ref(pl)}` |
| `TheChosenNumber` | `{VChosenNumber}` |
| `Trigger_AmountOfDamageDealt`, `Trigger_AmountOfLifeLost` | `{VEventAmount}` |
| `Trigger_ThatMuch`, `WouldDealDamage_ThatMuchDamage` | `{VEventAmount}` |
| `LifeLostThisWay` | `{VReceiptAmount, Word: "life-lost"}` |
| `ThatManyCounters` | `{VReceiptAmount, Word: "counters"}` |
| `AmountOfExcessDamageDealtThisWay` | `{VExcessDamage}` |

- `PlayerDevotionTo`'s second argument is one `Color` (schema `[Player, Color]`), lowered through CC-12's `Color.*`
  to a ColorSet. Before CC-12 lands, test it with `withStubs`.
- `NumSpellsCastByPlayerBeforeSpellThisTurn` means "spells matching `Filter` that `Ref` cast this turn before the
  event spell". Its third argument must be `Spell.Trigger_ThatSpell`, the only one in the whole corpus at
  `1553384` (Thousand-Year Storm). Any other third argument is refused as
  `GameNumber.NumSpellsCastByPlayerBeforeSpellThisTurn:spell`.

**Comparison** → `*ir.Compare{Op, V: c.Value(arg)}`:

| variant | Op |
|---|---|
| `EqualTo` | `CmpEQ` |
| `NotEqualTo` | `CmpNE` |
| `LessThan` | `CmpLT` |
| `LessThanOrEqualTo` | `CmpLE` |
| `GreaterThan` | `CmpGT` |
| `GreaterThanOrEqualTo` | `CmpGE` |

**CounterType** → the counter word gorge's state uses (`"P1P1"`, `"M1M1"`, `"CHARGE"`, `"STUN"`, `"LOYALTY"` appear as
literals in `engine/`):
- `PTCounter(p, t)` → `P` or `M` (for p ≥ 0 or p < 0) followed by |p|, then the same for t. So (1,1) gives `P1P1`,
  (-1,-1) gives `M1M1`, and (0,1) gives `P0P1`.
- Every `<Name>Counter` variant → `Name` upper-cased (`ChargeCounter` → `CHARGE`). Register every no-arg
  `CounterType` variant in the schema whose name ends in `Counter`.
- The gorge backend (CC-20) checks each word against gorge's counter vocabulary.

Deferred:
- `Comparison.OneOf` (a set, not one comparison).
- Every other `Comparison` variant (`Even`, `Odd`, `TheChosenQuality`, and so on): none is in the proof slice.

## Implementation steps

1. Add `fe_value.go` (GameNumber), `fe_compare.go` and `fe_counter.go`, each registering in `init()`.
2. Write the tests.

## Tests

In `engine/cardc/mtgish`:
- `TestValueLowering`: one synthetic JSON row per GameNumber row above. It uses `withStubs` for nested filter,
  reference and colour variants.
- `TestCompareLowering`: the six operators.
- `TestCounterWords`: `PTCounter` for (1,1), (-1,-1), (0,1) and (2,0), plus every registered `<Name>Counter`.
- `TestValueSliceVariantsRegistered`: `checkFamilyCoverage` over `GameNumber`, `Comparison` and `CounterType`, with
  deferred `{"Comparison.OneOf": "a set of values"}`.

## Done means

- [ ] `make test-one PKG=./engine/cardc/mtgish RUN='^(TestValueLowering|TestCompareLowering|TestCounterWords|TestValueSliceVariantsRegistered)$'` passes (not ENV).
- [ ] `make test-targeted BASE=origin/cardc` passes.
- [ ] `make test-smart BASE=origin/cardc` passes.

## Out of scope

- Any ValueKind beyond CC-01's list. A GameNumber variant outside the slice stays unregistered.
- Evaluating values (the gorge backend).
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

Depends on CC-02. Its only files are `fe_value.go`, `fe_compare.go`, `fe_counter.go` and their tests. It runs in
parallel with CC-10, CC-11, CC-12 and CC-14.
