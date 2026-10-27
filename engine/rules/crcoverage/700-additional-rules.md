# CR 700s: Additional rules (700, 701, 703 onward)

Coverage of this chapter of the Comprehensive Rules (2026-09-25) by gorge. Seeded from the CR audit of 2026-10-20 at `72dabd96c`; kept current by every change that touches a rule. See [README](README.md) for the status vocabulary and the reference syntax.

| rule | status | code | tests | notes |
|---|---|---|---|---|
| 700.1 | follows | `engine/events/event.go#Event` `engine/rules/trigger_blocks.go#Engine.attackerBlockedCandidates` | - | per-event vs per-batch trigger kinds distinct |
| 700.2 | follows | `engine/effects/charm_params.go#compileCharm` | - | modal carrier compiled |
| 700.2a | partial | `engine/rules/cast_asks.go#Engine.castModeAsk` `engine/rules/legal_targets.go#Engine.charmTargetsAvailable` | `TestModalCastOmitsModesWithNoLegalTargets` | gorge: repeatable charm with zero legal modes not uncastable (latent) |
| 700.2b | follows | `engine/rules/trigger_queue.go#Engine.askTriggerModes` | `TestModalTriggerWithNoTargetableModeIsRemoved` |  |
| 700.2c | follows | `engine/rules/target_modes.go#charmTargetSlots` | `TestModalCastTargetsComeFromTheAnnouncedMode` | targets only for chosen modes |
| 700.2d | follows | `engine/rules/target_modes.go#charmTargetSlots` `engine/effects/charm.go#CharmModeBounds` | `TestMysticConfluenceRepeatsTargetedModeAsksPerInstance` `TestRepeatedModeTargetsPerInstance` | each repeated mode is a separate instance with its own target group; the same target may fill two groups |
| 700.2e | deviates | `engine/effects/charm_params.go#compileCharm` `engine/rules/cast_asks.go#Engine.castModeAsk` | - | caster picks mode for Fatal Lore etc.; Note only at resolution |
| 700.2f | follows | `engine/state/object.go#Object.ChosenModes` | - | modes held apart from targets |
| 700.2g | follows | `engine/rules/target_modes.go#copyCharmModes` | `TestCopyCharmAsksEveryChosenTargetMode` | copies keep modes |
| 700.2h | follows | `engine/rules/cast_asks.go#Engine.castModeAsk` | `TestRequisitionRaidSpreeChargesEachChosenMode` `TestMetamorphicBlastSpreeUnaffordableModeNotOffered` |  |
| 700.2i | follows | `engine/effects/charm.go#CharmPawprints` `engine/effects/charm.go#CharmResolveOrder` | `TestPawprintBudget` `TestPawprintModesResolveInPrintedOrder` `TestPawprintBurrowTargetOrder` `TestModeDecisionPublishesPresentZeroBudget` `TestCharmPawprintPresenceAndPricing` | root `Pawprint$` publishes Decision.MaxSum over the modes' Option.Value; chosen modes resolve in printed order, repeats stable |
| 700.3 | partial | `engine/effects/twopiles.go#effTwoPiles` | - | gorge: Zone$/DefinedPiles$/LeftRight shapes move nothing, LOUD |
| 700.3a-c | follows | `engine/effects/twopiles.go#effTwoPiles` | - | piles are choices; cards stay until pick |
| 700.3d | partial | `engine/effects/twopiles.go#effTwoPiles` | - | one-card set forced into pile A |
| 700.4 | follows | `engine/rules/trigmatch/zone.go#ZoneChangeMatchesWithCapture` | - |  |
| 700.5 | deviates | `engine/effects/count_body_heads.go#evalCountBodyDotted` | `TestDevotionDualSumsBothColours` | two-colour devotion counts hybrid twice; test pins wrong 5 |
| 700.5a | partial | `engine/state/object.go#Object.faceSlow` | - | devotion modifier absent; layer timing otherwise ok |
| 700.6 | follows | `engine/effects/filter_word.go#historicMatches` | - | no historic card in pools |
| 700.7 | follows | `engine/effects/filter.go#predicates` | `TestDelayedDestroyAndSacrificeFindOnlyTheRememberedObject` | identity by ObjID / obj+version |
| 700.8-700.8c | follows | `engine/effects/count_agg.go#partySize` | - | 8b highest count done correctly |
| 700.8d | follows | `engine/effects/choose_control.go#chooseEachGroups` | - | Stick Together |
| 700.9 | deviates | `engine/effects/filter_attachment.go#modifiedPermanent` | - | internal Shield/Deathtouched markers count as counters |
| 700.10 | not-impl | - | - | Cut Short planeswalker branch fails closed |
| 700.11 | follows | `engine/effects/player_filter.go#descendedThisTurn` `engine/state/game.go#ZoneEntry` | - |  |
| 700.12-700.12a | follows | `engine/effects/filter_word.go#outlawSubtypes` `engine/effects/filter_word.go#outlawMatches` | - | five outlaw subtypes, layer-aware |
| 700.13 | follows | `engine/rules/trigmatch/actions.go#commitCrimeMatches` `engine/rules/turn_ledgers.go#Engine.recordTurnLedgers` | - |  |
| 700.14 | follows | `engine/rules/trigmatch/cast.go#ManaExpendMatches` `engine/rules/cast_commit.go#Engine.payCast` | - | convoke counted as spent (G-lane) |
| 700.15 | n/a | - | - | terminology |
| 700.16 | follows | `engine/effects/filter_word.go#wordMatches` | - |  |
| 701.1 | n/a | - | - | definitional |
| 701.2a | follows | `engine/rules/legal_activation.go#Engine.activatorAllows` | - | controller-only unless Activator$ |
| 701.3a | follows | `engine/rules/attach.go#Engine.attachRefused` `engine/rules/emit.go#Engine.emit` | `TestAttachToAnIllegalPermanentDoesNotMove` | an attach chooser may still offer an illegal bearer; the attach is refused |
| 701.3b | partial | `engine/rules/attach.go#Engine.attachRefused` `engine/effects/attach.go#emitAttach` | `TestReEquipToTheSameCreatureDoesNothing` `TestAttachToAnIllegalPermanentDoesNotMove` | same-bearer and illegal-bearer attaches do nothing; the third sentence (a non-attachment object) is not checked |
| 701.3c | follows | `engine/events/apply_zone.go#foldAttach` `engine/rules/layers.go#Engine.staticEffectsWalk` | `TestEquipAfterAbilityLossRestampsTheEquipment` `TestEquipBeforeAbilityLossStaysLost` | same-bearer attach keeps the stamp |
| 701.3d | partial | `engine/rules/trigmatch/unattached.go#unattachedMatches` `engine/rules/trigger_kinds.go#looksBackOnDeparture` `engine/rules/attach.go#Engine.attachmentSBAs` | `TestAttachedEquipmentLeavingBecomesUnattached` | the attachment's own departure fires Mode$ Unattached by look-back (no event); a player leaving the game is not modelled |
| 701.4a | follows | `engine/rules/cast_asks.go#Engine.beholdCostAsk` `engine/rules/pay/paidcost.go#EmitChoiceCosts` | - | no behold in pools |
| 701.4b | unknown | - | - | not reached |
| 701.5a-b | n/a | - | - | definitional; see 601 |
| 701.6a-b | follows | `engine/effects/resolve.go#effCounter` | - | no refund path |
| 701.7a | follows | `engine/effects/token.go#effToken` | - |  |
| 701.7b | partial | `engine/rules/token_entry_replacements.go#Engine.applyTokenEntryUpdates` | - | Clone/Animate entry replacements skipped on tokens silently |
| 701.7c | n/a | - | - | errata |
| 701.8a-c | follows | `engine/effects/zone_bulk.go#effDestroy` `engine/rules/sba.go#Engine.applyLegendBatch` `engine/rules/sba.go#Engine.destroyLethalDamage` | - | trig Destroyed unregistered (LOUD) |
| 701.9a-b | follows | `engine/effects/cardflow.go#effDiscard` | - | random via seeded RNG |
| 701.9c | unknown | - | - | not reached |
| 701.10a-c | follows | `engine/effects/count.go#NumForObject` `engine/effects/pump_params.go#numForObjectText` | - | signed +X covers negative case |
| 701.10d | follows | `engine/effects/life.go#effSetLife` `engine/events/event.go#LifeChange` | - |  |
| 701.10e | follows | `engine/effects/counters.go#effMultiplyCounter` | - | Mossborn Hydra, Zimone |
| 701.10f | follows | `engine/effects/mana_effect.go#effMana` | - |  |
| 701.10g | follows | `engine/rules/replacement_replace.go#Engine.replaceCount` | - | Twinflame Tyrant; gorge via ReplaceCount$ Twice |
| 701.11a-c | n/a | - | - |  |
| 701.12a | deviates | `engine/effects/exchange_control.go#effExchangeControl` `engine/rules/life_exchange.go#Engine.finishLifeExchange` | `TestExchangeLifeSettlesWhenCantGainLifeConsumesSide` | control exchange all-or-nothing; life exchange: one side prevented, other still applies |
| 701.12b | follows | `engine/effects/exchange_control.go#effExchangeControl` | - | same controller: nothing |
| 701.12c | deviates | `engine/rules/life_exchange.go#Engine.finishLifeExchange` | `TestExchangeLifeSettlesWhenCantGainLifeConsumesSide` | one side prevented, other still applies; test pins it (see 701.12a) |
| 701.12d-f | not-impl | - | - | 1 corpus card |
| 701.12g | partial | `engine/rules/life_exchange.go#Engine.ExchangeLifeVariant` `engine/rules/life_exchange.go#Engine.finishLifeExchange` | - | ExchangePower unregistered (LOUD) |
| 701.12h | partial | `engine/effects/changetext.go#effExchangeTextBox` | - | triggered/activated abilities not swapped |
| 701.13a | follows | `engine/state/ids.go#ZExile` | - |  |
| 701.14a | follows | `engine/effects/damage.go#effFight` `engine/effects/damage.go#fightPairs` `engine/effects/damage.go#fightMemberOK` `engine/effects/damage.go#emitFightHit` `engine/effects/damage.go#fightRider` `engine/rules/replacement_damage.go#Engine.damageReplacementMatches` `engine/rules/engine_damage.go#Engine.inFlightDamageSource` | `TestCR701_14aHansErikssonFightsTheRevealedCreature` `TestFightArenaDefinedTargetedGuard` `TestCR701_14aBloodFeudPairGuard` `TestFightTargetlessDefinedMisresolutionIsLoud` `TestFightCorpusShapeCensus` `TestRepeatEachChooseOrderEzurisPredationAsksBeforeLoop` `TestCR701_14aFightDamageSourceIsTheCreature` `TestFightParkedReplacementChoiceKeepsTheFighter` `TestFightReplacedSourceIsTheFighter` `TestFightGrantedPreventByReadsTheFighter` `TestFightCantPreventReadsTheFighter` `TestCR701_14aFightHitsAreSimultaneous` `TestCR701_14aInfectFightHitsAreSimultaneous` | Defined-only fights pair the first two Defined$ creatures; a selector that names one creature twice (Grothama, Ezuri's Predation) is a loud Note and no fight. |
| 701.14b | follows | `engine/effects/damage.go#fightPairs` `engine/effects/damage.go#fightMemberOK` | `TestCR701_14bNoncreatureFighterDealsNothing` |  |
| 701.14c | follows | `engine/effects/damage.go#effFight` | `TestCR701_14cSelfFightDealsTwicePower` |  |
| 701.14d | follows | `engine/effects/damage.go#emitObjectDamage` | - |  |
| 701.15a | follows | `engine/effects/misc.go#effGoad` `engine/events/apply_state_helpers.go#expireTurnGoads` | - | no goad in pools |
| 701.15b | deviates | `engine/rules/combat/goad.go#GoadMayAttack` `engine/rules/attack_cost.go#Engine.attackOffersCompute` | - | goader banned outright, not a requirement (multiplayer) |
| 701.15c-d | follows | `engine/rules/combat/goad.go#GoadedBy` `engine/events/apply_combat.go#foldGoad` | - |  |
| 701.16a | follows | `engine/effects/investigate.go#effInvestigate` | - |  |
| 701.17a | follows | `engine/effects/cardflow.go#effMill` | - |  |
| 701.17b | follows | `engine/effects/cardflow.go#effMill` `engine/rules/pay/costsettle.go#MillCostPayable` | `TestCR701_17bMillCostShortLibraryNotOffered` `TestCR701_17bMillManaAbilityEmptyLibraryUnpayable` `TestCR701_17bDeepSpawnShortLibraryCannotPay` | effect mills as many as possible; a mill cost the library cannot cover is not offered |
| 701.17c | follows | `engine/effects/cardflow.go#rememberMilled` | - |  |
| 701.17d | unknown | `engine/effects/cardflow.go#effMill` | - | not reached |
| 701.18a | follows | `engine/rules/legal.go#Engine.sorcerySpeed` `engine/rules/legal_walk_hand.go#legalWalk.handWalk` | - | priority, main phase, empty stack, land limit |
| 701.18b | follows | `engine/rules/mayplay.go#Engine.mayPlayPermissions` | - |  |
| 701.18c-e | n/a | - | - | terminology |
| 701.19a | follows | `engine/effects/regeneration.go#ReplaceDestruction` `engine/rules/combat.go#Engine.cleanupBody` | - | OpRegenerate unused by tables |
| 701.19b | unknown | - | - | static regeneration not located |
| 701.19c | follows | `engine/effects/regeneration.go#ReplaceDestruction` | - | Incinerate stat:CantRegenerate still on ratchet |
| 701.20a-c,e | follows | `engine/effects/cardflow.go#effReveal` `engine/effects/dig.go#digSecretTo` | `TestJacePlusTwoBottomedCardIsHiddenFromItsOwner` |  |
| 701.20d | unknown | - | - | not reached |
| 701.21a | partial | `engine/effects/zone_bulk.go#sacrificeObjectEligible` `engine/effects/zone_bulk.go#effSacrifice` | - | gorge object path lacks controller check |
| 701.22a | follows | `engine/effects/cardflow.go#effLookAndArrange` | - |  |
| 701.22b | follows | `engine/effects/cardflow.go#effLookAndArrange` | `TestScryZeroIsNoScryEvent` `TestScryOneStillTriggersOnEmptyLibrary` | scry 0 records nothing; scry N>0 over an empty library still records |
| 701.22c | partial | `engine/effects/cardflow.go#effLookAndArrange` | - | players scry sequentially |
| 701.22d | follows | `engine/rules/arrange.go#arrangeAnswerRecord` | - |  |
| 701.23a | follows | `engine/effects/zone_search.go#effSearchLibrary` `engine/effects/zone_search.go#originAlternativeZones` | `TestCROriginAlternativeGraveyardOnlySkipsLibrary` `TestCROriginAlternativeLibraryOnlyOmitsGraveyard` | "and/or" origins: the searcher elects the zones; an unsearched library is not searched, recorded or shuffled |
| 701.23b | follows | `engine/effects/filter.go#SearchStatesQuality` `engine/effects/zone_search.go#effSearchLibrary` | - | finding is optional for a stated quality |
| 701.23c,e-j | unknown | - | - | not reached (23c undefined quality, 23h-j APNAP multi-search, outside the game) |
| 701.23d | follows | `engine/effects/zone_search.go#effSearchLibrary` | `TestLibrarySearchMandatoryQualityRequiresFinding` | quantity search must find |
| 701.24a | follows | `engine/effects/zone_library.go#shuffleLibraryOrder` | - | seeded Fisher-Yates |
| 701.24b | follows | `engine/effects/zone_library.go#searchShuffleTail` `engine/effects/zone_library.go#placeLibraryObjects` | `TestCR70123SearchThenShuffleIsMandatory` `TestCR70124NoHostSearchShuffleNonMandatoryShuffles` | REVISED: cr-g09 landed, gorge's then-shuffle mandatory |
| 701.24c-d | deviates | `engine/effects/zone_change.go#effChangeZone` `engine/effects/zone_changeall.go#effChangeZoneAll` | - | no shuffle when objects missing/set empty |
| 701.24e-f | not-impl | - | - | cards refused by coverage gate |
| 701.24g | unknown | - | - | not reached |
| 701.25a | follows | `engine/effects/cardflow.go#effSurveil` | - |  |
| 701.25b | follows | `engine/effects/cardflow.go#effLookAndArrange` | - |  |
| 701.25c | follows | `engine/effects/cardflow.go#effSurveil` `engine/effects/cardflow.go#effLookAndArrange` | `TestSurveilZeroIsNoSurveilEvent` `TestCerebralDownloadWithNoArtifactsSurveilsZero` | |
| 701.25d | partial | `engine/effects/cardflow.go#effLookAndArrange` | - | marker before the look; no outcome found |
| 701.26a-b | follows | `engine/effects/combatfx.go#effTap` `engine/effects/combatfx.go#effUntapAll` `engine/effects/untap.go#effUntap` | - |  |
| 701.27a-b | follows | `engine/effects/resolve.go#effSetState` `engine/events/event.go#FlipFace` | - | OpTransform unused |
| 701.27c | partial | `engine/effects/resolve.go#effSetState` | - | any multi-face card can flip |
| 701.27d | not-impl | `engine/effects/resolve.go#effSetState` | - | no instant/sorcery face check |
| 701.27e,g | unknown | `engine/rules/trigger_eligibility.go#triggerModeEventRows` | - | not reached |
| 701.27f | not-impl | `engine/effects/resolve.go#effSetState` | - | no transformed-since-stacked guard |
| 701.28a-b | follows | `engine/effects/resolve.go#setStateWouldChange` `engine/events/event.go#FlipFace` | - | no DFCs in pools |
| 701.28c | partial | `engine/effects/resolve.go#setStateWouldChange` | - | as 701.27c |
| 701.28d-e | not-impl | `engine/effects/resolve.go#setStateWouldChange` | - | as 701.27d/f |
| 701.28f | not-impl | - | - |  |
| 701.29a | follows | `engine/effects/dig.go#digChooser` `engine/effects/dig.go#effDig` | `TestJacePlusTwoOnOpponentIsChosenByJacesController` `TestSpinIntoMythFatesealCasterChoosesAndOrdersTheTop` | REVISED: cr-g08 landed; the resolving ability's controller chooses and orders |
| 701.30a,d | follows | `engine/effects/clash.go#effClash` | `TestClashPlacementOneCardAndReverseChoices` `TestClashTriggerModeReadsWonOrientation` |  |
| 701.30b-c | partial | `engine/effects/clash.go#clashParticipants` | - | no opponent choice; not APNAP |
| 701.31a-d | deviates | `engine/rules/planar.go#Engine.currentPlane` `engine/events/apply_misc.go#foldPlanarWalk` | - | per-seat planes; no single planar controller |
| 701.32a-c | n/a | - | - | Archenemy absent from gorge |
| 701.33a-b | n/a | - | - | Archenemy absent |
| 701.34a | follows | `engine/effects/counters.go#effProliferate` `engine/effects/counters.go#proliferateOnce` | `TestCR701_34aProliferateTwiceIsTwoProliferations` `TestContagionEngineProliferatesTwice` `TestProliferateIgnoresInternalMarkers` | internal Shield/Deathtouched markers excluded |
| 701.34b | n/a | - | - | no 2HG |
| 701.35a | not-impl | `engine/effects/registry.go#Resolve` | - | Detain unregistered |
| 701.36a | partial | `engine/effects/copypermanent.go#effCopyPermanent` | `TestGrowingRanksPopulatesTheCreatureTokenYouControl` | never asks; first token in zone order |
| 701.36b | follows | `engine/effects/copypermanent.go#effCopyPermanent` | - |  |
| 701.37a,c | follows | `engine/effects/counters_put.go#effPutCounter` `engine/rules/legal_activation.go#Engine.monstrosityGateOK` | `TestCR70134MonstrosityDesignation` `TestHydraBroodmasterMonstrosityCreatesXTokens` `TestGigglingSkitterspikeMonstrosityMarksOnceAndGateWithholds` |  |
| 701.37b | partial | `engine/effects/counters_put.go#effPutCounter` | - | Monstrosity X=0 never marks monstrous |
| 701.38a-c | follows | `engine/effects/vote_effect.go#effVote` `engine/effects/vote_effect.go#voteWinner` | `TestCouncilsJudgmentExilesTheMostVoted` |  |
| 701.38d | not-impl | - | - | Brago's Representative inert |
| 701.39a | follows | `engine/effects/counters_put.go#putCounterBolster` | `TestBolsterReadsCurrentToughness` |  |
| 701.40a,e | follows | `engine/effects/zone_manifest.go#effManifest` | - |  |
| 701.40b-d | not-impl | `engine/rules/legal_walk_battlefield.go#legalWalk.battlefieldWalk` | - | no turn-face-up special action |
| 701.40f | unknown | `engine/rules/replacement_match.go#Engine.replacementMatchesRememberedUngatedBy` | - | Cage-vs-manifest test would settle |
| 701.40g | follows | `engine/effects/resolve.go#effSetState` `engine/events/apply_copy.go#foldTurnFaceUp` | `TestHauntwoodsRevealsNoncreatureAndLeavesItFaceDown` `TestHauntwoodsRejectsNoncreaturePermanentOnValidNewFace` `TestEffectTurnUpLeavesManifestedSorceryFaceDown` | effect turn-up reveals a manifested instant or sorcery and leaves it face down |
| 701.40h | n/a | - | - | cross-reference to 708 |
| 701.41a | deviates | `engine/effects/counters_put.go#putCounterSupport` | `TestLeadByExampleSpellSupportOffersEveryCreature` | support not targeted: ignores hexproof/ward |
| 701.42a-c | follows | `engine/effects/meld.go#Meld` `engine/effects/meld.go#meldRoles` `engine/effects/meld_api.go#effMeld` | - |  |
| 701.43a-d | follows | `engine/rules/combat.go#Engine.startExertAsks` `engine/rules/combat.go#Engine.exertAnswer` `engine/rules/turn.go#Engine.finishUntapStep` | - |  |
| 701.44a | follows | `engine/effects/explore.go#effExplore` `engine/effects/explore.go#exploreOnce` | - |  |
| 701.44b-c | deviates | `engine/effects/explore.go#exploreOnce` | - | empty library / departed explorer: nothing, no trigger |
| 701.44d | partial | `engine/effects/explore.go#effExplore` | - | Defined order, not APNAP |
| 701.45a | n/a | - | - | - |
| 701.46a | follows | `engine/effects/counters_put.go#effPutCounter` | `TestAdaptDBBodyCountsAndConditionsOnNoP1P1` |  |
| 701.47a | deviates | `engine/effects/amass.go#effAmass` | `TestAmassDoublingSeasonCountersEveryMint` `TestAmassUsesChangelingAsArmy` `TestAmassIgnoresNoncreatureArmy` | no Army choice; counters on every doubled mint |
| 701.47b | partial | `engine/effects/amass.go#effAmass` | - | amass 0 creates no token |
| 701.47c-d | follows | `engine/effects/amass.go#amassRiders` `engine/effects/amass.go#effAmass` | - |  |
| 701.48a | not-impl | - | - |  |
| 701.49a-d | follows | `engine/effects/venture.go#effVenture` `engine/effects/venture.go#ventureAdvance` | `TestFirstVentureOffersTheThreeDungeons` |  |
| 701.50a,d,f | follows | `engine/effects/connive.go#effConnive` `engine/effects/connive.go#conniveOnce` | `TestConnivesTriggerFiresForIronMonger` |  |
| 701.50b | deviates | `engine/effects/connive.go#conniveOnce` | - | departed conniver: no draw/discard (no LKI) |
| 701.50c | partial | `engine/effects/connive.go#effConnive` | - | Defined order, not APNAP |
| 701.50e | deviates | `engine/effects/connive.go#conniveOnce` | - | connive 0 still fires Connives triggers |
| 701.51a-c | not-impl | `engine/effects/registry.go#Supported` | - | OpenAttraction unregistered |
| 701.52a | n/a | - | - | no Attraction deck |
| 701.53a-b | follows | `engine/effects/incubate.go#effIncubate` `engine/effects/incubate.go#incubateLoop` | `TestIncubateMintsTokenWithTheCounters` `TestIncubatorTokenTransformsIntoThePhyrexianFace` |  |
| 701.54a,e | follows | `engine/effects/ring.go#effRingTemptsYou` `engine/effects/filter_word.go#wordMatches` | - |  |
| 701.54b | follows | `engine/state/game.go#Player` `engine/events/apply_zone.go#foldMoveZone` | - | designation is player state, not copiable |
| 701.54c | follows | `engine/events/apply_helpers.go#ringEmblemAbility` `engine/rules/trigger_match.go#Engine.checkRingEmblemTriggers` `engine/effects/ring.go#effRingBlockerSacrifice` `engine/effects/ring.go#ringEmblemLegendary` `engine/rules/combat/restrictions.go#RingBearerOutpowers` | `TestCR70154cRingLevel1BearerIsLegendary` `TestCR70154cRingLevel1GreaterPowerCantBlockBearer` `TestCR70154cRingLevel1LegendaryFollowsTheBearer` `TestCR70154cRingLevel2AttackLoots` `TestCR70154cRingLevel3BlockerSacrificedAtEndOfCombat` `TestCR70154cRingLevel3EachBlockerTriggersSeparately` `TestCR70154cRingLevel4CombatDamageDrainsEachOpponent3` | the granted Legendary does not reach the legend rule, which reads the printed face (704.5j, cr-g25) |
| 701.54d | follows | `engine/effects/ring.go#effRingTemptsYou` `engine/events/apply_misc.go#foldRingTemptsYou` | - | counts even with no creature |
| 701.55a-b | follows | `engine/effects/charm.go#effVillainousChoice` | `TestVillainousChoiceVisitsEveryDefinedOpponent` |  |
| 701.55c | not-impl | - | - | The Valeyard inert |
| 701.55d | partial | `engine/effects/context.go#definedSpec` | - | order from controller, not AP |
| 701.56a | partial | `engine/effects/time_travel.go#timeTravelObjects` | - | misses granted-suspend; offers 0-counter cards |
| 701.57a | deviates | `engine/effects/discover.go#effDiscover` | `TestCR70157DiscoverRandomBottomOrder` | bottom order not random (known) |
| 701.57b-c | follows | `engine/effects/discover.go#effDiscover` `engine/effects/discover.go#effDiscoverBottom` | `TestDiscoverExilesUntilCastableCardAndOffersItFree` |  |
| 701.58a,e | follows | `engine/effects/zone_manifest.go#effCloak` `engine/rules/chars/derive.go#BaseKeywords` | - |  |
| 701.58b-d | not-impl | - | - | no turn-face-up |
| 701.58f | unknown | - | - | not reached |
| 701.58g-h | n/a | - | - | no turn-face-up path exists; 58h cross-reference |
| 701.59a-b | follows | `engine/rules/pay/evidence.go#EvidenceGraveCandidates` `engine/rules/cast_targets.go#Engine.evidenceAsk` `engine/rules/cumulative.go#Engine.triggeredCostXAsk` `engine/rules/evidence_cost.go#triggeredEffectCost.evidenceStage` | `TestConditionCollectedGatesOnEvidence` `TestTriggerCollectEvidenceXAnnouncesAndPays` |  |
| 701.59c | follows | `engine/effects/conditions.go#conditionDefinedGroup` `engine/rules/cast_targets.go#castSubTargetsOwed` | - | linked via OptionalCost (traced shallowly) |
| 701.60a | deviates | `engine/events/apply_player.go#foldControlChange` | - | suspected cleared on control change |
| 701.60b-d | follows | `engine/rules/chars/derive.go#BaseKeywords` `engine/rules/combat/block.go#CanBlock` `engine/effects/misc.go#effAlterAttribute` | - |  |
| 701.61a | follows | `engine/rules/cast_asks.go#Engine.forageAsk` `engine/rules/pay/castparts.go#NonManaCastableP` `engine/rules/pay/sacparts.go#ManaForagePayable` `engine/rules/trigger_cost_forage.go#triggeredEffectCost.foragePayable` | `TestTriggeredForageCanBePaid` |  |
| 701.62a | follows | `engine/effects/zone_manifest.go#effManifestDread` | - |  |
| 701.62b | deviates | `engine/effects/zone_manifest.go#effManifestDread` `engine/effects/zone_manifest.go#manifestDreadMove` | - | empty library: no ManifestDread trigger |
| 701.63a | partial | `engine/effects/endure.go#endureCreateSpirit` | - | Spirit is 0/0 + rider; copies 0/0 |
| 701.63b | follows | `engine/effects/endure.go#effEndure` | `TestEndureCounterBranch` `TestEndureSpiritBranch` |  |
| 701.64a-b | follows | `engine/events/apply_player.go#foldAlterAttribute` | - |  |
| 701.65a-b | follows | `engine/effects/airbend.go#effAirbend` `engine/rules/airbend.go#Engine.airbendCastAvailable` | `TestAirbendRecastCastsExiledCardForTwo` | exile, owner may cast for {2}; ElementalBend air fires only if something exiled |
| 701.66a | deviates | `engine/cards/link.go#builtinSVars` | - | returns under owner, not earthbender |
| 701.66b | follows | `engine/effects/earthbend.go#effEarthbend` | - |  |
| 701.67a-b | follows | `engine/rules/pay/castpayment.go#ConvokePayment` `engine/rules/pay/castasks.go#ConvokeOffer` | - |  |
| 701.67c | deviates | `engine/rules/pay/paidcost.go#EmitConvokeTaps` | - | waterbend trigger only if something tapped |
| 701.68a-b | follows | `engine/effects/blight.go#effBlight` `engine/rules/pay/castparts.go#NonManaCastableP` | `TestBlightOpponentBlightsTheirOwnCreature` |  |
| 701.68c | unknown | - | - | not reached |
| 701.68d | n/a | - | - |  |
| 701.69a | follows | `engine/effects/heal.go#effHealDamage` `engine/effects/heal.go#HealMarkedDamage` | `TestHealDamageClearsMarkedDamage` |  |
| 701.70a | follows | `engine/effects/recruit.go#effRecruit` | - |  |
| 701.71a | follows | `engine/effects/empower.go#effEmpower` | - |  |
| 703.1 | follows | `engine/rules/turn.go#Engine.finishEnteredStep` `engine/rules/turn.go#Engine.finishStepBoundary` | - | TBAs run in step entry/exit code, never as stack objects |
| 703.1a | follows | `engine/rules/trigmatch/actions.go#PhaseMatches` | - | beginning-of-step abilities are Phase triggers queued to the stack |
| 703.2 | follows | `engine/rules/turn.go#Engine.finishEnteredStep` | - | engine performs TBAs; the active player only answers the declarations they require |
| 703.3 | follows | `engine/rules/turn.go#Engine.finishEnteredStep` | `TestFirstTurnDrawStepTriggerDoesNotFireInTwoPlayer` | TBA first, then triggers drained and Priority emitted |
| 703.4 | follows | `engine/rules/turn.go#Engine.finishEnteredStep` | - | index rule; members below |
| 703.4a | follows | `engine/rules/turn.go#Engine.finishUntapStep` | `TestCR702PhasedOutPermanentPhasesInAtUntap` `TestCR702PhasingKeywordTogglesAtUntapStep` |  |
| 703.4b | follows | `engine/rules/daynight.go#Engine.dayNightUntapCheck` | `TestCR731DayBecomesNightWhenThePreviousActivePlayerCastNothing` | - |
| 703.4c | follows | `engine/rules/turn.go#Engine.finishUntapStep` `engine/rules/untap.go#Engine.untapTurnPermanent` | `TestBasaltMonolithStaysTappedThroughItsUntapStep` `TestChainedUntapAndUpkeepSkips` | one untap action; stun and doesn't-untap honoured |
| 703.4d | follows | `engine/rules/turn.go#Engine.drawStepTurnAction` `engine/rules/turn.go#skipFirstDrawStep` | `TestDrawStepDeckOutEndsTheGameBeforePriority` `TestDrawTriggerResolvingInTheDrawStepDoesNotRedraw` | CR 103.8a first-turn skip included |
| 703.4e | n/a | - | - | Archenemy not modelled |
| 703.4f | partial | `engine/rules/saga.go#Engine.advanceSagas` `engine/rules/saga.go#Engine.sagaChapters` | `TestEntryCounterSagaLoreOrderIsDecidedBeforeEntry` |  |
| 703.4g | not-impl | - | - | LOUD: no Attraction deck |
| 703.4h | n/a | - | - | gorge plays attack-multiple-players (each attacker picks a defender) |
| 703.4i | follows | `engine/rules/combat.go#Engine.askAttackers` `engine/rules/combat.go#Engine.finishAttackers` | - |  |
| 703.4j | follows | `engine/rules/combat.go#Engine.askBlockers` `engine/rules/combat.go#Engine.handleBlockers` | - |  |
| 703.4k | follows | `engine/rules/combat.go#Engine.runCombatAssignments` `engine/rules/combat.go#Engine.askNextDivision` | - | damage assignment asked per attacker before dealing |
| 703.4m | follows | `engine/rules/combat.go#Engine.dealCombatDamage` `engine/rules/combat.go#Engine.damageStep` | - | all combat damage dealt in one pass |
| 703.4n | follows | `engine/rules/combat.go#Engine.cleanupBody` `engine/rules/combat.go#Engine.discardCleanup` `engine/rules/combat.go#Engine.maxHandSizeFor` | `Test3141DiscardDownToSevenIsAsked` `Test3141OnlyActivePlayerDiscards` `Test3141ExactlyMaxHandSizeAsksNothing` |  |
| 703.4p | follows | `engine/rules/combat.go#Engine.cleanupBody` | `TestCleanupClearsCombatDamageDeathtouchAndUntilEOTEffects` `TestCleanupDamageRemovalIsNotDamageDone` |  |
| 703.4q | follows | `engine/rules/turn.go#Engine.finishStepBoundary` | `TestFirebendingManaSurvivesCombatStepsAndEmptiesAtEndOfCombat` | stat:UnspentMana exemptions honoured in gorge |
| 704.1 | follows | `engine/rules/sba.go#Engine.checkStateBased` | `TestSBALoopsUntilStable` | SBAs applied directly, never stacked |
| 704.1a | follows | `engine/rules/trigmatch/actions.go#PhaseMatches` `engine/rules/sba.go#Engine.checkStateBased` | - | state triggers are ordinary triggered abilities, outside the SBA pass |
| 704.2 | follows | `engine/rules/turn.go#Engine.step` `engine/rules/engine.go#submitCommit` | - | engine-performed; checked at every priority point |
| 704.3 | deviates | `engine/rules/sba.go#Engine.checkStateBased` `engine/rules/sba.go#Engine.annihilateOppositeCounters` | `TestSBALoopsUntilStable` `TestSBABatchUsesPreDepartureBoard` `TestCleanupChecksStateBasedActionsAfterEndingEffects` | per-family batches, not one event |
| 704.4 | follows | `engine/rules/engine.go#submitCommit` `engine/rules/turn.go#Engine.step` | `TestDrawDuringResolutionDefersStateBasedActions` `TestCR704NoLifeSBAInsideSmallpoxDiscard` `TestMidCastSBADoesNotEndGameDuringManaWindow` | checked only at priority boundaries, not mid-resolution or mid-cast |
| 704.5 | follows | `engine/rules/sba.go#Engine.checkStateBased` | - | index rule; members below |
| 704.5a | follows | `engine/rules/sba.go#Engine.checkLoseConditions` `engine/rules/cantlose.go#Engine.playerLoses` | `TestZeroLifeEliminatesAPlayer` `TestNoPendingDecisionWithAZeroLifePlayerNotYetLost` | can't-lose replacements honoured |
| 704.5aa | partial | `engine/rules/speed.go#Engine.checkSpeedStart` | `TestStartYourEnginesSpeedStartsWithControlChange` |  |
| 704.5b | follows | `engine/rules/sba.go#Engine.checkLoseConditions` `engine/rules/cantlose.go#Engine.markDrawEmpty` | `TestEmptyLibraryDrawLosesAtTheNextStateBasedCheck` `TestEmptyLibraryDrawIsCheckedWhenTheCheckHappens` |  |
| 704.5c | follows | `engine/rules/sba.go#Engine.checkLoseConditions` | `TestInfectDamageToPlayerIsPoisonAndTenPoisonLoses` |  |
| 704.5d | follows | `engine/rules/sba.go#Engine.ceaseDeadTokens` | `TestATokenThatDiesCeasesToExist` `TestATokenLeavingTheBattlefieldByAnyRouteCeasesToExist` `TestATokenOnTheStackIsNotPrematurelyCeased` |  |
| 704.5e | follows | `engine/rules/sba.go#Engine.ceaseDeadTokens` | `TestPreparedExileCopyCeasesAfterCast` | spell copies leaving the stack and card copies off stack/battlefield cease |
| 704.5f | follows | `engine/rules/sba.go#Engine.destroyLethalDamage` | `TestZeroToughnessDies` `TestIndestructibleSurvivesLethalDamageButNotZeroToughness` `TestEarthbendZeroToughnessAnimatedLandDiesToSBA` | moved directly, no regeneration |
| 704.5g | follows | `engine/rules/sba.go#Engine.destroyLethalDamage` | `TestLethalDamageDestroysACreature` `TestRegenerationDestroy` `TestAnimatedLandMarkedLethalDamageDiesToSBA` | destruction replaceable (regeneration, umbra armor) |
| 704.5h | follows | `engine/rules/sba.go#Engine.destroyLethalDamage` | `TestDeathtouchDamageDestroysRegardlessOfAmount` `TestCR704NoncombatDeathtouchDestroysDamagedCreature` `TestRegenerationDeathtouch` | deathtouch mark cleared each check |
| 704.5i | follows | `engine/rules/sba.go#Engine.planeswalkerZeroLoyalty` | `TestFaceDownPlaneswalkerSurvivesZeroLoyaltySBA` `TestFaceUpPlaneswalkerWithoutLoyaltyStillSwept` `TestPlaneswalkerSBABatchUsesPreDepartureBoard` `TestCR306_9MoonedWalkerWithNoLoyaltyStays` | - |
| 704.5j | follows | `engine/rules/sba.go#Engine.legendGroups` `engine/rules/sba.go#Engine.legendExemptions` `engine/rules/sba_prefilter.go#Engine.mayHaveLegendPair` `engine/rules/sba_prefilter.go#Engine.sbaTypeFast` `engine/rules/sba.go#Engine.legendAnswer` | `TestCR704CorpusLegendDuplicatesCannotReachPriority` `TestFaceDownPlaneswalkerIsNotLegendaryForTheLegendRule` `TestIgnoreLegendRuleExemptsMatchingCreatures` `TestEffectIgnoreLegendRuleHallOfEchoes` `TestCR704_5jGrantedLegendaryCountsForLegendRule` `TestCR704_5jGrantedLegendaryGuards` | legendExemptions collects the printed statics and the registered Effect route (Hall of Echoes) |
| 704.5k | follows | `engine/rules/sba.go#Engine.worldRule` `engine/rules/sba.go#Engine.worldPermanents` | `TestWorldRuleNewestSurvives` `TestWorldRuleTieSendsAll` `TestWorldRuleReadsDerivedSupertype` |  |
| 704.5m | follows | `engine/rules/attach.go#Engine.attachmentSBAs` `engine/rules/attach.go#Engine.auraStillMatchesEnchant` `engine/events/apply_zone.go#move` | `TestCR704AuraBecomesIllegalAfterProtection` `TestFlickerUnattachesAura` | the bearer-departure fold queues the detach as UnattachPending (the live AttachedTo link stays while the bearer is off the battlefield, so the mid-resolution window still shows the attachment fastened; the bearer's re-entry breaks it before the SBA pass, where destroyLethalDamage would otherwise read the stale -2/-2 first); attachmentSBAs' pending arm publishes the same graveyard MoveZone and clears the link |
| 704.5n | partial | `engine/rules/attach.go#Engine.attachmentSBAs` `engine/events/apply_zone.go#move` | `TestFortificationDetachesWhenBearerStopsBeingALand` `TestFortifiedLandThatGainsATypeStaysAttached` `TestFlickerUnattachesAura` | SILENT: protection from the Equipment ignored; the bearer-departure fold queues the detach as UnattachPending (the live AttachedTo link stays while the bearer is off the battlefield; the bearer's re-entry breaks it before the SBA pass) and attachmentSBAs' pending arm publishes the same events.Unattached ("bearer left the battlefield") |
| 704.5p | not-impl | - | - |  |
| 704.5q | follows | `engine/rules/sba.go#Engine.annihilateOppositeCounters` | `TestCR704RepoCreatureOppositeCountersAnnihilate` | ordering vs deaths: see 704.3/704.8 |
| 704.5r | not-impl | - | - |  |
| 704.5s | follows | `engine/rules/saga.go#Engine.checkSagas` `engine/rules/saga.go#Engine.isChapterAbilityObject` | `TestSagaConclusionIsASacrifice` `TestSagaThatCantBeSacrificedStays` `TestSagaConclusionWaitsOnlyForChapterAbilities` |  |
| 704.5t | follows | `engine/rules/dungeon_room.go#Engine.dungeonCompletion` | `TestDungeonRoomAbilitiesTriggerResolveAndComplete` `TestCompletedDungeonIsNotReentered` |  |
| 704.5u | not-impl | - | - |  |
| 704.5v-704.5w | deviates | `engine/rules/sba.go#Engine.battleZeroDefense` | `TestDefeatedBattleIsExiledNotGraveyarded` `TestDefeatedBattleOffersTransformedCast` | battle exiled inside the SBA, not graveyarded |
| 704.5x-704.5y | partial | `engine/rules/replacement_etb.go#Engine.rechooseDepartedBattleProtector` `engine/rules/replacement_etb.go#Engine.protectorOpponents` | `TestSiegeBattleProtectorIsRechosenWhenProtectorLeaves` `TestNonSiegeBattleProtectorIsRechosenWhenProtectorLeaves` | protector re-derived (first opponent) without a choice |
| 704.5z | follows | `engine/rules/attach.go#Engine.roleSBA` | `TestRoleExclusivityOldRoleGoesToGraveyard` `TestRoleExclusivityDifferentKindSweepsOldRole` `TestRoleExclusivityPerController` `TestRoleExclusivityIsStateBased` `TestRoleExclusivityControlChange` | grouped per bearer by current controller; newest LayerTimestamp survives; plain graveyard move, not destruction |
| 704.6 | follows | `engine/rules/sba.go#Engine.checkLoseConditions` | - | index rule |
| 704.6a-704.6b | n/a | - | - | Two-Headed Giant not modelled |
| 704.6c | follows | `engine/rules/sba.go#Engine.checkLoseConditions` `engine/rules/combat.go#Engine.tallyCmdDamage` | `TestTwentyOneCommanderDamageLoses` `TestTwentyCommanderDamageDoesNotLose` `TestNonCombatCommanderDamageDoesNotTally` |  |
| 704.6d | follows | `engine/rules/replacement_cmdzone.go#Engine.commanderZoneSBA` `engine/rules/sba.go` | `TestCR903_9aDyingCommanderFiresDiesTriggersBeforeTheCommandZoneChoice` `TestCR903_9aDepartedOwnersAskDoesNotBlockLaterCommanders` |  |
| 704.6e | n/a | - | - | Archenemy not modelled |
| 704.6f | not-impl | - | - |  |
| 704.7 | partial | `engine/rules/cantlose.go#Engine.gameLossPrevented` `engine/rules/sba.go#Engine.checkLoseConditions` | - | CantHappen losses only; ReplaceWith GameLoss (Lich's Mirror) family out of scope, losses proposed per cause |
| 704.8 | deviates | `engine/rules/sba.go#Engine.checkStateBased` `engine/rules/sba.go#Engine.annihilateOppositeCounters` | `TestSBABatchUsesPreDepartureBoard` `TestSBABatchLookBackEndsBeforeNextPass` | LKI per family batch |
| 705.1 | follows | `engine/effects/flipcoin.go#FlipCoinWin` `engine/effects/flipcoin.go#effFlipCoin` | `TestFlipCoinFlippedHeadsTailsResolve` | seeded engine Rand |
| 705.2 | follows | `engine/effects/flipcoin.go#effFlipCoin` `engine/effects/flipcoin.go#FlipCoinNote` | `TestFlipCoinFlippedHeadsTailsResolve` `TestFlipCoinMemoryFlagsAreRead` |  |
| 705.3 | follows | `engine/effects/flipcoin.go#FlipCoinForced` | `TestFlipCoinModFirstBatchForcedThenOrdinary` `TestFlipCoinModScopesToController` |  |
| 706.1-706.1b | follows | `engine/effects/dice.go#effRollDice` | `TestNeverwinterHydraPublishesTheTotalOfXRolls` |  |
| 706.2 | follows | `engine/effects/dice.go#DieRollNote` `engine/effects/dice.go#DieRollResult` | `TestTriggerCountResultReadsTheDieRoll` |  |
| 706.2a-706.2b | unknown | - | - |  |
| 706.3-706.3c | partial | `engine/effects/dice.go#parseDieRanges` `engine/effects/dice.go#effRollDice` | `TestBoomflingerPublishesTheDifferenceBetweenRolls` | ResultSubAbilities$ results tables |
| 706.4 | follows | `engine/effects/dice.go#effRollDice` | `TestIronMastiffUsesOnlyItsHighestRoll` |  |
| 706.5 | unknown | - | - |  |
| 706.6 | partial | `engine/effects/dice.go#retainedDice` | `TestBerserkersFrenzyIgnoresItsLowerRoll` | IgnoreLower$ drops results |
| 706.7 | n/a | - | - | Planechase planar die; Planechase not offered |
| 706.8-706.8c | unknown | - | - |  |
| 707.1 | follows | `engine/effects/clone.go#effClone` `engine/effects/copypermanent.go#effCopyPermanent` `engine/effects/copy.go#effCopySpellAbility` | `TestMirageMirrorBecomesACopyOfTargetCreature` `TestGrowingRanksPopulatesTheCreatureTokenYouControl` | scope: become-a-copy, token copies, spell/ability copies |
| 707.2 | follows | `engine/state/copiable.go#Object.CopiableFace` `engine/events/apply_copy.go#foldClonePermanent` `engine/events/apply_copy.go#copyTokenBasis` | `TestCloneOfAFaceDownCreatureCopiesTheFaceDownValues` `TestTokenCopyOfAFaceDownCreatureIsANamelessTwoTwo` `TestHeraldicBannerTokenCopyAsksItsColour` | a copy of a face-down permanent copies its face-down values (708.2a); a token copy never carries the hidden card |
| 707.2a | follows | `engine/state/object.go#Object.faceSlow` `engine/events/apply_copy.go#foldClonePermanent` | `TestCloneNewNameAndGainThisAbility` | colour and abilities derive from the copied face; abilities are not doubled |
| 707.2b | follows | `engine/events/apply_copy.go#foldClonePermanent` | - | gorge snapshots the source face by value at the fold |
| 707.2c | unknown | - | - | gorge: not located, no static-ability copy effect found |
| 707.3 | follows | `engine/events/apply_copy.go#copyTokenBasis` `engine/events/apply_combat.go#foldMyriadCopy` `engine/events/apply_copy.go#foldClonePermanent` | `TestTokenCopyOfACloneCopiesWhatTheCloneCopies` `TestSixthDoctorNonLegendaryCopyOfCopy` | token, myriad and Clone copies all read the copiable face (the CopyFace basis) |
| 707.4 | follows | `engine/events/apply_copy.go#foldClonePermanent` `engine/state/object.go#Object.SetCopyFace` | `TestMirageMirrorBecomesACopyOfTargetCreature` `TestCloneKayaTargetTokenBecomesCopy` | in-place copy-face swap, no zone change |
| 707.5 | follows | `engine/effects/clone.go#effClone` `engine/rules/replacement.go#Engine.runReplaceWith` `engine/rules/cast_etbchoice.go#Engine.entryETBChoice` | `TestTokenCopyOfDiregrafGhoulEntersTapped` `TestFleshDuplicateETBGrantedVanishing` | copied enters-with/as-enters replacements and ETB triggers apply |
| 707.6 | follows | `engine/rules/cast_etbchoice.go#Engine.entryETBChoice` | `TestHeraldicBannerTokenCopyAsksItsColour` | the copy's controller makes its own as-enters choice |
| 707.7 | partial | `engine/state/object.go#Object.ExiledCards` `engine/state/object.go#Object.Imprinted` | - | gorge links are per-host-object lists (host-object model), not per ability pair |
| 707.8 | follows | `engine/events/apply_copy.go#foldClonePermanent` | - | copies the face currently up (FaceIdx) |
| 707.8a | follows | `engine/events/apply_copy.go#foldCopyToken` | - | token copy keeps the whole card and its face index |
| 707.9 | follows | `engine/effects/clone_params.go#compileClone` `engine/effects/copypermanent_params.go` | `TestCompileClone` | copy effects carry exception parameters |
| 707.9a-b | partial | `engine/events/copy_except.go#foldCopyExcept` `engine/effects/copy_except.go#copyExceptions` `engine/effects/copypermanent.go#effCopyPermanent` `engine/effects/clone.go#effClone` `engine/effects/clone_params.go#cloneETBShapeOK` | `TestCloneOfAnEternalizedTokenCopiesTheExceptions` `TestTokenCopyOfAnEternalizedTokenKeepsTheExceptions` `TestCloneOfAPhantasmalImageKeepsItsExceptions` `TestQuicksilverGargantuanEntersAsA77Copy` `TestVesuvanDoppelgangerEntersBlueWithItsUpkeepCopy` `TestAutonSoldierCopyIsNotLegendary` | type, colour, P/T, mana-cost, keyword and AddSVars/AddTriggers exceptions are copiable values; still layer effects: Changeling, AddAbilities$, RemoveKeywords$, CopyPermanent grants; the ETB copy route offers SetPower/SetToughness (literal), SetColor/AddColors and NonLegendary bodies |
| 707.9c-d | follows | `engine/events/copy_except.go#foldCopyExcept` `engine/effects/clone.go#effClone` `engine/effects/clone.go#clonePT` | `TestCloneChangelingExceptionIsAllCreatureTypesUntilStripped` `TestCloneChangelingCDAAfterEarlierStrip` | a P/T or colour exception drops the copied CDA; a Changeling exception stays a CDA marker seeded ahead of layer 4 |
| 707.9e | partial | `engine/effects/clone_etb_sub.go#CloneETBNoCopy` `engine/rules/replacement.go#Engine.runReplaceWith` | `TestMalleableImpostorCopy` `TestFleshDuplicateETBDeclineEntersAsItself` | rider skipped on decline; no check for a later copy effect (Altered Ego copying Clone) |
| 707.9f | partial | `engine/effects/clone_ifnew.go#cloneIfNewPayload` `engine/effects/clone_etb_sub.go#CloneETBSubChainOK` | `TestFleshDuplicateConditionalSilentOnVanishingTemplate` | Flesh Duplicate conditional rider real; Moritte's if-creature exception fails closed out of the election |
| 707.9g | unknown | - | - | gorge: not located |
| 707.10 | partial | `engine/events/apply_copy.go#foldStackCopy` `engine/rules/emit.go#Engine.emit` `engine/rules/emit.go#inheritCopyRecord` `engine/rules/engine_trigger_maps.go#engineTriggerMaps.castSubTargets` | `TestCopiedSpellIsNotWasCastFromGraveyard` `TestArcLightningCopyInheritsDivision` `TestCopyOfFusedSpellAsksPerDeclaration` `TestCR707_10CopiedFlingUsesTheOriginalSacrifice` `TestCR707_10CopiedCorpseLungeUsesTheOriginalExile` `TestCostPaidListsAreInheritedByAStackCopy` `TestCopiedSpellSpellingsAgree` | a copy inherits the original's cost-paid objects (sacrificedLKI, castExiled, castRevealed, castCounterPut; cr-g60); it does not inherit castSubTargets, so it asks the cast's pre-asked SubAbility$ targets at resolution |
| 707.10a | follows | `engine/state/object.go#Object.Ephemeral` `engine/effects/filter.go#matchesObjectPtr` | `TestCopiedInstantSpellCeasesWhenItLeavesTheStack` `TestZoneListsSkipEphemeralObjects` | off-stack copies are ephemeral and match nothing; code cites old number 707.10h |
| 707.10b | follows | `engine/events/apply_copy.go#foldStackCopy` `engine/rules/target_ask.go#Engine.resolvedAbilityTallyFor` | `TestUFActivationCopyResolvesTheAbility` `TestEachTriggerCopiesItsOwnActivation` | copy keeps Source and Ability |
| 707.10c | follows | `engine/rules/target_modes.go#Engine.AskCopyTargets` `engine/effects/copy.go#effCopySpellAbility` | `TestChainLightningMayChooseTargetUsesTheRealCorpus` `TestCopyOfTwoTargetSpellAsksForBothTargets` | inherited targets offered first even if illegal; new ones from the legal census |
| 707.10d | deviates | `engine/effects/copy.go#effCopySpellAbility` | - | CopyForEachCanTarget$ unread: one copy keeping the original targets (SILENT) |
| 707.10e | partial | `engine/effects/copy.go#copyDefinedTargets` | `TestFeatherChosenCopyAskPricedAndTargets` | DefinedTarget$ ChosenCard/Self only; other specs keep targets with a Note (LOUD) |
| 707.10f | follows | `engine/events/apply_zone.go#move` | `TestCopiedPermanentSpellEntersAsToken` | stack copy entering the battlefield becomes a token |
| 707.10g | follows | `engine/events/apply_copy.go#foldStackCopy` | - | spell copy carries the whole card and face index |
| 707.11 | follows | `engine/effects/context.go#DefinedSpec` | - | self references resolve by object id, not name |
| 707.12,707.12a | partial | `engine/rules/cast_begin.go#Engine.beginPlay` `engine/effects/play.go#effPlay` | `TestPlayCopyCardCastsCopyAndLeavesOriginalInExileKernel` `TestPrepareStubBackPreparedCopyCastable` | copy minted in a library holding zone, not the original's zone |
| 707.13 | not-impl | `engine/effects/play.go#effPlay` | - | Garth One-Eye's Play CopyFromChosenName$ has no population and returns: SILENT |
| 707.14 | unknown | `engine/effects/play.go#effPlay` | `TestMagarReturnsASpellAsA33FaceDownCreature` | shape: face-down 3/3 tested; casting the CopyCard copy untested, ZoneRegardless$ unread |
| 708.1 | follows | `engine/rules/chars/derive.go#Compute` `engine/rules/layers_types.go#Engine.faceDownPrintedHides` | `TestManifestedLandIsA22Creature` |  |
| 708.2-708.2a | follows | `engine/rules/chars/derive.go#PT` `engine/rules/chars/types.go#TypesAndAllCreatureTypes` `engine/state/mana_value.go#Object.FaceDownHidden` `engine/effects/colors.go#ColorMaskOf` `engine/effects/filter.go#nameCharacteristics` | `TestManifestedLandIsA22Creature` `TestFaceDownHidesPrintedSubtypes` `TestManaValueFaceDownPermanentIsZero` `TestFaceDownPermanentIsColourless` `TestFaceDownPermanentHasNoName` |  |
| 708.2b | follows | `engine/effects/resolve.go#effSetState` `engine/events/apply_copy.go#foldTurnFaceDown` | - | already face down: no-op |
| 708.3 | follows | `engine/events/facedown.go#IsFaceDownEntry` `engine/rules/layers_types.go#Engine.faceDownPrintedHides` | `TestEntryCounterGrantsFaceDownGrantsNothing` `TestFaceDownAquamorphEntityEntersWithoutItsModeChoice` | face-down entry runs no ETB replacement or trigger |
| 708.4 | partial | `engine/rules/chars/derive.go#Compute` `engine/rules/layers_types.go#Engine.faceDownPrintedHides` `engine/rules/legal_walk_hand.go#legalWalk.handWalk` `engine/state/mana_value.go#Object.FaceDownHidden` | - | face-down spell: name, MV, colour reads hide on the stack too (untested); types, P/T, abilities still printed |
| 708.5 | follows | `engine/view/card.go#projector.zoneCard` | `TestControllerSeesControlledFaceDownFace` `TestFaceDownStackCopyIsRedactedFromOpponents` | controller-only identity |
| 708.6 | n/a | - | - | physical bookkeeping; object IDs distinguish |
| 708.7 | partial | `engine/rules/morph_turnup.go#morphFaceUpCost` `engine/rules/morph_turnup.go#Engine.turnFaceUp` | `TestMorphTurnFaceUpIsASpecialActionPaysItsKeywordCost` `TestMegamorphTurnFaceUpAddsAPlusOnePlusOneCounter` | SILENT: only morph-family face-down casts turn up; manifested/cloaked morph never does |
| 708.8 | follows | `engine/events/apply_copy.go#foldTurnFaceUp` | `TestDisguiseTurnFaceUpEndsTheWardTwo` `TestMasterOfPearlsTurnFaceUpSelfTriggerFiresOnce` | same object, no ETB |
| 708.9 | follows | `engine/events/apply_zone.go#foldMoveZone` | `TestManifestedCardRevealsWhenItLeaves` `TestCloakedLeavesBattlefieldReveals` | permanents; spell case unverified |
| 708.10 | follows | `engine/rules/chars/derive.go#Compute` `engine/state/object.go#Object.faceSlow` | - | code-read, untested |
| 708.11 | follows | `engine/rules/replacement_phase.go#Engine.applyTurnFaceUpReplacements` | `TestMorphTurnFaceUpWaitsForSacrificeReplacementOrderChoice` `TestHoodedHydraTurnFaceUpReplacementPutsFiveCounters` | - |
| 708.12 | unknown | - | - | not located: no reveal-face-down-permanent path found |
| 709.1-709.2 | follows | `engine/cards/parse.go#ParseBytes` | - | one object, two Faces |
| 709.3-709.3b | follows | `engine/rules/cast_begin.go#Engine.beginCastWith` `engine/rules/legal_walk_hand.go#legalWalk.handWalk` | `TestSplitCardEachHalfIsSeparatelyCastable` `TestSplitHalfWithNoLegalTargetIsWithheld` `TestNonFuseSplitOffersBothHalvesButNoFusedCast` |  |
| 709.3c | follows | `engine/events/apply_copy.go#foldStackCopy` `engine/rules/cast_begin.go#Engine.beginPlay` | - | untested for split |
| 709.4 | follows | `engine/state/mana_value.go#Object.SplitHalves` `engine/effects/colors.go#ColorMaskOf` `engine/effects/filter_type.go#hasTypePrinted` | `TestManaValueSplitCardCombinesHalvesOffTheStack` `TestSplitCardColoursCombineOffTheStack` `TestSplitCardTypesCombineOffTheStack` |  |
| 709.4a | follows | `engine/effects/filter.go#nameCharacteristics` | `TestNamedSplitCardBothNamesOffBattlefield` |  |
| 709.4b-709.4c | partial | `engine/state/mana_value.go#manaValueAs` `engine/effects/colors.go#ColorMaskOf` `engine/effects/filter_type.go#hasTypePrinted` `engine/effects/colors.go#splitReadAsSpell` | `TestManaValueSplitCardCombinesHalvesOffTheStack` `TestDarkConfidantLosesSplitCardsCombinedManaValue` `TestSplitCardColoursCombineOffTheStack` `TestSplitCardTypesCombineOffTheStack` `TestColourCostModifierReadsTheHalfBeingCast` | MV, colours, types combined; both halves' abilities off the stack unverified |
| 709.4d | follows | `engine/rules/split.go#Engine.resolveFused` `engine/state/mana_value.go#Object.SplitHalves` | `TestSplitFusePaysCombinedCostAndResolvesBothHalves` `TestManaValueSplitCardCombinesHalvesOffTheStack` `TestSplitCardColoursCombineOffTheStack` |  |
| 709.5-709.5b | follows | `engine/rules/chars/room_name.go#RoomName` `engine/state/mana_value.go#roomManaValue` | `TestRoomCharacteristicsFollowDoors` `TestRoomNameByDoorState` `TestOracleRoomPlacedBothLockedHasNoName` | Room characteristics follow doors; 709.5b copy case unverified |
| 709.5c-709.5g | follows | `engine/state/object.go#Object.DoorUnlocked` `engine/rules/rooms.go#Engine.unlockRoomFaceCost` `engine/effects/unlockdoor.go#effUnlockDoor` | `TestPaidRoomUnlockPaysAndUnlocksSelectedDoor` `TestNonCastRoomHasNoLiveDoorsAndCastEntryDesignatesFace` `TestRoomRelockCastFaceThenUnlock` | designations, sorcery-timed unlock special action, lock via DoorLock |
| 709.5h-709.5i | follows | `engine/rules/rooms.go#Engine.checkUnlockTriggers` `engine/rules/trigmatch/room.go#fullyUnlockMatches` | `TestFearOfSleepParalysisFullyUnlockTapsOnceOnFinalDoor` `TestRoomRepeatFaceUnlockDoesNotQueueUnlockDoorTrigger` | - |
| 709.5j | follows | `engine/effects/unlockdoor.go#countUnlockedDoors` | `TestCountUnlockedDoorsCountsFacesNotRooms` | door is a half |
| 710.1-710.1c | follows | `engine/effects/resolve.go#effSetState` `engine/events/apply_copy.go#foldFlipFace` | `TestFlipFaceChangesActiveFace` | flipped face repeats ManaCost in the card data |
| 710.2 | deviates | `engine/events/apply_zone.go#move` | - | flipped card never resets on leaving battlefield |
| 710.3 | n/a | - | - | physical |
| 710.4 | deviates | `engine/effects/resolve.go#effSetState` | - | flip is (FaceIdx+1)%2: a second flip unflips |
| 710.5 | follows | `engine/cards/universe.go#Universe.Names` `engine/effects/cardflow.go#effNameCard` | `TestCorpusNameUniverseHasEveryNameableFace` | flipped name offered |
| 711.1-711.6 | follows | `engine/cards/kw_levelup.go#kwLevelUp` | `TestCoralhelmCommanderLevelsIntoThe23Band` `TestLevelUpKeywordIsSupported` | sorcery-speed level up; bands are ordinary statics |
| 711.7 | follows | `engine/state/object.go#Object.ClassLevel` | `TestClassLevelDesignationIsNotACounter` | designation, not counters |
| 712.1-712.3c | n/a | - | - | definitional/physical |
| 712.4-712.4b | follows | `engine/events/apply_meld.go#foldMeld` `engine/effects/meld.go#Meld` | `TestMeldGiselaBrunaBrisela` `TestMeldRefusesANonPair` |  |
| 712.4c | deviates | `engine/effects/resolve.go#effSetState` | - | result-carrying meld card can be transformed |
| 712.5-712.5g | follows | `engine/effects/meld.go#meldRoles` `engine/state/meld.go#MeldResultFace` | `TestMeldCorpusCarriers` `TestMeldVanilleFangRagnarok` `TestMeldAPIDispatchHanweir` | pairs from AlternateMode:Meld data |
| 712.6-712.7 | n/a | - | - | physical |
| 712.8 | follows | `engine/state/object.go#Object.Face` `engine/cards/parse.go#ParseBytes` | - | one Face per side |
| 712.8a | partial | `engine/events/apply_zone.go#move` | `TestTransformDFCLeavesBattlefieldFrontFace` `TestCR712ModalDFCLandBackResetsToFrontOnExit` | reset only on leaving battlefield; back-face spell leaving stack keeps back face |
| 712.8b | follows | `engine/events/apply_meld.go#unmeld` | - | - |
| 712.8c | follows | `engine/rules/cast_begin.go#Engine.beginCastWith` | `TestDefeatedBattleOffersTransformedCast` | defeat cast flips before the stack |
| 712.8d,f | follows | `engine/state/object.go#Object.Face` `engine/state/object.go#Object.faceSlow` | `TestManaValueModalDFCBackFaceIsItsOwn` | Face()=Faces[FaceIdx] |
| 712.8e | follows | `engine/state/mana_value.go#manaValueAs` | `TestManaValueTransformedBackFaceIsFrontFace` `TestSephirothTransformsOnlyOnFourthResolution` |  |
| 712.8g | follows | `engine/events/apply_meld.go#foldMeld` `engine/state/mana_value.go#manaValueAs` | `TestManaValueMeldedPermanentSumsFrontFaces` |  |
| 712.9-712.10 | deviates | `engine/effects/resolve.go#effSetState` `engine/effects/zone_change.go#applyTransformed` | - | any 2-face card transforms; no DFC, face-down or instant-face gate |
| 712.11-712.11a | follows | `engine/rules/cast_begin.go#Engine.beginCastWith` | `TestDefeatedBattleOffersTransformedCast` | front face by default; transformed cast puts back face up |
| 712.11b-712.11c | follows | `engine/rules/legal_walk_hand.go#legalWalk.handWalk` | `TestModalSpellBackFaceRestrictionWithholdsOffer` `TestModalSpellOfferSurvivesFrontFaceRestriction` | - |
| 712.11d | not-impl | - | - | LOUD: Disturb and More Than Meets the Eye keywords unregistered |
| 712.12 | follows | `engine/rules/legal_walk_hand.go#legalWalk.handWalk` | `TestCR712ModalDFCLandBackIsOfferedPlayedAndReplays` `TestCR712ModalDFCLandLandOffersBothFaces` | - |
| 712.13 | follows | `engine/rules/resolution_chain.go#Engine.moveResolvedOffStack` | - | face carried stack to battlefield |
| 712.13a | unknown | - | - | not located |
| 712.14 | partial | `engine/events/apply_zone.go#move` | - | stuck-back-face card (712.8a gap) re-enters back face up |
| 712.14a | partial | `engine/effects/zone_change.go#applyTransformed` | `TestChangeZoneTransformedEntersFlipped` `TestCraftEntersTransformed` | non-DFC "transformed" still enters, or flips if 2-faced |
| 712.14b | unknown | - | - | not located: no nonpermanent-face gate found on battlefield entry |
| 712.14c | follows | `engine/events/apply_meld.go#foldMeld` | `TestMeldGiselaBrunaBrisela` `TestMeldEntersTappedAndAttacking` | - |
| 712.15-712.15a | partial | `engine/events/apply_copy.go#foldTurnFaceUp` `engine/effects/resolve.go#effSetState` | - | turn-up keeps front; transform of face-down not blocked |
| 712.16 | deviates | `engine/effects/resolve.go#effSetState` `engine/events/apply_copy.go#foldTurnFaceDown` | - | DFC can be turned face down |
| 712.17 | n/a | - | - | physical |
| 712.18 | follows | `engine/events/apply_copy.go#foldFlipFace` | `TestIncubatorTokenTransformsIntoThePhyrexianFace` | FaceIdx change only, same object |
| 712.19 | follows | `engine/cards/universe.go#Universe.Names` `engine/cards/universe.go#Universe.FaceByName` `engine/effects/cardflow.go#effNameCard` | `TestCorpusNameUniverseHasEveryNameableFace` `TestCloneKillerCosplayCopiesANamedBackFace` | back-face and combined meld names offered; a chosen back-face name copies that face |
| 712.20 | follows | `engine/rules/replacement_phase.go#Engine.applyTransformReplacement` | `TestSephirothTransformRunsTheDestinationFaceReplacement` | destination-face as-transforms replacement applied during the flip |
| 712.21 | follows | `engine/events/apply_meld.go#unmeld` | `TestMeldTwoOwners` `TestMeldFoldPairsAndSplits` |  |
| 712.21a-712.21b | partial | `engine/events/apply_meld.go#unmeld` | - | fixed order result then partner; no arrange or timestamp choice |
| 712.21c | deviates | `engine/events/apply_meld.go#unmeld` | - | code-read: effects follow only the result object; partner card untracked after split |
| 712.21d | follows | `engine/events/apply_meld.go#unmeld` | - | code-read: partner follows the replaced destination; commander exemption untested |
| 712.21e | unknown | - | - | not located |
| 713.1-713.5 | n/a | - | - | physical substitute cards |
| 714.1-714.1a | follows | `engine/rules/saga.go#Engine.sagaChapters` `engine/cards/saga.go#SagaChapters` | `TestUrzasSagaChaptersAndLoreCounters` |  |
| 714.2-714.2c | follows | `engine/rules/saga.go#Engine.checkChapterTriggers` | `TestDoublingSeasonSagaEntryAndProgressionCounts` `TestUrzasSagaChaptersAndLoreCounters` | every crossed chapter queued |
| 714.2d-714.2e | follows | `engine/rules/saga.go#Engine.sagaChapters` | `TestSagaThatLostItsAbilitiesHasNoChapters` `TestBloodMoonUrzasSagaKeepsSagaSubtype` | REVISED: cr-g07 landed; chapters read current abilities, ability loss gives 0 |
| 714.3-714.3a | follows | `engine/events/entry_counters.go#EntryCounterGrants` | `TestEntryCounterSagaLoreOrderIsDecidedBeforeEntry` `TestSagaEntryLoreReplacementChaptersExactlyOnce` | entry lore counter as replacement |
| 714.3b | not-impl | `engine/cards/mayask.go#entryAskKeywords` | - | LOUD: kw Read ahead unregistered (10 corpus cards) |
| 714.3c | follows | `engine/rules/saga.go#Engine.advanceSagas` `engine/rules/turn.go#Engine.finishEnteredStep` | `TestOracleSetupSagaTakesMain1LoreCounter` | gate now current chapter abilities (cr-g07) |
| 714.4 | follows | `engine/rules/saga.go#Engine.checkSagas` | `TestSagaConclusionIsASacrifice` `TestSagaConclusionWaitsOnlyForChapterAbilities` `TestSagaThatCantBeSacrificedStays` | REVISED: cr-g07 landed; sacrificed, waits only on chapter abilities |
| 715.1-715.2 | follows | `engine/rules/adventure.go#adventureSpellFace` `engine/rules/adventure.go#isAdventureSpellFace` | `TestAdventureSpellFaceOfferedCastAndRecast` |  |
| 715.2a | follows | `engine/effects/filter_word.go#wordMatches` | `TestAdventureCardPredicate` | AdventureCard predicate reads the card, not the face up |
| 715.2b-715.2c | follows | `engine/state/object.go#Object.Card` `engine/events/apply_copy.go#foldStackCopy` | - | code-read: one object |
| 715.3-715.3b | follows | `engine/rules/cast_begin.go#Engine.beginCastWith` `engine/rules/adventure.go#adventureSpellFace` | `TestAdventureSpellFaceOfferedCastAndRecast` `TestAdventureFaceOfferPricesTheAdventureFace` `TestOracleAdventureSpellStackLabelIsAdventureFace` | - |
| 715.3c | follows | `engine/events/apply_copy.go#foldStackCopy` | - | code-read: copy keeps FaceIdx and FlagAdventure; untested |
| 715.3d | follows | `engine/rules/stack.go#spellRestZone` `engine/rules/legal_walk_alt.go#legalWalk.exileCastsWalk` | `TestAdventureSpellCounteredGoesToGraveyard` | controller vs owner unverified |
| 715.4 | follows | `engine/events/apply_zone.go#move` | `TestAdventureCardIsMainFaceOffTheStack` | - |
| 715.5 | follows | `engine/cards/universe.go#Universe.Names` `engine/effects/cardflow.go#effNameCard` | `TestCorpusNameUniverseHasEveryNameableFace` `TestNameChoicesOfferEveryNameableFace` | Adventure name offered |
| 716.1 | n/a | - | - | definitional |
| 716.2-716.2a | follows | `engine/cards/kw_class.go#kwClass` `engine/effects/class_level.go#effClassLevelUp` | `TestClassLevelActivatorIsExactNMinusOne` `TestPaladinClassBandUpIsSorcerySpeed` | exactly N-1, sorcery speed |
| 716.2b | follows | `engine/state/object.go#Object.ClassLevelValue` | - | code-read: object field outside copiable values; untested |
| 716.2c | follows | `engine/cards/kw_class.go#kwClass` | `TestCaretakersTalentLevelGainedTriggerFiresOnTheLevelUp` | - |
| 716.2d | follows | `engine/state/object.go#Object.ClassLevel` | `TestPaladinClassEntersAtLevel1LevelsUpAndTurnsOnItsStatic` | no level reads as 1 |
| 716.3 | follows | `engine/cards/kw_class.go#kwClass` | `TestClassGrantKeepsTheBodysOwnIsPresent` | - |
| 716.4 | follows | `engine/effects/filter_value.go#numericPred` | `TestClassLevelDesignationIsNotACounter` | classLevel_ reads designation, not counters |
| 717.1-717.6a | not-impl | `engine/rules/trigmatch/actions.go#rolledDieCommon` | - | LOUD: no Attraction deck |
| 718.1-718.5 | follows | `engine/state/prototype.go#PrototypeParams` `engine/state/mana_value.go#ManaValue` `engine/effects/colors.go#ColorMaskOf` `engine/rules/chars/derive.go#PT` `engine/events/apply_copy.go#foldCopyToken` | `TestPrototypeCharacteristicsOnStackAndBattlefield` `TestPrototypeNormalElsewhere` `TestPrototypeCopyIsPrototyped` | The inset values are derived from the face's K:Prototype param; the designation is a CastFlag outside the copiable values but copied by CR 718.3c/d |
| 719.1-719.2 | n/a | - | - |  |
| 719.3-719.3a | follows | `engine/effects/misc.go#effAlterAttribute` `engine/effects/filter.go#predicates` | `TestCaseBecomesSolvedAtEndStepAndAuditorTriggers` |  |
| 719.3b | follows | `engine/state/object.go#Object.Solved` `engine/events/apply_player.go#foldAlterAttribute` | `TestCaseSolvedClearsWhenTheCaseLeaves` | stays solved |
| 719.3c | follows | `engine/effects/filter.go#predicates` `engine/rules/legal_activation.go#Engine.activationConditionOK` | `TestCaseSolvedCorpusCensus` | IsSolved gates Solved statics/triggers |
| 720.1 | n/a | - | - |  |
| 720.2-720.2b,720.3-720.3b | follows | `engine/rules/adventure.go#omenSpellFace` `engine/rules/legal_walk_hand.go#legalWalk.appendOmenAlt` | `TestOmenFaceOfferedFromHandAndShuffledOnResolve` `TestOmenUsesOmenTimingAndCost` | hand-only Omen cast, as 715; other-zone permissions out of scope |
| 720.3c | follows | `engine/rules/stack.go#spellRestZone` `engine/state/object.go#CastProvenanceFlags` | - | code-read: FlagOmen is NOT a provenance bit, so a copy inherits it; the IsCopy arm still exiles |
| 720.3d | follows | `engine/rules/stack.go#spellRestZone` `engine/rules/resolution_chain.go#Engine.moveResolvedOffStack` | `TestOmenFaceOfferedFromHandAndShuffledOnResolve` `TestOmenCounteredGoesToGraveyard` | shuffle only on resolution; countered Omen goes to the graveyard |
| 720.2c,720.4 | follows | `engine/cards/ir.go#Card` `engine/events/apply_zone.go#move` | `TestOmenCardIsMainFaceOffTheStack` | face reset on every non-battlefield stack exit |
| 720.5 | follows | `engine/cards/universe.go#Universe.Names` | `TestCorpusNameUniverseHasEveryNameableFace` | Omen name offered |
| 721.1,721.3 | n/a | - | - |  |
| 721.2-721.2b | follows | `engine/effects/filter_value.go#numericPred` | `TestEveryStationCarrierStationsThroughTheStack` |  |
| 721.2c | deviates | `engine/rules/chars/derive.go#PT` | - | REVISED: SILENT |
| 721.4 | follows | `engine/rules/station.go#Engine.askStation` `engine/rules/station.go#Engine.handleStation` `engine/cards/kw_station.go#KeywordPushAbility` | `TestStationUsesTheStack` `TestStationNegativeGates` |  |
| 722.1 | n/a | - | - |  |
| 722.2-722.2c | follows | `engine/state/object.go#Object.HasPrepareSpell` | `TestPrepareCensusMatchesCorpus` `TestPrepareBackFaceHelperCensus` | inset face held on Card |
| 722.3 | follows | `engine/rules/legal.go#modalSpellBack` | - |  |
| 722.3a-722.3b | follows | `engine/events/apply_player.go#foldAlterAttribute` `engine/effects/misc.go#effAlterAttribute` | `TestPreparedUpkeepGateIsTrueOnlyWhileUnprepared` `TestPrepareCensusCountsPreparedAttributeCarriers` | grant needs a prepare spell, no double grant |
| 722.3c | partial | `engine/events/apply_helpers.go#grantPreparedCopy` `engine/rules/legal_walk_alt.go#legalWalk.exileCastsWalk` `engine/rules/cast_targets.go#Engine.pushCast` | `TestPrepareStubBackPreparedCopyCastable` `TestPreparedExileCopyCeasesAfterCast` `TestPreparedExileCopyCeasesWhenSourceLeavesBattlefield` | REVISED: exile copy minted on grant only |
| 722.3d | partial | `engine/state/object.go#CastProvenanceFlags` `engine/effects/filter.go#predicates` | - |  |
| 722.4 | follows | `engine/events/apply_helpers.go#grantPreparedCopy` | - | card always uses its main face |
| 722.5 | follows | `engine/cards/universe.go#Universe.Names` | `TestCorpusNameUniverseHasEveryNameableFace` | prepare name offered |
| 723.1 | follows | `engine/effects/choose_control.go#effControlPlayer` `engine/state/game.go#Game.LiveController` `engine/rules/engine_host.go#Engine.controlPlayerRedirect` `engine/rules/turn.go#Engine.expirePlayerControl` | `TestControlPlayerGrantRedirectsThenExpires` `TestControlPlayerRedirectDoesNotSpillBeforeControlledTurn` `TestMindslaverControlPlayerPrintedAbility` `TestControlGrantedDuringTheControlledSeatsTurnWaitsForItsNextTurn` | live only on the controlled seat's next turn |
| 723.1a | follows | `engine/events/apply_player.go#foldControlPlayerChange` | `TestControlPlayerChangeFoldsGrantExpiryAndSelf` |  |
| 723.1b | follows | `engine/rules/turn.go#Engine.expirePlayerControl` `engine/rules/engine_host.go#Engine.controlPlayerRedirect` | `TestControlPlayerRedirectDoesNotSpillBeforeControlledTurn` |  |
| 723.2 | partial | `engine/rules/engine_host.go#Engine.searchControlRedirect` | - | Opposition Agent redirects only search picks |
| 723.3 | follows | `engine/decision/decision.go#Decision.RedirectTo` `engine/rules/acting_view.go#actingView` | `TestControlledTurnActionsActForTheControlledSeat` | only the answering seat changes |
| 723.4 | partial | `engine/state/game.go#Game.LiveController` `engine/view/redact.go#controlsSeat` `server/host/match.go#controlledSeats` | `TestControllerSeesControlledHandNotThirdSeat` `TestControllerSeesControlledFaceDownFace` `TestMindslaverControlHidesTheHandUntilTheControlledTurn` `TestProjectNextHidesAnArmedControlBeforeTheControlledTurn` `TestControlledSeatsIsTheLiveActiveSeatOnly` | the widening is live only during the controlled turn (cr-g57); a BoardSeat bot's Board is not widened |
| 723.5-723.5a | follows | `engine/rules/engine_host.go#Engine.controlPlayerRedirect` `engine/rules/acting_view.go#actingView` | `TestControlledTurnOfferedOptionsPassTheSubmitGuard` `TestControlledTurnPaymentPlansUseTheControlledSeatsMana` `TestControlledTurnPonderArrangesAndShufflesTheControlledLibrary` | every decision redirected |
| 723.5b | n/a | - | - | out-of-game and tournament choices; the engine poses only rules decisions |
| 723.6 | partial | `engine/rules/acting_view.go#actingView` | - | REVISED: controller's concede concedes itself |
| 723.7 | partial | `engine/effects/effect.go#effEffect` | - | Word of Command restriction (Effect-granted CantBeActivated) only noted, not enforced |
| 723.8 | follows | `engine/rules/engine_host.go#Engine.controlPlayerRedirect` | `TestControlRedirectCensus` |  |
| 723.9 | deviates | `engine/effects/choose_control.go#effControlPlayer` `engine/events/apply_player.go#foldControlPlayerChange` | `TestControlPlayerEffectRejectsSelfTarget` `TestControlPlayerChangeFoldsGrantExpiryAndSelf` |  |
| 724.1 | follows | `engine/effects/endturn.go#effEndTurn` `engine/rules/turn.go#Engine.finishEndTurn` | `TestTimeStopExilesStackSkipsToCleanupAndReplays` `TestEndTurnObekaActivePlayerMayDeclineOrAccept` |  |
| 724.1a,724.1f | follows | `engine/rules/emit.go#Engine.emit` `engine/rules/turn.go#Engine.checkEndTurnExileTriggers` `engine/rules/turn.go#Engine.finishEndTurn` | `TestCR724_1fTriggerDuringEndTurnResolves` `TestCR724_1aTriggerBeforeEndTurnCeases` | - |
| 724.1b | follows | `engine/events/apply_turn.go#foldEndTurn` `engine/effects/endturn.go#effEndTurn` `engine/rules/turn.go#Engine.checkEndTurnExileTriggers` | `TestTimeStopExilesStackSkipsToCleanupAndReplays` `TestCR724_1fTriggerDuringEndTurnResolves` |  |
| 724.1c | follows | `engine/rules/turn.go#Engine.finishCleanupStep` | `TestTimeStopExilesStackSkipsToCleanupAndReplays` | unobservable with Time Stop |
| 724.1d-724.1e | follows | `engine/rules/turn.go#Engine.finishEndTurn` `engine/events/apply_turn.go#foldEndTurn` | `TestTimeStopExilesStackSkipsToCleanupAndReplays` |  |
| 724.2-724.2g | not-impl | - | - |  |
| 725.1 | follows | `engine/state/game.go#Game.IsMonarch` `engine/effects/game.go#effBecomeMonarch` | `TestCustodiLichFirstBecomeMonarchFiresSelfTrigger` |  |
| 725.2 | deviates | `engine/rules/turn.go#Engine.finishEnteredStep` `engine/rules/combat.go#Engine.runCombatAssignments` | `TestMonarchEndStepDrawUsesTheStack` `TestCombatDamageToMonarchTransfersTheCrown` | end-step draw triggers (borrows an arbitrary Source); combat-damage steal is immediate, not a trigger. SILENT |
| 725.3 | follows | `engine/events/apply_misc.go#foldMonarchChange` `engine/effects/game.go#effBecomeMonarch` | `TestCustodiLichRepeatedBecomeMonarchDoesNotFireSelfTrigger` | single holder field |
| 725.4 | not-impl | `engine/rules/cantlose.go#Engine.playerLoses` | - | SILENT: a departed monarch keeps it; playerLoses hands off only the initiative. multiplayer only |
| 725.5 | not-impl | `engine/rules/static_condition.go#staticCondition` `engine/effects/condition_census.go#conditionUnmodelledBare` | - | LOUD: Condition$ Monarch statics unmodelled (census-listed), fail closed |
| 726.1 | follows | `engine/state/game.go#Game.IsInitiative` `engine/effects/initiative.go#effTakeInitiative` | `TestAvengingHunterETBTakesInitiativeAndVenturesIntoUndercity` |  |
| 726.2 | deviates | `engine/rules/combat.go#Engine.runCombatAssignments` `engine/effects/initiative.go#effTakeInitiative` `engine/rules/turn.go#Engine.finishEnteredStep` | `TestInitiativeHolderVenturesAtTheirUpkeep` `TestCombatDamageToInitiativeHolderMovesItAndVentures` `TestInitiativeSimultaneousHitsAdjudicateByTurnOrder` | upkeep venture triggers; combat take immediate, effect's venture folded into resolution. SILENT |
| 726.3 | follows | `engine/events/apply_misc.go#foldInitiativeChange` | `TestInitiativeSimultaneousSameControllerOneTake` |  |
| 726.4 | follows | `engine/rules/cantlose.go#Engine.initiativeHandoffOnDeparture` | `TestInitiativeHolderLeavesHandsOffToActivePlayer` `TestInitiativeActiveHolderLeavesHandsOffToNextInTurnOrder` `TestInitiativeHolderConcedingHandsOff` | handoff implemented; effects/initiative.go comment saying otherwise is stale |
| 726.5 | follows | `engine/effects/initiative.go#effTakeInitiative` `engine/events/apply_misc.go#foldInitiativeChange` | - | holder retaking ventures again, one designation |
| 727.1-727.7 | follows | `engine/rules/restart.go#Engine.finishRestart` `engine/rules/restart.go#Engine.runRestartRest` `engine/state/restart.go#Game.Restart` | `TestCR727KarnRestartsTheGame` `TestCR727KarnReturnsTheExiledCardsJustBeforeTheFirstUntap` `TestCR727ResetForRestartKeepsOnlyConfig` | 727.6/727.7 have no subject (no subgames, no limited range) |
| 728.1 | partial | `engine/rules/turn.go#Engine.finishEnteredStep` `engine/effects/counters.go#effRadiationDrain` | `TestRadiationDrainIsSourceLessTriggeredAbility` `TestRadiationDrainMillsAndDrains` `TestRadiationDrainIsOnTheStackBeforeResolving` | source-less trigger |
| 728.1a | not-impl | `engine/effects/counters.go#effRadiationDrain` | - | LOUD: stat:GainLifeRadiation unregistered |
| 729.1-729.6 | not-impl | - | - |  |
| 730.1-730.2c | follows | `engine/events/apply_copy.go#foldMutate` `engine/rules/mutate.go#Engine.resolveMutate` | `TestEverquillPhoenixMutatesOntoTopAndCreatesFeather` `TestEverquillPhoenixMutatesUnderKeepsTargetOnTop` `TestMutatesCountAccumulatesAcrossTwoMutations` | same object keeps ID |
| 730.2d-730.2e | deviates | `engine/events/apply_copy.go#foldMutate` | - | SILENT: token and face-down status stay the target's, not the topmost component's |
| 730.2f-730.2j | not-impl | - | - | SILENT: face-down/flip/transform code never reads merged components |
| 730.3 | follows | `engine/events/apply_zone.go#move` | `TestMutateMergedCardsLeaveWithThePile` | under-cards follow the pile to its destination |
| 730.3a-730.3b | partial | `engine/events/apply_zone.go#move` | - | no arrangement or exile-timestamp choice; fixed pile order |
| 730.3c | unknown | - | - | not located: whether follow-up effects find every component |
| 730.3d-730.3e | partial | `engine/events/apply_zone.go#move` | - | under-cards take the replaced destination; 730.3e token/card split not modelled |
| 731.1-731.2c | follows | `engine/state/daynight.go#DayNight` `engine/events/apply_misc.go#foldDayNightChange` `engine/events/apply_turn.go#foldTurnChange` `engine/rules/daynight.go#Engine.dayNightUntapCheck` | `TestCR731DayboundPermanentMakesItDay` `TestCR731NightBecomesDayAfterTwoSpells` `TestDayNightChangeFoldAndPrevTurnSpells` | api:DayTime (14 files) and DayTime$ conditions unread |
| 732.1,732.1a,732.1c | n/a | - | - | informal and tournament shortcut conventions; engines pose each choice |
| 732.1b,732.2-732.3 | not-impl | `engine/effects/resolve.go#effRepeat` | - | SILENT: no shortcut proposal protocol; gorge only cuts no-progress repeats |
| 732.4,732.6 | deviates | `engine/rules/livelock.go#livelockWatcher.abort` `engine/rules/livelock.go#LivelockError` | `TestLivelockWatcherAbortsSameObjectSameKindRepeats` `TestLivelockWatcherAbortsMultiEventCycle` | gorge panics on a mandatory loop |
| 732.5 | n/a | - | - | neither engine forces a loop-breaking action |
| 733.1 | follows | `engine/rules/cast_commit.go#Engine.abortCast` `engine/rules/pay/castwindow.go#UndoWindowTap` | `TestCR733IllegalCounterProposalReverses` `TestCR733IllegalCastCannotTriggerPyromancer` `TestCR733AbortRestoresZoneEntryHistory` |  |
| 733.2 | partial | `engine/rules/cast_commit.go#Engine.abortCast` | `TestCR733UnderDelveReversalAllowsLegalRetry` `TestAbilityNoProgressAbortHoldsTheAbilityOut` | priority retained |
