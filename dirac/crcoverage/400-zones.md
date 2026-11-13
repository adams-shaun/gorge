# CR 400s: Zones

Coverage of this chapter of the Comprehensive Rules (2026-09-25) by the dirac kernel (two players, FDN and BLB pools). Seeded from the CR audit of 2026-10-20 at `36c2469`; kept current by every change that touches a rule. `inherits gorge` means the kernel copies gorge's behaviour, so gorge's row in `engine/rules/crcoverage/` decides it. See [README](README.md).

| rule | status | code | tests | notes |
|---|---|---|---|---|
| 400.1 | follows | `dirac/kop/table.go#Zone` `dirac/zones.go#State.place` | - | dirac: no command zone objects except emblems |
| 400.2 | follows | `dirac/project.go#State.projObj` | - |  |
| 400.3 | follows | `dirac/zones.go#State.place` | - |  |
| 400.4 | n/a | - | - | umbrella for 400.4a-b; see those rows |
| 400.4a | n/a | - | - | SILENT; no instant/sorcery battlefield gate (Forge has one); no supported card reaches it |
| 400.4b | n/a | - | - | planes kept in ZPlanarDeck, not command zone; not reached |
| 400.5 | follows | `dirac/zones.go#State.shuffleLib` | - | order changes only via explicit events |
| 400.6 | follows | `dirac/repl_gen.go#State.graveRedirect` | - |  |
| 400.7 | follows | `dirac/zones.go#State.place` `dirac/returnrem.go#returnLeftOrigin` `dirac/resolve.go#State.targetStillLegal` `dirac/trig_objmoved.go#State.trigObjMoved` `dirac/value_tappower.go#State.graveExileCands` `dirac/interp_ask.go#State.searchCands` `dirac/kgen/lower_hiddenpick.go#lowerCtx.graveExilePick` `dirac/zone_exiledwith.go#State.exwHas` `dirac/zone_handgravepick.go#State.handGraveArm` `dirac/trig_sacqueueorder.go#State.gameGraveCands` `dirac/resolve.go#State.objRef` | `TestReturnedSourceIsOtherInAResolvingFilterLockstep` `TestReturnedSourcePaysAnotherLockstep` `TestCrimeLockstep` `TestTriggerExileFindsNoNewObject` `TestSelfReturnFindsNoNewObject` `TestThisTurnEnteredLockstep` `TestThisTurnEnteredMutation` `TestThisTurnEnteredRing` `TestGraveExileAndTapPowerLockstep` `TestGraveExilePickMutation` `TestGraveExileChangeZoneAllMutation` `TestExwLockstep` `TestExwMutation` `TestGybfLockstep` `TestGybfMutation` `TestValueHeadsTrackedObjectLockstep` `TestValueHeadsTrackedObjectMutation` | every zone change is a new object; another card that died is matched by version against its trigger-time snapshot (Valkyrie's Call), now a shared LKI pool record keyed by (obj, ver) (`dirac/zone_lki.go`); `Count$ThisTurnEntered` reads the per-ADD ring `genState.entered`, so a card that entered twice counts twice and a moved-on card stays listed and is matched where it now lives (this rule makes it a new object); the ring saturates loudly at 24 with `StallEntered`; the exact `Graveyard_from_Battlefield_Creature` spelling still reads the per-add `died` tally (CR 603.10a); a hidden pick with `Origin$ Graveyard,Exile` (graveexile) walks every player's graveyard then exile in seat order, matching `ChangeType$` (gorge's gameWide effHiddenPick) and `ChangeZoneAll` over the same two zones; a plain `Origin$ Graveyard` pick onto the battlefield (Scavenger's Talent) walks the chooser's own graveyard with `ChangeType$` narrowed to that player's cards, and with `GainControl$ True` (`SrUnderYourControl`, Victor, Valgavoth's Seneschal) walks every player's graveyard in seat order matching the real owner (`gameGraveCands`; an ownerless clause without `GainControl$` still refuses) and the entrant is controlled by the chooser; the entrant gets the pick's `WithCountersType$` counter (amount 1, B bits 4..11, `handGraveArm`); other Origin lists and unread parameters refuse; `TestThisTurnEnteredLockstep` `TestThisTurnEnteredMutation`; the exiled-with source link (gorge exiledBySourceAt) ties an exiled card to the incarnation of the source that exiled it: the reverse link is live at the same ver (one ver on once the source has left the battlefield) so a flickered source finds nothing, and the forward list is cleared when the source re-enters other than from the stack (`genState.exw`); a multi-face card shares one text for the association, so a face whose other face reads the link also records exiles its own resolutions make (`exwRec` propagated across `Faces`), a craft's ExileCtrlOrGrave material exiled at ACTIVATION is recorded through `exwCostExilePick` (outside a resolution), and the stamp is re-based when the source leaves the battlefield (`exwSourceLeft`) so the returned source's ETB trigger reaches the material through the existing fire-ver +1 arm (Jade Seedstones/Jadeheart Attendant, `TestValueHeadsTrackedObjectLockstep`) |
| 400.7a-c | n/a | - | - | not reached |
| 400.7d | follows | `dirac/filters_gen.go#State.flags2Perm` `dirac/exprs.go#State.expr` | - | cast provenance (kicked etc.) carried onto the permanent |
| 400.7e | follows | `dirac/zones.go#State.fireLeave` | `TestLightningBoltAndDiesTrigger` | leaves-zone triggers look back (LKI) |
| 400.7f | n/a | - | - | LastBearer path not verified |
| 400.7g | follows | `dirac/legal.go#State.fbInstances` | `TestCleanupMarksFlashbackGrants` |  |
| 400.7h | unknown | - | - | not reached |
| 400.7i | n/a | - | - | not reached |
| 400.7j | follows | `dirac/interp_gen.go#State.execGen` `dirac/zone_rememberparams.go#State.markRemLKI` | `TestRememberLKIController` `TestRemparamsLockstep` | gorge Remembered; dirac exile links by ver; a RememberLKI$ move's captured controller survives the 400.7 reset for the rest of the resolution (gorge c.Snap.ChangeZone) |
| 400.7k | n/a | - | - | madness referent not checked |
| 400.7m | n/a | - | - | LOUD; stickers unsupported |
| 400.8 | follows | `dirac/zones.go#State.place` | - | gorge keeps exiler link on exile-to-exile move |
| 400.9 | n/a | - | - | no face-down command-zone objects |
| 400.10 | n/a | - | - | no command-to-command move arises |
| 400.11 | definition | - | - | defines outside the game |
| 400.11a-c | n/a | - | - | dirac: no sideboard |
| 400.12 | n/a | - | - |  |
| 401.1 | follows | `dirac/engine.go#State.Init` | - |  |
| 401.2 | follows | `dirac/project.go#State.projRest` | - |  |
| 401.3 | follows | `dirac/project.go#State.projRest` | - |  |
| 401.4 | follows | `dirac/zones.go#player.libTop` `dirac/act_rearrange.go#State.rearrangeDone` `dirac/zone_tolibrary.go#State.reverseLook` `dirac/act_libplace.go#State.libPlaceExt` | `TestRearrange` `TestUnreadLockstep` `TestLibPlaceAskOwnerOrders` `TestShuffleOwnersLockstep` | a ChangeZoneAll sweep into a library and a multi-target ChangeZone to its top or bottom bracket the move with kop.ExtLibPlace: the cards' owner orders two or more cards put at one position (gorge libraryPlacementOrder; the first pick nearest the top); a ChangeZone of a remembered or valid Defined$ group to a library is refused by kgen; OpRearrange: the controller orders every looked-at card (gorge KArrange), then MayShuffle$ asks yes/no; a hand put-back of several cards (Brainstorm) puts them on top in pick order, the first picked on top (gorge handLibraryTail); LibraryPosition$ 2 of the source leaving the battlefield (Braided Quipu, kop.LibThird) puts it min(2, the other cards left) cards down (`dirac/act_lockpermanent.go#State.toLibraryThird`, `TestBraidedLockstep`) |
| 401.5 | n/a | - | - | MayLookAt$ Player shown to owner only; kgen drops the view-only top-of-library `MayLookAt$ You/True/Player` (L1) as no static: gorge has no event, decision or legality reader (TestFreePlaySpellingsLockstep); the face-down battlefield shape (`Creature.faceDown+OppCtrl` / `+YouDontCtrl`, printed and Effect-delivered) also lowers to no static for the same reason (TestMaylookatLockstep) |
| 401.6 | n/a | - | - |  |
| 401.7 | n/a | - | - |  |
| 402.1 | follows | `dirac/zones.go#State.draw` | - |  |
| 402.2 | follows | `dirac/turn.go#State.beginStep` | - |  |
| 402.3 | follows | `dirac/project.go#State.projObj` | - |  |
| 403.1 | follows | `dirac/zones.go#State.enter` | - |  |
| 403.2 | follows | `dirac/zones.go#State.enter` | - | default zone is battlefield |
| 403.3 | follows | `dirac/zones.go#State.enter` | - |  |
| 403.4 | follows | `dirac/zones.go#State.enter` | `TestStashCounterLeavesWithLand` | as 400.7; counters kept exile-to-battlefield (dirac inherits) |
| 403.5 | definition | - | - | historical name |
| 404.1 | follows | `dirac/zones.go#State.place` | - |  |
| 404.2 | follows | `dirac/project.go#State.projRest` | - |  |
| 404.3 | n/a | - | - | SILENT; no arrange ask; outcomes unaffected |
| 405.1 | follows | `dirac/cast.go#State.beginCast` `dirac/cast.go#State.push` | `TestDelayedTriggerUsesTheStack` |  |
| 405.2 | follows | `dirac/cast.go#State.push` `dirac/resolve.go#State.resolveTop` | `TestStackHoldsGorgeDepth` |  |
| 405.3 | follows | `dirac/cast.go#State.placeTriggers` | `TestCastTriggersInBattlefieldOrder` |  |
| 405.4 | follows | `dirac/resolve.go#State.resolveTop` | - |  |
| 405.5 | follows | `dirac/turn.go#State.pass` | - |  |
| 405.6 | follows | `dirac/turn.go#State.pass` | - | umbrella for 405.6a-h |
| 405.6a | follows | `dirac/turn.go#State.fireDelayed` | `TestDelayedTriggerUsesTheStack` |  |
| 405.6b | follows | `dirac/chars_gen.go` | - |  |
| 405.6c | follows | `dirac/mana.go#State.pay` | `TestAutopayLandsBeforeDorks` | dirac autopays mana at cast time |
| 405.6d | follows | `dirac/legal.go#State.playLandNow` | - |  |
| 405.6e | follows | `dirac/turn.go#State.beginStep` | - |  |
| 405.6f | follows | `dirac/turn.go#State.sba` | `TestCleanupChecksStateBasedActions` |  |
| 405.6g | not-impl | - | - | gorge concede only at priority; dirac LOUD, documented no concede |
| 405.6h | n/a | - | - | dirac two-player |
| 406.1 | follows | `dirac/kop/table.go#ZoneExile` `dirac/filter_props.go#State.exileCardCands` | `TestBladeWithWarpLockstep` | a TgtCard slot whose filter zone is exile (Blade of the Swarm's "target exiled card with warp") offers the exiled cards, player 0's list then player 1's in list order, as gorge walks the inferred zone |
| 406.2 | definition | - | - | defines to exile / exiled card |
| 406.3 | follows | `dirac/project.go#State.projObj` `dirac/zone_exilefacedown.go#State.faceDownExile` `dirac/zone_exilefacedown.go#State.faceDownExiled` | `TestExilefdLockstep` `TestExilefdMutation` | a Dig's ExileFaceDown$ WithMayLook$ exile is face down and the controller may look at it: the mark rides the exiled-with link pool, ends when the card leaves exile; other face-down exiles (ChangeZone ExileFaceDown$) stay refused |
| 406.3a-b | follows | `dirac/zone_exilefacedown.go#State.FaceDownLooker` | `TestExilefdLockstep` | a Dig's face-down exile only (the looker is the resolving controller); dirac turns a face-down exiled card up only by it leaving exile |
| 406.4 | n/a | - | - | not reached |
| 406.5 | follows | `dirac/interp_gen.go#State.execGen` | - | dirac OpExileUntilLeaves links |
| 406.6 | follows | `dirac/interp_gen.go#State.execGen` `dirac/zone_exiledwithstatic.go#State.exwFlag` `dirac/zone_exiledwithstatic.go#State.exwPermFlag` | `TestExwStaticSource` `TestExwStaticsLockstep` `TestExwStaticsMutation` | the "exiled with this" reader of a printed static (Keen-Eyed Curator, Dawnhand Dissident, Maralen) reads the link through the FExiledWith filter flag against the static's own source object, only the cards its host exiled |
| 406.7 | follows | `dirac/zones.go#State.place` | - | as 400.8 |
| 406.8 | definition | - | - | historical name |
| 407.1-407.4 | n/a | - | - | ante not supported |
| 408.1 | follows | `dirac/chars_gen.go` | - |  |
| 408.2 | follows | `dirac/interp_gen.go#State.execGen` | - |  |
| 408.3 | n/a | - | - | current plane in ZPlanarDeck, not command zone |
