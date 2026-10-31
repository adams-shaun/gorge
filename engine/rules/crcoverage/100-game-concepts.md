# CR 100s: Game concepts

Coverage of this chapter of the Comprehensive Rules (2026-09-25) by gorge. Seeded from the CR audit of 2026-10-20 at `72dabd96c`; kept current by every change that touches a rule. See [README](README.md) for the status vocabulary and the reference syntax.

| rule | status | code | tests | notes |
|---|---|---|---|---|
| 100.1-100.1b | follows | `engine/rules/engine.go#Config.Names` | - | gorge 2-8 seats |
| 100.2-100.3 | follows | `engine/rules/rng.go#rng` `engine/effects/dice.go#effRollDice` | - | deck supplied by host; coin/die via seeded rng |
| 100.4-100.4d | follows | `engine/rules/genesis.go#Engine.genesisDeal` `engine/state/ids.go#ZSideboard` | - |  |
| 100.5-100.7 | n/a | - | - | deck legality / tournament / casual-only cards |
| 101.1 | unknown | - | - | general principle; not reached |
| 101.2-101.2a | follows | `engine/rules/cantlose.go#Engine.gameWinPrevented` | - | "can't" funnels |
| 101.3-101.4e | partial | `engine/effects/zone_bulk.go#effSacrifice` `engine/effects/cardflow.go#effDiscard` | `TestEachPlayerSacrificesChoosesThenMovesTogether` `TestEachPlayerSacrificesReplays` `TestEachPlayerDiscardsChoosesThenMovesTogether` | each player's sacrifice or discard chooses in order and moves together; other per-player effects not reached |
| 102.1-102.2 | follows | `engine/state/game.go#Game.Active` | - | - |
| 102.3-102.4 | n/a | - | - | no team modes |
| 103.1 | follows | `engine/rules/starting_player_choice.go#TossWinner` `engine/rules/starting_player_choice.go#Engine.AskStartingPlayer` | `TestMulliganRoundAsksTheTossWinnerFirst` |  |
| 103.1a-103.1c | n/a | - | - | shared-team/Archenemy/Power Play absent |
| 103.2-103.2a | follows | `engine/rules/genesis.go#Engine.genesisDeal` | - | sideboard set aside |
| 103.2b | not-impl | `engine/rules/trigger_match.go#init` | `TestCompanionPrimitiveIsRegistered` | companion reveal absent; kw counted supported: SILENT |
| 103.2c | follows | `engine/rules/genesis.go#Engine.genesisDeal` | - | commander to command zone |
| 103.2d-103.2e | n/a | - | - | no stickers/Conspiracy |
| 103.3-103.3a | follows | `engine/rules/genesis.go#Engine.genesisDeal` | - | shuffle precedes the choice; outcome-equivalent; no supplementary decks |
| 103.4 | follows | `engine/rules/genesis.go#newEngineShell` `server/host/match.go#Registry.newMatchWith` | - | 20 / Commander 40 |
| 103.4a-103.4e | n/a | - | - | 2HG/Vanguard/Brawl/Archenemy absent (103.4c follows) |
| 103.5-103.5a | deviates | `engine/rules/mulligan.go#newMulliganRound` `engine/rules/mulligan.go#Engine.stepPregame` `server/gorged/gorged.go#serveFlags` | `TestMulliganRedrawWaitsForAllSeats` `TestPregameSkippedWhenMulligansZero` | allowance cap (0 default, 1 served); bottoming deferred |
| 103.5b | not-impl | - | - | SILENT; Serum Powder text only |
| 103.5c | deviates | `engine/rules/mulligan.go#newMulliganRound` | `TestCR800MultiplayerFirstMulliganIsFree` | free mulligan counts toward allowance |
| 103.5d | n/a | - | - | shared team turns absent |
| 103.6-103.6c | follows | `engine/rules/opening_hand.go#Engine.stepOpening` `engine/rules/opening_hand.go#Engine.applyOpeningEffect` | `TestGemstoneCavernsIsOfferedOnlyAfterMulligans` | opening-hand actions after mulligans |
| 103.7 | deviates | `engine/rules/genesis.go#Engine.genesisDeal` | - | every seat reveals own plane; no phenomenon skip |
| 103.8-103.8a | follows | `engine/rules/turn.go#skipFirstDrawStep` | `TestFirstTurnDrawStepTriggerDoesNotFireInTwoPlayer` | - |
| 103.8b | n/a | - | - | no 2HG |
| 103.8c | follows | `engine/rules/turn.go#skipFirstDrawStep` | `TestCR800StartingPlayerDrawsInMultiplayer` | - |
| 104.1 | follows | `engine/rules/sba.go#Engine.checkGameOver` | - | - |
| 104.2-104.2a | follows | `engine/rules/sba.go#Engine.checkGameOver` `engine/rules/sba.go#Engine.gameEndsNow` | `TestCR104_2aLastPlayerWinsDespiteCantWin` `TestHeraldOfEternalDawnDoesNotPreventConcede` | - |
| 104.2b | follows | `engine/rules/cantlose.go#Engine.playerWins` | `TestHeraldOfEternalDawnPreventsOpponentWin` | - |
| 104.2c-104.2d | n/a | - | - | teams/Emperor absent |
| 104.3-104.3a | partial | `engine/rules/legal_priority.go#Engine.handlePriority` | `TestConcedeAtPriorityEndsTheGameForTheOtherSeat` `TestConcedeIsNotOfferedOnANonPriorityDecision` | gorge: concede only at priority |
| 104.3b | follows | `engine/rules/sba.go#Engine.checkLoseConditions` | - | 0 life SBA |
| 104.3c | follows | `engine/rules/sba.go#Engine.checkLoseConditions` `engine/rules/cantlose.go#Engine.markDrawEmpty` | `TestDrawStepDeckOutEndsTheGameBeforePriority` | empty-library draw |
| 104.3d | follows | `engine/rules/sba.go#Engine.checkLoseConditions` | - | no poison in pools |
| 104.3e | follows | `engine/rules/cantlose.go#Engine.playerLoses` `engine/effects/dice.go#effLosesGame` | - | LosesGame effect |
| 104.3f | unknown | - | - | not reached |
| 104.3g-104.3i | n/a | - | - | teams/range of influence/Emperor |
| 104.3j | follows | `engine/rules/sba.go#Engine.checkLoseConditions` | - | commander damage |
| 104.3k | n/a | - | - | tournament |
| 104.4-104.4a | follows | `engine/rules/sba.go#Engine.checkGameOver` | - | simultaneous loss is a draw |
| 104.4b | deviates | `engine/rules/livelock.go#livelockWatcher.abort` `server/host/match.go#Registry.play` | - | gorge crashes table (LOUD) |
| 104.4c | unknown | - | - | not reached |
| 104.4d-104.4i | n/a | - | - | teams/range/Emperor/tournament |
| 104.5 | follows | `engine/rules/sba.go#Engine.ceaseDepartedObjects` | `TestConcedeInFourSeatsLeavesTheRestPlayingOn` |  |
| 104.6 | follows | `engine/rules/restart.go#Engine.finishRestart` | `TestCR727KarnRestartsTheGame` | - |
| 105.1-105.3 | follows | `engine/rules/layer5colors.go#Engine.derivedColorTable` | - |  |
| 105.4 | follows | `engine/effects/choose.go#effChooseColor` | - | - |
| 105.5 | unknown | - | - | not reached |
| 106.1-106.3 | follows | `engine/state/game.go#Player.Pool` `engine/effects/mana_effect.go#effMana` `engine/rules/cast_window.go#Engine.castWindowProbeUnits` | `TestCastWindowChosenColourPaysHybridFace` | - |
| 106.4-106.4b | follows | `engine/rules/turn.go#Engine.finishStepBoundary` `engine/events/event.go#ManaClear` | - | pool empties between steps |
| 106.5 | unknown | - | - | not reached |
| 106.6-106.6a | partial | `engine/effects/mana_effect.go#effMana` `engine/rules/pay/payment.go#RestrictValidMatches` | - | LOUD (gorge register RestrictValid row) |
| 106.7-106.13 | unknown | - | - | not reached |
| 107.1-107.1c | follows | `engine/state/game.go#Player.Life` | - | integer state throughout |
| 107.2 | unknown | - | - | not reached |
| 107.3-107.3p | follows | `engine/rules/cast_subcounter.go#Engine.xAsk` `engine/events/apply_cast.go#foldCastInfo` | `TestExpendWakeLeavesNonXSpellXZero` `TestExpendWakeKeepsAnnouncedX` `TestPaidTeamworkLeavesNonXSpellXZero` `TestPaidTeamworkKeepsAnnouncedX` | cast X ask |
| 107.4-107.6 | follows | `engine/rules/cost/parse.go#ParseCost` `engine/rules/cast_payment.go#Engine.announceFeasible` | `TestCastWindowChosenColourPaysHybridFace` `TestCastWindowFaceChargesActivationFee` | mana/tap/untap symbols parsed; a hybrid face is offered only if the window can pay it |
| 107.7 | follows | `engine/rules/ability_loyalty.go#abilityLoyaltyFact` | - | loyalty cost symbols |
| 107.8-107.13 | unknown | - | - | not reached; not in pools |
| 107.14 | follows | `engine/rules/pay/board.go#EnergyPayable` | - | no energy in pools |
| 107.15-107.15b | follows | `engine/rules/saga.go#Engine.checkSagas` `engine/rules/saga.go#Engine.sagaChapters` | `TestSagaThatLostItsAbilitiesHasNoChapters` `TestSagaConclusionIsASacrifice` | chapter symbols; cr-g07 merged |
| 107.16-107.16a | follows | `engine/rules/class_level.go#Engine.classBandGateHolds` | `TestClassLevelActivatorIsExactNMinusOne` `TestClassLevelDesignationIsNotACounter` |  |
| 107.17-107.17a | n/a | - | - | Unfinity ticket counters |
| 107.18 | follows | `engine/effects/charm.go#CharmPawprints` | `TestPawprintBudget` | root `Pawprint$` budget on the covering KModes ask (MaxSum + Budgeted, options` Value) |
| 108.1-108.2b | n/a | - | - | reference rules |
| 108.3-108.4a | follows | `engine/state/object.go#Object.Owner` `engine/state/object.go#Object.Controller` | - | owner = starting deck holder |
| 108.5-108.6 | n/a | - | - | nontraditional cards; cross-reference |
| 109.1-109.5 | follows | `engine/state/object.go#Object` `engine/effects/cardflow.go#rearrangeChooser` | `TestCRRearrangeTargetLibraryCasterChooses` `TestCRRearrangePlayerNamesTheChooser` | structural; 109.2 not line-checked; RearrangeTopOfLibrary's "you" is the controller, RearrangePlayer$ names another chooser |
| 110.1-110.3 | follows | `engine/state/object.go#Object` | - | permanents on the battlefield |
| 110.4-110.4c | unknown | - | - | not reached |
| 110.5-110.5d | follows | `engine/state/object.go#Object.Tapped` `engine/state/object.go#Object.PhasedOut` | - | status fields; not characteristics |
| 111.1-111.6 | follows | `engine/effects/token.go#effToken` `engine/effects/token.go#runTokenMints` | - | - |
| 111.7 | follows | `engine/rules/sba.go#Engine.ceaseDeadTokens` | `TestATokenBouncedToHandCeasesToExist` `TestATokenThatDiesCeasesToExist` | - |
| 111.8 | follows | `engine/effects/zone_change.go#changeZoneMoveSkip` | `TestATokenFlickeredByChangeZoneDoesNotReturn` `TestANonTokenFlickeredByChangeZoneReturns` | - |
| 111.9-111.13 | unknown | - | - | not reached |
| 112.1-112.4 | follows | `engine/rules/stack.go#Engine.resolveTop` | - | spell on stack; copy owner per OPS.md |
| 113.1-113.3d | follows | `engine/cards/ir.go#SA` | - | ability IR categories |
| 113.4 | follows | `engine/rules/mana_activation.go#Engine.activateMana` | - |  |
| 113.5 | follows | `engine/rules/legal_loyalty.go#Engine.loyaltyAbilityLimit` | `TestJaceLoyaltyAbilityOncePerTurnAndOwnTurnOnly` | - |
| 113.6 | unknown | - | - | not reached |
| 113.7a | follows | `engine/rules/trigmatch/zone.go#ZoneChangeMatchesWithCapture` `engine/rules/pile.go#abilityOwnedFace` | `TestNegatedCopiedSpellTriggersCountExiledCard` `TestActivatedAbilityResolvesWithItsFacesSVars` | an ability object's stack exit is not a card changing zones |
| 113.7b-113.12 | unknown | - | - | not reached |
| 114.1-114.5 | follows | `engine/effects/effect.go#effEffect` | `TestKaitoEmblemOutlivesKaito` `TestTezzeretCruelCaptainEmblemFiresEveryCombat` `TestEffectPhaseTriggerRecursEachUpkeep` | an Effect's Mode$ Phase trigger recurs at every occurrence for the Effect's lifetime (OneOff$ True and a self-exiling body end it) |
| 115.1 | partial | `engine/rules/cast_targets.go#Engine.subTargetAsk` `engine/effects/targets_ask.go#poseTargetsAsk` | - | gorge: depth-2+ chain targets asked at resolution |
| 115.1a-115.1e | follows | `engine/rules/cast_targets.go#Engine.targetAsk` `engine/rules/aura_entry.go#auraEnchantSpec` | - | targeted spells/Auras/abilities/keywords |
| 115.2 | follows | `engine/rules/target_legal.go#Engine.candidatesForLimitInto` | - | - |
| 115.3 | follows | `engine/decision/decision.go#Decision.Validate` | - | no duplicate per "target" word |
| 115.4 | follows | `engine/rules/target_legal.go#Engine.legalTargetCandidates` | - | any target |
| 115.5 | follows | `engine/rules/target_legal.go#Engine.candidatesForLimitInto` `engine/rules/cast_targets.go#Engine.targetAsk` | - | spell not its own target |
| 115.6 | follows | `engine/rules/target_legal.go#Engine.resolvedTargetMin` | - | TargetMin$ 0 |
| 115.7-115.7f | partial | `engine/effects/choose_control.go#effChangeTargets` | - | gorge redirect not all-or-none; copies root-only |
| 115.8 | unknown | - | - | not reached |
| 115.9-115.9c | unknown | - | - | not reached |
| 115.10-115.10b | follows | `engine/effects/context.go#Defined` | - | Defined$ / Ref affect without targeting |
| 116.1 | follows | `engine/rules/legal_priority.go#Engine.handlePriority` | - | special actions at priority, no stack |
| 116.2-116.2a | follows | `engine/rules/legal_priority.go#Engine.handlePriority` | - | land play |
| 116.2b | partial | `engine/rules/morph_turnup.go#Engine.turnFaceUp` | - | manifest/cloak never turn up (LOUD register) |
| 116.2c-116.2e | unknown | - | - | not reached |
| 116.2f | follows | `engine/rules/legal_walk_hand.go#legalWalk.handWalk` `engine/rules/cast_commit.go#Engine.payCast` | - | suspend exile |
| 116.2g | not-impl | `engine/rules/trigger_match.go#init` | - | companion to hand absent; SILENT |
| 116.2h | follows | `engine/rules/legal_walk_hand.go#legalWalk.handWalk` `engine/rules/cast_commit.go#Engine.payCast` `engine/rules/cast_asks.go#Engine.castModeAsk` | `TestForetellingAModalCardAnnouncesNoModes` | foretell exile |
| 116.2i | not-impl | - | - | Planechase decks accepted, no planar-die roll; SILENT |
| 116.2j | n/a | - | - | no Conspiracy |
| 116.2k | follows | `engine/rules/legal_walk_hand.go#legalWalk.handWalk` | - | follows 702.170a (main phase); 116.2k text broader |
| 116.2m | follows | `engine/rules/legal_priority.go#Engine.handlePriority` | - | unlock a locked half |
| 116.3 | follows | `engine/rules/legal_priority.go#Engine.handlePriority` | - | priority returns after special action |
| 117.1-117.1c | follows | `engine/rules/statics_cast.go#Engine.spellTimingOK` | - | - |
| 117.1d | follows | `engine/rules/legal_priority.go#Engine.handlePriority` | - | no manual mana abilities (documented) |
| 117.2-117.2e | follows | `engine/rules/turn.go#Engine.priorityRound` | - | - |
| 117.3-117.3a | follows | `engine/rules/turn.go#Engine.priorityRound` | - | - |
| 117.3b | follows | `engine/rules/legal_priority.go#Engine.handlePriority` | - | AP priority after resolution |
| 117.3c | follows | `engine/rules/legal_priority.go#Engine.handlePriority` | - | - |
| 117.3d | follows | `engine/rules/legal_priority.go#Engine.handlePriority` | - | - |
| 117.4 | follows | `engine/rules/legal_priority.go#Engine.handlePriority` | - | all pass: resolve or advance |
| 117.5 | follows | `engine/rules/turn.go#Engine.priorityRound` | - | SBAs and triggers before priority |
| 117.6 | n/a | - | - | shared team turns |
| 117.7 | follows | `engine/rules/stack.go#Engine.resolveTop` | - | stack LIFO |
| 118.1-118.2 | follows | `engine/rules/pay/solve.go#ResolveManaWith` | `TestTriggerCostReturnsPermanentToHand` | - |
| 118.3-118.3c | follows | `engine/rules/pay/solve.go#ResolveManaWith` | - | - |
| 118.4-118.5a | follows | `engine/rules/cast_subcounter.go#Engine.xAsk` `engine/rules/mana.go#Engine.castOfferBase` | - | X ask; zero-cost casts are explicit options |
| 118.6-118.6a | follows | `engine/rules/nomanacost.go#isNoManaCost` `engine/rules/nomanacost.go#Engine.filterNoManaCostCasts` | - | unpayable cost |
| 118.7-118.7g | follows | `engine/rules/pay/costmods.go#CostMods.Apply` | - |  |
| 118.8-118.9d | follows | `engine/rules/cast_begin.go#Engine.beginCastWith` `engine/rules/cast_begin.go#Engine.beginPlay` | - | additional/alternative costs |
| 118.10-118.12 | unknown | - | - | not reached |
| 118.12a | follows | `engine/rules/pay/unless.go#UnlessCostPayable` `engine/rules/unless_tape.go#settleUnlessElection` `engine/rules/pay/unlessflow.go#AdvanceUnless` | `TestUnlessTapTypeCommandBridge` `TestUnlessTapTypeRentIsDue` `TestUnlessAddCounterSanctuaryWall` `TestUnlessAddCounterCheeringCrowd` `TestUnlessDamageYouXCombustionMan` `TestUnlessWaterbendLesson` `TestUnlessBlightGutsplitter` `TestUnlessExileFromStackSphinxsApproach` `TestUnlessDrawXArmorWars` | the payer may pay; the offer is gated on payability (118.3), never on parseability |
| 118.12b | unknown | - | - | not reached |
| 118.13-118.13c | follows | `engine/rules/potential_plan.go#Engine.planHybridCost` `engine/rules/cast_payment.go#Engine.manaAsk` | - |  |
| 118.14 | unknown | - | - | not reached |
| 119.1-119.3 | follows | `engine/events/apply_combat.go#foldDamage` `engine/effects/life.go#effGainLife` `engine/effects/life.go#effLoseLife` | - | starting life in 103.4 row |
| 119.4-119.4b | follows | `engine/rules/pay/solve.go#ResolveManaWith` | `TestAttackPayLifeInsufficientIsNeverOffered` | - |
| 119.5 | follows | `engine/effects/life.go#effSetLife` | - | set life = gain/lose difference; redistribution Noted |
| 119.6 | follows | `engine/rules/sba.go#Engine.checkLoseConditions` | - | - |
| 119.7 | partial | `engine/rules/replacement_life.go#Engine.emitLifeReplacement` | `TestArchfiendCantGainLifePreventsOpponentOnly` | exchange path not verified |
| 119.8 | not-impl | `engine/effects/effect.go#effEffect` | - | Effect-granted CantLoseLife Noted; LOUD |
| 119.9-119.10 | follows | `engine/rules/lifelink_batch.go#Engine.flushLifelinkBatch` `engine/rules/lifelink_batch.go#Engine.gainFromLifelink` | `TestLifelinkTrampleOverIsOneLifeGainEvent` `TestLifelinkTrampleOverAsksOneReplacementOrder` | REVISED: g one gain event per source per damage event (cr-g05 merged) |
| 120.1 | follows | `engine/events/apply_combat.go#foldDamage` | - | - |
| 120.1a | partial | `engine/events/apply_combat.go#foldDamage` | - | g fold marks damage on any non-PW/battle object |
| 120.2-120.2b | follows | `engine/rules/combat.go#Engine.damageStep` | - | - |
| 120.3-120.3h | follows | `engine/events/apply_combat.go#foldDamage` `engine/rules/emit.go#Engine.emit` `engine/rules/damage_walker_creature.go#Engine.recomputeWalkerCreatureMarker` | `TestBattleAttackDealsDefenseCounterDamage` `TestTwoLifelinkSourcesAreTwoLifeGainEvents` `TestCombatDamageToAnimatedWalkerMarksDamageAndRemovesLoyalty` `TestHealingAnAnimatedWalkerClearsItsMarkedDamage` | 120.3c+e: an animated planeswalker loses loyalty and is marked, for every damage emitter |
| 120.4-120.4d | follows | `engine/rules/emit.go#Engine.emit` | - |  |
| 120.5-120.6 | follows | `engine/rules/sba.go#Engine.destroyLethalDamage` | - | damage marked until cleanup |
| 120.7 | unknown | - | - | not reached |
| 120.8 | follows | `engine/effects/damage.go#emitObjectDamage` `engine/effects/damage.go#emitPlayerDamage` `engine/rules/trigmatch/combat.go#DamageMatchesWithCapture` | `TestCR120_8ZeroPowerFightDealsNoDamageAndTriggersNothing` `TestCR120_8ZeroDamageHasNoEventToReplace` `TestCR120_8NegativePowerFightLeavesMarkedDamage` | REVISED: g 0/negative damage emits nothing, matcher gated (cr-g06 merged) |
| 120.9 | unknown | - | - | not reached |
| 120.10 | follows | `engine/rules/excess_damage_emit.go#noteExcessHit` | `TestExcessDamageCombatPath` `TestExcessDamageNoncombatPath` | - |
| 121.1-121.2 | follows | `engine/rules/engine.go#Engine.drawCard` | - | one card at a time |
| 121.2a-121.2d | unknown | - | - | not reached |
| 121.3-121.3a | unknown | - | - | not reached |
| 121.4 | follows | `engine/rules/sba.go#Engine.checkLoseConditions` `engine/rules/cantlose.go#Engine.markDrawEmpty` | `TestDrawStepDeckOutEndsTheGameBeforePriority` | - |
| 121.5 | follows | `engine/events/event.go#MoveZone` | - | library-to-hand move is not a draw |
| 121.6-121.9 | unknown | - | - | not reached |
| 122.1-122.1a | follows | `engine/rules/chars/derive.go#PT` `engine/state/object.go#Object.CounterPTTotals` `engine/effects/filter_type.go#canReceiveCounter` | `TestPresentDefinedGateOverTheRealCorpusBodies` | P/T counters in layer 7c; any permanent can hold one |
| 122.1b | follows | `engine/cards/counter_keyword.go#CounterKeyword` | - |  |
| 122.1c | follows | `engine/effects/regeneration.go#ReplaceShieldCounter` `engine/rules/shield_counter.go#Engine.shieldCounterDamageMatches` | `TestCR122_1cShieldCounterReplacesDestruction` `TestCR122_1cShieldCounterPreventsDamage` | destruction order regen > shield > umbra is a deterministic CR 616.1 stand-in |
| 122.1d | deviates | `engine/effects/untap.go#TryUntap` | `TestBalothPrimeSacrificeTriggerRemovesAStunCounterInsteadOfUntapping` | stun spent on a doesn't-untap permanent |
| 122.1e | follows | `engine/rules/sba.go#Engine.planeswalkerZeroLoyalty` | `TestCombatDamageToPlaneswalkerAtZeroLoyaltyAppliesStateBasedAction` | - |
| 122.1f | follows | `engine/rules/sba.go#Engine.checkLoseConditions` | - | poison SBA; none in pools |
| 122.1g | follows | `engine/rules/sba.go#Engine.battleZeroDefense` | `TestBattleEntersWithPrintedDefense` | - |
| 122.1h | deviates | `engine/rules/replacement.go#Engine.finalityReplacementApplies` | - | finality applies to creatures only |
| 122.1i | follows | `engine/rules/turn.go#Engine.finishEnteredStep` | - | rad counters trigger |
| 122.1j | follows | `engine/rules/layers.go#Engine.staticEffectsWalk` | `TestCR122_1jHoneCounterPumpsEquippedCreature` | layer 7c; reads attachment, not the Equipment subtype (as Forge) |
| 122.2 | unknown | - | - | not line-checked |
| 122.3 | follows | `engine/rules/sba.go#Engine.annihilateOppositeCounters` | `TestCR704RepoCreatureOppositeCountersAnnihilate` | - |
| 122.4 | not-impl | - | - | stat:MaxCounter unregistered; LOUD (coverage report) |
| 122.5 | partial | `engine/effects/counters.go#effMoveCounter` | - | exotic shapes Noted (LOUD) |
| 122.6 | follows | `engine/rules/entry_counters.go#Engine.entryCounterGrants` | - | counters put on |
| 122.6a-122.9 | unknown | - | - | not reached |
| 123.1-123.9 | n/a | - | - | no sticker mechanic |
