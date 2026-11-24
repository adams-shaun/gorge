# feat(cardc): CC-34 burn-down: conditions, values, trigger events, predicates, references and targets

Kind: feature

Target branch: `cardc`. Complexity: medium. Implementer: Sonnet.
Shared rules, measurements and Done list: `cc-34-39-burn-down.md` beside this file; read it first. Runs first in wave
5, beside CC-36.

## Goal

Serve the 54 cards below (36 backend, 18 front end), and close the static diff's `cond` family (24 rows on 16 cards),
by printing or lowering the constructs they stop on in Forge's shape. This family unblocks the most cards.

## Cards (reason at `a0be533401`)

Backend (`unprintable:`):
- `cond:ref-matches` (8): Courageous Goblin, Dwynen's Elite, Garruk's Uprising, Nessian Hornbeetle, Ruby, Daring
  Tracker, Urza's Mine, Urza's Power Plant, Urza's Tower
- `cond:died-this-turn` (3): Cackling Prowler, Needletooth Pack, Tragic Banshee
- `cond:exists-player` Stromkirk Bloodthief; `cond:left-battlefield-this-turn` Fatal Push; `cond:receipt-non-empty`
  Scavenging Ooze
- `event:deals-combat-damage` (2): Quilled Greatwurm, Umezawa's Jitte; `event:or-mixed` (2): Arahbo, the First Fang,
  Vengeful Bloodwitch; `event:player-attacks` (2): Armasaur Guide, Battlesong Berserker; `event:plays-land` City of
  Traitors
- `value:power` (2): Eldrazi Mimic, Fiendish Panda; `value:distinct-mana-values` Lunar Insight; `value:excess-damage`
  Goblin Negotiation; `value:receipt-amount` Exsanguinate
- `pred:is` (2): Macabre Waltz, Ulamog, the Ceaseless Hunger; `pred:owned-by` Angel of Finality; `pred:counters-not`
  Oblivion Stone
- `ref:all` (3): Felidar Savior, Seismic Rupture, Zimone, Paradox Sculptor
- `target:multi` (2): Bite Down, Felling Blow; `filter:nil` Wildwood Scourge

Front end (`refused:`):
- `Permanents.WasKicked` (4): Gatekeeper of Malakir, Gnarlid Colony, Goblin Bushwhacker, Sun-Blessed Healer
- one each: `Permanents.SurgeCostWasPaid` Reckless Bushwhacker, `Permanents.SharesANameWithPermanent` Bile Blight,
  `Permanents.OnTheBattlefield` Blasphemous Edict, `Permanents.IsCreatureTypeVariable` Banner of Kinship,
  `Permanents.CastByPlayer` Nine-Lives Familiar, `Permanents.AttachedToPermanent` Fiery Annihilation,
  `Permanent.ThisGuest` Fishing Pole, `CardsInGraveyards.WasPutIntoGraveyardFromAnywhereThisTurn` Abyssal Harvester,
  `CardsInLibrary.SharesANameWithTheCardRevealedThisWay` Infernal Tutor, `Comparison.OneOf` Serra Avenger,
  `Target.NumberTargetGroupPermanents` Run Away Together, `Target.UptoNumberAnyTargetsExcept` Drakuseth, Maw of
  Flames, `Target.UptoNumberTargetGroupGraveyardCards` Soul-Shackled Zombie, `Actions.TargetedDistributed` Chandra,
  Flameshaper

Static diff `cond` family (served cards): `cardc diff -slice proof` rows with family `cond` (`SVarCompare`,
`CheckSVar`, `PlayerTurn`, `IsPresent`, `PresentCompare`, `ConditionSVarCompare`). Print the condition where and how
Forge does (on the trigger, the sub-ability or the activation; Forge's comparison form). Respelling non-`cond`
families is CC-39's.

## Workspace

- Backend: `engine/cardc/gorgebe/cond.go`, `value.go`, `filter.go`, `ref.go`, `target.go`, the event table in
  `ability_triggered.go` (new events only; CC-39 owns respelling existing rows), `emit.go`'s `condForm`.
- Readers that decide what gorge accepts: `engine/effects/conditions.go`, `engine/rules/trigger_condition.go`,
  `engine/effects/value_heads.go`, `engine/effects/filter_compiled.go`, `engine/effects/defined_selector_census_test.go`.
- Front end: `engine/cardc/mtgish/fe_filter.go`, `fe_cond.go`, `fe_compare.go`, `fe_target.go`, `fe_value.go`, `fe_ref.go`.

## Done means

The shared list, plus `make test-one PKG=./engine/cardc/gorgebe RUN='^(TestCondForms|TestCondRenderings|TestValueBodies|TestTriggerParams|TestTargetParams|TestProofSliceRefsFilters)$'`
and, when the front end changed, `make test-one PKG=./engine/cardc/mtgish RUN='^(TestFilter.*|TestCond.*|TestTarget.*|TestCompare.*|TestValue.*|TestRef.*)$'`.

## Collisions

Runs beside CC-36 (actions, modes, mana: `spell.go`, `step_flow.go`, `step_token.go`, `step_mana.go`, `fe_action_*.go`).
Ledger files, `ledger-counts.tsv` and `servedFloor` are rewritten by whichever lands second, after a rebase.
