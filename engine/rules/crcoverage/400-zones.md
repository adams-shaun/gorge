# CR 400s: Zones

Coverage of this chapter of the Comprehensive Rules (2026-09-25) by gorge. Seeded from the CR audit of 2026-10-20 at `72dabd96c`; kept current by every change that touches a rule. See [README](README.md) for the status vocabulary and the reference syntax.

| rule | status | code | tests | notes |
|---|---|---|---|---|
| 400.1 | follows | `engine/state/ids.go#ZLibrary` `engine/state/ids.go#ZCommand` | - |  |
| 400.2 | follows | `engine/state/ids.go#Zone.Hidden` `engine/view/redact.go#visibleTo` | - |  |
| 400.3 | follows | `engine/events/apply_state_helpers.go#zoneOwner` | `TestCounterMovesTheTargetedSpellToItsOwnersGraveyard` |  |
| 400.4 | not-impl | - | - | umbrella for 400.4a-b; see those rows |
| 400.4a | not-impl | - | - | SILENT; no instant/sorcery battlefield gate (Forge has one); no supported card reaches it |
| 400.4b | unknown | `engine/rules/planar.go#Engine.inPlanarDeck` | - | planes kept in ZPlanarDeck, not command zone; not reached |
| 400.5 | follows | `engine/events/apply_zone.go#foldLibraryOrder` `engine/events/apply_zone.go#foldShuffle` | - | order changes only via explicit events |
| 400.6 | follows | `engine/rules/replacement.go#Engine.applyReplacements` | - |  |
| 400.7 | partial | `engine/events/apply_zone.go#move` `engine/rules/stack.go#Engine.targetBecameNewObject` | `TestTargetThatDiedAndReturnedIsANewObject` `TestGraveyardSelfReturnFindsOnlyTheSameObject` | gorge Incarnation advances only across battlefield; cr-g15-object-identity-every-zone |
| 400.7a-c | unknown | - | - | not reached |
| 400.7d | follows | `engine/state/object.go#Object.CastFlags` | - | cast provenance (kicked etc.) carried onto the permanent |
| 400.7e | follows | `engine/rules/trigmatch/zone.go#ZoneChangeMatchesWithCapture` | - | leaves-zone triggers look back (LKI) |
| 400.7f | unknown | `engine/state/object.go#Object.LastBearer` | - | LastBearer path not verified |
| 400.7g | follows | `engine/state/object.go#Object.CastFlags` | `TestSnapcasterMageGrantsGraveyardFlashback` |  |
| 400.7h | unknown | - | - | not reached |
| 400.7i | unknown | - | - | not reached |
| 400.7j | follows | `engine/state/object.go#Object.Remembered` `engine/events/apply_zone.go#move` | - | gorge Remembered |
| 400.7k | unknown | - | - | madness referent not checked |
| 400.7m | not-impl | - | - | LOUD; stickers unsupported |
| 400.8 | partial | `engine/events/apply_zone.go#move` | - | gorge keeps exiler link on exile-to-exile move |
| 400.9 | n/a | - | - | no face-down command-zone objects |
| 400.10 | n/a | - | - | no command-to-command move arises |
| 400.11 | definition | - | - | defines outside the game |
| 400.11a-c | follows | `engine/state/ids.go#ZSideboard` `engine/effects/zone.go#ParseZone` | `TestChangeZoneWishFindsSideboard` `TestBurningWishEmptySideboardKeepsSelfExile` |  |
| 400.12 | follows | `engine/effects/zone_changeall.go#effChangeZoneAll` | `TestChangeZoneAllShufflesDestinationLibraries` |  |
| 401.1 | follows | `engine/rules/genesis.go#New` | - |  |
| 401.2 | follows | `engine/state/ids.go#Zone.Hidden` `engine/view/view.go#projector.unorderedLibrary` | - |  |
| 401.3 | follows | `engine/view/view.go#PlayerView.LibrarySize` | - |  |
| 401.4 | partial | `engine/effects/zone_library.go#placeTargetedLibraryObjects` | - | targeted path places in target order without asking (Plow Under) |
| 401.5 | deviates | `engine/rules/layers.go#Engine.staticEffectsWalk` `engine/rules/layers_static.go#Engine.MayLookAtLibraryTop` | - | MayLookAt$ Player shown to owner only; cr-g28-maylookat-player-reveal |
| 401.6 | follows | `engine/view/view.go#projectInto` | - |  |
| 401.7 | follows | `engine/effects/zone_library.go#libraryOrderPlacementAt` | - |  |
| 402.1 | follows | `engine/rules/engine.go#Engine.drawCard` | - |  |
| 402.2 | follows | `engine/rules/combat.go#Engine.maxHandSizeFor` | `Test3141ExactlyMaxHandSizeAsksNothing` |  |
| 402.3 | follows | `engine/state/ids.go#Zone.Hidden` `engine/view/redact.go#visibleTo` | - |  |
| 403.1 | follows | `engine/events/apply_zone.go#move` | - |  |
| 403.2 | follows | `engine/effects/zone.go#ParseZone` | - | default zone is battlefield |
| 403.3 | follows | `engine/events/apply_zone.go#move` | - |  |
| 403.4 | partial | `engine/events/apply_zone.go#move` `engine/rules/stack.go#Engine.targetBecameNewObject` | `TestTargetThatDiedAndReturnedIsANewObject` | as 400.7 |
| 403.5 | definition | - | - | historical name |
| 404.1 | follows | `engine/events/apply_zone.go#move` | - |  |
| 404.2 | follows | `engine/state/ids.go#Zone.Hidden` | - |  |
| 404.3 | not-impl | - | - | SILENT; no arrange ask; outcomes unaffected |
| 405.1 | follows | `engine/rules/cast_begin.go#Engine.beginCast` `engine/rules/trigger_queue.go#Engine.pushTrigger` | - |  |
| 405.2 | follows | `engine/rules/stack.go#Engine.resolveTop` | - |  |
| 405.3 | follows | `engine/rules/trigger_queue.go#Engine.putTriggersOnStack` `engine/rules/trigger_queue.go#Engine.sortPendingTriggers` | `TestSimultaneousTriggersStackInAPNAPOrder` |  |
| 405.4 | follows | `engine/rules/stack.go#Engine.resolveTop` | - |  |
| 405.5 | follows | `engine/rules/legal_priority.go#Engine.handlePriority` `engine/rules/stack.go#Engine.resolveTop` | `TestPriorityVisitsEverySeatInAPNAPOrder` |  |
| 405.6 | follows | `engine/rules/turn.go#Engine.step` | - | umbrella for 405.6a-h |
| 405.6a | follows | `engine/effects/delayed_trigger.go#effDelayedTrigger` | - |  |
| 405.6b | follows | `engine/rules/layers.go#Engine.staticEffects` | - |  |
| 405.6c | follows | `engine/rules/mana_activation.go#Engine.resolveManaAbility` | `TestGainedManaAbilityResolvesItsOwnSVars` |  |
| 405.6d | follows | `engine/rules/cast_landplay.go#Engine.finishLandPlay` | - |  |
| 405.6e | follows | `engine/rules/turn.go#Engine.finishEnteredStep` | - |  |
| 405.6f | follows | `engine/rules/sba.go#Engine.checkStateBased` | - |  |
| 405.6g | partial | `engine/rules/legal.go#Engine.legalActionsWalkWithWindow` | `TestConcedeAtPriorityEndsTheGameForTheOtherSeat` | gorge concede only at priority |
| 405.6h | follows | `engine/events/apply_player.go#foldPlayerLost` | - |  |
| 406.1 | follows | `engine/state/ids.go#ZExile` `engine/events/apply_zone.go#move` | - |  |
| 406.2 | definition | - | - | defines to exile / exiled card |
| 406.3 | follows | `engine/state/object.go#Object.FaceDown` `engine/effects/zone_change.go#applyFaceDownMarker` | `TestLibrarySearchFaceDownExileIsHiddenFromOpponents` |  |
| 406.3a-b | follows | `engine/effects/zone_change.go#applyFaceDownMarker` | `TestIxhelFaceDownExileIsVisibleOnlyToTheDigController` |  |
| 406.4 | unknown | - | - | not reached |
| 406.5 | follows | `engine/state/object.go#Object.ExileReturn` | - |  |
| 406.6 | follows | `engine/state/object.go#Object.ExiledCards` `engine/effects/context.go#exiledWithSet` | `TestFaceDownExileHasNoExiledWithAssociation` |  |
| 406.7 | partial | `engine/events/apply_zone.go#move` | - | as 400.8 |
| 406.8 | definition | - | - | historical name |
| 407.1-407.4 | n/a | - | - | ante not supported |
| 408.1 | follows | `engine/state/ids.go#ZCommand` | - |  |
| 408.2 | follows | `engine/effects/effect_params.go#isEmblemEffectName` | `TestKaitoEmblemOutlivesKaito` |  |
| 408.3 | partial | `engine/rules/planar.go#Engine.currentPlane` | - | current plane in ZPlanarDeck, not command zone |
