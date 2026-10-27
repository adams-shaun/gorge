# CR 300s: Card types

Coverage of this chapter of the Comprehensive Rules (2026-09-25) by gorge. Seeded from the CR audit of 2026-10-20 at `72dabd96c`; kept current by every change that touches a rule. See [README](README.md) for the status vocabulary and the reference syntax.

| rule | status | code | tests | notes |
|---|---|---|---|---|
| 300.1 | follows | `engine/cards/printed_facts.go#typeNames` | - | - |
| 300.2 | follows | `engine/rules/chars/types.go#IsCardType` | - | multiple card types |
| 300.2a | follows | `engine/rules/legal_walk_hand.go#legalWalk.handWalk` | - | land + other type played only as land |
| 300.2b | follows | `engine/rules/chars/types.go#IsCardType` | - | kindred follows its other type; none in pools |
| 301.1 | follows | `engine/rules/statics_cast.go#Engine.spellTimingOK` | - | sorcery-speed cast |
| 301.2 | follows | `engine/rules/stack.go#Engine.resolveTop` | - | resolves onto the battlefield |
| 301.3 | follows | `engine/rules/chars/types.go#Types` | - | subtype syntax |
| 301.4 | definition | - | - | no artifact-specific characteristic |
| 301.5-301.5a | follows | `engine/rules/attach.go#Engine.attachmentSBAs` `engine/rules/attach.go#Engine.isEquipment` | - | - |
| 301.5b | partial | `engine/effects/attach.go#Attachable` `engine/effects/attach.go#effAttach` | - | illegal attach moves then SBA fixes |
| 301.5c | partial | `engine/rules/attach.go#Engine.attachmentSBAs` | - | Equipment-creature not detached (probe) |
| 301.5d | unknown | - | - | gorge not reached |
| 301.5e | unknown | - | - | - |
| 301.5f | follows | `engine/effects/filter_attachment.go#canEnchantEquippedBy` | - |  |
| 301.6 | follows | `engine/rules/attach.go#Engine.isFortification` `engine/rules/attach.go#Engine.attachmentSBAs` | - | - |
| 301.7-301.7b | follows | `engine/cards/kw_crew.go#kwCrew` | `TestCrewAvengersQuinjetAnimatesUntilEndOfTurn` | see 208.3 for uncrewed filter read |
| 302.1 | follows | `engine/rules/statics_cast.go#Engine.spellTimingOK` | - | sorcery-speed cast |
| 302.2 | follows | `engine/rules/stack.go#Engine.resolveTop` | - | resolves onto the battlefield |
| 302.3 | follows | `engine/rules/chars/types.go#Types` | - | subtype syntax |
| 302.4 | partial | `engine/effects/filter_value.go#lacksPT` | - | same as 208.3 |
| 302.4a-302.4c | follows | `engine/rules/chars/derive.go#PT` | - | - |
| 302.5 | follows | `engine/rules/combat/attack.go#CanAttack` `engine/rules/combat/block.go#CanBlock` | - | - |
| 302.6 | follows | `engine/rules/pay/managates.go#TapFlagsSick` `engine/events/apply_turn.go#foldTurnChange` | - | summoning sickness |
| 302.7 | follows | `engine/rules/sba.go#Engine.destroyLethalDamage` | - | - |
| 303.1 | follows | `engine/rules/statics_cast.go#Engine.spellTimingOK` | - | sorcery-speed cast |
| 303.2 | follows | `engine/rules/stack.go#Engine.resolveTop` | - | resolves onto the battlefield |
| 303.3 | follows | `engine/rules/chars/types.go#Types` | - | subtype syntax |
| 303.4-303.4b | follows | `engine/cards/kw_enchant.go#kwEnchant` `engine/rules/aura_entry.go#auraEnchantSpec` | - | - |
| 303.4c | follows | `engine/rules/attach.go#Engine.attachmentSBAs` `engine/rules/attach.go#Engine.auraStillMatchesEnchant` | `TestCR704AuraBecomesIllegalAfterProtection` | - |
| 303.4d | not-impl | - | - | SILENT; no self-attach check |
| 303.4e | unknown | - | - | - |
| 303.4f-303.4g | follows | `engine/rules/aura_entry.go#auraEntryChoice` `engine/rules/aura_entry.go#auraEntryGate` | `TestCopiedAuraEntryAsksEnchant` `TestAuraTokenWithNothingToEnchantIsNotCreated` | - |
| 303.4h | unknown | - | - | - |
| 303.4i | follows | `engine/rules/aura_entry.go#auraEntryGate` | - | - |
| 303.4j | partial | `engine/effects/attach.go#Attachable` | - | Aura moved then SBA bins it |
| 303.4k | n/a | - | - | no face-down Aura in scope |
| 303.4m | follows | `engine/effects/filter_attachment.go#canEnchantEquippedBy` | - |  |
| 303.5-303.6 | definition | - | - | cross-references to 714 Saga / 716 Class |
| 303.7 | definition | - | - | Role subtype |
| 303.7a | follows | `engine/rules/attach.go#Engine.roleSBA` | `TestRoleExclusivityPerController` `TestRoleExclusivityIsStateBased` `TestRoleExclusivityControlChange` | CR 704.5z state-based action, grouped by the Roles' current controller |
| 304.1 | follows | `engine/rules/statics_cast.go#Engine.spellTimingOK` | - | instant timing |
| 304.2 | follows | `engine/rules/stack.go#Engine.resolveTop` | - | instant resolves to graveyard |
| 304.3 | follows | `engine/rules/chars/types.go#Types` | - | subtype syntax |
| 304.4 | not-impl | - | - | SILENT; gorge relies on script filters |
| 304.5 | follows | `engine/rules/statics_cast.go#Engine.spellTimingOK` | - | - |
| 305.1 | follows | `engine/rules/legal_walk_hand.go#legalWalk.handWalk` | - | - |
| 305.2-305.2a | follows | `engine/rules/cast_begin.go#Engine.effectLandPlayRefusal` `engine/rules/legal_mayplay.go#Engine.adjustLandPlays` | `TestCR305_2aEffectLandPlayUsesExtraDrop` | effect plays use the 1+AdjustLandPlays allowance and count toward it |
| 305.2b | follows | `engine/rules/legal_walk_hand.go#legalWalk.handWalk` | - | - |
| 305.3 | follows | `engine/rules/cast_begin.go#Engine.effectLandPlayRefusal` `engine/rules/host_read.go#Engine.CastProhibited` | `TestCR305_3EffectLandPlayNeedsOwnTurn` | effect plays need the player's own turn; the Play election filters the same gate |
| 305.4 | follows | `engine/events/apply_zone.go#foldLandPlayed` | - | - |
| 305.5 | follows | `engine/rules/chars/types.go#Types` | - | subtype syntax |
| 305.6 | follows | `engine/cards/intrinsic.go#IntrinsicManaAbility` `engine/rules/mana_activation.go#Engine.appendAvailableManaAbilitiesGateCompute` | - |  |
| 305.7 | follows | `engine/rules/chars/types.go#TypesAndLandSubtypeSet` `engine/rules/abilityloss.go#Engine.abilityLoss` `engine/effects/combatfx.go#parseAnimateGrant` | `TestBloodMoonStripsMutavaultRulesTextAbilities` `TestAddedBasicLandTypeKeepsRulesText` `TestCRAnimateRemoveLandTypesSetsBasicType` `TestCRAnimateAllRemoveLandTypesSetsBasicType` `TestCRAnimateChosenTypeResolvesStrips` `TestCRAnimateChosenTypeWithoutChoiceFailsClosed` | REVISED: set land type strips rules-text abilities (cr-g40 merged); Animate/AnimateAll RemoveLandTypes$ strips too, with a ChosenType element resolved against the source's recorded choice and no-choice failing closed (cr-g48); no Blood Moon in pools |
| 305.8-305.9 | follows | `engine/rules/legal_walk_hand.go#legalWalk.handWalk` | - | - |
| 306.1 | follows | `engine/rules/statics_cast.go#Engine.spellTimingOK` | - | sorcery-speed cast |
| 306.2 | follows | `engine/rules/stack.go#Engine.resolveTop` | - | resolves onto the battlefield |
| 306.3 | follows | `engine/rules/chars/types.go#Types` | - | subtype syntax |
| 306.4 | n/a | - | - | historical; legend rule |
| 306.5-306.5d | follows | `engine/rules/entry_counters.go#Engine.entryBodyCandidates` `engine/rules/legal_walk_battlefield.go#legalWalk.battlefieldWalk` | - | - |
| 306.6 | follows | `engine/rules/combat/attack.go#CanAttackPair` | `TestCreatureCanAttackPlaneswalkerAndCombatDamageRemovesLoyalty` | - |
| 306.7 | n/a | - | - | REVISED: historical redirection rule, nothing to implement (was follows) |
| 306.8 | follows | `engine/events/apply_combat.go#foldDamage` `engine/rules/damage_planeswalker.go#Engine.markDamageNotPlaneswalker` | `TestCR306_8LuxiorWalkerBoltIsMarkedDamage` `TestCR306_8LuxiorWalkerUntaggedDamageIsMarked` `TestCR306_8PlaneswalkerGuards` | - |
| 306.9 | follows | `engine/rules/sba.go#Engine.planeswalkerZeroLoyalty` `engine/rules/layers_derived.go#Engine.IsPlaneswalker` | `TestCR306_9MoonedWalkerWithNoLoyaltyStays` `TestCR306_8PlaneswalkerGuards` | - |
| 307.1 | follows | `engine/rules/statics_cast.go#Engine.spellTimingOK` | - | sorcery timing |
| 307.2 | follows | `engine/rules/stack.go#Engine.resolveTop` | - | sorcery resolves to graveyard |
| 307.3 | follows | `engine/rules/chars/types.go#Types` | - | subtype syntax |
| 307.4 | not-impl | - | - | SILENT; gorge relies on script filters |
| 307.5 | follows | `engine/rules/statics_cast.go#Engine.spellTimingOK` | - | - |
| 307.5a | follows | `engine/rules/mayflashsac.go#Engine.offSorceryAtCast` | - | - |
| 308.1 | follows | `engine/rules/chars/types.go#IsCardType` | - | no kindred in pools |
| 308.2 | follows | `engine/rules/chars/types.go#Types` | - | - |
| 308.3 | follows | `engine/rules/chars/types.go#IsCardType` `engine/effects/creature_types.go#cardTypeWords` | `TestKindredIsACardType` | Kindred is the current spelling; retired Tribal retained |
| 309.1-309.2c | follows | `engine/effects/venture.go#effVenture` `engine/effects/venture.go#ventureEnter` | - | no dungeons in pools |
| 309.2d | n/a | - | - | no corpus non-venture dungeon entry |
| 309.3 | follows | `engine/rules/dungeon.go#Engine.PutDungeon` | - | - |
| 309.4-309.5b | follows | `engine/effects/venture.go#ventureAdvance` `engine/rules/dungeon_room.go#Engine.checkDungeonRoomTriggers` | - | - |
| 309.6-309.7 | follows | `engine/rules/dungeon_room.go#Engine.dungeonCompletion` `engine/rules/dungeon.go#Engine.CompleteDungeon` | - | - |
| 310.1 | follows | `engine/rules/statics_cast.go#Engine.spellTimingOK` | - | no battles in pools |
| 310.2 | follows | `engine/rules/stack.go#Engine.resolveTop` | - | - |
| 310.3 | follows | `engine/rules/chars/types.go#Types` | - | subtype syntax |
| 310.4-310.4c | follows | `engine/rules/entry_counters.go#Engine.entryBodyCandidates` | `TestBattleEntersWithPrintedDefense` | - |
| 310.5-310.6 | follows | `engine/rules/attack_cost.go#Engine.canAttackBattle` `engine/events/apply_combat.go#foldDamage` | `TestBattleAttackDealsDefenseCounterDamage` `TestBattleIsOfferedAsAnAttackerOption` | - |
| 310.7 | deviates | `engine/rules/sba.go#Engine.battleZeroDefense` | - | SBA exiles directly instead of trigger |
| 310.8 | deviates | `engine/rules/sba.go#Engine.battleZeroDefense` | - | non-Siege also exiled (no corpus non-Siege) |
| 310.9 | follows | `engine/rules/replacement_etb.go#Engine.applySiegeProtector` | - | protector designation |
| 310.9a | deviates | `engine/rules/replacement_etb.go#Engine.protectorOpponents` | - | non-Siege protector from opponents (no corpus carrier) |
| 310.9b-310.9c | follows | `engine/rules/combat.go#Engine.blockAttackers` `engine/rules/attack_cost.go#Engine.canAttackBattle` | `TestProtectorCannotAttackOwnBattle` | - |
| 310.9d-310.9g | follows | `engine/rules/replacement_etb.go#Engine.rechooseDepartedBattleProtector` | - | - |
| 310.10 | not-impl | - | - | SILENT; unreachable in corpus |
| 310.11 | partial | `engine/rules/replacement_etb.go#Engine.rechooseDepartedBattleProtector` | - | re-choice derived, not asked; no-protector SBA absent |
| 310.12 | definition | - | - | Siege subtype |
| 310.12a | follows | `engine/rules/replacement_etb.go#Engine.applySiegeProtector` | `TestSiegeEntryPosesProtectorChoice` | - |
| 310.12b | deviates | `engine/rules/sba.go#Engine.battleZeroDefense` | - | modelled as SBA exile + owner offer, no trigger |
| 311.1-311.7 | partial | `engine/rules/genesis.go#Engine.genesisDeal` `engine/effects/planechase.go#effPlaneswalk` | - | engine-only Config.PlanarDecks; no planar-controller concept |
| 312.1-312.7 | not-impl | - | - | SILENT if a phenomenon is put in a planar deck |
| 313.1-313.7 | n/a | - | - | no Vanguard |
| 314.1-314.7 | n/a | - | - | no Archenemy |
| 315.1-315.7 | n/a | - | - | no Conspiracy Draft |
