# CR 702s: Keyword abilities

Coverage of this chapter of the Comprehensive Rules (2026-09-25) by gorge. Seeded from the CR audit of 2026-10-20 at `72dabd96c`; kept current by every change that touches a rule. See [README](README.md) for the status vocabulary and the reference syntax.

| rule | status | code | tests | notes |
|---|---|---|---|---|
| 702.1 | follows | `engine/cards/keywords.go#Face.expandKeywords` | - | keyword names expand to their abilities; row added in review |
| 702.1a,d | follows | `engine/effects/filter.go#init` | - | "with [kw]" reads derived keywords |
| 702.1b,c | unknown | - | - | variable-in-granted-keyword reevaluation unchecked |
| 702.2a,c,d,f | follows | `engine/rules/combat.go#Engine.lethalDamageNeed` `engine/rules/combat.go#Engine.runCombatAssignments` `engine/rules/engine_damage.go#Engine.damageKeywordsOf` | - | 1 is lethal for assignment; noncombat too |
| 702.2b | deviates | `engine/rules/sba.go#Engine.destroyLethalDamage` `engine/rules/combat.go#Engine.cleanupBody` | - | mark outlives the SBA check (other engines same) |
| 702.2e | follows | `engine/rules/engine_damage.go#damageKeywordLKI` `engine/rules/engine_damage.go#Engine.damageKeywordsOf` | - |  |
| 702.3a-c | follows | `engine/rules/combat/attack.go#CanAttack` `engine/rules/combat/attack.go#CanAttackPair` | - |  |
| 702.4a-e | follows | `engine/rules/combat.go#combatRound.firstStrikers` `engine/rules/combat.go#Engine.anyFirstStrike` `engine/rules/combat.go#Engine.actsThisDamageStep` | `TestCR510PriorityBetweenDoubleStrikeDamageSteps` | 702.4c/d follow from step-begin record |
| 702.5a,b | follows | `engine/rules/attach.go#Engine.auraStillMatchesEnchant` | - |  |
| 702.5c | n/a | `engine/rules/attach.go#Engine.auraStillMatchesEnchant` | - | no corpus face prints two enchant instances |
| 702.5d | follows | `engine/rules/emit.go#Engine.emit` | - |  |
| 702.6a-d | follows | `engine/cards/kw_equip.go#kwEquip` `engine/cards/kw_equip.go#kwAttachCost` | - | restriction field passed as ValidTgts |
| 702.6e | follows | `engine/cards/kw_equip.go#kwAttachCost` | - | comment-level evidence only |
| 702.7a-d | follows | `engine/rules/combat.go#Engine.actsThisDamageStep` `engine/rules/combat.go#combatRound.firstStrikers` | - |  |
| 702.8a,b | follows | `engine/rules/statics_cast.go#Engine.flashGrantCoversTargets` `engine/rules/statics_cast.go#Engine.spellTimingPre` `engine/rules/legal_walk_hand.go#legalWalk.handWalk` | - |  |
| 702.9a-c | follows | `engine/rules/combat/block.go#CanBlock` | - |  |
| 702.10a-d | follows | `engine/rules/combat/attack.go#AttackableCreature` `engine/rules/pay/managates.go#TapFlagsSick` `engine/events/apply_helpers.go#changeControl` | - | tap/untap abilities included |
| 702.11a-d,f-h | follows | `engine/rules/protection.go#Engine.hexproofBlocksTarget` `engine/rules/playerkeywords.go#Engine.playerHexproofBlocksTarget` | - |  |
| 702.11e | unknown | - | - | losing hexproof removes hexproof-from: removal path unchecked |
| 702.12a-c | follows | `engine/rules/sba.go#Engine.destroyLethalDamage` `engine/effects/zone_bulk.go#effDestroy` `engine/effects/zone_bulk.go#effDestroyAll` | - |  |
| 702.13a-c | partial | `engine/rules/combat/block.go#CanBlock` | - | artifact test reads printed face (bf.IsArtifact), colour derived |
| 702.14a-e | follows | `engine/rules/combat/block.go#CanBlock` `engine/rules/combat/block.go#LandwalkSpec` | - | layer-4 land types; nonbasic/snow forms |
| 702.15a-d,f | follows | `engine/rules/combat.go#Engine.damageStep` `engine/rules/combat.go#Engine.runCombatAssignments` `engine/rules/engine_damage.go#Engine.damageKeywordsOf` | - |  |
| 702.15e | deviates | `engine/rules/combat.go#Engine.runCombatAssignments` `engine/events/event.go#LifeChange` | - | one source to two recipients = two gain events |
| 702.16a-j,m | follows | `engine/rules/protection.go#Engine.protectedFrom` `engine/rules/protection.go#Engine.sourceHasQuality` `engine/rules/combat/block.go#CanBlock` | `TestProtectionFromATypeReadsTheSourceNow` |  |
| 702.16k | unknown | - | - | protection from a player not located |
| 702.16n,p | unknown | - | - | "doesn't remove" Auras not located |
| 702.17a-c | follows | `engine/rules/combat/block.go#CanBlock` | - |  |
| 702.18a,b | follows | `engine/rules/protection.go#Engine.shroudBlocksTarget` `engine/rules/playerkeywords.go#Engine.playerShroudBlocksTarget` | `TestPrintedShroudWithholdsCastWhenSoleTarget` `TestLightningGreavesGrantedShroudBlocksTargeting` `TestCR702KeywordFalseGapsAreRegistered` | - |
| 702.19a,d | follows | `engine/rules/combat.go#Engine.damageStep` `engine/rules/combat.go#Engine.lethalDamageNeed` | - | blocked-with-no-blockers trample hits player |
| 702.19b | partial | `engine/rules/combat.go#Engine.damageStep` `engine/rules/combat.go#Engine.lethalDamageNeed` | - | excess split is never a choice: all to player |
| 702.19c,e | deviates | `engine/rules/combat.go#Engine.damageStep` `engine/rules/combat.go#Engine.combatDefenderTarget` `engine/cards/face.go#KeywordHead` | - | Thrasta treated as plain trample; silent |
| 702.19f,g | follows | `engine/rules/combat.go#Engine.combatDefenderTarget` | - |  |
| 702.20a-c | follows | `engine/rules/combat.go#Engine.finishAttackers` `engine/rules/staging.go#NewStaged` | - |  |
| 702.21a | follows | `engine/cards/kw_ward.go#kwWard` `engine/rules/trigmatch/actions.go#becomesTargetMatches` | `TestWardDoesNotTriggerForFriendlyTargetingStackObject` | mana and life forms; triggers too |
| 702.21b | unknown | - | - | ward {X} timing not located |
| 702.22a-k,m | partial | `engine/rules/banding.go#Engine.startBandAsks` `engine/rules/banding.go#Engine.bandBlockPairs` `engine/rules/banding.go#Engine.divisionChooser` `engine/rules/banding.go#Engine.bandBlockerAssignments` `engine/effects/becomesblocked.go#effBecomesBlocked` | `TestCR702_22cBandIsDeclaredAndABlockerBlocksTheWholeBand` `TestCR702_22cBandTakesAtMostOneCreatureWithoutBanding` `TestCR702_22kActivePlayerDividesTheBandBlockersDamage` `TestCR702_22jDefendingPlayerDividesDamageBlockedByBanding` | plain banding follows; bands with other (702.22b) LOUD; a trample attacker keeps the deterministic split |
| 702.23a-c | not-impl | - | - | LOUD printed (15); SILENT granted (Rapid Fire) |
| 702.24a | follows | `engine/cards/kw_cumulativeupkeep.go#kwCumulativeUpkeep` `engine/rules/cumulative.go#scaleCost` `engine/rules/cumulative.go#Engine.startCumulativeUpkeep` | `TestPhyrexianSoulgorgerPaysCumulativeUpkeepWithAChosenCreature` | age counter at resolution; cost scaled per counter, hybrid chosen per counter |
| 702.24b | follows | `engine/cards/keywords.go#Face.expandKeywords` `engine/rules/cumulative.go#Engine.startCumulativeUpkeep` | - | per-instance trigger; no two-instance test |
| 702.25a | follows | `engine/cards/kw_flanking.go#kwFlanking` `engine/rules/trigger_blocks.go#isFlankingMarker` | `TestFlankingKnightOfTheHolyNimbusDebuffsBlockers` | one trigger per non-flanking blocker |
| 702.25b | follows | `engine/cards/kw_flanking.go#kwFlanking` `engine/rules/trigger_blocks.go#Engine.flankingInstances` | `TestFlankingSecondInstanceTriggersSeparately` `TestFlankingChainedGrantsStackInstances` | kw_flanking.go:16-18 comment wrongly calls this out of scope |
| 702.26a | follows | `engine/rules/turn.go#Engine.finishUntapStep` | `TestKatabaticWindsKeywordPhasesOutAndIn` `TestCR702PhasingKeywordTogglesAtUntapStep` `TestCR702KeywordFalseGapsAreRegistered` | - |
| 702.26b | follows | `engine/rules/layers_types.go#Engine.matchesWithCharsPT` `engine/rules/combat/block.go#CanBlock` `engine/rules/walkcache.go#Engine.scanBoardStaticsPrintedLists` | `TestCR702PhasedOutAttackerIsRemovedFromCombat` `TestCR702PhasedOutPermanentActivatedAbilityNotOffered` `TestCR702PhasedOutPermanentIsNotATarget` | treated as nonexistent; removed from combat |
| 702.26c | follows | `engine/events/apply_combat.go#foldPhaseOut` | - |  |
| 702.26d | follows | `engine/events/apply_combat.go#foldPhaseOut` | - |  |
| 702.26e | follows | `engine/rules/layers_types.go#Engine.matchesWithCharsPT` | `TestCR702PhasedOutPermanentStaticAbilityIsOff` | no continuous effect ever applies while phased out |
| 702.26f,k,n | unknown | - | - | (helper) |
| 702.26g,h | follows | `engine/events/apply_combat.go#foldPhaseOut` `engine/rules/turn.go#Engine.finishUntapStep` | `TestCR70226gAuraPhasesOutWithBearer` `TestCR70226hIndirectPhasesInWithHost` | attachments phase out indirectly with the host and phase in with it (PhasedOutIndirect) |
| 702.26i | follows | `engine/rules/attach.go#Engine.attachmentSBAs` | - | by construction; untested |
| 702.26j | follows | `engine/events/apply_combat.go#foldPhaseOut` | - | by construction |
| 702.26m | follows | `engine/rules/turn.go#Engine.beginTurn` | - |  |
| 702.26p | follows | `engine/rules/turn.go#Engine.finishUntapStep` | `TestCR702PhasedOutPermanentPhasesInAtUntap` | redundant by construction |
| 702.27a | follows | `engine/rules/cast_begin.go#Engine.beginCastWith` `engine/rules/stack.go#spellRestZone` | `TestBuybackConstantMistsReturnsToHand` `TestBuybackSearingTouchFizzleGoesToGraveyard` | its own cast mode; combining with alternative costs not verified |
| 702.28a-c | follows | `engine/rules/combat/block.go#CanBlock` | - | (helper) |
| 702.29a | follows | `engine/cards/kw_cycling.go#kwCycling` `engine/cards/kw_cycling.go#cyclingAbilitySA` | - |  |
| 702.29b | follows | `engine/cards/kw_cycling.go#cyclingAbilitySA` | - | by construction |
| 702.29c | follows | `engine/rules/trigger_zone_gate.go#Engine.zoneGate` | `TestCycledTriggerFiresOnTheCycleCostDiscard` |  |
| 702.29d | follows | `engine/rules/cast_commit.go#Engine.payCast` `engine/events/actions.go#Discard` | - | fires once by construction |
| 702.29e | follows | `engine/cards/kw_typecycling.go#kwTypeCycling` `engine/cards/kw_typecycling.go#typeCyclingAbilitySA` | `TestGrantedTypeCyclingCrossSeatActivationFiresCycledTrigger` |  |
| 702.29f | partial | `engine/rules/statics_costapply.go#Engine.abilityConstraintMatches` `engine/rules/turn_ledgers.go#Engine.activationMatches` | - | Activated.Cycling matchers skip typecycling (helper) |
| 702.30a | follows | `engine/cards/kw_echo.go#kwEcho` `engine/rules/echo.go#Engine.echoGateHolds` `engine/rules/echo.go#Engine.startEcho` | `TestEchoKarmicGuideControlChangeRearms` | X echo cost skipped LOUDLY (echo.go:139-148) |
| 702.30b | n/a | - | - |  |
| 702.31a-c | follows | `engine/rules/combat/block.go#CanBlock` | `TestHorsemanshipCanBlockOnlyHorsemanshipAttackers` |  |
| 702.32a | not-impl | - | - | LOUD (18 cards) |
| 702.33a,d,e,g | follows | `engine/rules/cast_altcost.go#kickerCost` `engine/rules/cast_targets.go#modeIsKicked` `engine/rules/cast_targets.go#modeFlags` | - | kicked-only targets gated; copies stay kicked |
| 702.33b,c,f | follows | `engine/rules/cast_altcost.go#twoPartKickerCosts` `engine/rules/cast_asks.go#Engine.multikickAsk` `engine/events/apply_cast.go#foldCastInfo` | `TestTimesKickedSurvivesColourReader` |  |
| 702.33h | unknown | - | - | sticker kicker (Unfinity) not checked |
| 702.34a | follows | `engine/rules/stack_keyword_exile.go#Engine.keywordExileDeparture` `engine/effects/resolve.go#effCounter` `engine/effects/misc.go#effWard` `engine/state/object.go#ExilesLeavingStack` | `TestCR702_34aFlashbackSpellExiledWhereverItWouldGo` `TestCR702_34aRemandOrdinarySpellStillGoesToHand` `TestCR702_34aReversedFlashbackCastReturnsToGraveyard` `TestFlashbackedSpellCounteredGoesToExile` | - |
| 702.35a | partial | `engine/rules/altcast.go#Engine.madnessReplacementApplies` `engine/rules/replacement.go#Engine.applyReplacementsDispatch` `engine/rules/altcast.go#Engine.offerMadness` | `TestCR702_35aMadnessExileIsMandatory` `TestMadnessEmrakulUsesMandatoryReplacementAndRespondableTrigger` `TestMadnessDiscardDuringARepeatExilesInPlace` | mandatory exile; granted madness ignored (printed face only) (helper) |
| 702.35b | follows | `engine/rules/cast_init.go#init` | - | (helper) |
| 702.35c | unknown | - | - | (helper) |
| 702.36a-c | partial | `engine/rules/combat/block.go#CanBlock` | - | artifact test reads printed face (helper) |
| 702.37a,c,d | follows | `engine/rules/cast_altcost.go#morphDownFamily` `engine/rules/legal_walk_hand.go#legalWalk.handWalk` | `TestMorphFaceDownCastPaysThreeAndEntersFaceDown` |  |
| 702.37b | follows | `engine/rules/cast_altcost.go#morphDownFamily` `engine/rules/morph_turnup.go#morphFaceUp.megamorph` | `TestMegamorphTurnFaceUpAddsAPlusOnePlusOneCounter` |  |
| 702.37e | follows | `engine/rules/morph_turnup.go#Engine.turnFaceUp` | `TestMorphTurnFaceUpIsASpecialActionPaysItsKeywordCost` | special action |
| 702.37f | follows | `engine/rules/morph_turnup.go#morphTurnUpCountAnnounced` `engine/rules/morph_turnup.go#Engine.turnFaceUp` | `TestMorphTurnFaceUpTriggerUsesAnnouncedX` |  |
| 702.37g | n/a | - | - | cross-reference to 708 |
| 702.38a,b | not-impl | - | - | LOUD (9) |
| 702.39a,b | not-impl | - | - | LOUD printed (8); SILENT granted (Hunter Sliver) |
| 702.40a | follows | `engine/cards/kw_storm.go#kwStorm` `engine/rules/trigger_referents.go#Engine.triggerReferents` `engine/effects/count_ref.go#evalTriggerCountOK` `engine/rules/storm_granted.go#Engine.checkGrantedStormTriggers` `engine/events/apply_trigger.go#StormGrantedBody` | `TestCR702_40aStormCountsSpellsCastBeforeIt` `TestStormCopiesTheSpellOncePerSpellCastBefore` `TestGrantedStormCountsSpellsCastBeforeIt` | count fixed when the trigger fires, granted storm included (helper) |
| 702.40b | follows | `engine/cards/keywords.go#Face.expandKeywords` | `TestGrantedStormCopiesForEachEarlierSpell` |  |
| 702.41a,b | follows | `engine/cards/kw_affinity.go#kwAffinity` | - | Frogmite, Scales of Shale, Claws Out lowered |
| 702.42a,b | follows | `engine/rules/cast_begin.go#Engine.beginCastWith` | `TestEntwinePaidChoosesEveryModeAndCharges` |  |
| 702.43a,b | not-impl | - | - | LOUD (24); SILENT granted (1 card) |
| 702.44a,b,d | follows | `engine/cards/kw_sunburst.go#kwSunburst` | `TestSunburstEtchedOracleEntersWithP1P1PerColour` `TestSunburstPentadPrismEntersWithChargePerColour` | counter kind from the printed type; one per line |
| 702.44c | not-impl | - | - | LOUD (coverage gate, through Modular) |
| 702.45a,b | not-impl | - | - | LOUD (37); SILENT granted (Fumiko the Lowblood) |
| 702.46a,b | not-impl | - | - | LOUD (26); SILENT granted (Kodama of the Center Tree) |
| 702.47a-e | not-impl | - | - | LOUD (30) |
| 702.48a-c | not-impl | - | - | LOUD (6) |
| 702.49a-d | follows | `engine/cards/kw_ninjutsu.go#kwNinjutsu` | `TestNinjutsuOrochiSoulReaverAndFallenShinobiFromHand` |  |
| 702.50a,b | not-impl | - | - | LOUD (5) |
| 702.51a-d | follows | `engine/rules/cast_altcost.go#Engine.hasCastConvoke` `engine/rules/cast_altcost.go#Engine.convokeCost` `engine/rules/pay/castasks.go#ConvokeAsk` | `TestAncientImperiosaurEntersWithTwoCountersPerConvoker` `TestTapPaymentsReadCurrentTypes` | player chooses creatures |
| 702.52a,b | follows | `engine/effects/cardflow.go#dredgeCandidates` | `TestDredgeUsesRealCorpusCard` | derived dredge (Yixlid Jailer) |
| 702.53a,b | follows | `engine/cards/kw_transmute.go#kwTransmute` | `TestTransmuteAndCyclingRealHandActivations` |  |
| 702.54a-c | follows | `engine/rules/replacement_etb.go#Engine.bloodthirstEntryMatch` `engine/rules/entry_counters.go#Engine.entryBodyCandidates` | `TestBloodthirstPrintedEntersWithCounters` |  |
| 702.55a-c | not-impl | - | - | LOUD (10) |
| 702.56a,b | follows | `engine/cards/kw_replicate.go#kwReplicate` `engine/rules/cast_altcost.go#replicateCost` `engine/rules/cast_asks.go#Engine.replicateAsk` `engine/events/apply_cast.go#foldCastInfo` | `TestReplicateCountBoundSeesReduceCostModifier` `TestReplicateCountSurvivesSpendReader` |  |
| 702.57a,b | not-impl | - | - | SILENT: upkeep/once/hand gate unmodelled |
| 702.58a,b | follows | `engine/cards/kw_graft.go#kwGraft` | - | no intervening-if on the move trigger |
| 702.59a | not-impl | - | - | LOUD (7) |
| 702.60a,b | not-impl | - | - | LOUD printed (5); SILENT granted (Thrumming Stone) |
| 702.61a-c | follows | `engine/rules/split_second.go#Engine.splitSecondHolds` `engine/rules/split_second.go#Engine.filterSplitSecondActions` | `TestVATSSplitSecondWithholdsCastsAndActivations` | mana abilities, special actions kept |
| 702.62a-d | follows | `engine/rules/cast_altcost.go#suspendCost` `engine/rules/resolution_suspend.go#Engine.Suspended` | - | not audited in depth |
| 702.63a-c | follows | `engine/cards/kw_vanishing.go#kwVanishing` | `TestFleshDuplicateETBGrantedVanishing` `TestVanishingDeepForestHermitUpkeepAndLastCounter` `TestCR702KeywordFalseGapsAreRegistered` | - |
| 702.64a-c | not-impl | - | - | SILENT (granted-only in corpus) |
| 702.65a,b | not-impl | - | - | LOUD (1) |
| 702.66a-c | follows | `engine/rules/cast_altcost.go#Engine.hasCastDelve` `engine/rules/cast_subcounter.go#Engine.delveAsk` `engine/rules/cast_altcost.go#Engine.delveCredit` | `TestGrantedDelveReachesTheCastSpell` |  |
| 702.67a-c | follows | `engine/cards/kw_fortify.go#kwFortify` `engine/cards/kw_equip.go#kwAttachCost` | `TestCAMPFortifyAttachesToLand` |  |
| 702.68a,b | follows | `engine/rules/trigger_blocks.go#Engine.checkAttackerUnblockedTriggers` | - | ordinary attack-unblocked triggers |
| 702.69a,b | follows | `engine/cards/kw_gravestorm.go#kwGravestorm` | `TestOminousHarvestGravestormCopiesPerDeath` |  |
| 702.70a,b | not-impl | - | - | SILENT: granted keyword does nothing |
| 702.71a | not-impl | - | - | LOUD (1) |
| 702.72a-c | not-impl | - | - | LOUD (12) |
| 702.73a | follows | `engine/effects/filter_type.go#IntrinsicAllCreatureTypes` `engine/effects/filter_type.go#changelingType` | `TestCloneChangelingCDAAfterEarlierStrip` |  |
| 702.74a | follows | `engine/rules/altcast_modes.go#altCastModes` `engine/rules/altcast.go#Engine.altCostEnter` | `TestEvokeCastPaysTheEvokeCostAndSacrifices` |  |
| 702.75a,b | follows | `engine/cards/kw_hideaway.go#kwHideaway` `engine/effects/cardflow.go#effHideaway` `engine/effects/cardflow.go#hideawayPicked` `engine/effects/cardflow.go#hideawayBottom` | `TestRiotAndHideawayUseRealCorpusCards` `TestHideawayExileIsSeatOnesSecretNotSeatZeros` `TestHideawayNoHostExileIsTheControllersSecret` `TestHideawayControllerStillSeesTheHiddenFace` `TestCRHideawayRestGoesToBottomInRandomOrder` | the exile is the controller's secret and look, fixed at the exile (the CR's look follows the permanent's controller); the rest goes to the bottom in a seeded random order |
| 702.76a | not-impl | - | - | LOUD (10); SILENT granted (Hunting Velociraptor) |
| 702.77a,b | not-impl | - | - | LOUD (11) |
| 702.78a,b | follows | `engine/cards/kw_conspire.go#kwConspire` `engine/rules/cast_asks.go#Engine.conspireAsk` | `TestConspirePrintedOfferPayAndCopy` |  |
| 702.79a | follows | `engine/cards/kw_persist.go#kwPersist` | - |  |
| 702.80a-d | follows | `engine/rules/combat.go#Engine.damageStep` `engine/rules/combat.go#Engine.runCombatAssignments` | `TestWitherVillagePillagersETBDamageAllDealsCounters` |  |
| 702.81a | follows | `engine/rules/cast_altcost.go#retraceExtra` | `TestGrantedRetraceChargesTheLandDiscard` |  |
| 702.82a-c | follows | `engine/cards/kw_devour.go#kwDevour` | - |  |
| 702.83a,b | follows | `engine/cards/kw_exalted.go#kwExalted` | `TestExaltedDragonAttackSacrificesALand` |  |
| 702.84a | follows | `engine/rules/unearth.go#Engine.unearthEnter` `engine/rules/unearth.go#Engine.unearthReplacementApplies` | `TestUnearthKeywordExpandsToGraveyardAbility` | haste granted UntilEOT (CR: no duration) |
| 702.85a | deviates | `engine/effects/cascade.go#effCascade` | - | rest bottomed in fixed, not random, order |
| 702.85b,c | follows | `engine/rules/cascade.go#Engine.queueCascadeTriggers` | - |  |
| 702.86a,b | follows | `engine/cards/kw_annihilator.go#kwAnnihilator` | `TestAnnihilatorArtisanSacrificesThePrintedAmount` |  |
| 702.87a-c | follows | `engine/cards/kw_levelup.go#kwLevelUp` | - |  |
| 702.88a-c | follows | `engine/rules/cast_commit.go#Engine.payCast` `engine/rules/resolution_chain.go#Engine.moveResolvedOffStack` | `TestReboundTargetRestrictionOnlyOffersPlayersKernel` `TestReboundCorpusCarrierExilesAndOffers` `TestCR702KeywordFalseGapsAreRegistered` | - |
| 702.89a,b | partial | `engine/rules/umbra.go#Engine.UmbraArmorAura` | `TestBearUmbraSavesBearerFromDestroySpell` | two umbra Auras: first chosen, no CR 616.1 choice |
| 702.90a-f | follows | `engine/rules/combat.go#Engine.damageStep` `engine/rules/combat.go#Engine.runCombatAssignments` | `TestInfectDamageToCreatureIsMinusOneCountersNotMarkedDamage` |  |
| 702.91a,b | follows | `engine/cards/kw_battle_cry.go#kwBattleCry` | `TestBattleCryPumpsOtherAttackingCreatures` |  |
| 702.92a | follows | `engine/cards/kw_livingweapon.go#kwLivingWeapon` | `TestBatterskullLivingWeaponCreatesAGermAndReachesAttach` |  |
| 702.93a | follows | `engine/cards/kw_undying.go#kwUndying` | `TestUndyingReturnsOnceWithACounter` |  |
| 702.94a,b | follows | `engine/rules/miracle.go#Engine.offerMiracle` `engine/rules/miracle.go#Engine.drawsThisTurn` | `TestMiracleOffersOnTheFirstDrawOnly` |  |
| 702.95a-d | follows | `engine/cards/kw_soulbond.go#kwSoulbond` `engine/effects/soulbond.go#effPair` | `TestSoulbondPairsCurrentCreatures` |  |
| 702.95e | unknown | - | - | unpair conditions not located |
| 702.96 | follows | `engine/rules/altcast_modes.go#altCastModes` `engine/rules/stack.go#Engine.resolveTop` | `TestOverloadedCastTargetsEachNotOne` `TestOverloadIsCastableWhenOnlyProtectedObjectsMatch` | Alt cost |
| 702.97 | not-impl | - | - |  |
| 702.98 | follows | `engine/rules/unleash.go#Engine.applyUnleashReplacement` `engine/rules/combat/block.go#CanBlock` | `TestUnleashChoicePosedAndCounterApplied` `TestGrantedUnleashCounteredDogCantBlock` | Entry +1/+1 election |
| 702.99 | follows | `engine/cards/kw_cipher.go#kwCipher` `engine/effects/cipher.go#effCipher` `engine/rules/trigger_granted.go#Engine.checkCipherTriggers` | `TestCipherArcaneHeistEncodesOntoCreature` `TestCipherCounteredSpellDoesNotEncode` | Encode on resolution |
| 702.100a | partial | `engine/cards/kw_evolve.go#kwEvolve` `engine/rules/trigmatch/zone.go#ZoneChangeMatchesWithCapture` | `TestEvolveGrowsOnlyForBiggerCreatures` |  |
| 702.100b-702.100d | follows | `engine/cards/kw_evolve.go#kwEvolve` `engine/rules/trigmatch/zone.go#ZoneChangeMatchesWithCapture` | `TestEvolvedDoesNotFireForAnEqualOrSmallerCreature` `TestEvolvingAdaptiveEvolvesOnlyForABiggerCreature` | Power/toughness comparison |
| 702.101 | follows | `engine/cards/kw_extort.go#kwExtort` `engine/effects/extort.go#effExtort` | `TestExtortUsesRealCorpusCard` | Cast trigger with optional {W/B} payment |
| 702.102 | follows | `engine/rules/split.go#Engine.fuseCost` `engine/rules/split.go#Engine.resolveFused` `engine/rules/split.go#Engine.fusedTimingOK` | `TestFuseCantBeCastEitherHalf` `TestCopyOfFusedSpellAsksPerDeclaration` | Fused split cast from hand |
| 702.103a-702.103d | follows | `engine/rules/bestow.go#bestowCost` `engine/rules/bestow.go#bestowedAttachSA` `engine/state/object.go#Object.BestowedAttached` | `TestBestowedSpellIsAuraNotCreature` `TestCelestialArchonBestowedCastAttachesAuraToBearer` | Bestowed Aura spell |
| 702.103e | follows | `engine/rules/stack.go#Engine.resolveTop` `engine/rules/bestow.go#Engine.bestowTargetIllegal` | `TestBoonSatyrBestowedOntoShockedBearsEntersAsCreature` `TestBoonSatyrUnbestowedIsACreatureToEntryReplacements` | REVISED: cr-g02 (e7bb8e319) resolves an illegal-target bestowed spell as a creature |
| 702.103f-702.103g | follows | `engine/state/object.go#Object.BestowedAttached` `engine/rules/bestow.go#bestowedAttachSA` | `TestEidolonOfCountlessBattlesBestowTransition` |  |
| 702.104 | not-impl | - | - |  |
| 702.105 | follows | `engine/cards/kw_dethrone.go#kwDethrone` `engine/rules/trigger_granted.go#Engine.checkGrantedDethroneTriggers` | `TestDethroneCountsOnlyTheAttackedPlayersLife` `TestGrantedDethroneTriggers` |  |
| 702.106 | not-impl | - | - |  |
| 702.107 | follows | `engine/cards/kw_outlast.go#kwOutlast` | `TestOutlastExpandsToASorcerySpeedTapActivation` `TestAbzanBattlePriestOutlastIsSorcerySpeedOnly` |  |
| 702.108 | follows | `engine/cards/kw_prowess.go#kwProwess` | `TestDoubleProwessTriggersTwice` `TestExaltedPumpsALoneAttackerAndProwessPumpsOnNoncreatureSpells` |  |
| 702.109 | follows | `engine/rules/altcast_modes.go#altCastModes` `engine/rules/altcast.go#Engine.altCostEnter` | `TestDashCastsGainHasteAndReturnAtTheEndStep` `TestDashDelayedReturnDoesNotFollowABlinkedPermanent` |  |
| 702.110 | follows | `engine/cards/kw_exploit.go#kwExploit` `engine/effects/exploit.go#effExploit` | `TestColonelAutumnExploitElectionSacrificesAndFiresItsOwnTrigger` `TestColonelAutumnExploitDeclineSacrificesNothing` | ETB optional sacrifice |
| 702.111 | follows | `engine/rules/combat.go#Engine.validateBlockers` `engine/rules/combat.go#Engine.blockPairScopeFor` | `TestMenaceBlockDeclarationNeedsZeroOrAtLeastTwoBlockers` `TestMenaceAgainstOneLegalBlockerPosesNoPair` |  |
| 702.112 | follows | `engine/cards/kw_renown.go#kwRenown` `engine/state/object.go#Object.Renowned` | `TestConstableOfTheRealmRenownPutsTwoCountersOnce` `TestCR702KeywordFalseGapsAreRegistered` | - |
| 702.113 | not-impl | - | - |  |
| 702.114 | follows | `engine/rules/chars/derive.go#Compute` | `TestDevoidCreatureIsColourless` `TestGrantedProtectionCountsAndDevoidIsColourless` |  |
| 702.115 | not-impl | - | - |  |
| 702.116 | follows | `engine/cards/kw_myriad.go#kwMyriad` `engine/effects/myriad.go#effMyriad` | `TestMyriadMayChooseEachOpponentIndependently` `TestMyriadMayDeclineEveryOpponent` |  |
| 702.117 | follows | `engine/rules/cast_altcost.go#surgeCost` | `TestSurgeNeedsAnotherSpellThisTurn` `TestSurgeRealCorpusRecklessBushwhacker` |  |
| 702.118 | follows | `engine/rules/combat/block.go#CanBlock` | `TestSkulkBlocksOnlyGreaterPowerBlockers` `TestGrantedSkulkStaticBlocksGreaterPowerBlockers` | Greater-power creatures cannot block |
| 702.119 | follows | `engine/rules/emerge.go#emergeCost` `engine/rules/emerge.go#Engine.applyEmergeReduction` | `TestEmergeCastPaysReducedCost` `TestEmergeCastReductionExceedsGenericKeepsColored` |  |
| 702.120 | follows | `engine/rules/cast_asks.go#Engine.castModeAsk` `engine/rules/resolution_chain.go#Engine.applyCastModes` | `TestEscalateManaEscalateChargesPerExtraMode` `TestEscalateOneModeChargesNoExtraCost` |  |
| 702.121 | follows | `engine/cards/kw_melee.go#kwMelee` `engine/rules/melee.go#Engine.meleeRemembered` | `TestTitaniaMeleeCountsDistinctAttackedOpponents` `TestTitaniaAndAdrianaGrantTwoMeleeInstances` |  |
| 702.122 | follows | `engine/cards/kw_crew.go#kwCrew` `engine/cards/kw_crew.go#crewAbilitySA` `engine/events/apply_combat.go#foldCrew` | `TestCrewAvengersQuinjetAnimatesUntilEndOfTurn` `TestCrewAbilityParsesOnTheRealVehicle` | Tap untapped creatures with total power N |
| 702.123 | follows | `engine/cards/kw_fabricate.go#kwFabricate` | `TestFabricateAngelOfInventionChoosesServosOrCounters` `TestFabricateCountScalesWithTheKeyword` |  |
| 702.124a-702.124e | follows | `engine/deck/partner.go#IsPartnerPair` `engine/rules/commander_config.go#partnerPairOK` | `TestPartnerSeatsTwoCommanders` `TestFoodFellowshipPartnerPairSeatsBothCommanders` | Two commanders seated |
| 702.124f,i | deviates | `engine/deck/partner.go#partnerHead` `engine/deck/partner.go#IsPartnerPair` | - | Pairs by head word only |
| 702.124g-702.124h | follows | `engine/deck/partner.go#IsPartnerPair` | `TestPartnerSeatsTwoCommanders` |  |
| 702.124j | follows | `engine/cards/kw_partner_with.go#kwPartnerWith` `engine/deck/partner.go#partnerWithNames` | `TestPartnerWithETBOffersNamedPartnerSearch` `TestPartnerWithExpansionSearchesTheFullPartnerName` |  |
| 702.124k | not-impl | - | - |  |
| 702.124m | partial | `engine/deck/partner.go#isDoctorCard` | `TestDoctorCompanionSeatsBothCommanders` `TestIsPartnerPairDoctorCompanionCorpus` | Accepts any Doctor |
| 702.124n | follows | `engine/deck/partner.go#partnerHead` | - |  |
| 702.125 | follows | `engine/cards/kw_undaunted.go#kwUndaunted` | `TestUndauntedReducesGenericAndNotColoredCost` `TestUndauntedDoesNotReducePlainSpell` |  |
| 702.126 | follows | `engine/rules/cast_altcost.go#Engine.improviseCost` `engine/rules/cast_altcost.go#Engine.hasCastImprovise` | `TestImproviseBottleCapBlastTapsTwoArtifactsPaysFullPrice` `TestImproviseArtifactCreatureQualifies` `TestTapPaymentsReadCurrentTypes` |  |
| 702.127 | follows | `engine/rules/legal.go#aftermathAlternateFace` `engine/state/object.go#ExilesLeavingStack` `engine/rules/stack_keyword_exile.go#Engine.keywordExileDeparture` | `TestAftermathCastsAlternateFaceFromGraveyardAndExiles` `TestAftermathCounteredGoesToExile` | Second half from graveyard only |
| 702.128 | follows | `engine/cards/kw_embalm.go#kwEmbalm` `engine/effects/copypermanent.go#effCopyPermanent` `engine/events/copy_except.go#foldCopyExcept` | `TestAngelOfSanctionsEmbalmExilesAndMintsAWhiteZombieCopy` `TestEmbalmOnlyOfferedFromTheGraveyard` `TestEmbalmedTokenHasNoManaCost` | white, no mana cost and Zombie are the token's copiable values |
| 702.129 | follows | `engine/cards/kw_embalm.go#kwEmbalm` `engine/effects/copypermanent.go#effCopyPermanent` `engine/events/copy_except.go#foldCopyExcept` | `TestTimelessDragonEternalizeExilesAndMintsAFourFourBlackZombieCopy` `TestCloneOfAnEternalizedTokenCopiesTheExceptions` | black, 4/4, no mana cost and Zombie are the token's copiable values |
| 702.130 | follows | `engine/cards/kw_afflict.go#kwAfflict` `engine/rules/trigger_granted.go#Engine.checkGrantedAfflictTriggers` | `TestPrintedAfflictDrainsTheDefender` `TestUnblockedAfflictDrainsNothing` |  |
| 702.131 | follows | `engine/rules/ascend.go#Engine.ascendPossible` `engine/rules/ascend.go#Engine.grantSpellBlessing` `engine/rules/ascend.go#Engine.checkBlessingGrants` | `TestAscendPermanentGrantsTheBlessingAtTenPermanents` `TestAscendSpellResolutionGrantsTheBlessing` | City's blessing at ten permanents |
| 702.132 | not-impl | - | - |  |
| 702.133 | follows | `engine/rules/cast_altcost.go#jumpstartExtra` `engine/state/object.go#ExilesLeavingStack` `engine/rules/stack_keyword_exile.go#Engine.keywordExileDeparture` | `TestRadicalIdeaJumpstartCastsDiscardsAndExiles` `TestJumpstartedSpellCounteredGoesToExile` | Graveyard cast plus discard |
| 702.134 | follows | `engine/cards/kw_mentor.go#kwMentor` `engine/rules/mentor.go#Engine.mentorAdmits` | `TestMentorCounterOnLesserPowerAttacker` `TestGrantedMentorFiresAndDoesNotStackWithAPrintedKeyword` |  |
| 702.135 | follows | `engine/cards/kw_afterlife.go#kwAfterlife` | `TestAfterlifeTitheTakerMintsSpiritTokens` `TestAfterlifeCountScalesWithTheKeyword` |  |
| 702.136 | follows | `engine/rules/cast_etbchoice.go#Engine.entryETBChoice` `engine/events/entry_counters.go#EntryCounterGrants` | `TestRiotAndHideawayUseRealCorpusCards` `TestRiotCopyKeepsTokenEffectRiders` |  |
| 702.137 | not-impl | - | - |  |
| 702.138 | follows | `engine/rules/altcast.go#Engine.escapeCost` `engine/rules/legal_walk_alt.go#legalWalk.graveyardCastsOver` | `TestKroxaEscapeCastDoesNotSacrificeOnETB` `TestUnderworldBreachGrantsEscapeAndTheEscapeCastResolves` | Graveyard cast exiling other cards |
| 702.139 | not-impl | `engine/rules/trigger_match.go` | - |  |
| 702.140a | follows | `engine/rules/mutate.go#mutateCost` `engine/rules/mutate.go#mutateTargetSA` | `TestBrokkosMayPlayMutateFromTheGraveyard` |  |
| 702.140b | follows | `engine/rules/stack.go#Engine.resolveTop` `engine/rules/mutate.go#Engine.mutateVictim` | `TestGemrazerMutatingOntoShockedTargetEntersAsCreature` | REVISED: cr-g02 (e7bb8e319) resolves an illegal-target mutating spell as a creature |
| 702.140c | deviates | `engine/rules/mutate.go#Engine.mutatePlaceAsk` | `TestBotPolicyCorpusMutatePlacesUnder` |  |
| 702.140d-702.140f | follows | `engine/rules/mutate.go#Engine.resolveMutate` `engine/rules/trigmatch/mutate.go#mutatesMatches` | `TestEverquillPhoenixMutatesOntoTopAndCreatesFeather` `TestCountTimesMutatedDrivesTheRealReader` | Merged permanent takes top card plus all abilities |
| 702.141 | follows | `engine/cards/kw_encore.go#kwEncore` `engine/effects/encore.go#effEncore` | `TestEncoreActivatesFromTheGraveyardIntoHastedTokenCopies` `TestEncoreCreatesOneDelayedTriggerForAllOpponentTokens` | Exile from graveyard |
| 702.142 | follows | `engine/rules/legal_loyalty.go#Engine.boastGateOK` | `TestBroadsideBombardiersBoastGateOfferedAfterAttacking` `TestBroadsideBombardiersBoastOncePerTurn` |  |
| 702.143 | follows | `engine/rules/altcast.go#Engine.foretellCastAvailable` `engine/rules/scriptfacts/foretell_cost.go#ForetellCost` `engine/rules/cast_commit.go#Engine.payCast` `engine/view/describe.go#Describe` | `TestDelayedBlastFireballForetellCastDealsFive` `TestCosmosChargerWidensForetellToAnyPlayersTurn` `TestForetellActionNamesNoCardToTheOpponent` `TestForetellActionIsAForetellNotACast` | {2} exile face down on your turn; only the foreteller may look, and the action reads "foretells" (cr-g62, cr-g64) |
| 702.144 | follows | `engine/cards/kw_demonstrate.go#kwDemonstrate` `engine/effects/demonstrate.go#effDemonstrate` | `TestSilverquillLecturerDemonstrateCopyAndOpponentToken` `TestDemonstrateElectionDeclineCopiesNothing` | Optional copy |
| 702.145 | follows | `engine/rules/daynight.go#Engine.syncDayNightFaces` `engine/rules/daynight.go#Engine.dayNightStateCheck` `engine/events/daynight_entry.go#entersTransformedAtNight` `engine/effects/daynight_bound.go#dayNightBound` | `TestCR702_145cDayboundTransformsAsItBecomesNight` `TestCR702_145fNightboundTransformsAsItBecomesDay` `TestCR702_145bDayboundEntersTransformedAtNight` `TestCR702_145bDayboundCantBeTransformedByOtherEffects` `TestCR702_145gNightboundAloneMakesItNight` | 35/36 carriers playable; Tovolar also needs api:DayTime |
| 702.146 | not-impl | - | - |  |
| 702.147 | follows | `engine/rules/combat/restrictions.go#ParseHiddenKeyword` `engine/rules/trigger_granted.go#Engine.checkGrantedAttackKeywordTriggers` | `TestDecayedCannotBlock` `TestDecayedCounterGrantsBothRules` | Can't block |
| 702.148 | follows | `engine/rules/cleave.go#cleaveSA` `engine/rules/cleave.go#stackSpellSA` `engine/rules/legal_walk_hand.go#legalWalk.handWalk` | `TestCR702_148aPathOfPerilCleaveDestroysEveryCreature` `TestCR702_148aLunarRejectionCleaveTargetsAnyCreature` `TestCR702_148aDigUpCleaveSearchesForAnyCard` | cleave cost offered from hand; the bracket-free text is Forge's second NonBasicSpell$ SP |
| 702.149 | follows | `engine/cards/kw_training.go#kwTraining` `engine/rules/trigger_granted.go#Engine.checkGrantedTrainingTriggers` | `TestTrainingCounterOnAttackWithABiggerCreature` `TestTrainingIgnoresAnEqualPowerAttacker` |  |
| 702.150 | follows | `engine/events/entry_counters.go#EntryCounterGrants` `engine/state/object.go#Object.CompleatedLifePaid` | `TestCompleatedLoyaltyFollowsPhyrexianLifePaid` `TestCR702KeywordFalseGapsAreRegistered` | - |
| 702.151 | follows | `engine/cards/kw_reconfigure.go#kwReconfigure` `engine/effects/attach.go#effAttach` | `TestRazorfieldRipperReconfigureAttachAndUnattach` `TestAttachedReconfigureIsNotACreature` | Sorcery-speed attach/unattach |
| 702.152 | follows | `engine/rules/cast_altcost.go#Engine.blitzCosts` `engine/rules/blitz.go#Engine.blitzEnter` | `TestBlitzedCreatureIsSacrificedAtTheNextEndStep` `TestBlitzedCreatureDrawsWhenItDies` |  |
| 702.153 | follows | `engine/rules/cast_altcost.go#Engine.casualtySpec` | `TestCasualtyPrintedSpellSacrificeCopiesTarget` `TestCR702KeywordFalseGapsAreRegistered` | - |
| 702.154 | deviates | `engine/rules/enlist.go#Engine.applyEnlist` `engine/rules/enlist.go#Engine.enlistOfferList` | `TestAradeshEnlistElectionPumpAndTrigger` | No reflexive trigger, X frozen |
| 702.155 | not-impl | - | - |  |
| 702.156 | follows | `engine/cards/kw_ravenous.go#kwRavenous` | `TestJackedRabbitRavenousX5CountersAndDraw` `TestJackedRabbitRavenousX3CountersNoDraw` | Enters with X counters |
| 702.157 | follows | `engine/cards/kw_squad.go#kwSquad` `engine/rules/cast_altcost.go#squadCost` | `TestSquadKeywordExpansionAddsTheETBTrigger` `TestSquadNonManaCostsParse` | Repeatable additional cost |
| 702.158-702.159,702.161-702.162 | not-impl | - | - |  |
| 702.160 | follows | `engine/state/prototype.go#PrototypeParams` `engine/state/mana_value.go#ManaValue` `engine/rules/cast_begin.go#Engine.beginCastWith` `engine/rules/legal_walk_hand.go#legalWalk.handWalk` | `TestPrototypeOffersInsetCast` `TestPrototypeCharacteristicsOnStackAndBattlefield` `TestPrototypeNormalCastStaysPrinted` `TestPrototypeNormalElsewhere` `TestPrototypeCopyIsPrototyped` `TestPrototypeReversalClears` | Inset cost, colour and P/T only on the stack and battlefield; a copy keeps the designation |
| 702.163 | follows | `engine/cards/kw_formirrodin.go#kwForMirrodin` | `TestGlimmerLensForMirrodinCreatesARebelAndAttaches` |  |
| 702.164 | follows | `engine/rules/layers_derived.go#Engine.ToxicValue` `engine/rules/combat.go#Engine.runCombatAssignments` | `TestToxicIxhelAddsPoisonOnCombatDamage` `TestToxicInstancesCumulate` | Combat damage to player adds N poison |
| 702.165 | follows | `engine/cards/kw_backup.go#kwBackup` | `TestGuardianScalelordBackupCountersAndCopiesAbilities` `TestGuardianScalelordBackupSelfTargetGrantsNothing` | ETB counters |
| 702.166 | follows | `engine/rules/cast_begin.go#Engine.beginCastWith` `engine/rules/cast_targets.go#modeFlags` | `TestBargainEligibilityOffersArtifactEnchantmentTokenOnly` `TestBargainConditionGateScriesOnlyWhenBargained` `TestBargainOffersCreatureToken` `TestBargainStillRefusesNontokenCreature` | Optional sacrifice of artifact, enchantment or token |
| 702.167 | partial | `engine/cards/kw_craft.go#kwCraft` `engine/cards/kw_craft.go#CraftShapeSupported` | `TestCraftAsksAcrossBothZonesAndBotAnswerValidates` `TestCraftEntersTransformed` |  |
| 702.168 | follows | `engine/rules/morph_turnup.go#Engine.turnFaceUp` `engine/rules/morph_turnup.go#morphFaceUpCost` | `TestDisguiseFaceDownCastHasWardTwo` `TestDeadlyDisguiseMorphCarriersAreFullySupported` | Face-down 2/2 with ward {2} |
| 702.169 | follows | `engine/rules/trigmatch/case.go#caseSolvedMatches` `engine/rules/legal_activation.go#activationConditionOKCodes` `engine/state/object.go#Object.Solved` | `TestCaseBecomesSolvedAtEndStepAndAuditorTriggers` `TestCaseSolvedClearsWhenTheCaseLeaves` |  |
| 702.170 | follows | `engine/rules/plotzone.go#plotZoneAllowed` `engine/rules/legal_plotzone_walk.go#plotZoneWalk` | `TestKeywordCostParamPlotOfferAndCastUseFirstField` `TestBecomesPlottedSelfTrigger` | Sorcery-speed plot from hand |
| 702.171 | follows | `engine/cards/kw_saddle.go#kwSaddle` `engine/cards/kw_saddle.go#saddleAbilitySA` | `TestGrantedSaddleIsSorcerySpeed` `TestBecomesSaddledFirstTimeOncePerTurn` | Tap other creatures with power N, sorcery speed |
| 702.172 | follows | `engine/rules/spree.go#modeCostTotal` `engine/rules/spree.go#Engine.modeCostFeasible` | `TestRequisitionRaidSpreeChargesEachChosenMode` `TestRequisitionRaidSpreeZeroModeCastNotOffered` |  |
| 702.173 | not-impl | - | - |  |
| 702.174 | follows | `engine/rules/cast_asks.go#Engine.giftAsk` `engine/effects/gift.go` | `TestGiftKitnapPromisedDrawsAndSkipsStunCounters` `TestGiftCopyCarriesNoPromise` | Promise an opponent at cast |
| 702.175 | partial | `engine/rules/offspring.go#Engine.offspringRawParam` `engine/cards/kw_offspring.go#kwOffspring` | `TestOffspringPrintedPayMintsOneOneTokenCopy` `TestOffspringGrantedByZinniaReachesTheCast` | First instance only |
| 702.176 | follows | `engine/rules/impending.go#impendingTickGrant` `engine/rules/altcast.go#Engine.altCostEnter` | `TestImpendingCastEntersWithTimeCountersAndIsNotACreature` `TestImpendingEndStepRemovesATimeCounterAndWakesAtTheLast` |  |
| 702.177a | follows | `engine/rules/legal_loyalty.go#Engine.activationLimitBlocked` | `TestExhaustMaiJadedEdgeWithheldAfterOneUse` `TestExhaustInlineWithheldSameTurnAndLaterTurn` |  |
| 702.177b | unknown | `engine/rules/legal_loyalty.go#Engine.additionalActivationLimit` | - |  |
| 702.178 | follows | `engine/rules/speed.go#Engine.maxSpeedAbilities` | `TestMaxSpeedAddAbilityOfferedExactlyOnce` `TestMaxSpeedPlayerProperty` |  |
| 702.179 | follows | `engine/rules/speed.go#Engine.checkSpeedStart` `engine/rules/speed.go#Engine.checkSpeedGain` | `TestStartYourEnginesSpeedLifecycle` `TestStartYourEnginesSpeedStartsWithControlChange` | Speed starts at 1 |
| 702.180 | follows | `engine/rules/cast_altcost.go#harmonizeCost` `engine/rules/cast_altcost.go#Engine.harmonizePayment` | `TestHarmonizePaysDerivedPowerNotPrinted` `TestHarmonizeAnnouncementSkipsNonCreaturePower` `TestHarmonizeTapsAnimatedLandForItsPower` | Graveyard cast; tap one creature to reduce by power |
| 702.181 | follows | `engine/cards/kw_mobilize.go#kwMobilize` | `TestMobilizeSacrificesAtTheNextEndStep` `TestMobilizeSecondTriggerCountsOnlyItsOwnTokens` |  |
| 702.182 | follows | `engine/cards/kw_jobselect.go#kwJobSelect` | `TestJobSelectExpandsToHeroEtbAttach` `TestBlackMagesRodJobSelectCreatesHeroAndAttaches` |  |
| 702.183 | follows | `engine/rules/spree.go#modeCost` | `TestFireMagicTieredChargesTheChosenMode` |  |
| 702.184 | partial | `engine/rules/station.go#Engine.stationAmount` `engine/rules/station.go#Engine.handleStation` | `TestEveryStationCarrierStationsThroughTheStack` `TestSetAudit_eoe_TapestryWarden_StationUsesToughness` |  |
| 702.185 | follows | `engine/rules/altcast.go#Engine.warpRecastAvailable` `engine/rules/altcast.go#Engine.altCostEnter` | `TestWarpCastsFromTheGraveyardExileAtEndStepAndRecastFromExile` `TestWarpedSpacePaysZeroForExileCast` | Hand alt cost; end-step exile |
| 702.186 | follows | `engine/state/object.go#Object.Harnessed` `engine/effects/filter.go#predicates` | `TestHarnessedGatesTheInfinityTrigger` |  |
| 702.187 | follows | `engine/rules/altcast.go#Engine.mayhemCastCost` `engine/rules/altcast.go#Engine.mayhemDiscardedThisTurn` | `TestAbominationWorldRavagerMayhemOfferedAndCasts` `TestMayhemLandPlayNotOfferedWithoutThisTurnDiscard` |  |
| 702.188 | follows | `engine/rules/webslinging.go#Engine.webSlingingCosts` `engine/rules/webslinging.go#webSlingingReturnExtra` | `TestWebSlingingCastChargesSubstitutedCostAndReturnsCreature` `TestWebSlingingNotOfferedWithoutPayment` |  |
| 702.189 | follows | `engine/cards/kw_firebending.go#kwFirebending` | `TestGrantedFirebendingAddsManaOnAttack` `TestFirebendingManaSurvivesCombatStepsAndEmptiesAtEndOfCombat` |  |
| 702.190 | follows | `engine/rules/sneak.go#Engine.sneakCosts` `engine/rules/sneak.go#Engine.sneakTimingOK` `engine/rules/sneak.go#Engine.sneakEnter` | `TestSneakNotOfferedOutsideDeclareBlockers` `TestNinjaTeenLevelThreeSneaksCreatureFromGraveyard` |  |
| 702.191 | partial | `engine/cards/kw_increment.go#kwIncrement` `engine/rules/trigmatch/cast.go#incrementAdmits` | `TestIncrementSpendMustExceedPowerAndToughness` | No is-a-creature check |
| 702.192 | follows | `engine/rules/paradigm.go#Engine.paradigmMayPlay` `engine/rules/paradigm.go#Engine.paradigmResolved` | `TestParadigmOffersFreeCopyFromExile` `TestSetAudit_sos_RestorationSeminar_ParadigmNextMainCopyCast` | First resolution exiles |
| 702.193 | follows | `engine/rules/legal_loyalty.go#Engine.activationLimitBlocked` `engine/rules/legal_activation.go#Engine.ownManaReduction` | `TestPowerUpFixtureWithholdsSecondActivation` `TestSetAudit_msh_BraveBrawler_PowerUpCostReducedOnEntryTurn` | Once-only activation |
| 702.194 | follows | `engine/rules/teamwork.go#teamworkOffer` `engine/rules/teamwork.go#paidTapCastInfo` | `TestCR702194bSpellTeamworkValidSA` `TestAgentMariaHillTeamworkTapTrigger` | Optional tap-a-creature additional cost |
| 702.195 | follows | `engine/rules/storied.go#Engine.storiedPossible` `engine/rules/storied.go#Engine.checkEnduringStoryGrants` | `TestBomburUntapsWithAnEnduringStory` `TestDainEnduringStoryPricesTheAttackTax` |  |
