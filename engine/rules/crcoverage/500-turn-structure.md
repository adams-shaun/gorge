# CR 500s: Turn structure

Coverage of this chapter of the Comprehensive Rules (2026-09-25) by gorge. Seeded from the CR audit of 2026-10-20 at `72dabd96c`; kept current by every change that touches a rule. See [README](README.md) for the status vocabulary and the reference syntax.

| rule | status | code | tests | notes |
|---|---|---|---|---|
| 500.1 | follows | `engine/rules/turn.go#Engine.advanceStep` | - |  |
| 500.2 | follows | `engine/rules/legal_priority.go#Engine.handlePriority` | - |  |
| 500.3 | follows | `engine/rules/turn.go#Engine.priorityRound` `engine/rules/turn.go#Engine.finishUntapStep` | - |  |
| 500.4 | partial | `engine/rules/layers_effect.go#Engine.AddContinuous` | `TestCloneUntilYourNextTurnSurvivesCleanupThenExpires` | until-your-next-turn expires at prior cleanup; early only in 514.3a window |
| 500.5 | follows | `engine/rules/turn.go#Engine.finishStepBoundary` | `TestFirebendingManaSurvivesCombatStepsAndEmptiesAtEndOfCombat` |  |
| 500.5a | follows | `engine/rules/turn.go#Engine.finishStepBoundary` | `TestCR511UntilEndCombatPumpExpiresBeforeMain` |  |
| 500.5b | follows | `engine/rules/layers_effect.go#Engine.EndOfTurnCleanup` | - |  |
| 500.6 | follows | `engine/rules/trigmatch/actions.go#phaseTriggerMatches` | - |  |
| 500.7 | follows | `engine/rules/turn.go#Engine.advanceStep` `engine/events/apply_turn.go#foldExtraTurn` | `TestFinalFortuneGrantsAndSpendsAnExtraTurn` |  |
| 500.8 | partial | `engine/rules/turn.go#Engine.extraPhaseBoundary` `engine/events/apply_turn.go#foldExtraPhase` | `TestCR500_8MostRecentExtraPhaseOccursFirst` `TestAureliaExtraCombatSplicesASecondCombat` | most recent first; a grant's FollowedBy$ main is skipped when an older grant at the same splice point follows it |
| 500.9 | follows | `engine/effects/addphase.go#parseExtraPhaseValue` `engine/rules/turn.go#Engine.extraPhaseBoundary` | - | steps ride the AddPhase grant queue |
| 500.10 | follows | `engine/effects/addphase.go#effAddPhase` | `TestAggravatedAssaultExtraCombatFollowedByMain2` |  |
| 500.10a | not-impl | `engine/effects/addphase.go#effAddPhase` | - | SILENT; grant taken on another player's turn (Obeka) |
| 500.11 | follows | `engine/rules/replacement_phase.go#Engine.applyBeginPhaseReplacement` `engine/effects/dice.go#effSkipTurn` | `TestChainedUntapAndUpkeepSkips` |  |
| 500.12 | follows | `engine/rules/turn.go#Engine.setStep` | - |  |
| 501.1 | follows | `engine/rules/turn.go#Engine.advanceStep` | - |  |
| 502.1 | follows | `engine/rules/turn.go#Engine.finishUntapStep` | `TestCR702PhasedOutPermanentPhasesInAtUntap` `TestCR702PhasingKeywordTogglesAtUntapStep` |  |
| 502.2 | follows | `engine/rules/daynight.go#Engine.dayNightUntapCheck` `engine/rules/turn.go#Engine.finishUntapStep` | `TestCR731DayBecomesNightWhenThePreviousActivePlayerCastNothing` `TestCR731NightBecomesDayAfterTwoSpells` | after the phasing scan; a skipped untap step skips it |
| 502.2a | n/a | - | - | shared team turns not offered |
| 502.3 | follows | `engine/rules/turn.go#Engine.finishUntapStep` | `TestBasaltMonolithStaysTappedThroughItsUntapStep` |  |
| 502.4 | follows | `engine/rules/turn.go#Engine.priorityRound` | - |  |
| 503.1 | follows | `engine/rules/turn.go#Engine.finishEnteredStep` | - |  |
| 503.1a | follows | `engine/rules/trigger_queue.go#Engine.putTriggersOnStack` | - |  |
| 503.2 | unknown | - | - | not reached |
| 504.1 | follows | `engine/rules/turn.go#Engine.drawStepTurnAction` | `TestDrawStepDeckOutEndsTheGameBeforePriority` |  |
| 504.2 | follows | `engine/rules/turn.go#Engine.priorityRound` | `TestDrawTriggerResolvingInTheDrawStepDoesNotRedraw` |  |
| 505.1 | follows | `engine/rules/turn.go#Engine.advanceStep` | - |  |
| 505.1a | follows | `engine/state/ids.go#StepMain1` | - |  |
| 505.1b | unknown | - | - | not reached |
| 505.2 | follows | `engine/rules/legal_priority.go#Engine.handlePriority` | - |  |
| 505.3 | n/a | - | - | Archenemy not supported |
| 505.4 | follows | `engine/rules/saga.go#Engine.advanceSagas` `engine/rules/turn.go#Engine.finishEnteredStep` | `TestDoublingSeasonSagaEntryAndProgressionCounts` |  |
| 505.5 | not-impl | - | - | LOUD; Attractions unsupported |
| 505.6 | follows | `engine/rules/turn.go#Engine.priorityRound` | - |  |
| 505.6a | follows | `engine/rules/legal.go#Engine.sorcerySpeed` `engine/rules/statics_cast.go#Engine.spellTimingOK` | `TestAbzanBattlePriestOutlastIsSorcerySpeedOnly` |  |
| 505.6b | follows | `engine/rules/legal_mayplay.go#Engine.adjustLandPlays` `engine/rules/cast_landplay.go#Engine.finishLandPlay` | `TestAdjustLandPlaysResetsAtUntap` |  |
| 506.1 | follows | `engine/rules/turn.go#Engine.advanceStep` `engine/rules/combat.go#Engine.enterEndCombat` | `TestCR508EmptyAttackSkipsBlockersAndDamage` |  |
| 506.2 | follows | `engine/rules/combat.go#Engine.askAttackers` `engine/rules/combat/attack.go#AttackableCreature` | `TestBattleIsOfferedAsAnAttackerOption` |  |
| 506.2a | follows | `engine/rules/combat.go#Engine.askAttackers` | - | gorge: attack-multiple-players, every opponent defends |
| 506.2b | n/a | - | - | shared team turns not offered |
| 506.3 | follows | `engine/rules/combat/attack.go#CanAttack` `engine/rules/combat/block.go#CanBlock` | - |  |
| 506.3a | follows | `engine/rules/combat_removal.go#Engine.removeNoncreatureCombatants` `engine/events/apply_combat.go#foldTokenAttacks` | - | noncreature entrant removed in the SBA-loop scan |
| 506.3b | partial | `engine/effects/token.go#applyTokenMintRiders` `engine/events/apply_combat.go#foldTokenAttacks` | - | non-active controller's enters-attacking not rejected (D17) |
| 506.3c | follows | `engine/effects/entering_defender.go#EnteringAttackLegal` `engine/events/apply_combat.go#foldTokenAttacks` | `TestCR508_4bNinjaDoesNotAttackADepartedPlayer` `TestCR508_4bNinjaDoesNotAttackAPlaneswalkerThatChangedControl` `TestCopyPermanentTokenAttackingSkipsADepartedDefender` | every enters-attacking emitter re-checks its defender |
| 506.3d | follows | `engine/events/apply_combat.go#foldTokenAttacks` `engine/rules/combat.go#Engine.damageStep` | - | empty BlockedBy = unblocked |
| 506.3e | not-impl | - | - | SILENT: TokenBlocking$ unread (D11, Brimaz) |
| 506.3f-506.3g | not-impl | - | - | SILENT but unreachable: no printed creature battle; a battle becomes a creature only via an Animate ValidTgts$ Permanent (4 lines) that ends on a zone change (cr-g55 census) |
| 506.4 | partial | `engine/events/apply_zone.go#move` `engine/events/apply_helpers.go#changeControl` `engine/rules/combat_removal.go#Engine.removeNoncreatureCombatants` | `TestCR506ReturnedBlockerIsRemovedFromCombat` `TestCR702PhasedOutAttackerIsRemovedFromCombat` | attacked planeswalker stays attacked after control change or type loss (D9) |
| 506.4a | follows | `engine/rules/combat.go#Engine.validateAttackDeclarationChosen` `engine/rules/combat_removal.go#Engine.removeNoncreatureCombatants` | - | restrictions read only at declaration |
| 506.4b | follows | `engine/rules/combat.go#Engine.damageStep` | - | damage pass ignores tapped state |
| 506.4c | follows | `engine/rules/combat.go#Engine.combatDefenderTarget` | - | departed planeswalker/battle: attacker stays attacking, deals no damage |
| 506.4d-506.4e | partial | `engine/rules/combat_removal.go#Engine.removeNoncreatureCombatants` `engine/rules/combat.go#Engine.combatDefenderTarget` | - | REVISED from unknown: blocker half via noncreature scan; attacked type loss not modelled (D9) |
| 506.5 | follows | `engine/rules/trigmatch/combat.go#AttacksMatches` `engine/rules/trigmatch/combat.go#declaredAttackerCount` `engine/rules/combat.go#Engine.finishAttackers` `engine/rules/combat.go#Engine.validateAloneDeclaration` | `TestCR506_5AttacksAloneSpansTheWholeDeclaration` `TestCR506_5LoneAttackerStillExalted` `TestExaltedPumpsALoneAttackerAndProwessPumpsOnNoncreatureSpells` `TestCantAttackOrBlockAlone` | - |
| 506.6 | follows | `engine/rules/trigmatch/combat.go#AttackersDeclaredMatchedAttackers` `engine/rules/trigmatch/combat.go#attackedDefenderMatches` | `TestAttackersDeclaredOneTargetStillFiresPerDefender` `TestCR508_3PlaneswalkerAttackIsNotAnAttackOnItsController` | a planeswalker/battle attack is not a player attack |
| 506.7 | follows | `engine/rules/combat.go#Engine.finishAttackers` `engine/events/apply_combat.go#foldHadToAttack` `engine/effects/filter.go#predicates` | `TestCR506_7FirkraagRewardsARequiredAttacker` `TestCR506_7FirkraagIgnoresAFreeAttacker` `TestCR506_7HadToAttackPredicateIsRegistered` | recorded at declaration from the requirement set (HadToAttack event); hadToAttackThisCombat reads it under the combat stamp |
| 506.8 | follows | `engine/rules/activation_phases.go#Engine.activationPhasesOK` | `TestActivationAfterBlockersExcludesEarlierSteps` |  |
| 506.8a-506.8e | follows | `engine/rules/activation_phases.go#Engine.activationPhasesOK` | `TestBerserkFirstCombatOnly` `TestActivationAfterBlockersTrapRunnerOfferedOnceBlockersDeclared` | ActivationFirstCombat$ reads CombatsThisTurn |
| 506.8f | deviates | `engine/rules/activation_phases.go#Engine.activationPhasesOK` | - | D13: after-blockers window admitted in end of combat when blockers skipped |
| 506.8g | follows | `engine/rules/activation_phases.go#Engine.activationPhasesOK` | `TestActivationPhasesUpkeepAbilityOfferedOnlyInUpkeep` |  |
| 507.1 | n/a | - | - | gorge: every opponent is a defending player (no choice) |
| 507.2 | follows | `engine/rules/turn.go#Engine.step` `engine/rules/turn.go#Engine.priorityRound` | `TestKamahlBeginCombatTriggerFiresOnlyAtBeginCombat` |  |
| 508.1 | follows | `engine/rules/combat.go#Engine.askAttackers` `engine/rules/combat.go#Engine.handleAttackers` | - |  |
| 508.1a | follows | `engine/rules/combat/attack.go#AttackableCreature` | - | battle-creature exclusion absent (see 506.3f-g) |
| 508.1b | follows | `engine/rules/combat.go#Engine.askAttackers` `engine/rules/combat/attack.go#CanAttackPair` | `TestBattleIsOfferedAsAnAttackerOption` | GorgeMustAttack mirrors gorge |
| 508.1c | partial | `engine/rules/combat/restrictions.go#AttackBlocked` `engine/rules/combat.go#Engine.validateAttackDeclarationChosen` | `TestCantAttackConditionGate` | unwhitelisted restriction shapes skipped (AGENTS combatrestriction1, D15) |
| 508.1d | partial | `engine/rules/combat.go#Engine.mustAttackRequired` `engine/rules/combat/attack.go#AttackRequirements` | `TestCR508CorpusRequirementsUnderAttackRestriction` | RequiredQuota counts creatures, not requirements (D15) |
| 508.1e | not-impl | - | - | LOUD: banding unregistered |
| 508.1f | follows | `engine/rules/combat.go#Engine.finishAttackers` `engine/rules/host_emit.go#Engine.emitTap` | - | D16 minor: taps emitted after attack triggers are checked |
| 508.1g | follows | `engine/rules/combat.go#Engine.startExertAsks` | - | exert election |
| 508.1h-508.1j | follows | `engine/rules/attack_cost.go#Engine.attackPairCharge` `engine/rules/attack_cost.go#Engine.openCombatPayPlan` `engine/rules/attack_cost.go#Engine.payCombatExtras` | `TestGhostlyPrisonChargesPerAttackerFromThePool` `TestAtomicChargePaysGenericAndPipFromOnePool` `TestAttackWindowCountsChosenColour` |  |
| 508.1k | follows | `engine/rules/combat.go#Engine.finishAttackers` `engine/events/apply_combat.go#foldDeclareAttackers` | - |  |
| 508.1m | follows | `engine/rules/combat.go#Engine.finishAttackers` | `TestAttacksTriggerFiresForTheDeclaredAttacker` |  |
| 508.2 | follows | `engine/rules/turn.go#Engine.step` `engine/rules/turn.go#Engine.priorityRound` | `TestCR508PriorityAfterAttackDeclaration` |  |
| 508.2a | follows | `engine/rules/trigmatch/combat.go#AttacksMatches` | `TestAttacksTriggerDoesNotFireForANonAttacker` | keyed on the DeclareAttackers event only |
| 508.2b | follows | `engine/rules/combat.go#Engine.finishAttackers` `engine/rules/turn.go#Engine.priorityRound` | - |  |
| 508.3 | follows | `engine/rules/trigmatch/combat.go#AttacksMatches` `engine/rules/trigmatch/combat.go#AttackersDeclaredMatchedAttackers` | - | umbrella for 508.3a-f |
| 508.3a-508.3b | follows | `engine/rules/trigmatch/combat.go#AttacksMatches` `engine/rules/trigmatch/combat.go#attackedDefenderMatches` `engine/effects/player_filter.go#SplitDefenderSpec` | `TestAttacksAttackedOpponentScopesToTheTriggersOpponent` `TestCR508_3PlaneswalkerAttackIsNotAnAttackOnItsController` `TestCR508_3PlayerAttackStillTriggers` | player alternatives match player events, Planeswalker./Battle alternatives the attacked permanent |
| 508.3c-508.3d | follows | `engine/rules/trigmatch/combat.go#AttacksMatches` `engine/rules/trigmatch/combat.go#AttackersDeclaredMatchedAttackers` | `TestAttackersDeclaredBatchFiresOnceOnSplitAttack` |  |
| 508.3e | follows | `engine/rules/trigmatch/combat.go#AttackersDeclaredMatchedAttackers` `engine/rules/trigmatch/combat.go#attackedDefenderMatches` | `TestCR508_3PlaneswalkerAttackIsNotAnAttackOnItsController` |  |
| 508.3f | follows | `engine/rules/trigger_blocks.go#Engine.checkAttackerUnblockedTriggers` | `TestAttackerUnblockedValidCardAndDefenderGates` |  |
| 508.4 | partial | `engine/effects/token.go#tokenAttackingRider` `engine/events/apply_combat.go#foldTokenAttacks` | `TestCR508_4SigiledSwordKnightEntersAttackingUntapped` `TestCR508_4HeroOfBladeholdSoldiersEnterTappedAndAttacking` `TestAdelineTokenAttackingRememberedPlayerAttacksThatOpponent` | enters tapped only when its text says so; the controller does not choose the defender yet (cr-g11b) |
| 508.4a-508.4b | follows | `engine/effects/entering_defender.go#EnteringAttackLegal` | `TestCR508_4bNinjaDoesNotAttackADepartedPlayer` `TestCR508_4bNinjaDoesNotAttackAPlaneswalkerThatChangedControl` `TestSneakPreservesBattleDefenderThroughEntry` `TestCopyPermanentTokenAttackingSkipsADepartedDefender` `TestEnteringAttackLegalBattleProtector` `TestEnteringAttackLegalBattleProtectorLeft` | departed player, gone/stolen/retyped planeswalker, unprotected battle, battle whose protector left |
| 508.4c | follows | `engine/events/apply_combat.go#foldTokenAttacks` | - | entry bypasses declaration validation |
| 508.4d | follows | `engine/events/apply_combat.go#foldTokenAttacks` | - | no BlockedBy recorded, so unblocked |
| 508.5-508.5a | partial | `engine/effects/trigger_referents.go#TriggerContext.DefendingPlayer` `engine/state/object.go#Object.Attacking` | `TestAttacksTriggerRemembersTheDefendingPlayer` | REVISED from unknown: triggers bind attacked seat (Attacking survives removal); non-trigger readers not traced |
| 508.6 | follows | `engine/effects/player_filter.go#attackedThisTurn` | `TestAttackedThisTurnTruthTable` |  |
| 508.7-508.7e | partial | `engine/effects/changecombatants.go#effChangeCombatants` `engine/events/apply_combat.go#foldCombatRetarget` | `TestChangeCombatantsAnsweredRetargetsAndUnblocksKernel` | D12: no planeswalker reselect; AttackingBattle kept (partly LOUD) |
| 508.8 | partial | `engine/rules/turn.go#Engine.advanceStep` | `TestCR508EmptyAttackSkipsBlockersAndDamage` | D8: creatures put onto battlefield attacking don't stop the skip |
| 509.1 | follows | `engine/rules/combat.go#Engine.askBlockers` `engine/rules/combat.go#Engine.handleBlockers` | - |  |
| 509.1a | follows | `engine/rules/combat/block.go#CanBlock` `engine/rules/combat.go#Engine.validateBlockers` | `TestCR509OneBlockerCannotBlockTwoAttackers` |  |
| 509.1b | partial | `engine/rules/combat/restrictions.go#BlockRestricted` `engine/rules/combat.go#Engine.blockPairScopeFor` | `TestCR509MinMaxBlockerCountRestriction` `TestCantBlockRemovesEveryBlockOption` | LOUD: Effect statics CantBlock/MinMaxBlocker Noted (AGENTS row) |
| 509.1c | partial | `engine/rules/combat/block.go#MustBlockCandidates` `engine/effects/mustblock_api.go#effMustBlock` `engine/rules/combat.go#Engine.askBlockers` `engine/decision/block_required.go#Decision.blockRequiredCore` | `TestCrashingBoarsMustBlockChoicePool` `TestCR509_1cLurePrintedKeyword` `TestCR509_1cLureRuntimeGrant` `TestCR509_1cLureExemptsTappedAndCostedBlockers` `TestMustBlockBoundDutyCountsOnlyItsAttacker` | Lure (kw:AllMustBlock, KW$ grants) binds each able, cost-free blocker to the attacker; printed S: AddHiddenKeyword$ Lure statics (8 files) and MustBeBlockedBy/MustBeBlockedByAll unread; RequiredQuota counts creatures (D15) |
| 509.1d-509.1f | follows | `engine/rules/attack_cost.go#Engine.blockPairCharge` `engine/rules/attack_cost.go#Engine.blockChargeOf` `engine/rules/attack_cost.go#Engine.payCombatExtras` | `TestQalSismaBehemothBlockCharge` |  |
| 509.1g | follows | `engine/rules/combat.go#Engine.handleBlockers` `engine/events/apply_combat.go#foldDeclareBlockers` | - |  |
| 509.1h | follows | `engine/events/apply_combat.go#foldDeclareBlockers` `engine/events/apply_zone.go#move` `engine/effects/filter.go#predicates` | `TestAttackerWhoseBlockerIsRemovedDealsNoDamageWithoutTrample` `TestBecomesBlockedByEffect` `TestBlockedFilterProperty` | zero tombstone keeps the attacker blocked |
| 509.1i | follows | `engine/rules/trigger_blocks.go#Engine.checkBlocksTriggers` `engine/rules/trigger_blocks.go#Engine.checkAttackerBlockedTriggers` | `TestBlocksTriggerFiresPerPair` |  |
| 509.2 | follows | `engine/rules/turn.go#Engine.step` `engine/rules/turn.go#Engine.priorityRound` | `TestCR509PriorityAfterBlockDeclaration` |  |
| 509.2a | follows | `engine/rules/turn.go#Engine.step` | - |  |
| 509.3 | follows | `engine/rules/trigger_blocks.go#Engine.checkBlocksTriggers` `engine/rules/trigger_blocks.go#Engine.checkAttackerBlockedTriggers` | - | umbrella for 509.3a-g |
| 509.3a-509.3c | follows | `engine/rules/trigger_blocks.go#Engine.checkBlocksTriggers` `engine/rules/trigger_blocks.go#Engine.checkAttackerBlockedTriggers` | `TestBlocksTriggerFiresPerPair` `TestAttackerBlockedCountsEachBlockingCreature` |  |
| 509.3d | partial | `engine/rules/trigger_blocks.go#Engine.attackerBlockedByPairCandidates` | `TestAttackerBlockedByCreatureBlockerRoleFires` | no creature enters blocking, so that trigger case can't arise |
| 509.3e-509.3f | follows | `engine/rules/trigger_blocks.go#Engine.checkAttackerBlockedTriggers` | `TestAttackerBlockedBlockerAmount` | evaluated at the declaration event |
| 509.3g | follows | `engine/rules/trigger_blocks.go#Engine.checkAttackerUnblockedTriggers` `engine/rules/trigger_blocks.go#Engine.checkAttackerUnblockedOnceTriggers` | `TestAttackerUnblockedValidCardAndDefenderGates` |  |
| 509.4-509.4b | not-impl | - | - | SILENT: TokenBlocking$ unread (D11) |
| 510.1 | follows | `engine/rules/combat.go#Engine.combatStep` `engine/rules/combat.go#Engine.damageStep` | - |  |
| 510.1a | follows | `engine/rules/statics_assignment.go#Engine.combatDamageAmount` `engine/rules/combat.go#Engine.damageStep` | `TestAssaultFormationUnblockedAttackerDealsToughnessToPlayer` | power 0 or less assigns nothing |
| 510.1b | follows | `engine/rules/combat.go#Engine.combatDefenderTarget` | `TestAttackingPlayerStillDealsLifeDamage` `TestBattleAttackDealsDefenseCounterDamage` |  |
| 510.1c | partial | `engine/rules/combat.go#Engine.divisionNeeding` `engine/rules/combat.go#Engine.damageStep` `engine/rules/combat.go#Engine.lethalDamageNeed` | `TestCR510ControllerChoosesMultiBlockDamageDivision` `TestFirstStrikeAttackerTakesSurvivingBlockersRegularDamage` | D5: trample assignment automatic |
| 510.1d | deviates | `engine/rules/combat.go#Engine.damageStep` | - | D4: multi-blocker deals full power to each attacker |
| 510.1e | follows | `engine/rules/combat.go#Engine.divisionOptions` `engine/rules/combat.go#Engine.handleDamageDivision` | - | only legal divisions are offered |
| 510.2 | partial | `engine/rules/combat.go#Engine.runCombatAssignments` | `TestFirstStrikeReplacementOrderSettlesBeforeRegularPass` | D14: replacement-order ask splits the pass with SBAs between |
| 510.3 | follows | `engine/rules/combat.go#Engine.regularDamageRound` | `TestCR510PriorityBetweenDoubleStrikeDamageSteps` |  |
| 510.3a | follows | `engine/rules/combat.go#Engine.completeCombatPass` `engine/rules/combat.go#Engine.regularDamageRound` | `TestFirstStrikeDiesTriggerResolvesBeforeRegularDamage` |  |
| 510.4 | partial | `engine/rules/turn.go#Engine.advanceStep` `engine/rules/combat.go#Engine.actsThisDamageStep` `engine/rules/combat.go#Engine.collectFirstStrikers` | `TestCR510PriorityBetweenDoubleStrikeDamageSteps` `TestFirstStrikeGainedAfterTheFirstStepStillDealsRegularDamage` | D7: one gorge step, mana pools not emptied between passes |
| 511.1 | follows | `engine/rules/combat.go#Engine.enterEndCombat` | `TestCR511AttackerPersistsThroughEndCombatStep` |  |
| 511.2 | partial | `engine/rules/layers_effect.go#Engine.continuousLive` `engine/rules/layers_effect.go#Engine.expireUntilEndOfCombat` `engine/rules/turn.go#Engine.finishStepBoundary` `engine/rules/trigmatch/actions.go#phaseTriggerMatches` | `TestCR511UntilEndCombatAppliesFromCreation` `TestCR511UntilEndCombatDoesNotReturnInExtraCombat` `TestCR511UntilEndCombatPumpExpiresBeforeMain` `TestFrostbeastEndCombatTriggerFiresOnlyAtEndCombat` | continuous effects follow; UntilEndOfCombat delayed triggers still step-keyed (trigger_delayed.go); one created postcombat ends at cleanup |
| 511.3 | follows | `engine/rules/turn.go#Engine.finishStepBoundary` `engine/events/apply_combat.go#foldEndCombatReset` | `TestLeavingEndCombatRemovesAttackerBeforePostcombatMain` `TestEndCombatResetClearsIsAttackingAndBlockedBy` |  |
| 512.1 | follows | `engine/rules/turn.go#Engine.advanceStep` | - |  |
| 513.1 | follows | `engine/rules/turn.go#Engine.step` `engine/rules/trigmatch/actions.go#phaseTriggerMatches` | `TestBraidsEndOfTurnTriggerFiresOnlyAtTheEndStep` |  |
| 513.1a | n/a | - | - | errata note only |
| 513.2 | follows | `engine/rules/trigmatch/actions.go#phaseTriggerMatches` | - | step triggers fire on the StepChange only |
| 514.1 | follows | `engine/rules/combat.go#Engine.cleanupBody` `engine/rules/combat.go#Engine.discardCleanup` `engine/rules/combat.go#Engine.maxHandSizeFor` | `Test3141DiscardDownToSevenIsAsked` `Test3141OnlyActivePlayerDiscards` |  |
| 514.2 | follows | `engine/rules/combat.go#Engine.cleanupBody` `engine/rules/layers_effect.go#Engine.EndOfTurnCleanup` | `TestCleanupClearsCombatDamageDeathtouchAndUntilEOTEffects` `TestEndOfTurnCleanupDropsUntilEOTEffects` `TestCR612UntilEndOfTurnTextChangeEndsAtCleanup` | until-end-of-turn text changes (ChangeText without Duration$ Permanent) end here too |
| 514.3 | follows | `engine/rules/turn.go#Engine.finishCleanupStep` | `TestCleanupEmptyStackPassRepeatsCleanup` |  |
| 514.3a | follows | `engine/rules/turn.go#Engine.finishCleanupStep` `engine/rules/turn.go#Engine.repeatCleanup` | `TestCleanupChecksStateBasedActionsAfterEndingEffects` `TestCleanupTriggerResolutionGivesActivePlayerPriority` `Test3143SecondCleanupStep` |  |
