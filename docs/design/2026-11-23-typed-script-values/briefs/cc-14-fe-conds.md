# feat(cardc): CC-14 front end, conditions (Condition)

Kind: feature

Target branch: `cardc`. Complexity: medium. Seat: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, section 1 (the condition family).
Runs after CC-02 (`cc-02-mtgish-reader`).

## Goal

Register lowerings from mtgish's `Condition` enum to `*ir.Cond`, for every variant the proof slice uses except the
deferred list.

## Behaviour

- `c.Cond(x)` returns the node below.
- Nested players, permanents, filters, comparisons and values go through `c.Ref`, `c.Filter`, `c.Compare` and
  `c.Value`.
- Argument order comes from each variant's schema line in `oracle_cards.d.ts`. At `1553384`, for example:
  `PlayerPassesFilter` is `[Player, Players]`, `NumPermanentsPassFilter` is `[Comparison, Permanents, Permanents]`,
  and `IsPlayersNthTurn` is `[Player, Comparison]`.

## Design

Decided.

| variant | `ir.Cond` |
|---|---|
| `PlayerPassesFilter(pl, ps)`, `PermanentPassesFilter(p, ps)` | `{CRefMatches, Ref: arg 1, Filter: arg 2}` |
| `EnteringPermanentPassesFilter(ps)` | `{CRefMatches, Ref: {RefEventObject}, Filter}` |
| `DeadPermanentPassesFilter(ps)` | `{CRefMatches, Ref: {RefEventObject}, Filter}` |
| `APlayerPassesFilter(ps1, ps2)` | `{CExistsPlayer, Filter: {FAll, Args: [ps1, ps2]}}` (Stromkirk Bloodthief: an opponent that lost life) |
| `NumPermanentsPassFilter(cmp, ps1, ps2)` | `{CCountCompare, Filter: {FAll, Args: [ps1, ps2]}, Cmp, R}` (Blasphemous Edict) |
| `CostWasPaid` | `{CCostPaid}` (see below) |
| `IsPlayersTurn(pl)` | `{CIsTurnOf, Ref}` |
| `IsAPlayersTurn(ps)` | `{CIsTurnOf, Filter}` |
| `IsPlayersNthTurn(pl, cmp)` | `{CNthTurn, Ref, Cmp, R}` |
| `ACardWasExiledThisWay(cs)` | `{CReceiptNonEmpty, Receipt: RcExiled, Filter}` |
| `ACardWasRevealedThisWay(cs)` | `{CReceiptNonEmpty, Receipt: RcRevealed, Filter}` |
| `ACreatureOrPlaneswalkerDiedThisTurn(ps)` | `{CDiedThisTurn, Filter}` |
| `APermanentLeftTheBattlefieldThisTurn(ps)` | `{CLeftBattlefieldThisTurn, Filter}` |

- `Cmp` and `R` come from `c.Compare(cmp)`.
- `And(list)` → `{CAll, Args}` and `Or(list)` → `{CAny, Args}`. The schema has no `Condition.Not`; `CNot` is for
  CC-15's `Unless` action.
- `CostWasPaid` asks whether the nearest earlier cost step in the same action list (`MayCost`, `PlayerMayCost` or
  `MustCost`) was paid. The payer need not be you: Daze asks the target spell's controller, Chain Lightning the
  target's controller. Frenzied Goblin and Flamewake Phoenix are the plain "you may pay" case. CC-15 binds the step;
  CC-14 emits the bare node.
- A condition that splits a filter in two arguments (the `FAll` rows) keeps both, in order.

Deferred:
- `Condition.NumGroupCardsWereMilledThisWay`: it needs `GroupFilter`, which no wave-1 ticket lowers.

## Implementation steps

1. Add `fe_cond.go`, registering every row in `init()`.
2. Write the tests.

## Tests

In `engine/cardc/mtgish`:
- `TestCondLowering`: one synthetic JSON row per table row and for `And` and `Or`. It uses `withStubs` for nested
  reference, filter and comparison variants.
- `TestCondSliceVariantsRegistered`: `checkFamilyCoverage` over `Condition`, with deferred
  `{"Condition.NumGroupCardsWereMilledThisWay": "needs GroupFilter"}`.

## Done means

- [ ] `make test-one PKG=./engine/cardc/mtgish RUN='^(TestCondLowering|TestCondSliceVariantsRegistered)$'` passes (not ENV).
- [ ] `make test-targeted BASE=origin/cardc` passes.
- [ ] `make test-smart BASE=origin/cardc` passes.

## Out of scope

- Intervening-if placement and the `Rule.If` wrapper (CC-16).
- Any CondKind beyond CC-01's list.
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

Depends on CC-02. Its only files are `fe_cond.go` and its test. It runs in parallel with CC-10..13.
