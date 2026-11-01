# CR 800s: Multiplayer rules

Coverage of this chapter of the Comprehensive Rules (2026-09-25) by gorge. Seeded from the CR audit of 2026-10-20 at `72dabd96c`; kept current by every change that touches a rule. See [README](README.md) for the status vocabulary and the reference syntax.

| rule | status | code | tests | notes |
|---|---|---|---|---|
| 800.1 | follows | `server/host/table.go#TableConfig.validated` `engine/state/game.go#Game.AliveFrom` | `TestRepoDecksPlayAtEverySeatCount` | 2-8 seat tables |
| 800.2 | n/a | - | - | descriptive |
| 800.3 | n/a | - | - | tournament rules, outside engine |
| 800.4 | follows | `engine/rules/sba.go#Engine.checkLoseConditions` `engine/events/apply_player.go#foldPlayerLost` | `TestConcedeInFourSeatsLeavesTheRestPlayingOn` `TestThreeSeatGameContinuesWhenOnePlayerIsEliminatedMidTurn` | game continues past a departure |
| 800.4a | partial | `engine/rules/sba.go#Engine.ceaseDepartedObjects` `engine/rules/cantlose.go#Engine.playerLoses` `engine/rules/control.go#Engine.expireControl` | `TestCR800DepartedOwnersCardsLeaveEveryZone` `TestCR800DepartedControllersStackObjectsCease` `TestCR800_4aDepartedPlayersControlledObjectIsExiled` | leftover controlled permanents exiled; cleanup waits for SBA pass; controlled non-owned card spells cease instead of exile |
| 800.4b | partial | `engine/rules/trigger_delayed.go#Engine.delayedRegistrationLive` `engine/events/apply_player.go#foldPlayerLost` | `TestDepartedSeatsEffectsAreRemovedOnce` | player-control ends; no central guard on control change/token/entry under departed |
| 800.4c | follows | `engine/rules/control.go#Engine.expireControl` | `TestCR800_4cEndedControlEffectExilesWhenDefaultControllerLeft` | - |
| 800.4d | partial | `engine/rules/trigger_queue.go#Engine.dropDepartedTriggers` `engine/rules/sba.go#Engine.ceaseDepartedObjects` | `TestATriggerWhoseControllerLeftIsNotPushedByItsDecider` `TestCR704DepartedTokenIsNotAnExiledObject` | triggers dropped; departed-owned new objects swept next SBA, not uncreated |
| 800.4e | deviates | `engine/rules/sba.go#Engine.ceaseDepartedObjects` | `TestCR800NoCombatDamageToDepartedDefender` | damage correct, but attackers removed from combat (EndCombatReset) |
| 800.4f | unknown | - | - | not located: no departed-payer guard in unlessPayerTargets |
| 800.4g | deviates | `engine/rules/replacement.go#Engine.replacementAskPlayer` `engine/rules/sba.go#Engine.releasePendingDecisionOfDepartedPlayer` | `TestOptionalTriggerWhoseDeciderLeftTheGameCeasesToExist` | departed chooser's choice declined/released; CR has controller pick another player |
| 800.4h | deviates | `engine/rules/replacement.go#Engine.applyReplacementsDispatch` `engine/rules/sba.go#Engine.askLegendChoice` | `TestLegendRuleDepartedControllerDeclines` | deterministic default, not next player in turn order |
| 800.4i | follows | `engine/events/apply_player.go#foldPlayerLost` | `TestDepartedStolenSourceCreditsLastController` | REVISED: departed Player record retained; reads see last-known info |
| 800.4j | follows | `engine/rules/turn.go#Engine.grantPriority` `engine/rules/turn.go#Engine.drawStepTurnAction` | `TestEliminatedActivePlayersTurnContinuesToCleanup` `TestNoPriorityIsGrantedToTheEliminatedActivePlayer` `TestDrawStepSkipsEliminatedActivePlayer` | turn runs on without active player |
| 800.4k | follows | `engine/rules/turn.go#Engine.advanceStep` `engine/state/game.go#Game.NextAlive` | `TestNextTurnBelongsToTheNextLivingSeat` `TestEliminatedSeatsAreSkipped` | departed seats' turns and extra turns skipped |
| 800.4m | partial | `engine/rules/layers_effect.go#Engine.AddContinuous` `engine/rules/layers_effect.go#Engine.rescheduleNextTurnBoundaries` | - | post-departure effect falls back to source-leaves; extra turns don't move boundary |
| 800.4n | n/a | - | - | gorge models no ante zone |
| 800.4p | not-impl | - | - | no planar controller; unreachable hosted (Planechase engine-only) |
| 800.5 | follows | `server/host/match.go#Registry.newMatchWith` | - | seat order fixed by table config |
| 800.6 | partial | `engine/rules/mulligan.go#newMulliganRound` `engine/rules/mulligan.go#mulliganRound.bottomCount` | `TestCR800MultiplayerFirstMulliganIsFree` `TestMultiplayerSecondMulliganBottomsOne` | free mulligan still counts toward Config.Mulligans limit |
| 800.7 | follows | `engine/rules/turn.go#skipFirstDrawStep` | `TestCR800StartingPlayerDrawsInMultiplayer` | no Two-Headed Giant |
| 801.1-801.18 | not-impl | - | - | range of influence; LOUD (unreachable: host offers no such option) |
| 802.1 | follows | `engine/rules/combat.go#Engine.askAttackers` `engine/rules/combat/attack.go#CanAttackPair` | `TestKazuulDoesNotFireWhenAThirdSeatIsAttacked` | attack multiple players is the only mode |
| 802.2 | follows | `engine/rules/combat.go#Engine.askAttackers` | - | every opponent is a defending player |
| 802.2a | follows | `engine/state/object.go#Object.Attacking` | `TestKazuulDoesNotFireWhenAThirdSeatIsAttacked` | defending player per attacker |
| 802.3 | follows | `engine/rules/combat.go#Engine.handleAttackers` | - | per-creature player/planeswalker/battle choice |
| 802.3a | partial | `engine/rules/combat/attack.go#AttackRequirements` `engine/rules/combat.go#Engine.mustAttackRequired` | - | AGENTS.md Known approximation combatrestriction1 |
| 802.3b | follows | `engine/rules/banding.go#Engine.bandCandidates` | `TestCR702_22cBandIsDeclaredAndABlockerBlocksTheWholeBand` | a band's members attack one player, planeswalker or battle |
| 802.4 | follows | `engine/rules/combat.go#Engine.askBlockers` | `TestCR802BlockDeclarationsFollowAPNAP` | APNAP block declarations |
| 802.4a | follows | `engine/rules/combat/block.go#CanBlock` | - | blocker controller must be attacked player |
| 802.4b | follows | `engine/rules/combat.go#Engine.handleBlockers` | - | per-defender block legality |
| 802.5 | follows | `engine/rules/combat.go#Engine.runCombatAssignments` | - | damage assigned in APNAP order |
| 803.1-803.1b | not-impl | - | - | attack left/right; LOUD (unreachable: host offers no such option) |
| 804.1-804.2 | not-impl | - | - | deploy creatures; LOUD (unreachable: no teams) |
| 805.1-805.10f | not-impl | - | - | shared team turns; LOUD (unreachable: no teams) |
| 806.1 | follows | `server/host/table.go#TableConfig.validated` | `TestConcedeInFourSeatsLeavesTheRestPlayingOn` | Free-for-All is the multiplayer mode |
| 806.2-806.2c | follows | `engine/rules/combat.go#Engine.askAttackers` | - | attack multiple, no range of influence, no deploy |
| 806.3 | deviates | `server/host/match.go#Registry.newMatchWith` `engine/rules/genesis.go#Engine.resolveToss` | `TestStartingPlayer` | seats not randomised; starting toss random; no gameplay effect |
| 807.1-807.5b | not-impl | - | - | Grand Melee; LOUD (unreachable: host offers no such variant) |
| 808.1-808.5 | not-impl | - | - | Team vs. Team; LOUD (unreachable: no teams) |
| 809.1-809.7 | not-impl | - | - | Emperor; LOUD (unreachable: no teams) |
| 810.1-810.11 | not-impl | - | - | Two-Headed Giant; LOUD (unreachable: no teams) |
| 811.1-811.5 | not-impl | - | - | Alternating Teams; LOUD (unreachable: no teams) |
