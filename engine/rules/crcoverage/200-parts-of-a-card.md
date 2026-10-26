# CR 200s: Parts of a card

Coverage of this chapter of the Comprehensive Rules (2026-09-25) by gorge. Seeded from the CR audit of 2026-10-20 at `72dabd96c`; kept current by every change that touches a rule. See [README](README.md) for the status vocabulary and the reference syntax.

| rule | status | code | tests | notes |
|---|---|---|---|---|
| 200.1 | n/a | - | - | descriptive list of card parts |
| 200.2 | follows | `engine/rules/chars/derive.go#PT` | - | characteristics derived per CR 613 / fixed order |
| 200.3 | follows | `engine/state/object.go#Object.CopyFace` `engine/state/object.go#Object.faceSlow` | - | copies/tokens carry characteristics only |
| 201.1 | n/a | - | - | print layout |
| 201.2 | follows | `engine/cards/ir.go#Face.Name` | - | English Oracle names from the corpus |
| 201.2a | follows | `engine/effects/filter.go#nameCharacteristics` `engine/effects/filter.go#sharesName` `engine/state/mana_value.go#Object.FaceDownHidden` | `TestFaceDownPermanentHasNoName` |  |
| 201.2b | partial | `engine/effects/count_zone_fold.go#zoneCountFold.visit` `engine/effects/count_ref_property.go#evalRefProperty` | - | gorge: face-down counted by printed name |
| 201.2c | partial | `engine/effects/filter.go#sharesNameWithObject` | - | same root as 201.2a |
| 201.3 | not-impl | - | - | SILENT: interchangeable names not modelled in play |
| 201.3a | not-impl | - | - | SILENT; same as 201.3 |
| 201.3b | follows | `engine/cards/registry.go#Registry.Add` `engine/cards/registry.go#nameIndexOf` | - | decklist alias lookup |
| 201.3c | n/a | - | - | print indicator |
| 201.4 | follows | `engine/effects/namecard.go#NameChoices` `engine/cards/universe.go#Universe.Names` | - | universe = registry cards, no tokens |
| 201.4a | follows | `engine/effects/namecard.go#nameChoicesFiltered` | - | ValidCards$ over printed face |
| 201.4b | follows | `engine/cards/universe.go#NameableFace` `engine/effects/namecard.go#nameChoicesFiltered` | `TestNameChoicesFilterEachFaceByItsOwnCharacteristics` | either half nameable; matched by that half alone |
| 201.4c-201.4f | follows | `engine/cards/universe.go#NameableFace` `engine/cards/universe.go#Universe.Names` `engine/effects/namecard.go#nameChoicesFiltered` | `TestNameChoicesOfferEveryNameableFace` `TestCorpusNameUniverseHasEveryNameableFace` | flip, back, meld and adventure names offered and matched by that face |
| 201.4g | not-impl | - | - | SILENT; no interchangeable-name model |
| 201.5 | follows | `engine/effects/filter.go#predicates` | - | self-reference by name = this object (Self / RefSelf) |
| 201.5a-201.5c | unknown | - | - | settle: Gutter Grime token + second Gutter Grime scenario |
| 201.6 | follows | `engine/cards/ir.go#Face.Aliases` | - | flavour name has no game effect |
| 202.1 | n/a | - | - | layout |
| 202.1a | follows | `engine/rules/cast_pricing.go#Engine.manaToPay` | - | - |
| 202.1b | follows | `engine/cards/face.go#cmcFromManaCost` | - | no mana cost |
| 202.2 | follows | `engine/effects/colors.go#ColorMaskOf` | - | - |
| 202.2a-202.2c | follows | `engine/effects/colors.go#colorBit` | - | - |
| 202.2d | follows | `engine/effects/colors.go#ColorMaskOf` | - | hybrid/Phyrexian colours |
| 202.2e | follows | `engine/effects/colors.go#ColorMaskOf` | - | indicator read only when cost colourless |
| 202.2f | follows | `engine/rules/layer5colors.go#Engine.derivedColorTable` | - | - |
| 202.3 | partial | `engine/state/mana_value.go#ManaValue` `engine/cards/face.go#cmcFromManaCost` | - | subrule gaps below |
| 202.3a | follows | `engine/cards/face.go#cmcFromManaCost` | - | - |
| 202.3b | follows | `engine/state/mana_value.go#manaValueAs` | `TestManaValueTransformedBackFaceIsFrontFace` `TestSephirothTransformsOnlyOnFourthResolution` | copy of a back face reads its printed no cost |
| 202.3c | follows | `engine/state/mana_value.go#manaValueAs` | `TestManaValueMeldedPermanentSumsFrontFaces` |  |
| 202.3d | follows | `engine/state/mana_value.go#manaValueAs` `engine/state/mana_value.go#Object.SplitHalves` `engine/state/mana_value.go#SpellManaValue` | `TestManaValueSplitCardCombinesHalvesOffTheStack` `TestCascadeSkipsSplitCardByCombinedManaValue` `TestSpellCostModifierReadsTheHalfBeingCast` |  |
| 202.3e | follows | `engine/state/mana_value.go#stackX` `engine/effects/filter_value.go#cmcValue` | `TestManaValueXOnTheStackIsTheChosenValue` `TestVoidWinnowerRechecksChosenXOnTheStack` |  |
| 202.3f | follows | `engine/cards/face.go#cmcFromManaCost` `engine/state/mana_value.go#faceManaValue` | `TestManaValueTwobridCountsTheGenericComponent` |  |
| 202.3g | follows | `engine/cards/face.go#cmcFromManaCost` | - | - |
| 202.4 | follows | `engine/cards/face.go#cmcFromManaCost` | - | additional costs not in mana value |
| 203.1 | n/a | - | - | no game effect |
| 204.1 | n/a | - | - | layout |
| 204.2 | follows | `engine/effects/colors.go#ColorMaskOf` | - | - |
| 205.1 | n/a | - | - | layout |
| 205.1a | partial | `engine/rules/chars/types.go#bestowedTypeSwitch` `engine/rules/chars/types.go#applyTypeEffect` | - | gorge: RemoveTypes keeps orphan subtypes; no instant/sorcery retention |
| 205.1b | follows | `engine/rules/chars/types.go#appendTypeOnce` | - | - |
| 205.2 | definition | - | - | heading |
| 205.2a | follows | `engine/effects/creature_types.go#cardTypeWords` `engine/rules/chars/types.go#cardTypeWords` | `TestEvalCountCardTypesCountsKindred` `TestKindredIsACardType` | Kindred and the retired Tribal both count as card types |
| 205.2b | follows | `engine/effects/filter_type.go#hasType` | - | - |
| 205.2c | follows | `engine/state/object.go#Object.faceSlow` | - | tokens/copies carry card types |
| 205.3 | definition | - | - | heading |
| 205.3a-205.3c | follows | `engine/rules/chars/types.go#Types` | - | - |
| 205.3d | not-impl | `engine/rules/chars/types.go#appendLandTypes` | - | SILENT; scripts usually filter by card type |
| 205.3e | partial | `engine/effects/creature_types.go#creatureSubtypeWords` | - | "Time Lord" split into Time + Lord |
| 205.3f | follows | `engine/cards/printed_facts.go#typeNames` | - | Oracle subtypes from corpus |
| 205.3g-205.3k | follows | `engine/cards/printed_facts.go#typeNames` | - | vocabulary from corpus |
| 205.3m | partial | `engine/effects/creature_types.go#creatureSubtypeWords` | - | extra/split tokens vs CR list |
| 205.3n-205.3r | n/a | - | - | planes/dungeon/battle subtype lists |
| 205.4 | definition | - | - | heading |
| 205.4a | follows | `engine/rules/chars/types.go#SupertypeWords` | - |  |
| 205.4b | follows | `engine/rules/chars/types.go#applyTypeEffect` | - | - |
| 205.4c | follows | `engine/cards/face.go#Face.IsBasic` | - | basic land |
| 205.4d | follows | `engine/rules/sba.go#Engine.legendGroups` | - | legend rule |
| 205.4e | follows | `engine/rules/statics_cast.go#Engine.legendarySpellLocked` `engine/rules/statics_cast.go#Engine.castRestrictedCost` | `TestCR205_4eLegendarySorceryNeedsALegend` `TestCR205_4eNonlegendarySorceryUnaffected` | every cast route via castRestrictedCost; no pool card |
| 205.4f | follows | `engine/rules/sba.go#Engine.worldRule` | `TestWorldRuleNewestSurvives` `TestWorldRuleTieSendsAll` | - |
| 205.4g | follows | `engine/effects/filter_type.go#hasType` | - | no snow in pools |
| 205.4h | n/a | - | - | no Archenemy |
| 206.1-206.5 | n/a | - | - | no game effect / errata note |
| 207.1-207.4 | n/a | - | - | text-box presentation |
| 207.5 | unknown | - | - | Cryptic Spires circle |
| 208.1 | follows | `engine/rules/chars/derive.go#PT` | - | - |
| 208.2 | follows | `engine/rules/chars/derive.go#PT` | - | *-P/T on the battlefield via CDA |
| 208.2a | follows | `engine/effects/filter_value.go#numericPred` `engine/effects/filter.go#LayerTables` `engine/rules/host_read.go#Engine.cdaPTTable` `engine/rules/chars/cda.go#CDASetPT` | `TestCR208_2aDigReadsLibraryCDAPower` `TestCR208_2aDigCDAGuard` | - |
| 208.2b | follows | `engine/cards/face.go#Face.Power` | - | not line-checked |
| 208.3 | partial | `engine/effects/filter_value.go#lacksPT` | - | printed-PT noncreature (Vehicle) treated as having P/T |
| 208.3a | unknown | - | - | - |
| 208.4-208.4b | follows | `engine/rules/chars/derive.go#PT` | `TestBasePowerPredicateUsesLayerSevenBSet` | - |
| 208.5 | follows | `engine/cards/face.go#parsePT` | - | - |
| 209.1 | follows | `engine/events/entry_counters.go#EntryCounterGrants` | - | - |
| 209.2 | follows | `engine/rules/legal_walk_battlefield.go#legalWalk.battlefieldWalk` | `TestJaceLoyaltyAbilityOncePerTurnAndOwnTurnOnly` | - |
| 210.1 | follows | `engine/events/entry_counters.go#EntryCounterGrants` | `TestBattleEntersWithPrintedDefense` | - |
| 211.1 | n/a | - | - | no Vanguard |
| 212.1 | n/a | - | - | no Vanguard |
| 213.1-213.1g | n/a | - | - | no game effect |
