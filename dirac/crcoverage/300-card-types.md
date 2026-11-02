# CR 300s: Card types

Coverage of this chapter of the Comprehensive Rules (2026-09-25) by the dirac kernel (two players, FDN and BLB pools). Seeded from the CR audit of 2026-10-20 at `36c2469`; kept current by every change that touches a rule. `inherits gorge` means the kernel copies gorge's behaviour, so gorge's row in `engine/rules/crcoverage/` decides it. See [README](README.md).

| rule | status | code | tests | notes |
|---|---|---|---|---|
| 300.1 | follows | `dirac/kop/filters.go#TypeMask` | - | - |
| 300.2 | follows | `dirac/kop/filters.go#TypeMask` | - | multiple card types |
| 300.2a | follows | `dirac/legal.go#State.priorityOptions` | - | land + other type played only as land |
| 300.2b | n/a | - | - | kindred follows its other type; none in pools |
| 301.1 | follows | `dirac/legal.go#State.sorcerySpeed` | - | sorcery-speed cast |
| 301.2 | follows | `dirac/resolve.go#State.resolveTop` | - | resolves onto the battlefield |
| 301.3 | follows | `dirac/kop/table.go#CardDef.Subtypes` | - | subtype syntax |
| 301.4 | definition | - | - | no artifact-specific characteristic |
| 301.5-301.5a | follows | `dirac/resolve.go#State.resolveTop` `dirac/turn.go#State.sba` | `TestEquipmentFallsOffNoncreature` | - |
| 301.5b | unknown | - | - | illegal attach moves then SBA fixes |
| 301.5c | partial | `dirac/static_unattachcreature.go#State.sbaAttachedCreature` `dirac/chars.go#State.matchPerm` | `TestUnattachLockstep` `TestEquipNotOfferedItself` | an Equipment that becomes a creature is no longer attached to its bearer (704.5p) and is not offered itself as an Equip target (kgen lowers the Equip target filter with FOther, gorge excludeSelf for an Attach ability); the Equipment may still be attached again to another creature. SILENT: Fortification and the Reconfigure creature form (no lowered carrier) |
| 301.5d | follows | `dirac/legal.go#State.canActivate` | - | gorge not reached; dirac equip by Equipment controller |
| 301.5e | unknown | - | - | - |
| 301.5f | follows | `dirac/exprs.go#State.refObjCtx` | - | dirac RefAttached |
| 301.6 | n/a | - | - | - |
| 301.7-301.7b | n/a | - | - | see 208.3 for uncrewed filter read |
| 302.1 | follows | `dirac/legal.go#State.sorcerySpeed` | - | sorcery-speed cast |
| 302.2 | follows | `dirac/resolve.go#State.resolveTop` | - | resolves onto the battlefield |
| 302.3 | follows | `dirac/kop/table.go#CardDef.Subtypes` | - | subtype syntax |
| 302.4 | follows | `dirac/chars.go#State.pt` `dirac/exprs.go#State.expr` | - | same as 208.3; a total-power sum (Count$Valid ...$CardPower, ExSumPower: Ghalta, formidable, pack tactics) reads the current power, as gorge zoneFoldPropCardPower since 4ca21a7dd; constructed carrier with an anthem and pumps: oracle TestCycleLockstep matchup 1 (kcycle) |
| 302.4a-302.4c | follows | `dirac/chars.go#State.pt` | - | - |
| 302.5 | follows | `dirac/combat.go#State.canAttack` `dirac/combat.go#State.canBlock` | - | - |
| 302.6 | follows | `dirac/legal.go#State.canActivate` | - | summoning sickness |
| 302.7 | follows | `dirac/turn.go#State.sba` | - | - |
| 303.1 | follows | `dirac/legal.go#State.sorcerySpeed` | - | sorcery-speed cast |
| 303.2 | follows | `dirac/resolve.go#State.resolveTop` | - | resolves onto the battlefield |
| 303.3 | follows | `dirac/kop/table.go#CardDef.Subtypes` | - | subtype syntax |
| 303.4-303.4b | follows | `dirac/kop/table.go#CardDef.Enchant` `dirac/filter_props.go#State.wordPerm2` | `TestFilterWordsAttackingEnchantedLockstep` `TestFilterWordsAttackingEnchantedMutation` | EnchantedBy Aura.YouCtrl: an Aura its controller controls is attached to the candidate (Archon of the Wild Rose) |
| 303.4c | follows | `dirac/turn.go#State.sba` | `TestAuraOnIllegalHostGoesToGraveyard` | - |
| 303.4d | n/a | - | - | SILENT; no self-attach check |
| 303.4e | unknown | - | - | - |
| 303.4f-303.4g | follows | `dirac/interp_gen.go#State.auraEntry` `dirac/zone_handmoveaura.go#State.handAuraEnter` `dirac/static_roletokens.go#State.createAttachedToken` `dirac/zone_originalternative.go#State.czAttachHost` `dirac/zone_handmoveaura.go#State.handAuraArmed` | `TestCzattachLockstep` `TestCzattachMutation` `TestReturnedAuraChoosesWhatItEnchants` `TestReturnedAuraNoHostStaysExiled` `TestLinkReturnFindsTheBoardAfterTheBatch` `TestRoleWithheldNoBearer` `TestRoleGateNoCandidate` `TestHandauraLockstep` `TestHandauraMutation` `TestEnteringObjectsLockstep` | a graveyard return with AttachedTo$ (RetAttachTo: Self / ParentTarget) enters an Aura attached to its named bearer or leaves it in the graveyard when that bearer is not among the Aura's legal objects (CR 303.4g, gorge markChangeZoneAttach), and attaches a returned Equipment to a creature bearer after it enters; a Role minted with AttachedTo$ is withheld when its bearer is gone or nothing can be enchanted; a DigUntil found card enters plainly only for a non-Enchantment Valid$ (kgen refuses a clause that may admit an Aura); a one-card hand move onto the battlefield whose filter may admit an Aura (SrAuraEntry) enchants what its controller chooses (raGenEnchant) or stays in the hand with no host (CR 303.4g), a counted move still refuses one |
| 303.4h | n/a | - | - | - |
| 303.4i | follows | `dirac/interp_gen.go#State.auraEntry` | - | - |
| 303.4j | n/a | - | - | Aura moved then SBA bins it |
| 303.4k | n/a | - | - | no face-down Aura in scope |
| 303.4m | follows | `dirac/exprs.go#State.refObjCtx` `dirac/filters_gen.go#State.matchPermOne` `dirac/filter_notattached.go#DebugNotAttachedPositive` | `TestNotenchLockstep` `TestNotenchMutation` | dirac RefAttached; "other than enchanted creature" (!EnchantedBy / !EquippedBy, kop.FNotAttachedBySrc) excludes only the source's live host, nothing once the Aura has left |
| 303.5-303.6 | definition | - | - | cross-references to 714 Saga / 716 Class |
| 303.7 | definition | `dirac/kop/table.go#CardDef.Subtypes` | `TestRoleAttachOwnTarget` | Role subtype; WOE Role tokens are the first pool Roles |
| 303.7a | follows | `dirac/static_roletokens.go#State.roleSBA` `dirac/turn.go#State.sba` | `TestRoleSweepSameController` `TestRoleCrossControllerCoexist` `TestRoleWickedSweepTrigger` `TestRoleAtMostOneOld` | Role exclusivity is a state-based action per Role controller: all but the newest-stamped Role one player controls on a bearer go to the graveyard; different controllers coexist |
| 304.1 | follows | `dirac/legal.go#State.priorityOptions` | - | instant timing |
| 304.2 | follows | `dirac/resolve.go#State.resolveTop` | - | instant resolves to graveyard |
| 304.3 | follows | `dirac/kop/table.go#CardDef.Subtypes` | - | subtype syntax |
| 304.4 | follows | `dirac/cardset/pooldata.go#PoolData` | - | SILENT; gorge relies on script filters; dirac pool filters |
| 304.5 | follows | `dirac/legal.go#State.sorcerySpeed` | - | - |
| 305.1 | follows | `dirac/legal.go#State.priorityOptions` | - | - |
| 305.2-305.2a | follows | `dirac/legal.go#State.priorityOptions` `dirac/statics_gen.go#State.landLimit` | - | effect plays capped at one land regardless of extra drops; |
| 305.2b | follows | `dirac/legal_gen.go#State.priorityGen` | - | - |
| 305.3 | follows | `dirac/legal.go#State.priorityOptions` | - | effect-play path lacks the turn check (probe); |
| 305.4 | follows | `dirac/legal.go#State.playLandNow` | - | - |
| 305.5 | follows | `dirac/kop/table.go#CardDef.Subtypes` | - | subtype syntax |
| 305.6 | follows | `dirac/mana.go#State.tapFor` `dirac/mana_multicolour.go#prepMultiMana` `dirac/kgen/lower_multimana.go#lowerCtx.refuseMultiMana` `dirac/static_landtype.go#State.blgView` `dirac/static_landtype.go#State.blgPrintedMask` | `TestMultiManaPays` `TestMultiManaUnitOrder` `TestMultiManaWindow` `TestMultiManaShapes` `TestBasicLandGrantMana` `TestBasicLandGrantWindow` `TestStdMkmReprosLockstep` | dirac: compiled from gorge registry faces; a dual's or triome's several fixed-colour abilities are one deterministic autopay unit of their colours (gorge census: one unit, an alternative per ability), and the payment window takes the first ability; kgen refuses any other multi-mana-ability shape beyond a plain {T} ability plus one modelled gap; a granted basic land type (AddType$ AllBasicLandType) adds its intrinsic ability's colour to a plain land's unit (printed ability first in the window), any other affected shape stalls (StallLandGrant); a nonbasic makes a granted colour before its own (CompareSteps reads the ability Kind first) |
| 305.7 | n/a | - | - | REVISED: set land type strips rules-text abilities (merged); no Blood Moon in pools |
| 305.8-305.9 | follows | `dirac/legal.go#State.priorityOptions` | - | - |
| 306.1 | follows | `dirac/legal.go#State.sorcerySpeed` | - | sorcery-speed cast |
| 306.2 | follows | `dirac/resolve.go#State.resolveTop` | - | resolves onto the battlefield |
| 306.3 | follows | `dirac/kop/table.go#CardDef.Subtypes` | - | subtype syntax |
| 306.4 | n/a | - | - | historical; legend rule |
| 306.5-306.5d | follows | `dirac/legal.go#State.canActivate` | - | - |
| 306.6 | follows | `dirac/combat.go#State.attackedPerm` | - | - |
| 306.7 | n/a | - | - | REVISED: historical redirection rule, nothing to implement (was follows) |
| 306.8 | follows | `dirac/interp.go#State.damageBetween` | - | gorge keys on printed face (probe); |
| 306.9 | follows | `dirac/turn.go#State.sba` | - | gorge keys on printed face; |
| 307.1 | follows | `dirac/legal.go#State.sorcerySpeed` | - | sorcery timing |
| 307.2 | follows | `dirac/resolve.go#State.resolveTop` | - | sorcery resolves to graveyard |
| 307.3 | follows | `dirac/kop/table.go#CardDef.Subtypes` | - | subtype syntax |
| 307.4 | follows | `dirac/cardset/pooldata.go#PoolData` | - | SILENT; gorge relies on script filters; dirac pool filters |
| 307.5 | follows | `dirac/legal.go#State.sorcerySpeed` | - | - |
| 307.5a | n/a | - | - | - |
| 308.1 | n/a | - | - | no kindred in pools |
| 308.2 | n/a | - | - | - |
| 308.3 | follows | `dirac/value_gravecounts.go#State.graveTypeAgg` `dirac/value_validagg.go#State.validAgg` | `TestValidAggCountHeads` | Kindred is a card type, never a subtype (TKindred): both card-type folds count it (gorge TestKindredIsACardType) |
| 309.1-309.2c | partial | `dirac/act_initiative.go#State.dungeonEnter` | `TestInitLockstep` | the Undercity only (kgen lowers the token script as a dungeon card: CardDef.DungeonNext, one latent ability per room); it is put into the command zone as a dungeon object, never a permanent. Plain venture (the other dungeons) refuses |
| 309.2d | n/a | - | - | no corpus non-venture dungeon entry |
| 309.3 | n/a | - | - | - |
| 309.4-309.5b | follows | `dirac/act_initiative.go#State.venture` `dirac/act_initiative.go#State.markerEnter` | `TestInitLockstep` | the marker enters the top room, then follows one arrow or the venturer picks between two (ChRoom, gorge's "room" KChoose in printed order); each room entered queues its ability as a trigger the dungeon's owner controls, sourced by the dungeon (gorge checkDungeonRoomTriggers) |
| 309.6-309.7 | follows | `dirac/act_initiative.go#State.dungeonSBA` `dirac/act_initiative.go#State.dungeonRemove` | `TestInitLockstep` | completion: venturing from the bottommost room, or CR 704.5t once the bottom room's ability has left the stack; the object ceases. The completed-dungeon count is not kept (no pool card reads it; DungeonsCompleted$ refuses in kgen) |
| 310.1 | partial | `dirac/kgen/lower_battle.go#lowerCtx.refuseBattle` | `TestBattleRefusals` `TestBattleTransformLifted` | Part B accepts a 2-face DFC whose front is a Battle Siege with numeric Defense 1..127 and no enters replacement; other Battle shapes stay refused (token, non-DFC, battle back face, bad defense, enters replacement) |
| 310.2 | n/a | - | - | - |
| 310.3 | n/a | - | - | subtype syntax |
| 310.4-310.4c | partial | `dirac/face_battle.go#State.enterDefense` | `TestBattleEntersWithDefense` | a printed-face battle enters with CardDef.Defense CtrDefense counters through the counter replacements (gorge EntryCounterGrants); a face-down entry grants none |
| 310.5-310.6 | partial | `dirac/face_battle.go#State.dmgCounters` | `TestBattleCombatDamage` `TestBattleSpellDamage` | damage to a battle removes that many defense counters and is not marked, combat and spell alike (gorge foldDamage); infect damage is ordinary |
| 310.7 | partial | `dirac/face_battle.go#State.addBattleOffers` | `TestBattleAttackOffers` | the active player may attack any battle whose protector is the opponent, whoever controls it, never one they protect (gorge attackOffersCompute); SBA exiles directly instead of trigger |
| 310.8 | n/a | - | - | non-Siege also exiled (no corpus non-Siege) |
| 310.9 | n/a | - | - | protector designation |
| 310.9a | n/a | - | - | non-Siege protector from opponents (no corpus carrier) |
| 310.9b-310.9c | n/a | - | - | - |
| 310.9d-310.9g | n/a | - | - | - |
| 310.10 | partial | `dirac/face_battle.go#State.enterBattle` | `TestBattleEntersWithDefense` | two players: the sole opponent is recorded in perm.chosen as the protector, no ask (gorge applySiegeProtector); a control change keeps it |
| 310.11 | partial | `dirac/face_battle.go#State.defeatAsk` `dirac/face_battle.go#State.defeatQueue` | `TestBattleDefeatExiles` `TestBattleLockstep` `TestBattleMutation` | the defeated battle's DurDefeated play record is queued for the owner's transformed free cast; Part B asks whether to cast it (a no declines), then casts the back face free; re-choice derived, not asked; no-protector SBA absent |
| 310.12 | n/a | - | - | Siege subtype |
| 310.12a | n/a | - | - | - |
| 310.12b | follows | `dirac/face_battle.go#State.defeatAsk` | `TestBattleLockstep` | modelled as SBA exile + owner offer, no trigger |
| 311.1-311.7 | n/a | - | - | engine-only Config.PlanarDecks; no planar-controller concept |
| 312.1-312.7 | n/a | - | - | SILENT if a phenomenon is put in a planar deck |
| 313.1-313.7 | n/a | - | - | no Vanguard |
| 314.1-314.7 | n/a | - | - | no Archenemy |
| 315.1-315.7 | n/a | - | - | no Conspiracy Draft |
