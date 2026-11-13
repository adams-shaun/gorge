# CR 600s: Spells, abilities and effects

Coverage of this chapter of the Comprehensive Rules (2026-09-25) by gorge. Seeded from the CR audit of 2026-10-20 at `72dabd96c`; kept current by every change that touches a rule. See [README](README.md) for the status vocabulary and the reference syntax.

| rule | status | code | tests | notes |
|---|---|---|---|---|
| 601.1 | definition | - | - | casting a spell defined |
| 601.1a | n/a | - | - | terminology only |
| 601.2 | follows | `engine/rules/cast_begin.go#Engine.beginCastWith` `engine/rules/cast_begin.go#Engine.continueCast` | - | cast step machine; subrule order deviations below |
| 601.2a | follows | `engine/rules/cast_begin.go#Engine.continueCast` `engine/rules/cast_targets.go#Engine.pushCast` | `TestCR601_2aDelveCannotExileTheSpellBeingCast` `TestCR601_2aSpellIsOnTheStackAtItsFirstCostChoice` | spells pushed before every 601.2b-h choice; abilities: 602.2a |
| 601.2b | follows | `engine/rules/cast_begin.go#Engine.continueCast` `engine/rules/cast_asks.go#Engine.castModeAsk` `engine/rules/cast_subcounter.go#Engine.chosenModesTargetableAt` `engine/rules/pay/costparts.go#RevealOrChooseCandidates` | `TestCR601ModesAnnouncedBeforePayment` `TestCR601_2bModesAreAnnouncedBeforeX` `TestCloseEncounterChoosesACurrentCreature` | modes first, then additional costs, then X (bounded by the chosen modes); kicker/entwine intent rides the cast option |
| 601.2c | follows | `engine/rules/cast_targets.go#Engine.targetAsk` `engine/rules/legal_targets.go#targetSAAvailableGift` `engine/rules/legal_targets.go#Engine.xBoundTargetPayable` `engine/rules/cast_window.go#Engine.castWindowCensusComplete` | `TestCR601TargetsPrecedeManaPayment` `TestXBoundTargetNeedsACandidate` `TestXBoundTargetNeedsAnAffordableAnnouncement` `TestTargetAskCountsARestrictedAbilityBesideAPlainOne` | targets before payment; per-word uniqueness |
| 601.2d | partial | `engine/rules/cast_division.go#Engine.divisionDecision` `engine/rules/cast_division.go#castDivisionAPI` `engine/effects/division.go#AnnouncedDivision` | `TestArcLightningAnnouncesDivisionWhileCasting` `TestArcLightningIllegalTargetLosesItsShare` `TestMoggMobActivationAnnouncesDivision` | REVISED: g DealDamage divided at cast min 1 (cr-g04a); counters/prevent/triggers pending g04b/c; d resolution-time |
| 601.2e | follows | `engine/rules/cast_targets.go#Engine.recheckIllegal` | `TestCR601RecheckAfterXAndLegalTarget` | illegal proposal reversed |
| 601.2f | follows | `engine/rules/cast_pricing.go#Engine.manaToPay` `engine/rules/pay/costmods.go#CostMods.FaceSensitiveP` `engine/rules/cast_pricing.go#Engine.repriceForTargets` `engine/rules/statics_costmods.go#Engine.costUncounted` `engine/rules/cast_provenance.go#Engine.castProvenanceAdmitsPending` `engine/rules/potential_plan.go#Engine.potentialTargetPricedPlan` `engine/rules/potential_plan.go#Engine.potentialAltAddPlan` `engine/rules/potential_plan.go#Engine.potentialModeCastPlan` `engine/rules/legal_walk_alt.go#legalWalk.mayPlayAffordable` `engine/rules/cast_pricing.go#Engine.repriceForTargets` | `TestRaiseCostReadsAChainLinksTarget` `TestXAnnouncementCountsCostReductions` `TestNonHandReductionPricesTheOffer` `TestCR601TotalCostIncludesModifiers` `TestCR601AdditionalSacrificeIsPartOfTotalCost` `TestCR601_2fFlashbackSpellDoesNotCountItself` `TestCR601_2fHandSpellDoesNotCountItself` `TestPotentialPlanPricesTheCheapestTarget` `TestPotentialPlanPricesAnEitherOrCost` `TestPotentialPlanPricesAMayPlayAsItCosts` `TestMayPlayCastPricedAtCheapestTarget` `TestTwobridGenericFaceTakesTheReduction` `TestTwobridLaterPipsTakeTheReduction` `TestTwobridFaceOfferedByTheSourceWindow` | total locked; reduction order fixed, not player-chosen; a reduction composes on the faces the announced monocolour-hybrid pips resolved to (the offer, the pip ask and the source-window probe walk the faces: `CostMods.FaceSensitiveP`) |
| 601.2g | partial | `engine/rules/cast_paymentplan.go#Engine.manaWindowAsk` `engine/rules/cast_window.go#Engine.castWindowReachable` `engine/rules/payment_plan.go#Engine.planCastPaymentChecked` `engine/rules/mana_activation.go#owedColoursFirst` `engine/rules/cast_asks.go#Engine.altAddPartPayable` | `TestCR601ManaWindowBeforePayment` `TestCastWindowFaceChargesActivationFee` `TestCastPlanLeavesTheArtifactItsCostSacrifices` `TestAbilityAutoPayOutdatedByHelpersOpensThePlainWindow` `TestColourAskOffersTheOwedColourFirst` `TestFreeCastEitherOrManaPartReadsUntappedLands` `TestCastWindowFeeColourIsThePayersChoice` | g window only when pool short; a source's colours list what a fixed pip still owes first; d no manual mana, autopayer ignores hand reserve; the reachability probe tries every split of a filter fee |
| 601.2h | partial | `engine/rules/cast_begin.go#Engine.costObjectAsks` `engine/rules/cast_commit.go#Engine.payCast` `engine/rules/potential_plan.go#Engine.potentialWitnessKeeping` | `TestCR601_2hTargetsAreChosenBeforeTheSacrifice` `TestCR601_2hOneObjectPaysOneCost` `TestPotentialXCastKeepsItsOnlyTarget` | h objects chosen at payment, after the targets (c) and the g mana window; random/mill order; d autopay sources |
| 601.2i | follows | `engine/rules/cast_commit.go#Engine.payCast` `engine/rules/cast_commit.go#Engine.fireDeferredCastTrigger` | `TestCR601CastTriggerWaitsForCompletedProposal` | cast triggers after payment, caster priority |
| 601.3 | partial | `engine/rules/statics_cast.go#Engine.castRestricted` `engine/effects/filter_value.go#numericPred` `engine/effects/filter_value.go#parityPredicate` | `TestVoidWinnowerForbidsOpponentEvenManaValueSpells` `TestEffectCantBeCastBilbosGambit` `TestCantBeCastPrintedOriginKunoros` | REVISED note: cmcEven fixed (cr-g03); Effect-delivered CantBeCast registers via castRestrictionSources (cr-g136a), Origin$ read on both routes |
| 601.3a | follows | `engine/rules/statics_cast.go#Engine.castRestrictedCost` `engine/rules/statics_cast.go#Engine.xAnnouncementEscapes` `engine/rules/cast_targets.go#Engine.recheckIllegal` | `TestCR601_3aXSpellEscapesManaValueProhibition` `TestCR601_3aFreeCastHasNoXChoice` `TestCR601_3aEscapeMustBePayable` `TestCR601_3aFlashbackRouteAnnouncesNoX` `TestCR601_3aFreeMayPlayOfferStillProhibited` `TestCR601_3aPotentialWalkSeesPayableEscape` `TestVoidWinnowerRechecksChosenXOnTheStack` | offer looks ahead only over payable announced X (cost carries {X}, printed face carries {X}, priced through the offer's affordability helper against the walk's pricing pool); 601.2e recheck enforces the announced X; d no prohibition statics in pool |
| 601.3b | unknown | `engine/rules/statics_cast.go#Engine.castWithFlash` | - | settle: Aura-flash static + bestow card test; d no bestow |
| 601.3c | follows | `engine/rules/mayflashsac.go#mayFlashSacFace` `engine/rules/cast_begin.go#castModeMayflash` | - | MayFlashCost/MayFlashSac; d none in pool |
| 601.3d | unknown | - | - | g not traced; d no conditional flash |
| 601.3e | unknown | `engine/rules/legal_walk_hand.go#legalWalk.handWalk` | - | g Garruk's Horde/morph path not traced; d none |
| 601.3f | unknown | - | - | g not traced; d no face-down exile casting |
| 601.4 | follows | `engine/rules/cast_asks.go#Engine.castModeAsk` | - | kicker known before modes |
| 601.5 | partial | `engine/rules/cost_target_bound.go#Engine.costPartDeferred` `engine/rules/cost_target_bound.go#Engine.targetBoundCostAsks` | - | g target-bound costs deferred, other cost picks precede targets |
| 601.6 | follows | `engine/rules/cast_targets.go#Engine.recheckIllegal` | - | recheck after proposal; d no lapsing permission |
| 601.6a | follows | `engine/rules/cast_targets.go#Engine.recheckIllegal` | - | d no conditional flash |
| 601.7 | follows | `engine/rules/target_ask.go#Engine.poseOpponentPick` `engine/rules/target_ask.go#Engine.targetAskChooser` | - | opponent target picks; d none in pool |
| 601.7a | follows | `engine/rules/target_ask.go#Engine.opponentPicker` | - | two+ opponents: controller picks which; d two players |
| 601.7b | unknown | - | - | not traced |
| 601.8 | follows | `engine/rules/cast_pricing.go#Engine.manaToPay` `engine/rules/pay/costmods.go#CostMods.FaceSensitiveP` | - | cost computed at the spell's own payment only |
| 602.1 | follows | `engine/rules/activate.go#Engine.beginActivationPaid` | - | cost/effect/instructions split |
| 602.1a | follows | `engine/rules/activate.go#Engine.beginActivationPaid` | - | activator pays |
| 602.1b | follows | `engine/rules/legal_activation.go#Engine.activationConditionOK` `engine/rules/legal_activation.go#Engine.activatorAllows` | - | activation instructions gate the offer |
| 602.1c | definition | - | - | only activated abilities are activated |
| 602.1d | n/a | - | - | Oracle errata terminology |
| 602.1e | unknown | - | - | not traced |
| 602.2 | follows | `engine/rules/legal_activation.go#Engine.activatorAllows` `engine/events/apply_delayed.go#foldDelayedPush` `engine/events/event.go#Event.IsActivationMint` `engine/rules/trigmatch/cast.go#abilityCastMatches` | `TestSelfGrantedActivationIsAnActivatedAbility` `TestGrantedActivationFiresActivationTriggers` | controller-only unless Activator$; a self-granted activation is an activated ability on the stack; every granted or gained activation fires "whenever you activate" |
| 602.2a | follows | `engine/rules/cast_targets.go#Engine.pushActivation` `engine/rules/cast_commit.go#Engine.abortCast` `engine/rules/host_cast_ledger.go#Engine.costMovesInWindow` | `TestCR602_2aAbilityIsOnTheStackBeforeItsTarget` `TestCR602_2aAbortedActivationKeepsItsOncePerTurnUse` `TestCostMovesSurviveTheSourceBeingCastAgain` | ability pushed before every 602.2b choice; reversal undoes the census (ActivationReversed) |
| 602.2b | follows | `engine/rules/activate.go#Engine.beginActivationPaid` `engine/rules/cast_begin.go#Engine.continueCast` `engine/rules/announce_pay.go#Engine.announcedAutoFillPlan` `engine/rules/cast_paymentplan.go#Engine.convokeCommitted` `engine/rules/cast_asks.go#Engine.castModeAsk` `engine/rules/mana_cost_adjust.go#Engine.pricedManaCC` | `TestCR602ActivationTargetsPrecedePayment` `TestCR602ManaWindowDuringActivation` `TestCR602ActivationCostIncludesReduction` `TestCostMovesReadOwnActivation` `TestCostMovesEndAtEveryKindOfPush` `TestAbilityAutoPaySurvivesItsAnnouncements` `TestOwnTapSourcePaysNoMana` `TestModalActivatedAbilityAnnouncesItsMode` `TestModalActivatedAbilityFizzlesWithoutItsTarget` `TestModalActivatedAbilityMayTargetItsSource` `TestReducedManaAbilityCostIsPaid` `TestRaisedManaAbilityCostIsPaid` `TestSurchargeThatSparesManaAbilitiesLeavesThem` `TestReducedFilterLandPaysTheCast` | shared build path; a mana ability is an activated ability, so the cost statics that reach it price it (a land discount frees a filter land, a surcharge taxes a bare {T}; `Engine.pricedManaCC`, which the payability gate, the charge and the payment planner all read) |
| 602.3 | follows | `engine/rules/target_ask.go#Engine.targetAskChooser` | - | d none in pool |
| 602.3a | follows | `engine/rules/target_ask.go#Engine.opponentPicker` | - | as 601.7a; d two players |
| 602.3b | unknown | - | - | as 601.7b, not traced |
| 602.4 | follows | `engine/rules/cast_pricing.go#Engine.manaToPay` `engine/rules/pay/costmods.go#CostMods.FaceSensitiveP` | - | as 601.8 |
| 602.5 | follows | `engine/rules/statics_cast.go#Engine.abilityRestricted` `engine/rules/legal_walk_battlefield.go#legalWalk.battlefieldWalk` `engine/effects/effect.go#effEffect` | `TestEffectCantBeActivatedBraidedNet` `TestCantBeActivatedValidSAPowerUpPrinted` `TestCantBeActivatedValidSALoyaltyImmortalSun` | Effect-delivered CantBeActivated registers via the continuous registry (cr-g136b); ValidSA$ Activated.PowerUp and Activated.Loyalty are read |
| 602.5a | follows | `engine/rules/pay/managates.go#TapCostSick` `engine/rules/pay/managates.go#TapFlagsSick` | `TestCR602NewNoncreatureCanActivateTapAbility` | {T} sickness, haste, control change |
| 602.5b | follows | `engine/rules/legal_loyalty.go#Engine.activationUsedCount` | `TestCR602OncePerTurnActivationRestriction` | per object across control change |
| 602.5c | unknown | `engine/rules/legal_loyalty.go#Engine.activationUsedCount` | - | g granted identity = SVar name; two grantors may share tally |
| 602.5d | follows | `engine/rules/legal_walk_battlefield.go#legalWalk.battlefieldWalk` | `TestMaxSpeedGrantKeepsSorceryTiming` | sorcery timing |
| 602.5e | follows | `engine/rules/pay/board.go#InstantSpeedOnly` `engine/rules/cast_paymentplan.go#Engine.untappedManaSource` | - | d no InstantSpeed$ abilities |
| 603.1 | follows | `engine/cards/parse.go#ParseTriggerLine` `engine/rules/trigger_match.go#Engine.checkTriggers` `engine/rules/static_trigger.go#Engine.staticTriggerAskFree` | `TestStaticTriggerStaysOffTheStack` | trigger lines parsed; matched on events |
| 603.1a | unknown | - | - | not audited |
| 603.1b | unknown | - | - | not found in g |
| 603.2 | partial | `engine/rules/trigger_match.go#maxTriggerFires` `engine/rules/trigger_match.go#Engine.checkFaceTriggers` `engine/rules/trigmatch/actions.go#valiantFirstTimeTargetedByYouThisTurn` `engine/rules/trigmatch/cast.go#activatorCastCount` `engine/rules/trigmatch/cast.go#stackOnlyBareCardCast` | `TestValiantFiresOncePerTurn` `TestValiantCountsCasterNotOwner` `TestValiantDedupesSameSpellTargets` `TestActivatorThisTurnCastCountsValidCardSpells` `TestBecomesTargetAbilitySourceIncludesTriggered` `TestStackZoneCastTriggerSeesOnlyItsOwnCast` | g silent 256-fires-per-match cap per trigger line; the "for the first time each turn" once-per-turn gate (Valiant$ ability word) is real, per-controller with the caster read off the cast's PutOnStack (never a spell card's Owner) and one fire per targeting spell (cr-g74); a stack-zone SpellCast trigger naming the bare ValidCard$ Card matches only its own card's cast (Storied Veteran, Hundred-Battle Veteran) |
| 603.2a | follows | `engine/rules/trigger_match.go#Engine.triggerMatchesWithSVars` | - | no cast/activate gate on triggers |
| 603.2b | follows | `engine/rules/trigger_match.go#Engine.checkTriggers` `engine/rules/trigger_delayed.go#Engine.checkDelayedTriggers` | `TestEffectFirstCombatPhaseTriggerFiresOnce` | beginning-of-step triggers |
| 603.2c | follows | `engine/rules/trigger_match.go#Engine.checkFaceTriggers` `engine/rules/trigger_match.go#triggerTurnLimitFor` `engine/rules/trigger_match.go#grantedLimitKey` `engine/rules/trigger_match.go#Engine.latchDamageBatch` `engine/rules/trigger_granted.go#Engine.grantedDamageBatchAdmits` | `TestCR603_2cRaiseThePastIsOneEntryEvent` `TestSacrificedOncePerBatch` `TestTokenCreatedOncePerBatch` `TestDropkickBomberGrantLatchesOneDamageBatch` `TestDropkickBomberEachGrantTriggers` `TestGrantedLineKeepsItsActivationLimit` `TestChangesZoneAllFiresOnceForAMultiTargetMove` | once per occurrence; Mode$ SacrificedOnce / TokenCreatedOnce are "one or more" batches (one per sacrifice action / token-creation resolution, not per turn), and OnlyFirst$ keeps its per-turn cap |
| 603.2d | follows | `engine/rules/statics_echo.go#Engine.queueTrigger` `engine/rules/statics_echo.go#Engine.panharmoniconEchoes` `engine/rules/statics_echo.go#panharmoniconModes` | `TestChapterTriggersAdditionalTimes` `TestBlockTriggersAdditionalTimes` `TestUnblockedTriggersAdditionalTimes` `TestGrantedTriggerAdditionalTimes` `TestSpellTriggerAdditionalTimesReadsValidZone` `TestRoomAbilityAdditionalTimes` `TestEffectCombatTriggerNotAdditional` `TestUnlockTriggerDoubledByFracturedRealm` `TestEntryScopedDoublerLeavesUnlockTriggerAlone` `TestPanharmoniconDoublesOnlyTheListedTriggerModes` | every ability of an object triggers additional times, each static read against what caused it and a ValidMode$ against the doubled trigger's own Mode$; delayed, reflexive and sourceless inherent triggers do not |
| 603.2e | follows | `engine/rules/trigmatch/tap.go#tapsMatches` `engine/effects/combatfx.go#effTap` | - | d not reached |
| 603.2f | unknown | - | - | g no hidden-object gate found |
| 603.2g | follows | `engine/rules/trigmatch/combat.go#DamageMatchesWithCapture` `engine/rules/disable_triggers.go#triggerWatchesEntry` | `TestCR603_2gDamageRemovalIsNotDamageDealt` `TestCR603_2gHealIsNotPreventedByProtection` `TestDisableTriggersEnteringIsNotLeaving` | REVISED: g non-positive Damage never matches (cr-g06); d removal fires nothing, zero guard unverified; an entering-only DisableTriggers static stops lines that watch the entry, not "leave your graveyard" (Forge purges those too; XMage does not) |
| 603.2h | partial | `engine/rules/trigger_match.go#Engine.triggerActivationLimitAllows` `engine/rules/trigger_match.go#Engine.triggerLimitSpentBeforeMove` | `TestTriggerLimitLooksBackAtTheDepartingSource` | g reserved at queue; d keyed by Obj not version, cap 8; a source tested against its own departure reads the stint that move ended (603.10a), so a spent limit stays spent |
| 603.3 | follows | `engine/rules/turn.go#Engine.priorityRound` `engine/rules/trigger_queue.go#Engine.putTriggersOnStack` `engine/rules/potential_script.go#scriptSearch.visit` | `TestPrefixScriptPassesForATriggerBeforeASorceryPlay` `TestPrefixScriptDoesNotTapTheAbilitysSourceForAnothersCost` | placed when a player would get priority; a trigger a tap-another mana prefix queues is stacked before a sorcery-speed play it funds |
| 603.3a | partial | `engine/rules/trigger_match.go#Engine.checkFaceTriggers` | `TestAuraDiesTriggerBelongsToTheAurasController` | g post-change controller for control-loss triggers; Forge's TriggerController$ is ignored, since the source's controller decides |
| 603.3b | partial | `engine/rules/trigger_queue.go#Engine.putTriggersOnStack` `engine/rules/trigger_queue.go#Engine.askTriggerOrder` | - | g no two-part order, no SBA recheck between rounds |
| 603.3c | follows | `engine/rules/trigger_queue.go#Engine.askTriggerModes` | `TestCR603TriggerModesChosenAtPlacement` | modes at placement |
| 603.3d | follows | `engine/rules/target_ask.go#Engine.askTarget` `engine/rules/trigger_subtargets.go#Engine.askTriggerSubTargets` `engine/rules/target_ask.go#Engine.askTargetChained` `engine/rules/target_unique_chain.go#Engine.triggerUniqueChainViable` | `TestTriggerWithoutAChainTargetTargetsNothing` `TestTriggerUniqueChainNeedsTwoCreatures` | no legal target: removed; a TargetUnique$ chain is judged jointly |
| 603.4 | follows | `engine/rules/trigger_match.go#Engine.triggerMatchesWithSVars` `engine/rules/stack.go#Engine.resolveTop` `engine/rules/trigger_condition.go#Engine.boonsOf` | `TestCR603InterveningIfCheckedAtTriggerTime` `TestCR603InterveningIfRecheckedAtResolution` `TestOneTimeBoonWaitsForItsConditionAndFiresOnce` `TestUnderbridgeWarlockSeesItsBoon` | checked at trigger and resolution; an Effect's Mode$ Phase registration checks at the phase occurrence only |
| 603.5 | follows | `engine/rules/trigger_queue.go#Engine.askOptionalAtResolution` `engine/rules/trigger_queue.go#Engine.deciderFromSpec` | `TestCR603OptionalEffectWaitsForResolution` `TestOptionalDeciderTriggeredPlayerAsksThatPlayer` `TestOptionalDeciderAttackingPlayerAsksTheAttacker` `TestOptionalDeciderActivatorAsksTheCaster` `TestOptionalDeciderEnchantedControllerAsksThatController` | may asked at resolution, of the seat OptionalDecider$ names |
| 603.6 | follows | `engine/rules/stack.go#Engine.resolveTop` `engine/rules/stack.go#Engine.targetBecameNewObject` | - | object found in new zone only |
| 603.6a | partial | `engine/rules/entry_batch.go#Engine.holdEntryCheck` `engine/rules/entry_batch.go#Engine.flushHeldEntries` `engine/rules/trigger_match.go#Engine.closeZoneBatch` `engine/effects/zone_changeall.go#effChangeZoneAll` `engine/effects/zone_change.go#effChangeZone` | `TestCR603_6aRaiseThePastEntrantsSeeEachOther` `TestCR603_6aLivingDeathWardensSeeEachOther` `TestCR603_6aCopyTokenOwnEntryTriggerStillFires` | a zone batch's entries are checked after its last member enters (ChangeZoneAll and multi-object ChangeZone to the battlefield, Token, CopyPermanent, Dig, RepeatEach ChangeZoneTable); library-search fetches (effSearchLibrary, effHiddenPick) still enter one by one |
| 603.6b | follows | `engine/rules/trigger_match.go#Engine.checkFaceTriggers` | - | untested for lose-all-abilities ETB |
| 603.6c | follows | `engine/rules/trigger_kinds.go#Engine.faceLookBackZones` `engine/rules/trigger_snapshot_pool.go#Engine.lookBackObserver` | `TestCR704EveryDyingBloodArtistWitnessesWholeBatch` `TestCR704SimultaneousDeathsPreserveBloodArtistWitness` | LTB look-back for ChangesZone Origin Battlefield |
| 603.6d | follows | `engine/rules/cast_etbchoice.go#Engine.entryETBChoice` `engine/effects/combatfx.go#effTap` | - | as-enters is static |
| 603.6e | unknown | - | - | d no such Aura |
| 603.7 | follows | `engine/rules/trigger_delayed.go#Engine.checkDelayedTriggers` `engine/rules/trigger_delayed.go#Engine.checkEventDelayedTriggers` `engine/effects/ateot.go#scheduleAtEOTGroup` `engine/effects/ateot.go#scheduleAtEOTNone` | `TestPitAutomatonCopiesNextExhaust` `TestAdventureDelayedTriggerOutlivesItsFace` `TestDepartedCopyKeepsItsPromises` `TestExtraTurnStaticDelayedStaysOffTheStack` `TestPumpAllSacrificeRiderIsOneTrigger` `TestChangeZoneAllSacrificeRiderIsOneTrigger` `TestEmptyRememberedCopyRiderStillTriggers` `TestCopyPermanentSacrificeRiderOverSeveralCopiesIsOneTrigger` `TestAtEOTCorpusCensus` | a plain AtEOT$ sacrifice over several objects of one resolution (Token, CopyPermanent, Pump, PumpAll, ChangeZoneAll) is one delayed triggered ability naming all of them, and one over none (X = 0, an empty remembered set) is still created and does nothing; event modes SpellCast, ChangesZone, ChangesController, DamageDone, AttackersDeclared, BecomeMonarch, AbilityCast, SpellAbilityCast; d drops silently past MaxDelayed=8 / maxDProg=6 |
| 603.7a | follows | `engine/effects/delayed_trigger.go#effDelayedTrigger` | `TestDelayedTriggerPhaseRegisters` | created at resolution; not audited separately |
| 603.7b | follows | `engine/rules/trigger_delayed.go#Engine.checkDelayedTriggers` | - | one-shot removal at DelayedPush for a plain CR 603.7 promise; an api:Effect Mode$ Phase trigger is recurring (EffectRepeat) unless OneOff$ True, see 114.1-114.5; a boon ends after BoonAmount$ firings, one by default |
| 603.7c | partial | `engine/effects/zone_change.go#definedDelayRememberedTargets` `engine/effects/zone_change.go#effChangeZone` `engine/effects/zone_bulk.go#effDestroy` `engine/effects/delayed_trigger.go#delayedRegisterStampSuffix` | `TestDelayedReturnFindsTheObjectItsOwnTriggerExiled` | g new-object check only for move/destroy/sac bodies; a move the registering resolution made names the moved object |
| 603.7d-603.7e | follows | `engine/effects/delayed_trigger.go#effDelayedTrigger` `engine/effects/earthbend.go#registerEarthbendReturn` `engine/rules/trigger_delayed.go#Engine.checkEventDelayedTriggers` | `TestEarthbendReturnIsSourcedOnItsCreator` | controller and source from the creating ability |
| 603.7f | unknown | `engine/rules/altcast.go#Engine.altCostEnter` | - | not verified |
| 603.7g | unknown | - | - | not audited |
| 603.7h | unknown | - | - | not traced |
| 603.8 | follows | `engine/rules/trigger_condition.go#Engine.stateTriggerOutstanding` `engine/rules/trigger_match.go#Engine.checkFaceTriggers` | `TestCR603StateTriggerFiresWhenConditionBecomesTrue` | d no state triggers in pool |
| 603.9 | follows | `engine/rules/trigmatch/ordinary.go#losesGameMatches` | - | d two players: loss ends game |
| 603.10 | follows | `engine/rules/trigger_match.go#Engine.checkTriggers` | - | post-event state by default |
| 603.10a | partial | `engine/rules/trigger_kinds.go#looksBackOnDeparture` `engine/rules/trigger_kinds.go#computeFaceLookBackZones` `engine/rules/trigger_match.go#Engine.checkFaceTriggers` `engine/rules/trigger_match.go#Engine.checkTriggersNow` `engine/rules/trigmatch_board.go#trigBoard.MatchesObject` `engine/rules/trigger_inactive_face.go#triggerPushAmount` `engine/effects/count_player.go#countEnteredAs` `engine/rules/trigger_match.go#Engine.triggerStint` | `TestThisTurnEnteredCreatureReadsTheEntry` `TestSacrificedOnceReadsTheFoodAsItLastExisted` `TestCR603_10aMorbidOpportunistSeesItsOwnWrath` `TestCR603_10aMayhemDevilSeesSacrificesWithIt` `TestDepartedCopyTriggersOnLeaving` `TestCR603_10aLookBackReadsStartingLife` `TestCR603_10aLookBackReadsTurnLedgers` `TestCR603_10aLookBackFilterReadsDerivedPower` `TestCR603_10aOnceEachTurnSpansTheSourcesOwnDeath` `TestTriggerLimitHoldsOnLookBack` `TestEffectZoneLookbackSeesAnimatedLandTypes` | ChangesZone/ChangesZoneAll Origin$ Battlefield and Sacrificed lines look back at a battlefield departure, reading the object's derived characteristics (an Effect-owned registration reads the departing permanent's layer-4 types held across its emission) and the no-event ledgers a gate reads; a card leaving a graveyard and a public object put into a hand or library are still matched on the board after the event |
| 603.10b | deviates | `engine/rules/trigger_match.go#Engine.checkFaceTriggers` | - | d no phasing |
| 603.10c | unknown | `engine/rules/trigmatch/unattached.go#unattachedMatches` | - | not verified |
| 603.10d | deviates | `engine/rules/trigger_match.go#Engine.checkFaceTriggers` | - | g thief controls Kharn-style trigger; d none in pool |
| 603.10e | not-impl | `engine/rules/trigmatch/registry.go#Lookup` | - | LOUD: no Countered mode; cards unsupported |
| 603.10f | follows | `engine/rules/trigger_queue.go#Engine.dropDepartedTriggers` | - | d two players |
| 603.10g | follows | `engine/rules/trigger_match.go#Engine.checkTriggers` | - | - |
| 603.11 | unknown | - | - | not audited |
| 603.12 | partial | `engine/rules/reflexive.go#Engine.QueueReflexiveTrigger` `engine/effects/immediate.go#effImmediateTrigger` | - | g granted reflexive bodies resolve inline, silently |
| 603.12a | unknown | - | - | not traced |
| 604.1-604.2 | follows | `engine/rules/chars/cda.go#StaticZoneAdmits` `engine/rules/chars/cda.go#EffectZoneOK` | - | statics always on in their zone |
| 604.3 | follows | `engine/rules/chars/cda.go#CDAPTStatic` `engine/rules/chars/cda.go#CDASetPT` `engine/rules/host_read.go#Engine.cdaPTTable` | `TestCR208_2aDigReadsLibraryCDAPower` | - |
| 604.3a | unknown | - | - | not traced |
| 604.4 | follows | `engine/effects/filter_attachment.go#attachedBy` | - | attachment statics don't target |
| 604.5-604.6 | follows | `engine/rules/chars/cda.go#EffectZoneOK` `engine/rules/chars/cda.go#StaticZoneAdmitsStatic` | - | stack/hand-zone statics; untested in g |
| 604.7 | unknown | - | - | g not traced |
| 605.1 | definition | - | - | mana ability criteria header |
| 605.1a | follows | `engine/cards/ability_class.go#IsManaAbilitySA` `engine/cards/ability_class.go#movesLibraryCard` | `TestCR605_1aManaAbilityLibraryGateMovesTheLibraryCarriers` `TestCR605_1aLibraryMoveShapes` `TestCR605_1aLibraryMovingManaAbilitiesUseTheStack` | target, loyalty (rules-side `isLoyaltyAbility`) and library-move criteria |
| 605.1b | partial | `engine/rules/mana_activation.go#Engine.isTriggeredManaAbility` `engine/rules/cast_window.go#Engine.castWindowCensusComplete` | `TestTargetMenuKeepsATargetATapTriggerPaysFor` | g only TapsForMana shape; d no triggered mana |
| 605.2 | follows | `engine/cards/ability_class.go#IsManaAbilitySA` | - | - |
| 605.3 | follows | `engine/rules/mana_activation.go#Engine.activateManaFor` | - | header; exceptions below |
| 605.3a | partial | `engine/rules/legal_walk_battlefield.go#legalWalk.manaSection` `engine/rules/cast_paymentplan.go#Engine.manaWindowAsk` `engine/rules/paid_mana_census.go#Engine.unlessWindowUnits` | `TestUnlessCostReachesChoiceMana` `TestCounterUnlessPaidFromChoiceMana` | g window gated |
| 605.3b | follows | `engine/rules/mana_activation.go#Engine.activateManaFor` `engine/rules/mana_activation.go#Engine.commitManaDiscard` `engine/rules/unless_tape.go#tapeManaWindow` `engine/rules/payment_plan.go#Engine.paymentPlanManaUnitsOnlyCompute` | `TestManaCostAbilityGoesOnTheStackAndResolves` `TestUnlessPayerActivatesAChoiceLandInsideAManaAbility` `TestPotentialPlanPricesATappedSacrificeSource` | no stack; a sacrifice-only source pays tapped (no {T} in its cost) |
| 605.3c | follows | `engine/rules/mana_activation.go#Engine.activateManaFor` | - | activation completes atomically |
| 605.4-605.4a | follows | `engine/rules/mana_activation.go#Engine.resolveTriggeredManaAbilities` `engine/rules/mana_activation.go#Engine.isTriggeredManaAbility` `engine/rules/potential.go#Engine.addPotentialTapExtra` | `TestTapManaFoldOfferedWhenExtraCompletesCost` | resolve immediately; d none in pool; a potential action counts the folded extra of a static TapsForMana trigger beside the tapped source's own mana |
| 605.5-605.5a | follows | `engine/cards/ability_class.go#IsManaAbilitySA` `engine/cards/ability_class.go#requiresTarget` | `TestWarringTriadTargetingManaAbilityUsesTheStack` | d none in pool |
| 605.5b | follows | `engine/cards/ability_class.go#IsManaAbilitySA` | - | SP$ Mana is a spell; d no mana rituals |
| 606.1 | definition | - | - | loyalty abilities defined |
| 606.2 | follows | `engine/rules/legal_loyalty.go#isLoyaltyAbility` | `TestLoyaltyOptionFacts` | - |
| 606.3 | follows | `engine/rules/legal_loyalty.go#Engine.loyaltyAbilityLimit` `engine/rules/legal_loyalty.go#Engine.loyaltyActivationsThisTurn` `engine/rules/legal_walk_battlefield.go#legalWalk.battlefieldWalk` | `TestJaceLoyaltyAbilityOncePerTurnAndOwnTurnOnly` | main phase, empty stack, once per turn |
| 606.4-606.5 | follows | `engine/rules/pay/costmods.go#PlusRaiseExtra` `engine/rules/replacement_match.go#Engine.replacementMatchesRememberedUngatedBy` | `TestRaiseCostCarthAddsALoyaltyCounter` `TestRaiseCostCarthNetsAMinusLoyaltyAbility` `TestDoublingSeasonLeavesLoyaltyCostUndoubled` | a loyalty cost's counters are a cost, never doubled by EffectOnly; d 606.5 N/A, no modifiers |
| 606.6 | follows | `engine/rules/pay/castparts.go#NonManaCastableP` | - | - |
| 607.1 | partial | `engine/state/object.go#Object.Imprinted` `engine/state/object.go#Object.ExiledWith` | - | g links per object, not per ability pair |
| 607.1a-607.1d | unknown | - | - | not audited |
| 607.2 | definition | - | - | kinds of linked abilities |
| 607.2a | follows | `engine/effects/context.go#definedSpec` `engine/state/object.go#Object.ExiledWith` `engine/effects/context.go#chapterLinkedRemembered` `engine/effects/context.go#svarsReadExiledWith` | `TestOblivionRingExiledWithResolvesOpponentOwnedTarget` `TestHandExileRecordsExiledWith` `TestMeasureOfAManChapterIIIReturnsChapterIExile` `TestBestOfBothWorldsChapterIIIReturnsTheAssimilated` | d link pool of 4: fifth exile never returns, silent; a Saga chapter that has remembered nothing reads the Saga's list, what its earlier chapters remembered (The Measure of a Man, The Best of Both Worlds); a hidden pick from the hand into exile links the card when the source reads ExiledWith (All Good Things . . .) |
| 607.2b-607.2c | unknown | - | - | not audited |
| 607.2d,e,f,g,m,p | partial | `engine/state/object.go#Object.ChosenName` | - | g one chosen value per object |
| 607.2h | unknown | - | - | not audited |
| 607.2i-607.2j | follows | `engine/effects/filter_word.go#wordMatches` | - | - |
| 607.2k | unknown | - | - | champion; not audited; d no champion |
| 607.2n | unknown | - | - | pregame exile; not audited; d none in pool |
| 607.2q | unknown | - | - | cast-cost exile link; not audited; d none in pool |
| 607.3 | unknown | - | - | not traced |
| 607.4 | follows | `engine/state/object.go#Object.ChosenColor` | - | Paradise Plume shape |
| 607.5 | partial | `engine/state/object.go#Object.ExiledWith` | - | acquired pairs share printed slots |
| 607.5a | deviates | `engine/events/apply_zone.go#move` | - | g no undefined choice after copy |
| 608.1 | follows | `engine/rules/legal_priority.go#Engine.handlePriority` | - | all pass in succession |
| 608.2 | follows | `engine/rules/stack.go#Engine.resolveTop` `engine/effects/registry.go#Resolve` | - | instant/sorcery/ability resolution steps |
| 608.2a | follows | `engine/rules/stack.go#Engine.resolveTop` | `TestCR603InterveningIfRecheckedAtResolution` | intervening-if recheck |
| 608.2b | partial | `engine/rules/stack.go#Engine.resolveTop` `engine/rules/stack.go#Engine.targetBecameNewObject` `engine/rules/target_legal.go#Engine.protectionSource` `engine/effects/conditions.go#linkConditionMet` | `TestCR608ResolutionRechecksVinesTargetRestriction` `TestCharmAllModesIllegalDoesNotResolve` `TestCR400_7GraveyardTargetThatLeftAndReturnedIsANewObject` `TestCR400_7AnimateDeadTargetThatLeftAndReturnedIsANewObject` `TestLinkConditionReadsTheLinksOwnTarget` | fizzle, partial skip and the new-object test (any zone change) follow; protection reads live source, not LKI |
| 608.2c | follows | `engine/effects/registry.go#Resolve` `engine/effects/context.go#playerForTarget` `engine/effects/context.go#resolvedRemembered` | `TestTargetedHandOwnerIgnoresALaterLinksCreature` `TestBlightningDiscardsTwoAtEitherTarget` `TestRememberWriterThatWroteNothingMovesNothing` | order written; a link past one that remembers reads only what was written; a player read of the chain's targets names target players, and a controller read target objects |
| 608.2d | follows | `engine/rules/unless_payment.go#Engine.UnlessCostPayableFromCtx` | - | divided-untargeted half not audited |
| 608.2e | deviates | `engine/effects/zone_bulk.go#effSacrifice` `engine/effects/cardflow.go#effDiscard` `engine/effects/repeateach.go#effRepeatEach` | `TestEachPlayerSacrificesChoosesThenMovesTogether` | a single Sacrifice or Discard line over several players chooses, then moves as one event; a multi-step line scripted through RepeatEach RepeatPlayers$ (Pox) runs every step per player |
| 608.2f | partial | `engine/effects/context.go#definedSpec` | - | g order starts at controller, not AP; d APNAP |
| 608.2g | follows | `engine/rules/cast_targets.go#Engine.finishTargetedCast` `engine/rules/unless_payment.go#Engine.answerUnlessMana` | - | d ward/unless mana tapped in battlefield order |
| 608.2h | partial | `engine/effects/registry.go#Resolve` `engine/effects/filter_attachment.go#objectAttachedToSource` `engine/effects/pump.go#effPump` `engine/effects/combatfx.go#effPumpAll` `engine/rules/trigger_entered_object.go#Engine.followTriggerObject` `engine/rules/trigger_entered_object.go#Engine.settleFollowedObject` `engine/effects/count_body_paid.go#castProvenanceSource` `engine/effects/conditions.go#conditionDefinedMet` `engine/effects/filter.go#copyCeased` `engine/effects/context.go#targetsControllerLKI` `engine/rules/replacement_damage.go#Engine.damageSourceLeftThisTurn` `engine/rules/trigger_entered_object.go#Engine.keepTriggerLKI` `engine/effects/count_ref.go#sacrificedValidCount` | `TestConditionTargetedReadsLKIState` `TestPumpAmountIsReadOnce` `TestEnteredCreaturePowerReadsThePermanent` `TestEnteredCreatureThatLeftReadsItsLastPower` `TestCastTriggerReadsXInSpellManaValue` `TestDepartedCreatureKeepsItsManaSpent` `TestDepartedCreatureKeepsItsColoursSpent` `TestDepartedCreatureStaysKicked` `TestPermanentBaseReadsTheCardTypeOffTheBattlefield` `TestDamageReplacementReadsACeasedTokenCopySource` `TestMovedTargetControllerReadsLastKnownInformation` `TestDamageSourceThatLeftReadsItsOwnEntry` `TestGrantedDeathTriggerReadsLastKnownPower` `TestTargetedControllerAfterTheTargetLeft` `TestSacrificedValidReadsTheTypesItHadOnTheBattlefield` `TestGowronCountsASacrificedKlingonToken` | LKI field sets limited |
| 608.2i | follows | `engine/effects/count_body_paid.go#evalCountBodyPaid` | - | d not reached |
| 608.2j | follows | `engine/effects/filter_compiled.go#compileSpec` `engine/effects/filter_compiled.go#compiledBaseMatch` | - | d not reached |
| 608.2k | follows | `engine/effects/context.go#definedSpec` | - | d not reached |
| 608.2m | follows | `engine/rules/resolution_chain.go#Engine.moveResolvedOffStack` `engine/rules/stack.go#Engine.ensureLeftTheStack` | - | d not reached |
| 608.2n | follows | `engine/rules/resolution_chain.go#Engine.moveResolvedOffStack` | `TestCR608CorpusOrdinaryBurnFinishesInGraveyard` | graveyard / exile if flashback |
| 608.2p | not-impl | `engine/rules/trigmatch/registry.go#Lookup` | - | LOUD: no AbilityResolves matcher (Tom Bombadil) |
| 608.3 | follows | `engine/rules/stack.go#Engine.resolveTop` | - | permanent spell resolution header |
| 608.3a | follows | `engine/events/apply_zone.go#move` | - | spell controller |
| 608.3b | follows | `engine/rules/stack.go#Engine.resolveTop` `engine/rules/bestow.go#Engine.bestowTargetIllegal` `engine/rules/mutate.go#Engine.mutateVictim` | `TestBoonSatyrBestowedOntoShockedBearsEntersAsCreature` `TestGemrazerMutatingOntoShockedTargetEntersAsCreature` | REVISED: g bestow/mutate illegal target resolves as creature (cr-g02); d Aura only |
| 608.3c | follows | `engine/rules/stack.go#Engine.resolveTop` `engine/rules/resolution_chain.go#Engine.moveResolvedOffStack` | `TestCelestialArchonBestowedCastAttachesAuraToBearer` | Aura enters attached to its target |
| 608.3d | follows | `engine/rules/mutate.go#Engine.resolveMutate` | - | d no mutate |
| 608.3e | follows | `engine/rules/resolution_chain.go#Engine.moveResolvedOffStack` | - | d full board = documented capacity draw |
| 608.3f | follows | `engine/events/apply_zone.go#move` `engine/rules/trigmatch/zone.go#resolvedCopyLKI` | `TestCopiedPermanentSpellEntersAsToken` `TestCopiedSpellDoesNotFireItsOwnNontokenTrigger` | copy enters as token (Move fold); d not traced |
| 608.3g | follows | `engine/rules/altcast.go#Engine.altCostEnter` | `TestDashCastsGainHasteAndReturnAtTheEndStep` | d no dash/blitz/warp |
| 609.1 | follows | `engine/effects/registry.go#Resolve` | - | effects are spell/ability results |
| 609.2 | follows | `engine/effects/pump.go#effPump` | - | battlefield-only unless zone stated |
| 609.3 | follows | `engine/effects/prevent_damage.go#effPreventDamage` | - | spot-checked only |
| 609.4-609.4b | unknown | - | - | "as though" effects not audited |
| 609.5 | n/a | - | - | no default tie rule exists |
| 609.6 | definition | - | - | see 614/615 rows |
| 609.7-609.7a | unknown | - | - |  |
| 609.7b | follows | `engine/rules/replacement_damage.go#Engine.damageReplacementMatches` | - | CoP:Red ValidSource rechecked at damage time; d no such shield |
| 609.7c | unknown | - | - |  |
| 610.1 | definition | - | - | - |
| 610.2 | unknown | - | - | deferred to 603.7 slice |
| 610.3-610.3b | follows | `engine/effects/zone_change.go#exileHostGoneFor` `engine/effects/zone_change.go#changeZonePrelude` `engine/rules/emit.go#Engine.sweepExileReturn` `engine/effects/zone_change.go#exileUntilOpponentMonarchMet` | `TestCR610_3AuraReturnFindsTheBoardAfterTheDestroyAll` `TestCR610_3AuraReturnEnchantsTheSurvivor` `TestCR610_3AuraReturnFindsTheBoardAfterLethalDamage` `TestPalaceJailerExilesNothingWhenOpponentTookCrownAfterTrigger` `TestPalaceJailerExilesWhenOpponentWasMonarchBeforeTrigger` | host gone: no exile (610.3a/b); an opponent became the monarch after the push: no exile (610.3b, push-time stamp); a departure in a zone batch returns its cards when the batch closes |
| 610.3c | unknown | - | - | returns under owner |
| 610.3d | partial | `engine/rules/emit.go#Engine.flushExileReturns` | `TestCR610_3dReturnsFromOneEventAreSimultaneous` | one zone batch for triggers; the moves are ordered, so a returning Aura can choose a creature returning with it |
| 610.4-610.4d | unknown | - | - | phase out "until" |
| 610.5 | unknown | - | - | - |
| 611.1 | definition | - | - | - |
| 611.2 | follows | `engine/rules/layers_effect.go#Engine.AddContinuous` `engine/effects/effect.go#effEffect` | `TestEffectOwnerHoldsTheMayPlayGrant` | resolution-generated continuous effects; an Effect's look and may-play grants belong to its EffectOwner$ player, as Forge makes the Effect card that player's |
| 611.2a | follows | `engine/state/continuous.go#ContinuousEffect` | `TestCR611IndefinitePumpSurvivesCleanup` | no duration = game |
| 611.2b | follows | `engine/effects/choose_control.go#effGainControl` | - |  |
| 611.2c | follows | `engine/effects/combatfx.go#effPumpAll` `engine/effects/effect.go#effEffect` `engine/effects/parent_targets.go#notePlayerLink` | `TestEffectGoesOnForTheOtherTarget` `TestQuickDrawSweepsTheTargetOpponentsCreatures` `TestInvokeJusticeDistributesAmongTheTargetPlayersCreatures` | set locked at resolution; d per-perm EOT deltas |
| 611.2d | follows | `engine/effects/pump.go#effPump` | - | X fixed at resolution |
| 611.2e | unknown | - | - | Valkyrie's Call return would test it |
| 611.2f | unknown | - | - | - |
| 611.3 | follows | `engine/rules/layers.go#Engine.staticEffectsWalk` `engine/rules/layers_static.go#Engine.continuousGateHolds` | `TestBeamMeUpGrantHoldsInItsPhasesOnly` | static-generated continuous effects |
| 611.3a | follows | `engine/rules/layers.go#Engine.staticEffectsWalk` `engine/rules/layers.go#Engine.activeBuild` `engine/rules/layers_types.go#affectsZone` | `TestBattlefieldStaticGrantsPermanentsOnly` `TestEffectStaticZones` | statics re-read every derive; a static reaches permanents unless it names another zone |
| 611.3b | follows | `engine/rules/layers.go#Engine.staticEffectsWalk` | - | source leaves: effect gone |
| 611.3c | follows | `engine/rules/chars/derive.go#Compute` | - | no enter-then-change gap; derived on demand |
| 611.3d | unknown | - | - | - |
| 612.1 | follows | `engine/cards/textchange.go#Face.WithTextChanges` `engine/state/object.go#Object.faceSlow` | `TestCR612ChangedTypeWordDrivesTheLordStatic` `TestChangeTextCarriersUnsupportedIsExact` | the changed words reach abilities, keywords and the type line (cr-g36a) |
| 612.2 | follows | `engine/cards/textchange.go#SubstituteTextWord` `engine/effects/changetext.go#effChangeText` | `TestSubstituteTextWordRules` `TestCR612ChangedLandWordRewritesLandwalk` `TestNewBloodChangeTextEndToEnd` | whole words with non- negation, landwalk/plural suffixes; names and script references kept |
| 612.2a | unknown | - | - | token type text change not audited; likely as 612.2 |
| 612.3 | follows | `engine/rules/chars/derive.go#Compute` | - | granted abilities untouched |
| 612.4 | partial | `engine/state/object.go#Object.faceSlow` | - | tokens take the same object path; not separately tested |
| 612.5 | partial | `engine/effects/changetext.go#registerTextSet` `engine/effects/changetext.go#effExchangeTextBox` | - | Exchange of Words swaps text+keywords only |
| 612.6 | not-impl | - | - | SILENT; GainTextOf$ unread, Volrath's Shapeshifter "playable" |
| 612.7 | not-impl | - | - | SILENT; AddNames$ unread, Spy Kit names missing |
| 612.8 | follows | `engine/rules/chars/derive.go#Name` `engine/rules/chars/derive.go#Compute` | - | SetName / Witness Protection |
| 612.9 | unknown | - | - | stickers |
| 612.10 | follows | `engine/rules/splice.go#Engine.spliceAsk` `engine/rules/splice.go#Engine.runSplicedSegments` | `TestSpliceOntoArcaneRevealAndCost` `TestSpliceAddedTextTargetsAreItsOwn` `TestSpliceNonManaCostSacrifices` `TestSpliceCounteredLeavesTheCardsInHand` `TestSpliceQualityAndPayabilityGateTheOffer` | spliced text is added to the spell as it is cast; the card stays in hand |
| 613.1 | follows | `engine/rules/layers.go#compareContinuousPtr` `engine/rules/layers.go#continuousPtrsSorted` `engine/rules/chars/derive.go#Compute` | `TestCR613HumilitySetsBaseBeforePumpAndRemovesAbilities` `TestAttackTriggerReadsAttackerTypesPastColorTable` | a build under a live match keeps private buffers (Scratch.Borrowed) |
| 613.1a | partial | `engine/state/object.go#Object.CopyFace` | `TestCloneModifiersApplyAtTheirOwnLayers` | copy as face swap; see 707 slice |
| 613.1b | follows | `engine/rules/control.go#Engine.RegisterControl` `engine/rules/control.go#Engine.expireControl` | - | grants in timestamp order |
| 613.1c | follows | `engine/state/object.go#Object.faceSlow` `engine/events/copy_except.go#foldTextChange` | `TestCR612ChangedSpellTextOnTheStack` `TestCR612TextChangeIsNotCopiableAndEndsOnLeaving` | layer 3 rewrites Face(); not copiable (707.2); ends on a zone change except spell to permanent (400.7a) |
| 613.1d | follows | `engine/rules/chars/types.go#TypesAndLandSubtypeSet` `engine/rules/chars/types.go#applyTypeEffect` `engine/rules/statics_cast.go#Engine.staticTimingGate` `engine/cards/ir.go#Card.ChangesTypes` | `TestFerociousFlashReadsCurrentCreatures` `TestSweepSkipsAGodBelowItsDevotion` `TestEveryTypeChangingStaticArmsThePoolGate` |  |
| 613.1e | follows | `engine/rules/chars/derive.go#Compute` `engine/rules/static_colorgate.go#Engine.rewalkStaticColorGates` | `TestDerivedColorsLayerCompose` `TestContinuousColorGateReadsDerivedColors` | a static's as-long-as gate naming a colour reads the derived colours |
| 613.1f | partial | `engine/rules/chars/derive.go#Compute` `engine/rules/abilityloss.go#Engine.abilityLoss` `engine/rules/abilityloss.go#Engine.grantLostTo` `engine/rules/abilityloss.go#Engine.pruneLostSourceAbilities` `engine/rules/replacement.go#Engine.replacementFace` | `TestLayer6GrantsKeywords` `TestAbilityRemovalTimestampStillDominates` `TestAnimateGrantedStaticFollowsAbilityLoss` `TestAnimateGrantedStaticCDASpelling` `TestLifeReplacementLostWithTheAbilities` `TestBlockedTriggerNeedsTheCreatureToKeepItsAbilities` | gorge: keyword counters lost regardless of ts; a static gained through Animate ends with a newer loss |
| 613.1g | partial | `engine/rules/chars/derive.go#PT` | - | layer 7; g gaps at 613.4d |
| 613.2-613.2a | partial | `engine/state/object.go#Object.CopyFace` | - | "as enters" copiable P/T not checked |
| 613.2b | follows | `engine/rules/chars/derive.go#Compute` `engine/rules/chars/types.go#TypesAndLandSubtypeSet` | - |  |
| 613.2c | follows | `engine/state/object.go#Object.CopyFace` | - |  |
| 613.3 | follows | `engine/rules/chars/types.go#TypesAndAllCreatureTypes` `engine/effects/colors.go#ColorMaskOf` | `TestChangelingTypeCDASettlesBeforeAbilityRemoval` |  |
| 613.4 | partial | `engine/rules/chars/derive.go#PT` | - | layer 7 sublayers; g 7d gap |
| 613.4a | follows | `engine/rules/chars/derive.go#PT` `engine/rules/chars/derive.go#cdaPTLost` `engine/rules/chars/cda.go#CDASetPT` | `TestCR613_4aCDAPTLostUnderDressDown` `TestCR613_4aCDAPTLossKillsConsumingAberration` `TestCR613_4aCDAPTGuards` | - |
| 613.4b | follows | `engine/rules/chars/derive.go#PT` | `TestSetBeforeModifyRegardlessOfTimestamp` `TestCardBasePowerStopsAtLayer7bAndDrivesCount` | - |
| 613.4c | follows | `engine/rules/chars/derive.go#PT` | - | order-equivalent |
| 613.4d | follows | `engine/rules/chars/derive.go#PT` `engine/effects/combatfx.go#registerPumpEffects` `engine/cards/hiddenkeyword.go#IsHiddenSwitchPTKeyword` | `TestCR613_4dTwistedImageSwitchesAfterCounters` `TestCR613_4dSwitchToZeroToughnessDies` `TestCR613_4dTwoSwitchesCancel` | switches applied after counters by parity; all 27 corpus carriers are Pump/PumpAll KW$ grants |
| 613.5 | follows | `engine/rules/chars/derive.go#Compute` | - | derived on demand |
| 613.6 | partial | `engine/rules/chars/derive.go#lockedLayer4Types` `engine/rules/abilityloss.go#Engine.removalApplies` | - | approximations per layer |
| 613.7 | follows | `engine/rules/layers.go#compareContinuousPtr` | - | timestamp order; gaps in subrules |
| 613.7a | partial | `engine/rules/layers.go#Engine.staticEffectsWalk` | - | grant's own ts ignored |
| 613.7b | follows | `engine/rules/layers_effect.go#Engine.AddContinuous` | - | - |
| 613.7c | partial | `engine/rules/chars/derive.go#BaseKeywords` | - | keyword counters at base; P/T counters order-irrelevant |
| 613.7d | follows | `engine/events/apply_zone.go#move` | - |  |
| 613.7e | follows | `engine/events/apply_zone.go#foldAttach` `engine/events/apply_zone.go#restampLayer` `engine/state/object.go#Object.LayerTimestamp` | `TestEquipAfterAbilityLossRestampsTheEquipment` `TestEquipBeforeAbilityLossStaysLost` | restamps LayerStamp; entry Timestamp kept as identity |
| 613.7f | follows | `engine/events/apply_copy.go#foldTurnFaceUp` `engine/events/apply_copy.go#foldTurnFaceDown` `engine/events/apply_zone.go#restampLayer` | `TestTurnFaceUpRestampsThePermanent` |  |
| 613.7g | follows | `engine/events/apply_copy.go#foldFlipFace` `engine/events/apply_zone.go#restampLayer` | `TestTransformRestampsThePermanent` | transforming DFC on the battlefield only |
| 613.7h | unknown | - | - | variant cards |
| 613.7i | unknown | - | - | - |
| 613.7j | unknown | - | - | - |
| 613.7k | unknown | - | - | - |
| 613.7m | unknown | - | - | APNAP for simultaneous stamps |
| 613.7n | unknown | - | - | - |
| 613.8-613.8a | partial | `engine/rules/chars/types.go#TypesAndLandSubtypeSet` `engine/rules/chars/derive.go#abilityDependencyOrder` `engine/rules/abilityloss.go#Engine.abilityLoss` | `TestLayer6KeywordDependencyChainResolvesTransitively` `TestLayer6DependencyOrderCavalryMasterOlderThanSidewinder` | WP+Imprisoned diverges (layer plan L3) |
| 613.8b | partial | `engine/rules/chars/types.go#TypesAndLandSubtypeSet` `engine/rules/chars/derive.go#abilityDependencyOrder` | - | loops fall back to timestamp |
| 613.8c | partial | `engine/rules/chars/types.go#TypesAndLandSubtypeSet` `engine/rules/chars/derive.go#abilityDependencyOrder` | `TestLayer6NegativeKeywordDependencyAppliesDependentAfter` | layer 6 graph not re-evaluated |
| 613.9 | follows | `engine/rules/layers.go#continuousPtrsSorted` | - | later override wins |
| 613.10 | follows | `engine/rules/playerkeywords.go#Engine.playerKeywords` | - | additive player grants |
| 613.11 | follows | `engine/rules/combat.go#Engine.mustAttackRequired` | - | rules-mods read outside layer walk; weak evidence |
| 614.1-614.1a | follows | `engine/rules/replacement.go#Engine.applyReplacements` `engine/rules/replacement.go#Engine.applyReplacementsDispatch` | - | "instead" replacements |
| 614.1b | follows | `engine/effects/dice.go#effSkipTurn` `engine/rules/turn.go#Engine.beginTurn` | - |  |
| 614.1c | partial | `engine/rules/static_trigger.go#staticBodyCompetes` `engine/rules/entry_counters.go#Engine.entryCounterGrants` `engine/rules/entry_counters.go#Engine.entryBodyCandidates` | `TestStaticTriggerEventBodyStack` `TestCR614EntryBodyCounterIsReplaceable` `TestEntryCountersRequestedAndRiderAreOnePut` `TestEntryCountersEffectBorneBodyIsOnePut` `TestEntryCountersEffectBorneAndStaticRiderAreOnePut` `TestEntryCountersDoublerUnchanged` | enters with counters; a Static$ True bonus stays an ability only where a replacement of its own event can ask, since nothing off the stack can serve the ask |
| 614.1d | follows | `engine/rules/replacement.go#Engine.applyReplacementsDispatch` `engine/rules/replacement_updated.go#Engine.composeUpdatedReplacements` | - | "as enters" replacements compose |
| 614.1e | follows | `engine/rules/replacement_phase.go#Engine.applyTurnFaceUpReplacements` | - | as turned face up |
| 614.2 | follows | `engine/rules/replacement_damage.go#Engine.applicableDamageReplacements` `engine/rules/replacement_damage.go#Engine.damageReplacementMatches` | - | damage replacements |
| 614.3 | follows | `engine/rules/replacement.go#Engine.applyReplacements` | - | no special restriction modelled |
| 614.4 | follows | `engine/rules/replacement_match.go#Engine.activeZonesGateOK` | `TestCR614RestInPeaceInHandCannotReplace` `TestCR614ActivePhasesRestrictsDrawReplacement` | must exist before the event |
| 614.5 | follows | `engine/rules/replacement_updated.go#Engine.dropAppliedReplacements` | `TestCR614AllApplicableEntryReplacementsApplyOnce` | each applies once; x4 not x2/inf |
| 614.6 | follows | `engine/rules/replacement.go#Engine.applyReplacementsDispatch` | - | modified event replaces the original |
| 614.7 | follows | `engine/rules/replacement_match.go#Engine.replacementMatchesRememberedUngatedBy` | - | - |
| 614.7a | follows | `engine/effects/damage.go#emitObjectDamage` | `TestCR120_8ZeroDamageHasNoEventToReplace` | 0 damage is no event, never replaced (cr-g06) |
| 614.8 | partial | `engine/effects/regeneration.go#ReplaceDestroyLine` `engine/effects/regeneration.go#ReplaceDestruction` `engine/effects/regeneration.go#ReplaceUmbraArmor` | `TestDestroyLineSparesFromDestroyEffects` `TestDestroyLineSparesFromLethalDamage` | fixed order: R:Event$ Destroy lines in scan order, then shield, then umbra |
| 614.9 | follows | `engine/effects/replacement.go#effReplaceEffect` | `TestCR614RememberedDamageRedirectToObject` `TestCR614RememberedDamageRedirectToPlayer` `TestCR614RememberedDamageSkipsLostPlayer` | damage redirect via ReplaceEffect |
| 614.10 | partial | `engine/effects/skipphase.go#effSkipPhase` `engine/rules/turn.go#skipCombatPhase` | `TestFalsePeaceSkipsTheTargetsNextCombats` | all combat phases of the next turn only (False Peace, Empty City Ruse); other SkipPhase shapes are a loud Note |
| 614.10a-614.10b | unknown | - | - | not audited |
| 614.11 | follows | `engine/rules/replacement.go#Engine.applyNonMoveReplacements` `engine/rules/replacement_match.go#drawMatchAmount` | `TestLivingConundrumSkipsTheDrawOnAnEmptyLibrary` `TestThoughtReflectionDrawsMeetNotionThief` | draw replaced on an empty library |
| 614.11a-614.11b | unknown | - | - | not audited |
| 614.12 | follows | `engine/rules/replacement_match.go#Engine.activeZonesGateOK` `engine/effects/restrictions.go#replacementMovedBodyLive` `engine/rules/replacement_match.go#Engine.replacementMatchesRememberedUngatedBy` `engine/rules/replacement_updated.go#Engine.entersUntappedApplies` `engine/rules/replacement_match.go#Engine.enteringObjectMatches` | `TestEffectReplacementEntersTappedLightstall` `TestTokenCopyOfExiledCloneAppliesElection` `TestNahiriTokenCopyOfVisageBanditAppliesElection` `TestTokenCopyOfCloneCardDeclineEntersAsItself` `TestEffectTappedEntryYieldsToEntersUntapped` `TestOrbOfDreamsTapsAnEntryFromAnyZone` `TestOrbOfDreamsTapsAFaceDownInstant` | Effect-delivered enters-tapped replacement (Lightstall Inquisitor); a token copy of a clone-like card applies its recorded election (Mimic Vat, Nahiri) |
| 614.12a | follows | `engine/rules/replacement_etb.go#Engine.applyETBChoiceReplacement` | - | choices before entering |
| 614.12b | unknown | - | - | - |
| 614.12c | unknown | - | - | - |
| 614.13-614.13c | unknown | - | - | not reached |
| 614.14 | unknown | - | - | - |
| 614.15 | follows | `engine/effects/registry.go#Resolve` | - | self-replacement inline in effect bodies before emit |
| 614.16 | deviates | `engine/rules/replacement_match.go#Engine.replacementMatchesRememberedUngatedBy` | - | cost counters doubled if stack non-empty |
| 614.17 | follows | `engine/rules/replacement.go#Engine.applyReplacementsDispatch` | `TestCR614CantEnterBattlefieldPrevention` | - |
| 614.17a | follows | `engine/rules/replacement.go#Engine.applyReplacementsDispatch` | - | checked at event time; not audited separately |
| 614.17b | unknown | - | - | - |
| 614.17c | unknown | - | - | - |
| 614.17d | follows | `engine/rules/replacement.go#Engine.applyReplacementsDispatch` | `TestCR614CantEnterBattlefieldPreventionKunoros` | - |
| 615.1-615.1a | follows | `engine/rules/replacement.go#Engine.applyNonMoveReplacements` | - | - |
| 615.2 | follows | `engine/rules/replacement_damage.go#Engine.damageReplacementMatches` | - | as 609.7 |
| 615.3 | follows | `engine/effects/prevent_damage.go#effPreventDamage` | - | - |
| 615.4 | follows | `engine/effects/prevent_damage.go#effPreventDamage` | - | shields registered at resolution |
| 615.5 | partial | `engine/effects/prevent_damage.go#effPreventDamage` `engine/rules/replacement_damage.go#Engine.runPreventionShieldRider` | `TestVengefulArchonShieldRiderDealsPrevented` | unbound rider scope LOUD |
| 615.6 | follows | `engine/rules/replacement.go#Engine.applyNonMoveReplacements` | - | - |
| 615.7 | partial | `engine/effects/prevent_damage.go#effPreventDamage` | - | no choice across simultaneous sources |
| 615.8 | follows | `engine/effects/effect.go#effEffect` `engine/rules/replacement.go#Engine.applyReplacement` | - | CoP:Red Effect exiles itself after first use |
| 615.9 | follows | `engine/rules/replacement_damage.go#Engine.damageReplacementMatches` | - | ValidSource recheck at damage time |
| 615.10 | follows | `engine/rules/replacement.go#Engine.applyNonMoveReplacements` | - | per-event ReplaceDamage |
| 615.11 | follows | `engine/effects/prevent_damage.go#effPreventDamage` | - | per-recipient shields |
| 615.12 | deviates | `engine/rules/replacement_damage.go#Engine.applicableDamageReplacements` `engine/rules/replacement.go#Engine.applyNonMoveReplacements` | - | shields skipped; riders never run |
| 615.12a | follows | `engine/rules/replacement_damage.go#Engine.applicableDamageReplacements` | - | follows from skip (applied 0 times) |
| 615.13 | follows | `engine/rules/replacement_damage.go#Engine.applyReplaceDamageTail` | `TestDamagePreventedOnceFiresPerPrevention` | - |
| 616.1 | follows | `engine/rules/replacement.go#Engine.replacementAskPlayer` `engine/rules/resolve_mayask.go#tapeTextMayAsk` `engine/rules/replacement.go#Engine.applyReplacementsDispatch` | `TestCR616AffectedControllerChoosesReplacement` `TestMayAskSeesASpellsOwnMove` `TestParkedDeathsAskOncePerCreature` | affected player/controller orders |
| 616.1a | follows | `engine/effects/registry.go#Resolve` | - | structural: self-replacement inline before emit |
| 616.1b | not-impl | `engine/cards/kw_etbreplacement.go#kwETBReplacement` | - | SILENT; Layer$ Control never read |
| 616.1c | not-impl | `engine/cards/kw_etbreplacement.go#kwETBReplacement` | - | SILENT; ETBReplacement Copy layer dropped |
| 616.1d | unknown | - | - | no corpus layer tag; not reached |
| 616.1e | follows | `engine/rules/replacement.go#Engine.applyReplacementsDispatch` | - | - |
| 616.1f | follows | `engine/rules/replacement_updated.go#Engine.redirectRecheck` `engine/rules/replacement_updated.go#Engine.applyRedirectReplacements` | - | others reconsidered after one applies |
| 616.1g | unknown | - | - | - |
| 616.2 | follows | `engine/rules/replacement_updated.go#Engine.composeUpdatedReplacements` `engine/rules/replacement_updated.go#Engine.redirectRecheck` | `TestThoughtReflectionDrawsMeetNotionThief` | - |
