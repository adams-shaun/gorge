# CR 900s: Casual variants

Coverage of this chapter of the Comprehensive Rules (2026-09-25) by gorge. Seeded from the CR audit of 2026-10-20 at `72dabd96c`; kept current by every change that touches a rule. See [README](README.md) for the status vocabulary and the reference syntax.

| rule | status | code | tests | notes |
|---|---|---|---|---|
| 900.1-900.2 | n/a | - | - | descriptive |
| 901.1-901.2 | partial | `engine/rules/engine.go#Config.PlanarDecks` | `TestPlanarDeckGenesisAndWalkReplay` | engine-only scaffold; no hosted table deals a planar deck |
| 901.3 | partial | `engine/rules/engine.go#Config.PlanarDecks` | - | no planar deck size/duplicate validation; engine-only |
| 901.3a | follows | `engine/effects/planar.go#effRollPlanarDice` | `TestFracturedPowerstoneRollsThePlanarDie` | planar die faces |
| 901.4 | deviates | `engine/state/ids.go` | `TestPlanarDeckZoneIsPerSeatAndCloned` | planes live in private ZPlanarDeck, not command zone |
| 901.5 | deviates | `engine/rules/genesis.go#Engine.genesisDeal` | `TestPlanarDeckGenesisAndWalkReplay` | every seat reveals a plane at genesis |
| 901.6 | not-impl | - | - | no planar controller; SILENT engine-side, unreachable hosted |
| 901.7 | partial | `engine/rules/planar.go#Engine.checkChaosEnsuesTriggers` `engine/rules/planar.go#Engine.checkPlaneswalkTriggers` | `TestPlanechaseVerbsNeverEmitUnimplemented` | only chaos and planeswalk triggers fire |
| 901.7a | unknown | - | - | not located: PlanarWalk keeps the plane's ObjID |
| 901.8 | partial | `engine/rules/planeswalk.go#Engine.Planeswalk` `engine/rules/planeswalk.go#Engine.PlaneswalkTo` | `TestPlaneswalkFaceWalksToTheNextPlane` | engine-only; no planar-die trigger |
| 901.9-901.9d | not-impl | - | - | no planar-die special action; SILENT engine-side, unreachable hosted |
| 901.10-901.15c | not-impl | - | - | departure, 2HG, Grand Melee, single deck; unreachable hosted |
| 902.1-902.7 | not-impl | - | - | Vanguard; LOUD (unreachable: host offers no such variant) |
| 903.1 | follows | `engine/rules/engine.go#Format` | `TestCommanderTableBuildsACommanderMatch` | FormatCommander |
| 903.2 | follows | `server/host/table.go#Format` | `TestCommanderTableServesACommanderGameEndToEnd` | 2-8 seat FFA attack-multiple |
| 903.3 | follows | `engine/deck/deck.go#IsCommanderEligible` `engine/state/game.go#Player.Commanders` | `TestIsCommanderEligibleAcceptsLegendaryVehicle` `TestLegendarySpacecraftCommanderLegalityFollowsPTBox` | designation keyed to card object id |
| 903.3a | follows | `engine/deck/deck.go#IsCommanderEligible` | `TestValidateCommanderAcceptsOracleMarkedCommander` | oracle 'can be your commander' phrase |
| 903.3b | partial | `engine/effects/meld.go#Meld` `engine/state/game.go#Player.Commanders` | - | REVISED: melded permanent keeps result card's id; commander partner half loses status |
| 903.3c | partial | `engine/state/object.go#Object.MergedCards` | - | REVISED: pile keeps target's id; commander mutating onto another creature loses status (inferred) |
| 903.3d | follows | `engine/effects/filter_word.go#wordMatches` | - | IsCommander filter reads Player.Commanders per zone |
| 903.3e | follows | `engine/effects/filter_attachment.go#sharesTypeReferents` | `TestSharesCreatureTypeWithCommander` | REVISED: commander objects read live in any zone |
| 903.4 | follows | `engine/cards/face.go#Face.deriveColourIdentity` `engine/cards/ir.go#Card.ColourIdentity` | `TestColourIdentityAcrossTheCorpus` `TestDeriveColourIdentityFromScriptFields` | cost, indicator, rules text, CDA |
| 903.4a | follows | `engine/cards/face.go#Face.ColourIdentity` | `TestColourIdentitySurvivesGobRoundTrip` | identity fixed from card data |
| 903.4b | partial | `engine/rules/commander_color.go#Engine.stepColorRound` `engine/cards/face.go#cdaSetColours` `engine/deck/deck.go#File.ValidateCommander` | `TestPregameCommanderColourOneAskPerSeatInTurnOrder` | game honours choice; deck validator ignores ChosenColor |
| 903.4c | follows | `engine/cards/face.go#oracleColours` | `TestDeriveColourIdentityFromScriptFields` | parenthesised reminder text skipped |
| 903.4d | follows | `engine/cards/ir.go#Card.ColourIdentity` | `TestCardIdentityUnionsFaces` | union over faces |
| 903.4e | follows | `engine/cards/ir.go#Card.ColourIdentity` | `TestCardIdentityUnionsFaces` | adventure faces included in union |
| 903.4f | follows | `engine/rules/pay/planner.go#CommanderIdentityColours` | `TestCommandTowerNonCommanderGameStaysFailClosed` | no commander: fails closed |
| 903.5 | follows | `engine/deck/deck.go#File.ValidateCommander` | `TestValidateCommanderAcceptsLegalDeck` | - |
| 903.5a | follows | `engine/deck/deck.go#File.ValidateCommander` | `TestValidateCommanderRejectsWrongCount` | exactly 100 |
| 903.5b | partial | `engine/deck/deck.go#File.ValidateCommander` `engine/deck/deck.go#Parse` | `TestValidateCommanderRejectsNonSingleton` | per-entry count; duplicate entries not merged |
| 903.5c | follows | `engine/deck/deck.go#File.ValidateCommander` | `TestValidateCommanderRejectsOutsideColourIdentity` | - |
| 903.5d | follows | `engine/deck/deck.go#basicLandCouldProduce` | `TestValidateCommanderRejectsBasicLandTypeProduction` | - |
| 903.5e | partial | `server/host/match.go#Registry.newMatchWith` `engine/deck/deck.go#File.ValidateCommander` | - | sideboard not rejected; host passes it in |
| 903.6 | follows | `engine/rules/genesis.go#Engine.genesisDeal` | `TestCommanderGenesisPlacesCommandersBeforeTheDeal` | - |
| 903.7 | follows | `server/host/match.go#Registry.newMatchWith` `engine/rules/genesis.go#newEngineShell` | `TestCommanderTableDefaultsLife40` | 40 life |
| 903.8 | follows | `engine/rules/mana.go#Engine.commanderTaxFor` `engine/rules/legal_walk_alt.go#legalWalk.commandZoneWalk` | `TestCommanderCastTaxesEveryCastBeyondTheFirst` `TestCommanderCastFromHandIsNeitherTaxedNorCounted` | - |
| 903.9 | partial | `engine/rules/replacement_cmdzone.go#Engine.commanderZoneReplacementApplies` | `TestCommanderZone` | see 903.9a/b |
| 903.9a | deviates | `engine/rules/replacement.go#Engine.applyReplacementsDispatch` `engine/rules/replacement_cmdzone.go#Engine.commanderZoneReplacementApplies` | `TestDestroyedCommanderOffersTheOwnerTheChoiceAndAcceptSendsItToTheCommandZone` | HIGH: replacement not SBA; dies triggers lost; cr-g01a |
| 903.9b | partial | `engine/rules/replacement_cmdzone.go#Engine.handleCmdZone` `engine/rules/emit.go#Engine.emit` | `TestOwnersCommanderInGraveyardReturnedToHandAsks` `TestCommanderReturnedToHandDeclinedKeepsTheHand` | always first; decline skips other replacements; cr-g01b cr-g50 |
| 903.9c | unknown | - | - | not located |
| 903.10 | follows | `engine/rules/sba.go#Engine.checkLoseConditions` | - | - |
| 903.10a | follows | `engine/rules/sba.go#Engine.checkLoseConditions` `engine/rules/combat.go#Engine.tallyCmdDamage` | `TestTwentyOneCommanderDamageLoses` `TestNonCombatCommanderDamageDoesNotTally` | - |
| 903.11 | partial | `engine/effects/zone_change.go#effChangeZone` | `TestChangeZoneWishFindsSideboard` | wishes reach sideboard; host passes Commander sideboard in |
| 903.11a | not-impl | `engine/effects/zone_change.go#effChangeZone` | - | SILENT: no name or colour-identity check on wished card |
| 903.12-903.12h | not-impl | - | - | Brawl; LOUD (unreachable: host offers no such format) |
| 903.13-903.13g | not-impl | - | - | Commander Draft; LOUD (unreachable: no draft in gorge) |
| 904.1-904.13d | not-impl | - | - | Archenemy; LOUD (unreachable: host offers no such variant) |
| 905.1-905.6 | not-impl | - | - | Conspiracy draft; LOUD (unreachable: no draft in gorge) |
