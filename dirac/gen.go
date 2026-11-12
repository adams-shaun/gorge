package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The generated-table extension: the interpreter for everything dirac/kgen
// appended to kop (kgen -ops writes the contract, OPS.md). It lives in
// the *_gen.go files and reaches the core's core through one-line hooks guarded
// by k.gen != nil, so the hand-written tables never take these paths.

// genPrep is a generated table's precomputation.
type genPrep struct {
	// creatSub: the matchup subtypes that are creature types (CR 205.3m):
	// "all creature types" (an animated Soulstone Sanctuary) is no Forest
	creatSub    kop.SubMask
	graveTrig   []CardID // cards with a triggered ability that works from the graveyard
	graveDies   bool     // one of them is a Dies trigger (Furious Forebear; fireGraveDies)
	graveDrawn  bool     // some graveyard trigger is a Drawn (trig_gravedrawn.go)
	graveDmg    bool     // some graveyard trigger is a DamageDone (trig_gravedamage.go)
	graveETB    bool     // some graveyard trigger is an ETB (Fear of Infinity; face_fullyunlock.go)
	graveSac    bool     // some graveyard trigger is a Sacrificed (Curious Cadaver; act_sweeps.go)
	anyCastProv bool     // some trigger reads a spell's cast provenance (Tf2CastFromHand, Tf2Convoked): pushBuild records stackEntry.prov (trig_gravedrawn.go)
	anyCastBF   bool     // some card reads Count$LastStateBattlefieldWithFallback (CardDef.CastBF): beginCast freezes the count into the spell entry's trigMV (value_castbattlefield.go)
	// flags and the sparse small values (cardflags.go): the
	// eleven per-card bools and the dense int8/int16 sentinel arrays this
	// struct used to carry are one uint32 per card plus a record per card
	// that actually has a value.
	flags     []uint32
	smallRecs []cardSmall
	leyline   bool // some card is a Leyline
	// mana abilities a card only grants (Imprisoned in the Moon's "{T}: Add
	// {C}"): grantPC[i] is the mana source the permanent it is granted to
	// becomes for the autopay planner (manaPCGen)
	grantPC []pcard
	grantOf []grantRef
	// grantRestr: parallel to grantPC, the grant's spend restriction (a
	// cast-provenance word: Mm'menon's Spell.!wasCastFromYourHand; 0 none)
	grantRestr []kop.FilterID
	// mana abilities whose colour or amount is the permanent's own state
	// (Heraldic Banner's chosen colour, Elvish Archdruid's count): per card
	// the first of its precomputed planner views (gfManaVar marks one; the
	// chosen colour W..G or the amount 0..maxDynMana), varAb its ability
	varPC []pcard
	// fast-path gates: the table has an override static (an Aura that
	// rewrites what it enchants), a play-from-zone static, an ability that
	// works from the graveyard
	anyOvrStat      bool
	anyLoseStat     bool
	fxNeed          uint8 // CR 613 layer walk triggers (static_layerwalk.go fxNeed* bits); 0: the fixed-order code
	enteredNeed     bool  // some expression reads the ring
	actRingOn       bool  // some expression reads the ring (ExThisTurnActGrave): the activation push records it
	anyTurnF        bool  // some table reads this turn's per-player coin-flip or Exhaust-activation bits (genState.turnF; static_turnbits.go)
	castTapOn       bool  // castRingOn, or some card carries a Tf2ManaFromSelf reader: the planner's tap sites record a Treasure or observed source (noteTapSrc)
	anyManaHaste    bool  // some mana ability carries a ManaHasteValid rider (mana_spentrider.go)
	castRingOn      bool  // some expression reads the ring (ExThisTurnCastSpec): the push records it
	embSubGrant     bool  // an emblem has a Continuous subtype grant (walked on every walk slot)
	anyTypeValid    bool  // some card's as-enters type ask is narrowed by ValidTypes$ (CardDef.TypeValid; act_choosetype.go)
	anyAddChosen    bool  // some static is AddType$ ChosenType (kop.Static.AddChosen; act_choosetype.go)
	chosenCardTgt   bool  // some card target's or spell-cast trigger's filter reads the source permanent's chosen type (act_choosetype.go)
	castBoundSrc    bool  // some spell-cast trigger's filter reads a value, with the trigger's source as the source (trig_castbound.go)
	anyLoseRec      bool  // some override record removes abilities
	tyCombat        bool  // some override sets types or makes a creature: a combatant can stop being one (CR 506.4 removeNoncreatureCombatants; kperf2)
	l6RecOnly       bool  // fxNeedL6 comes only from an effect's keyword removal (Debuff): a slot without an fx record keeps the fixed-order layer 6 (l6On; kperf2)
	lossSlow        bool  // anyLoseStat: genLost always asks ovrLoses (kperf-blb, kperf2)
	ctrlSBA         bool  // the state-based check settles control (a control static, a source-bound OpGainControl)
	anyPTDelta      bool  // some Override is a +N/+M pump record (kop.Override.PTDelta): ptGen adds it (static_ptdelta.go)
	anyNTOv         bool  // an OpOverride / OpOverrideAll lasts until its controller's next turn
	legendGrant     bool  // some Override adds or sets the Legendary supertype (legendRule reads derived types)
	anyMayPlay      bool
	anyMayRaise     bool     // some may-play static owes a non-mana RaiseCost$ (cost_mayplayraise.go)
	anyMayKind      bool     // some OpMayPlay grants a record with a kind rider (cast_maypermission.go)
	anyEmbCold      bool     // some activated or triggered ability creates an emblem: the cold buffer holds maxEmbCold more per player (static_emblemcold.go)
	anyMayCold      bool     // some OpMayPlay grants over a remembered set: the cold buffer holds maxMayCold more records (cast_maycold.go)
	anyMayLimit     bool     // some OpMayPlay record carries a MayPlayLimit cap (cast_maypermission.go)
	sCls            []uint16 // per card: its static classes (static_classgates.go scls*)
	anyGraveAct     bool
	anyHandAct      bool // an activated ability works from the hand (cycling; repl_regenerate.go)
	anyRemPersist   bool // a card's enters trigger remembers what its leaves trigger reads (CardDef.RemPersist; zone_rememberpersist.go)
	anyNinjutsu     bool // a card has a K:Ninjutsu ability (kop.AfNinjutsu; kw_ninjutsu.go)
	anyPowerUp      bool // a card has a printed PowerUp$ ability (kw_powerup.go)
	anyManaOnce     bool // a card's first mana ability is activate-only-once (Loot, the Pathfinder; mana.go canTapForManaPC)
	anyStackTgt     bool // some target slot is a TgtStackObj (tgt_stackability.go)
	anyCopyAb       bool // some target slot is a TgtCopyAb or TgtCopyTrig (copy_ability.go)
	anyBestow       bool // a card has printed bestow (CardDef.HasBestow; kw_bestow.go)
	anyKickCost     bool // a card has a non-mana kicker cost (CardDef.KickerCost; kw_kicker.go)
	anyEvTargets    bool // a spell's CollectEvidence<X> is the targets' total mana value (kop.EvidenceTargetsMV; cost_collectevidencex.go)
	anyOptCost      bool // a card has an OptionalCost part (CardDef.OptCost/OptBehold; cost_optional.go)
	equipAnim       bool // an Equip ability and a Creature-adding override: an Equipment can be a creature (CR 704.5p sba arm; static_equipanimated.go)
	tyPost          bool // anyBestow || anyImpending: typesGen's switches after the overrides (kw_impending.go)
	anyDiscAll      bool // some trigger is Mode$ DiscardedAll (trig_discardedall.go)
	anyAbilityCast  bool // some trigger is Mode$ AbilityCast (trig_abilitycast.go)
	anyCanAtkDef    bool // a card has a StatCanAttackDefender static (static_canattackdefender.go)
	anyPanharm      bool // a card has a StatPanharmonicon static (trig_doubling.go)
	anyOwnReduce    bool // a card has an activated ability with its own ReduceCost$ (kop.Ability.OwnReduce; cost_ownreduce.go)
	anyAbReduce     bool // a card has a StatReduceAbility static (cost_reduceability.go)
	anyVergeOrder   bool // a gated Verge whose first ability is not its lowest colour: the window activates ability 0 (mana_gate.go)
	anyGatePair     bool // a gated same-colour larger mana ability is a pair unit (Whisperer of the Wilds; mana_gate.go)
	anyLandwalk     bool // a card has printed landwalk (repl_regenerate.go)
	anyEvasion      bool // a card has a printed CardDef.Evasion keyword (act_remember.go)
	anyAloneAtk     bool // a card has printed "can't attack alone" (CardDef.AloneAtk; combat_alone.go)
	anyAloneBlk     bool // a card has printed "can't block alone" (CardDef.AloneBlk; combat_alone.go)
	anyStatGoad     bool // a card has a StatGoad static (act_goad.go)
	anyTokEndCombat bool // some CreateTokenEx carries the AtEOT$ SacrificeCombat rider (trig_tokenateot.go)
	anyProwessGrant bool // some Continuous static grants prowess: onCast counts one trigger per static (kw_prowess.go)

	anyMinBlk                bool // a card has a printed MinMaxBlocker floor (CardDef.MinBlockers; combat.go)
	anyExtraBlock            bool // a card has a printed CardDef.ExtraBlocks (combat_extrablock.go)
	anyUntapOther            bool // a card has a StatUntapOther static (static_untapother.go)
	anyTapExtra              bool // a card has a StatTapExtra static (mana_tapextra.go)
	anyMaxBlk                bool // a card has a MinMaxBlocker Max$ 1 static (kop.StatMaxBlockers; combat_maxblockers.go)
	anyCantGain, anyCantDraw bool // StatCantGainLife / StatCantDraw statics (static_cantdraw.go)
	anyAsIfHaste             bool // a StatAsIfHaste static (static_asifhaste.go)
	anyCantSuspect           bool // a StatCantBeSuspected static (act_suspect.go suspectOne)
	anyNoPrev                bool // a StatCantPreventDamage static, printed or on an emblem (static_cantprevent.go)
	anyTypeCreat             bool // a StatAnyTypeCreatures static (mana_convertcreature.go)
	anyCdmgTough             bool // a card has a StatCombatDmgToughness static (combat_toughnessassign.go)
	anyDisETB                bool // a card has a DisableTriggers enters static (kop.StatDisableETB; trig_disable.go)
	anyLatentDies            bool // a card has a latent dies trigger an override or static can grant (trig_diesleaves.go)
	anyMadness               bool // a card has printed madness (CardDef.HasMadness; kw_madness.go)
	anyMiracle               bool // a card has printed miracle (CardDef.HasMiracle; kw_miracle.go)
	anyHexCol                bool // a card has printed hexproof from a colour (CardDef.HexColors; kw_hexproofcolor.go)
	anyHexOwn                bool // a card has a StatHexproofOwnColors static (kw_hexproofcolor.go)
	anyProtCol               bool // a card has printed protection from a colour (CardDef.ProtColors; kw_protection.go)
	// grantTrigEv: events (bit per TrigEvent) of triggered abilities a
	// Continuous static grants to the permanent its source is attached to
	// (AddTrigger$): their listeners include those hosts (kw_protection.go trigSubs)
	grantTrigEv uint32
	// globTrigGrant: a Continuous static grants a triggered ability beyond its
	// source's host ("artifact creatures you control have ..."), so any
	// permanent can hold the trigger (trigger_granted.go grantTrigHosts)
	globTrigGrant bool
	anyProtF      bool // a card has printed K:Protection:<spec> (CardDef.ProtFrom), or a granted one (anyProtGrant)
	anyProtGrant  bool // a Continuous static has a ProtFrom quality (kop.Static.ProtFrom; kw_protection.go)
	// manaTaps: the table has a Taps trigger (TrigTaps) and a granted mana
	// ability, which can tap a Taps listener for mana: the mana taps fire it
	// (manaTapped)
	manaTaps bool
	// anyTapAll: a TrigTaps line with Tf2Batch (Mode$ TapAll,
	// trig_tapall.go): the tapping actions open tap batches and every mana
	// tap fires the Taps listeners (manaTaps)
	anyTapAll bool
	// tapsWhose: a TrigTaps ability reads the tapper (Trig.Whose, "whenever
	// you tap ..."; trig_tapsvalid.go)
	tapsWhose bool
	// anyTWTap: a TrigTaps trigger with N == TrigNTeamworkTap; anyGameOnce: a
	// trigger with Tf3GameOnce (trig_params.go)
	anyTWTap, anyGameOnce bool
	// anySelfDisc: a card's own non-cycled "when you discard this card" line
	// (TrigDiscarded, TfSelf, Zone graveyard; fireSelfDisc)
	anySelfDisc bool
	// anyInfect: a card has printed infect or toxic (CardDef.Infect /
	// Toxic; kw_infect.go); infectStall: it shares the table with a shape its
	// damage-to-counters conversion does not order like gorge (StallInfect)
	anyInfect      bool
	infectStall    bool
	twoSeatOnly    bool           // a card plays only two seats (stallclass.go prepTwoSeat): StallTwoSeat in a larger game
	fbStampOwnCost bool           // a stamped flashback grant beside an own-cost flashback static (kw_fbgrant.go prepFbStamp): StallFbGrantStamp
	anyWitherGrant bool           // some table part grants kop.KwWither (kw_wither.go); sets anyInfect
	anyEOTEmb      bool           // some emblem ends at cleanup (kop.Emblem.UntilEOT; kw_protection.go)
	anyNTEmb       bool           // some emblem ends at its controller's next turn (kop.Emblem.UntilNextTurn; static_effectdelivered.go)
	anyNTEndEmb    bool           // some emblem ends at its controller's next turn's cleanup (kop.Emblem.UntilEndOfNextTurn; static_emblemdurations.go)
	embObjEv       uint32         // object events (bit per TrigEvent) some emblem trigger listens to (fireEmblemsObj)
	embCombatEv    uint32         // events (DamageDone, AttackersDeclared, LifeGained) some emblem trigger listens to (trig_effectuntileot.go)
	anyGrant       bool           // some override or static grants an ability
	meleeGrant     bool           // some static grants Melee (kop.Ability.MeleeGrant): slotTriggers places it as gorge's keyword walk does (kw_melee.go)
	anyWard        bool           // some card has printed ward: abilities get a stack identity (cast.go push; wardedIndex)
	anyWardGrant   bool           // some Continuous static grants ward {N} (kop.Static.Ward; kw_ward.go)
	btWatch        bool           // some BecomesTarget trigger watches other objects (Pawpatch Recruit; trig_becomestarget.go)
	btSpell        bool           // ... and one counts a creature spell on the stack (kop.Tf2SpellToo: Surrak)
	anyContGrant   bool           // some card has contGrant: the grant walk and availFor read it
	anySelfGrant   bool           // some granted mana ability reflects its grantee's colours (grantSelf)
	grantSelf      []bool         // parallel to grantPC: the grant reads the grantee's own colours (selfPC)
	selfPC         [32]pcard      // a self grant's planner view per colour mask (mana_owncolours.go)
	blgOn          bool           // a static or override adds a basic land type: manaPCGen reads the granted intrinsic abilities (static_landtype.go)
	blgSub         [5]uint8       // the Plains Island Swamp Mountain Forest subtype indexes (subOutside: not in the matchup)
	blgIdx         []int16        // per card: its block of 32 views in blgPC, -1 when not a plain-shaped land
	blgPC          []pcard        // a plain land's planner view per granted colour mask
	globGrant      bool           // a global override static grants an ability (Ygra; static_layerwalk.go globalGrants)
	anyHist        bool           // some trigger reads the turn history bank (expend, the N-th spell; value_turnhistory.go)
	anyPrevFrom    bool           // some program grants KwPreventCombatDamageFrom (repl_regenerate.go)
	anyTokRew      bool           // a ReplRewriteTokens, ReplAddToken or ReplCopyFirstToken ability (repl_tokrewrite.go, repl_tokorder.go)
	anyPlusCre     bool           // some program is ExtTurnRepl TurnReplPlusCreatureCtr: the cold buffer holds its per-player count and source (repl_turnrepl.go)
	anyDmgMark     bool           // some program is ExtDmgMark: every damage site reads the per-player marks (repl_dmgmark.go)
	anyFiltRepl    bool           // a filtered damage doubler or token multiplier, or an x3 one (repl_multipliers.go)
	untapRepl      bool           // a ReplOthersDoesntUntap or ReplAnyUntap card: the untap step runs untapStepGen (act_energy.go)
	anyUntapBlock  bool           // some card has ReplAnyUntap: every untap site consults State.untapBlocked (static_cantuntap.go)
	anyCtrBlock    bool           // some card has StatCantPutCounter: every counter placement consults State.putCounterBlocked (ctr_cantput.go)
	anyCastSpent   bool           // some SpellCast trigger carries Tf2CastSpent (opus)
	anySpentGate   bool           // some SpellCast trigger carries a Spell.ManaSpent gate in Trigger.N (trig_spentvalid.go)
	anySpent       bool           // some expression reads the mana spent to cast a spell (ExCastManaSpent; copy_asenters.go)
	anyTgtSpent    bool           // some expression reads the resolving spell's target spell's spend (ExTgtSpent; resolve.go snapTgtSpent)
	anyDmgAmt      bool           // some card reads the damage its object received this turn (CardDef.DmgAmt; value_dmgamount.go)
	anyBecomeCopy  bool           // some enters program is OpBecomeCopy (an as-enters clone): a permanent it made a copy is projected as its printed card, and a token copy of a clone is asked its election (ctr_entryperkind.go)
	anyEntryMerge  bool           // the table has an AddCounter replacement and more than one source of entry counters: enterGen merges them into one put per kind (ctr_entryperkind.go)
	anyEffEtbCtr   bool           // some instruction is ExtEffEtbCtr: enterGen puts a cast spell's pending extra counters on (ctr_effectentry.go)
	anyEtbOwnOther bool           // some card's own enter body is one prepared grant: the effect's rider count and the body compete over the entry, the CR 616.1 order is asked (repl_etborder.go)
	anyStaticTrig  bool           // some trigger carries Tf2Static: placeTriggers resolves it off the stack before the ordering ask (trig_mayplaystatic.go)
	anyColSpent    bool           // some expression reads the colours of mana spent to cast a spell (ExConverge, Tf3CastConverge): payment tallies stackEntry.spc (mana_converge.go)
	anyBatch       bool           // some trigger is a "one or more" line (Tf2Batch; trig_oneormore.go)
	anyLinkCtr     bool           // some until-leaves exile carries a return rider's counters (zone_untilleaves.go)
	anyLinkRet     bool           // some program exiles until its source leaves (OpExileUntilLeaves): departure frames hold the return's enters check (zone_untilleavesbatch.go)
	anyDmgRef      bool           // some damage trigger reads the damaged referent: Tf2PerReferent, Tf2ToYou or a Filter2 (trig_damagedonce.go)
	anyUnion       bool           // some op's subject is a target-slot union (kop.RefTargets01..; tgt_union.go)
	anyUnblocked   bool           // some trigger fires for unblocked attackers (kop.Tf2Unblocked; act_discover.go)
	anyOrCopy      bool           // some SpellCast trigger fires for spell copies too (kop.Tf2OrCopy; act_discover.go)
	anySkipUntap   bool           // some program skips a next untap step (OpSkipUntap): the untap step runs untapStepGen (act_discover.go)
	anyDelayUp     bool           // some program registers an upkeep or end-of-combat delayed program (kop.DelayUpkeep, kop.DelayEndCombat; trig_delayedtiming.go)
	anyDelayDies   bool           // some program registers a delayed dies program (kop.DelayDiesThisTurn; trig_delayeddies.go)
	anyDelaySpell  bool           // some program registers a delayed SpellCast program (kop.DelaySpellCast; trig_delayedspell.go)
	anyRaise       bool           // some card has a generated StatIncreaseCost (RaiseCost; trig_delayedtiming.go)
	anyRaiseLife   bool           // some card has a StatIncreaseCost with StRaiseLife (RaiseCost Cost$ PayLife<N>; cost_raiselife.go)
	anyCantCast    bool           // some card has a StatCantBeCast (trig_delayedtiming.go castForbidden)
	anyCantAct     bool           // some card has a StatCantActivate (cast_cantactivate.go actForbidden), or a StatCantSacrifice
	anyCantSac     bool           // some card has a StatCantSacrifice (static_cantsacrifice.go sacBlocked)
	anyCantTgt     bool           // some card has a StatCantTarget (tgt_canttarget.go cantTargetGen)
	anyFaces       bool           // some card has faces (CardDef.Faces; face_multiface.go)
	anyWarp        bool           // some card has warp (CardDef.HasWarp; kw_warp.go)
	anyDash        bool           // some card has dash (CardDef.HasDash; kw_dash.go)
	anyEvoke       bool           // some card has evoke (CardDef.HasEvoke) or some expression is ExAdamant (kw_evoke.go)
	anyImpending   bool           // some card has impending (CardDef.Impending; kw_impending.go)
	anyPrepare     bool           // some card is a preparation front (FacePrepare; face_prepare.go)
	anyPlot        bool           // some card has plot (CardDef.HasPlot; kw_plot.go)
	anyParadigm    bool           // some card has Paradigm (CardDef.Paradigm; kw_paradigm.go)
	anyNetLock     bool           // some program is ExtLockActs: canActivate and the untap/leave sites read the lock list (act_lockpermanent.go)
	anyUnearth     bool           // some program has OpUnearth: leaves check the unearth exile promise (cast_altfree.go)
	anyFinality    bool           // some program puts finality counters (kop.CtrFinality): dieZone exiles their creatures
	anyHone        bool           // some program puts hone counters (kop.CtrHone): ptGen adds +1/+0 per hone counter on an attached permanent (CR 122.1j; ctr_hone.go)
	anyBattle      bool           // some card is a Battle (kop.TBattle front; face_battle.go)
	anySneak       bool           // some card has sneak (CardDef.HasSneak; kw_sneak.go)
	anyWebSling    bool           // some card has web-slinging (CardDef.HasWebSling; kw_websling.go)
	anyMayFlash    bool           // some card has K:MayFlashCost (CardDef.HasMayFlash; kw_mayflash.go)
	anyDelve       bool           // some card has printed or granted Delve (CardDef.Delve, StatStackKw Delve; kw_delve.go)
	anyConvoke     bool           // some card has printed or granted convoke/improvise or harmonize (CardDef.Convoke/Improvise/Harmonize, StatStackKw Improvise/Convoke; kw_convoke.go)
	anyBeam        bool           // some card has printed beam me up (CardDef.BeamMeUp; kw_beammeup.go)
	anyBeamBar     bool           // some card has a StatCantBeBeamedUp static
	anyHarmonize   bool           // some card has printed harmonize (CardDef.Harmonize; kw_harmonize.go)
	anyCascade     bool           // some card has printed cascade (CardDef.Cascade; kw_cascade.go)
	anyStackKw     bool           // some card has a StatStackKw: AffectedZone$ Stack keyword grant (static_stackkeyword.go)
	anyConspire    bool           // some card has a StatStackKw Conspire grant (kw_conspire.go)
	anyStorm       bool           // some card has a StatStackKw Storm grant (kw_storm.go)
	anySplitSecond bool           // some card has a StatStackKw Split second grant (kw_splitsecond.go)
	anyStackLife   bool           // some card has a StatStackKw Lifelink grant (static_stacklifelink.go)
	anySplit       bool           // some card is a split card (kop.FaceSplit: its mana value off the stack; face_castface.go)
	anyTwobrid     bool           // some card has a twobrid pip in its printed cost (kop.ManaCost.Tb; mana_twobrid.go)
	anyBeamGrant   bool           // some card has a StatBeamGrant static
	anyFbGrant     bool           // some card has a StatFbGrant static (kw_fbgrant.go)
	anyWarpGrant   bool           // some card has a StatWarpGrant static (kw_warpgrant.go; sets anyWarp too)
	warpGrantCost  kop.ManaCost   // the first StatWarpGrant's warp cost (kw_warpgrant.go warpGrantCost's fallback)
	partyRole      [4]uint8       // the matchup subtype index of Cleric, Rogue, Warrior, Wizard (subOutside: absent; act_remember.go partySize)
	outlawSub      [5]uint8       // the matchup subtype index of Assassin, Mercenary, Pirate, Rogue, Warlock (subOutside: absent; filter_valueheads.go PredOutlaw)
	anyRemSac      bool           // some OpSacrifice remembers what it sacrifices (B.N&1, RememberSacrificed$; act_remember.go)
	anyDiscard     bool           // some OpDiscard: a held discard has room (coldArrays.discHeld)
	anyRemDisc     bool           // some OpDiscard remembers what it discards (B.N&1, RememberDiscarded$; act_remember.go)
	anySpellMana   bool           // some spell ability's program carries an OpAddMana (mana_dbresolve.go)
	cpxPT          bool           // some copy exception sets P/T (Offspring; copy_exceptions.go)
	cpxSub         bool           // some copy exception adds a subtype (Abyssal Harvester)
	cpxCol         bool           // some copy exception sets the colours (embalm)
	cpxNoCost      bool           // some copy exception removes the mana cost (embalm)
	cpxTy          bool           // some copy exception adds a card type or supertype (copy_riders.go)
	cpxNoLeg       bool           // some copy exception removes Legendary (copy_nonlegendary.go)
	anyProlif      bool           // some trigger is TrigProliferate (act_proliferate.go)
	anyExplEvTrig  bool           // some trigger is TrigExplores or TrigCollectEvidence (trig_exploreevidence.go)
	anyScryTrig    bool           // some trigger is TrigScry or TrigSurveil (trig_scrysurveil.go)
	anyLandPlayed  bool           // a trigger is TrigLandPlayed (trig_modes.go)
	anyCzPair      bool           // a TrigLeaves line has a destination mask (N) (trig_zonepairs.go)
	anyLeavesGrave bool           // a TrigLeaves line also fires for a graveyard leave (kop.Tf2LeavesGrave; trig_leavesgrave.go)
	anyKwAct       bool           // a trigger is TrigSearchedLibrary, TrigForage, TrigDiscover or TrigManifestDread
	anyDilemma     bool           // some table reads ExDilemmasFaced or TrigFacesDilemma (trig_dilemma.go)
	anyScryTally   bool           // some expr is ExScriedThisTurn (value_scrytally.go)
	anyTokCr       bool           // some trigger is TrigTokenCreated (act_proliferate.go)
	anyMonstrosity bool           // some program is OpMonstrosity or some trigger is TrigBecomeMonstrous (act_monstrosity.go)
	anyAirbend     bool           // some program is ExtAirbend (act_airbend.go)
	anyMayWhileSrc bool           // some OpMayPlay lasts DurWhileSrcMay (cast_whilesource.go)
	anyGoad        bool           // some program is OpGoad (act_goad.go)
	anyPermMark    bool           // the table gives some kop.PredPermMark mark an arena mask (filter_permmarks.go)
	anyTappedTurn  bool           // the table gives mechTappedTurn an arena mask (a Taps FirstTime$ line; filter_permmarks.go)
	anyCombatHit   bool           // some card records its combat hits on players (cdRec) or some expression reads the legendary-hit bit (value_turntallies.go)
	cdLegOn        bool           // some expression is ExOppsLostLife N 1: record cdLeg
	anyAddPhase    bool           // some program is ExtAddPhase, some trigger is Tf3FirstAttack or some expression is ExPhaseCount (act_addphase.go)
	anySkipPhase   bool           // some program is ExtSkipPhase (act_skipphase.go)
	anyAttackedObj bool           // the table gives mechAttackedObj an arena mask: finishAttacks marks the declared attackers
	anySuspect     bool           // the table gives mechSuspected an arena mask (act_suspect.go)
	anySacSub      bool           // some ExRefCount is a Sacrificed$Valid type read (N 1): snapLKI captures the subtype set (value_countheads.go, zone_lki.go)
	ix             [numMech]uint8 // mechanic -> index in State's ms or mo, by its width; arenaNone (0xFF) when the table does not use it (arenaAssign; arena.go)
	anyInit        bool           // some program is OpTakeInitiative (act_initiative.go)
	dungeon        CardID         // anyInit: the dungeon card it ventures into
	anyFaceDown    bool           // some card has Morph, Megamorph or Disguise (CardDef.MorphFamily; face_facedown.go)
	anyTurnUpRepl  bool           // a ReplCantTurnUp or ReplTurnUpProgram card: the turn-up special action asks face_turnfaceup.go
	anyTransformed bool           // some trigger is TrigTransformed (face_transform.go)
	crimeOn        bool           // some trigger is TrigCommitCrime or some expression is ExCommittedCrime (act_commitcrime.go): guards every crime site
	anyEmpower     bool           // some program is OpEmpower (act_empower.go): guards the op dispatch on FDN/BLB
	anyTapPower    bool           // some ability costs CostTapPower (Crew, Saddle; kw_crewsaddle.go)
	anyEarthbend   bool           // some program earthbends (OpEarthbend; act_earthbend.go)
	anyLeaveRet    bool           // some program registers a this-turn "when it's put into a graveyard, return it" record (Melira; trig_leavereturn.go)
	anyOnePoison   bool           // some card has ReplOnePoison (Melira; trig_leavereturn.go)
	anyCopyElect   bool           // some copy may choose new targets (copyElect holds its resolution)
	anyPermCopy    bool           // some OpCopySpell may copy a permanent spell: the copy becomes a token (OpCopySpell C&2; copy_permanentspell.go)
	anyCharmNum    bool           // some ability picks several modes (ModesPick > 1; modal_charmnum.go)
	anyCharmV      bool           // some card has CardDef.ModesPickV: a dynamic charm pick count (modal_charmcmp.go)
	anyCharmPick   bool           // some charm carries ChoiceRestriction$ (Ability.ModeRestrict; modal_charmpick.go)
	anyWaterbend   bool           // some ability costs CostWaterbend (act_waterbend.go)
	anyXBoundAb    bool           // some activated ability's target is bounded by its X (xBoundTarget): the announced X can leave it no target, which reverses it
	anyLure        bool           // some card is a lure (CardDef.Lure), or the table can make one (StatLure, mechLure; combat_lure.go)
	anyLureStat    bool           // a StatLure static (combat_lure.go isLure)
	anyMustBlock   bool           // some card is printed must-be-blocked or the table gives mechMustBlocked an arena mask (combat_mustbeblocked.go)
	anyFaceExile   bool           // some Dig exiles face down (kop.DigFaceDown; zone_exilefacedown.go)
	anyExW         bool           // some card reads the exiled-with source link (CardDef.ExiledWith; zone_exiledwith.go)
	exwRec         []uint8        // per card: its resolutions record the cards they exile (anyExW only): exwRecRead, exwRecRev
	anyCloneCopy   bool           // some program is ExtCloneCopy: permanent copies record their source (fxCopyPerm; copy_clone.go)
	anyAegis       bool           // some program is ExtAegisClone, or some card has a TrigAttached ability (zone_exilethenattach.go)
	anyHideaway    bool           // some program is ExtHideaway (kw_hideaway.go)
	anyLoyaltyX    bool           // some ability costs a planeswalker's [-X] (CostLoyaltyX; cost_loyaltyx.go)
	anyLifeX       bool           // some spell or ability costs an announced PayLife<X> (Cost.N 0; cost_paylifex.go)
	anySacX        bool           // some spell or ability costs an announced Sac<X/Spec> (CostSac with Cost.N 0; cost_sacx.go)
	anyXMin        bool           // some spell or ability floors its announced X (Cost.XMin; cost_sacx.go)
	anyCopyCast    bool           // some OpCastFree casts a copy (C bit 6; cast_copycast.go)
	anyExileGrant  bool           // some program holds an ExtGrantExile (cast_exilegrant.go)
	anyNumTimes    bool           // some ability announces a pay-N-times count (Ext NumTimes; cast_announcetimes.go)
	anyExHaste     bool           // some ability can't be blocked except by creatures with haste (Ext ExHaste; static_cantblockexcept.go)
	anyColGrant    bool           // some ability grants by a chosen colour (Ext ColorGrant; static_colourgrant.go)
	anyDiscRef     bool           // some expression reads RefDiscarded (value_discardedref.go)
	anySpellCond   bool           // some spell ability carries a cast restriction in Trig.Cond (cast_spellcondition.go)
	anyEntUntap    bool           // some ReplOthersEnterProgram untaps the entrant (ExtEnterUntapped): a tap/untap pair asks the CR 616.1 order (repl_entryorder.go)
	anyEntAttack   bool           // some ReplOthersEnterProgram enters the entrant attacking (ExtEnterAttacking): beside another enter body it competes (repl_entryorder.go)
	anyEachTgt     bool           // some ExtEachDamage reads damager target slots (extop.go)
	anyXBadMV      bool           // a TgfMaxTotalMV slot: later ops on its objects see them as the resolution began (tgt_setconstraints.go)
	anyTgtSetBf    bool           // a TgfSharedCrType slot on the battlefield (tgt_setconstraints.go)
	anyUniqLater   bool           // some ability has a mandatory TgfDistinct slot after its first (tgt_uniquechain.go)
	anySaddle      bool           // some program saddles (OpSaddle; kw_crewsaddle.go)
	anySolve       bool           // some program solves a Case (OpSaddle C=1; kw_crewsaddle.go)
	anyStation     bool           // some ability is a Station ability (AfStation; ctr_kinds.go)
	anyPoisonHigh  bool           // some expression reads the opponents' highest poison (ExHighestOppPoison; value_highestpoison.go)
	anyDiesNC      bool           // some dies trigger can match a noncreature permanent (trig_diesnoncreature.go diesNC)
	anyHandSize    bool           // some static or emblem is NoMaxHandSize (maxHand; static_playerstats.go)
	anyYouHex      bool           // some Continuous static gives its controller hexproof (StYouHexproof; ctr_shield.go)
	anyRole        bool           // some card carries the Role subtype: the SBA runs the Role exclusivity check (static_roletokens.go)
	roleSub        uint8          // the matchup subtype index of Role (static_roletokens.go)
	anyWhileHost   bool           // some override or emblem ends when its host leaves (kop.Override/Emblem.HostBound; filter_combatwords.go)
	anyEmbCtrl     bool           // some emblem ends when its host's controller changes (kop.Emblem.AsLongAsControl; static_emblemdurations.go)
	anyWhileTapped bool           // some override also ends at its host's untap (kop.Override.WhileTapped; static_animatewhiletapped.go)
	anyOvCantAtk   bool           // some override says its permanent can't attack (kop.Override.CantAttack; filter_combatwords.go)
	anySrcMark     bool           // some filter reads F2DamagedBy or F2CrewedSrc: the source-mark records are kept (filter_combatwords.go)
	dmgBy          []bool         // per card: its damage is recorded (an ability's filter reads F2DamagedBy; nil when none)
	crewerRec      []bool         // per card: the Vehicles it crews are recorded (an ability's filter reads W7CrewedBySrc; nil when none; filter_props.go)
	crewBy         []bool         // per card: its crewers are recorded (an ability's filter reads F2CrewedSrc; nil when none)
	// the damage / token / counter / zone replacements and the player
	// hexproof static (replBits per card, rb* bits); anyRepl gates every
	// site, so a table without these cards never takes their paths
	replBits     []uint32
	anyRepl      uint32
	exileOnLeave []kop.FilterID // per card: ReplExileOnLeave's filter (nil when no card has one)
	exileAnyC    []bool         // per card: its ReplExileOnLeave is the any-zone reading (Repl.C 1; nil when none; repl_exileonleave.go)
	destroyHeal  []kop.FilterID // per card: ReplDestroyHeal's filter (nil when no card has one; repl_destroyheal.go)
	anyExileAny  bool           // some ReplExileOnLeave has Repl.C 1: a move to a graveyard from any zone checks it (placeFrom)
	altFree      []uint8        // per card: 1 + the ability index of its own free StatAlternativeCost (nil when no card has one; cast_altfree.go)
	// anyDeferred: some card has a deferred mana source (gfDeferred; mana_deferred.go)
	anyDeferred bool
	// anyTapsDefer / anyTapsPain: some card has a lowered Taps watcher of
	// class kop.TapPlanDefer / TapPlanPain (gfTapsDefer / gfTapsPain; mana_tapsdefer.go)
	anyTapsDefer, anyTapsPain bool
	// anyManaRider: some card has a counter-rider mana ability (mana_rider.go); it sets anyTapsDefer too
	anyManaRider bool
	// anyCondSplit: some card has a Condition-split mana source (mana_condsplit.go); it sets anyTapsDefer too
	anyCondSplit bool
	// anySelfTaps: some card has a lowered self Taps watcher (TfSelf). kgen
	// lowers one on a card with a printed mana ability only as the vetted
	// "deals N damage to you" line (TapPlanPain, City of Brass: priced, never a
	// gap), so any other can only defer a granted mana ability's tap
	// (act_waterbend.go windowCensusComplete).
	anySelfTaps bool
	// anyDiscHand: some card carries gfDiscHand (mana_discardhand.go).
	anyDiscHand bool
	// dhMix: a Diamond beside another census-gap kind (mana_diamondmix.go prepDiamondMix): StallDiamondMix
	dhMix bool
	// eachCol: card i's "one mana of each colour among" ability (Bloom
	// Tender), -1 none: a source gorge's planner census misses, reached by
	// a potential play's scripted prefix (script_gen.go)
	anyEachCol bool
	anyRestr   bool // some card has restr (kperf-blb: the ward / unless window's gate)
	// anyRestrScript: some card has a restricted source gorge's census still
	// misses (a reflected one: Plaza of Heroes), a scripted-prefix gap
	// (gfRestrScript), or a deferred source (gfDeferred; mana_deferred.go); anyRestrSrc: some card has a restricted source the
	// payment planner counts per payment (gorge; restrSrcAb), and
	// rs holds its records (mana_restrictedsource.go)
	anyRestrScript bool
	anyRestrSrc    bool
	rs             *restrData
	// restr: card i's first mana ability when it is a tap ability with a
	// spend restriction (Giada's Angel-only {W}), -1 otherwise: another
	// census gap, which gorge's prefix search activates in battlefield
	// order with the Bloom Tenders (script_gen.go scriptDeadEnd)
	// restrF: the distinct spend restrictions (ManaRestrict filters) of the
	// table's restricted mana abilities, in table order; the restricted
	// mana floating beside a pool names its restriction by index
	// (a State.rb batch's kind), so restrKinds are encodable (Giada's Angel
	// spells, Secluded Courtyard's creature spells of the chosen type, a
	// Standard table's three restricted lands)
	restrF [restrKinds]kop.FilterID
	// restrV: restrF's activated-half variants (restrVarOf), and restrVar the
	// distinct activated halves of the table's restricted abilities: two
	// sources of one spell half with different activated halves are two
	// restrictions (Castle Doom's bare artifact-spell mana against Guidelight
	// Optimizer's, which also activates abilities)
	restrV   [restrKinds]uint8
	restrVar [restrVars]struct {
		ab kop.FilterID
		ns bool
	}
	// anyR: card i's restricted any-colour tap mana ability (Secluded
	// Courtyard's "{T}: Add one mana of any color. Spend this mana only to
	// cast a creature spell of the chosen type ..."), -1 none: a census
	// gap too, whose activation gorge's prefix search branches on (its
	// unrestricted first mana ability, {C}, then each colour)
	anyAnyR bool
	// anyRefl: some card has a reflected mana ability (gfManaRefl; Plaza of
	// Heroes): an anyR-only board takes the exact prefix search (reflScript)
	anyRefl bool
	// anyMultiR: some card has a restricted two-mana any-colour ability
	// (restrWideAnyMana; Interdimensional Web Watch)
	anyMultiR bool
	// filt: card i's filter mana ability (Hidden Grotto's "{1}, {T}:
	// Add one mana of any color", beside a free {T} first one), -1 none: a
	// paid census gap, which gorge's prefix search funds and activates
	// (mana_filter.go)
	anyFilt bool
	// anyFiltPip: some filter is a gated one-pip fixed-colour one
	// (mana_filterpip.go)
	anyFiltPip bool
	// anyFiltFirst: some card's filter is its first mana ability (Abzan
	// Devotee, Prophetic Prism; mana_filterfirst.go)
	anyFiltFirst bool
	// gapOther: card i has another mana ability gorge's census misses
	// (a restricted, each-colour or paid one that is not filt: Lupinflower
	// Village's creature-only {W}); beside one the filter stays unmodelled
	// divCard: card i has a target slot whose damage divides (bsDivide's scan)
	// anyScriptGap: some card has eachCol, restr or anyR (a scripted play
	// is possible)
	anyScriptGap bool
	// anyWindowGap: some card has eachCol, restr or a deferred source (gorge's
	// ward / unless window activates them, mana.go windowTap)
	anyWindowGap bool
	// l6Stall: a global lose-all override static shares the table with a
	// subtype lord the layer-4 walk reads (the loss check would re-enter
	// the walk): the game stalls at its start (static_layer6.go prepL6)
	l6Stall bool
	// anyTriggerAttackers: some program reads RefTriggerAttackers
	// (trig_attackers.go)
	anyTriggerAttackers bool
	// anyBatchPool: some trigger picks among its batch's members
	// (trig_batchmembers.go); it shares the attacker-group pool
	anyBatchPool bool
	// anyFirstCast: some expression is ExFirstCast, so the cast path stamps the
	// first spell each seat casts (cast_firstcast.go)
	anyFirstCast bool
}

// Replacement bits (genPrep.replBits).
const (
	rbDblDmg       uint32 = 1 << iota // ReplDoubleDamageToOpps
	rbPrevNC                          // ReplPreventNoncombatToOthers
	rbDblTok                          // ReplDoubleTokens
	rbDblCtr                          // ReplDoubleCounters
	rbGraveLib                        // ReplGraveToLibrary (the card itself, any zone)
	rbPlHex                           // StatPlayerHexproof
	rbProtAll                         // printed protection from everything: its damage is prevented (Progenitus)
	rbProtCol                         // protection from a colour, printed or granted by a static (Akroma's Memorial): damage from a source of that colour is prevented
	rbPlusDmg                         // ReplDamagePlus (trig_oneormore.go)
	rbDblLife                         // ReplLifeDouble (act_discover.go)
	rbPlusLife                        // ReplLifePlus (act_discover.go)
	rbShield                          // the table places SHIELD counters (CR 122.1c; ctr_shield.go): set in anyRepl only
	rbExileOnLeave                    // ReplExileOnLeave
	rbDblCtrOn                        // ReplDoubleCountersOn
	rbPlusCtrOn                       // ReplAddCountersOn
	rbDblCtrYou                       // ReplDoubleCountersYou (trig_leavereturn.go)
	rbPrevCol                         // Ext PreventColor: damage from sources of a chosen colour is prevented this turn (anyRepl only; cast_altfree.go)
	rbDblLoss                         // ReplLifeLossDouble (repl_lifelossdouble.go)
	rbPlProt                          // StatPlayerHexproof with Static.A == 1: protection from each opponent, its damage is prevented (static_playerstats.go)
	rbDrawMore                        // ReplDrawMore (repl_draw.go)
	rbPlusCtrEOT                      // Ext CounterPlusEOT: the table can put one more of each kind for a player until end of turn (anyRepl only; ctr_pluseot.go)
	rbDestroyHeal                     // ReplDestroyHeal (repl_destroyheal.go)
	rbPrevDmgTo                       // CardDef.PreventDmgTo: every damage event to the permanent is prevented (combat_prevention.go)
	rbPrevCombatTo                    // CardDef.PreventCombatTo: its combat damage events are prevented (combat_prevention.go)
	rbPrevCombatBy                    // CardDef.PreventCombatBy: the combat damage it would deal is prevented (combat_preventfrom.go)
	rbDiscETB                         // ReplDiscardOppToBattlefield (repl_discardetb.go)
	rbPrevMill                        // ReplPreventMillOpps (repl_preventmill.go)
	rbPrevCtrSelf                     // ReplPreventDamageCounters (repl_selfdamage.go)
	rbHealSelf                        // ReplDamageHealFirst (repl_selfdamage.go)
	rbPayETB                          // ReplPayBeforeETB (repl_paybeforeetb.go)
)

// rbCtrOn is either filtered counter replacement.
const rbCtrOn = rbDblCtrOn | rbPlusCtrOn

// rbDmgCompete are the damage-replacement kinds that do not commute with a
// plus-N addition: a doubling, a prevention or a shield. A table whose
// anyRepl has none of them competes on additive amount changes only, whose
// composition is the same in every order (trig_oneormore.go).
const rbDmgCompete = rbDblDmg | rbPrevNC | rbPrevDmgTo | rbPrevCol | rbShield | rbPrevMill | rbPrevCombatTo | rbPrevCombatBy

// replPlusOnly reports whether the table's only damage replacement kind is
// ReplDamagePlus, so two or more of them competing for one event are a sum
// no order changes (CR 616.1e; trig_oneormore.go).
func replPlusOnly(g *genPrep) bool {
	return g != nil && g.anyRepl&rbPlusDmg != 0 && g.anyRepl&rbDmgCompete == 0
}

type grantRef struct {
	card CardID
	ab   uint8
}

// noManaPC is the planner's view of a permanent whose mana ability an
// override took away.
var noManaPC = pcard{manaAb: -1}

// tableHasPrevent reports whether any card carries a printed K:Prevent damage
// fact (CardDef.Prevent*), which prepareGen must carry in replBits even on an
// otherwise featureless table.
func tableHasPrevent(t *kop.Table) bool {
	for i := range t.Cards {
		c := &t.Cards[i]
		if c.PreventDmgTo || c.PreventCombatTo || c.PreventCombatBy {
			return true
		}
	}
	return false
}

// prepareGen precomputes a generated table (nil for a hand-written one).
func prepareGen(t *kop.Table, p *prep) *genPrep {
	p.phasing = tablePhasing(t) // kw_phasing.go: whatever else the table reads
	if t.Exprs == nil && t.TypeVocab == nil && t.Conds == nil && !tableHasStatic(t, kop.StatCantPhaseIn) && !tableHasPrevent(t) && !tableHasEnteredFrom(t) {
		return nil
	}
	enteredNeed := tableHasEnteredFrom(t)
	for i := range t.Exprs {
		if k := t.Exprs[i].Kind; k == kop.ExThisTurnEntered || k == kop.ExRevolt || k == kop.ExLeftZone {
			enteredNeed = true
		}
	}
	g := &genPrep{
		enteredNeed: enteredNeed,
		flags:       make([]uint32, len(t.Cards)),
		replBits:    make([]uint32, len(t.Cards)),
	}
	for i := range t.Cards {
		// the printed K:Prevent facts (dirac/kgen/lower_preventkw.go)
		c := &t.Cards[i]
		if c.PreventDmgTo {
			g.replBits[i] |= rbPrevDmgTo
		}
		if c.PreventCombatTo {
			g.replBits[i] |= rbPrevCombatTo
		}
		if c.PreventCombatBy {
			g.replBits[i] |= rbPrevCombatBy
		}
	}
	for i, n := range t.Subtypes {
		if creatureTypeWords[n] {
			g.creatSub = g.creatSub.With(i)
		}
	}
	for i := range t.Overrides {
		if t.Overrides[i].LoseAbilities {
			g.anyLoseRec = true
		}
		if t.Overrides[i].PTDelta {
			g.anyPTDelta = true
		}
	}
	for i := range t.Overrides {
		for _, gr := range t.Overrides[i].Grant {
			if gr != 0 {
				g.anyGrant = true
			}
		}
	}
	g.cpxPT, g.cpxSub, g.cpxCol, g.cpxNoCost, g.cpxTy, g.cpxNoLeg = copyExcFlags(t)
	for i := range t.Exprs {
		if t.Exprs[i].Kind == kop.ExYourTurns && t.Exprs[i].N == 0 {
			p.anyYourTurns = true // value_exprs.go
		}
		if t.Exprs[i].Kind == kop.ExOppsAttacked {
			p.anyOppsAtt = true // value_oppsattacked.go
		}
		if t.Exprs[i].Kind == kop.ExRedNCDmg {
			p.anyRedNC = true // the tally rides replDamage, which anyRepl&rbPlusDmg gates open (value_damagetally.go)
		}
		if t.Exprs[i].Kind == kop.ExRefCount && t.Exprs[i].N == 1 {
			g.anySacSub = true // Sacrificed$Valid Card/creature-type: snapLKI keeps the subtype set
		}
	}
	prepParadigm(t, g) // kw_paradigm.go: before the ring's Treasure scan
	for i := range t.Exprs {
		if t.Exprs[i].Kind == kop.ExThisTurnCast && t.Exprs[i].N == 0 {
			g.anyHist = true // Count$ThisTurnCast reads the history bank's spell count
		}
		if t.Exprs[i].Kind == kop.ExThisTurnCastSpec {
			g.castRingOn, g.anyHist = true, true // the push count is in the history bank
		}
		if t.Exprs[i].Kind == kop.ExVoid {
			g.castRingOn, g.anyHist, g.enteredNeed = true, true, true // value_void.go: both rings record
		}
		if t.Exprs[i].Kind == kop.ExThisTurnActGrave {
			g.actRingOn = true
		}
		if t.Exprs[i].Kind == kop.ExFirstCast {
			g.anyFirstCast = true // cast_firstcast.go
		}
		if t.Exprs[i].Kind == kop.ExDilemmasFaced {
			g.anyDilemma = true // trig_dilemma.go
		}
		if t.Exprs[i].Kind == kop.ExScriedThisTurn {
			g.anyScryTally = true // value_scrytally.go
		}
		if t.Exprs[i].Kind == kop.ExCommittedCrime {
			g.crimeOn = true
		}
		if t.Exprs[i].Kind == kop.ExCastManaSpent {
			g.anySpent, g.anyHist = true, true // recorded at payment by histCast
		}
		if t.Exprs[i].Kind == kop.ExTgtSpent {
			g.anyTgtSpent, g.anySpent, g.anyHist = true, true, true // the target spell's spend is recorded at its payment
		}
		// value_dmgamount.go: every damage site records the amount for a reader card
		g.anyDmgAmt = g.anyDmgAmt || t.Exprs[i].Kind == kop.ExDmgAmountReceived
	}
	if tableNthFiltered(t) {
		g.castRingOn, g.anyHist = true, true
	}
	if g.castRingOn {
		for i, n := range t.Subtypes {
			if n == "Treasure" {
				for c := range t.Cards {
					if t.Cards[c].Subtypes.Has(i) {
						p.cards[c].castSrc |= csTreasure
					}
				}
				break
			}
		}
	}
	g.anyDelayUp = tableDelaysUpkeep(t) // true with anyDelaySpell: fireDelayedGen must keep the SpellCast records
	g.anyDelaySpell = tableDelaysSpell(t)
	g.anyDelayDies = tableDelaysDies(t)
	g.anyUnblocked = tableUnblocked(t)
	g.anyOrCopy = tableTrigFlag2(t, kop.TrigSpellCast, kop.Tf2OrCopy)
	if tableTrigFlag2(t, kop.TrigSpellCast, kop.Tf2Increment) {
		g.anySpent, g.anyHist = true, true // the Increment comparison (fire and resolution) reads the spell's recorded spend (kw_increment.go); histCast itself is anyHist-gated. Never anyCastSpent: no planner decline
	}
	prepCtrAdded(t, p, g) // ctr_addedthisturn.go
	if tableSpentGate(t) {
		g.anySpentGate, g.anySpent, g.anyHist = true, true, true // the Spell.ManaSpent gate reads the spell's recorded spend (trig_spentvalid.go); never anyCastSpent: no planner decline
	}
	prepSkipUntap(t, g)
	prepOpus(t, g)
	prepConverge(t, g)     // mana_converge.go
	prepEvoke(t, g)        // kw_evoke.go
	prepEffEtbCtr(t, p, g) // ctr_effectentry.go: the rider counts, and the prepared bodies the entry order is asked over
	prepCtrPlusEOT(t, g)   // ctr_pluseot.go
	prepTurnRepl(t, g)     // repl_turnrepl.go
	prepGrantPanharm(t, g) // act_grantpanharm.go
	prepStaticTrig(t, g)   // trig_mayplaystatic.go
	prepCrewSaddle(t, g)
	prepManaSrc(t, g, p) // mana_spendtrigger.go
	prepRoles(t, g)
	prepEarthbend(t, g)
	prepP3(t, g)       // trig_leavereturn.go
	prepTokAtEOT(t, g) // trig_tokenateot.go
	prepCharmNum(t, g)
	prepCharmCmp(t, g) // modal_charmcmp.go
	prepCharmPick(t, g)
	prepPermCopy(t, g)
	prepCopyCast(t, g)
	prepExileGrant(t, g) // cast_exilegrant.go
	prepWaterbend(t, g)
	prepCostEvidence(t, g)
	prepLure(t, g)
	prepMustBlock(t, g) // combat_mustbeblocked.go
	prepColGrant(t, g)
	prepExHaste(t, g) // static_cantblockexcept.go
	prepHideaway(t, g)
	prepAegis(t, g)
	prepCloneCopy(t, g)
	prepNumTimes(t, g)
	prepLoyaltyX(t, g)
	prepLifeX(t, g)
	prepSacX(t, g)
	prepDiscarded(t, g)
	prepUniqueChain(t, g)
	prepTgtSet(t, g) // tgt_setconstraints.go
	prepEachDamage(t, g)
	prepEntUntap(t, g)
	prepKC(t, g)
	prepPoisonHigh(t, g)
	prepShield(t, g)
	prepDiesNC(t, g)
	prepMcombat(t, g)
	prepAltFree(t, g, p) // cast_altfree.go
	prepUnearth(t, g)    // cast_altfree.go
	prepPlotTrig(t, p)   // kw_plot.go
	prepPlotGrant(t, g)  // kw_plot.go
	prepNetLock(t, g)    // act_lockpermanent.go
	prepFinality(t, g)   // ctr_finality.go
	prepHone(t, g)       // ctr_hone.go
	prepPrevCol(t, g)    // cast_altfree.go
	prepStackKw(t, g, p) // static_stackkeyword.go
	g.anyCdmgTough = tableHasStatic(t, kop.StatCombatDmgToughness)
	g.anyRaise = tableRaises(t)
	g.anyRaiseLife = tableRaiseLife(t)
	g.anyTwobrid = tableTwobrid(t)
	g.anyBeamGrant = tableHasStatic(t, kop.StatBeamGrant)
	g.anyCantCast = tableHasStatic(t, kop.StatCantBeCast) || prepSpellCond(t, g)
	prepCastLimit(t, g) // static_castlimit.go
	prepCantSac(t, g)   // static_cantsacrifice.go, before prepCantAct reads it
	prepCantAct(t, g)   // cast_cantactivate.go
	g.anyCantTgt = tableHasStatic(t, kop.StatCantTarget)
	g.anyMaxBlk = tableHasStatic(t, kop.StatMaxBlockers)
	g.anyUntapOther = tableHasStatic(t, kop.StatUntapOther)
	g.anyTapExtra = tableHasStatic(t, kop.StatTapExtra) || p.manaEach // an all-at-once source's own extra (mana_eachcolour.go)
	g.anyTurnF = tableHasStatic(t, kop.StatFlipCoinMod) || tableHasStatic(t, kop.StatActivationLimit)
	for i := range t.Exprs {
		if t.Exprs[i].Kind == kop.ExThisTurnActExhaust {
			g.anyTurnF = true // a standalone Count$ThisTurnActivated exhaust reader records the bit too (static_turnbits.go)
		}
	}
	g.anyDisETB = tableHasStatic(t, kop.StatDisableETB)
	g.anyCantGain = tableHasStatic(t, kop.StatCantGainLife)
	g.anyCantDraw = tableHasStatic(t, kop.StatCantDraw)
	g.anyNoPrev = tableHasStatic(t, kop.StatCantPreventDamage)
	g.anyAsIfHaste = tableHasStatic(t, kop.StatAsIfHaste)
	g.anyCantSuspect = tableHasStatic(t, kop.StatCantBeSuspected)
	g.anyLureStat = tableHasStatic(t, kop.StatLure)
	g.anyTypeCreat = tableHasStatic(t, kop.StatAnyTypeCreatures)
	g.anyDiscard = tableHasOp(t, kop.OpDiscard)
	g.anyRemDisc = tableRemembersDiscards(t)
	g.anySpellMana = prepDbMana(t) // mana_dbresolve.go
	g.anyMayKind = tableMayPlayKinds(t)
	g.anyMayCold = tableMayPlayRemembered(t)
	g.anyEmbCold = tableEmblemRepeats(t)
	g.anyMayRaise = tableMayRaise(t)
	if tableCastFromGrave(t) {
		g.anyCastProv = true // castFromGrave reads the cast's zone (cast_mayplayoffers.go)
	}
	prepWasCast(t, g) // cast_wascastenter.go
	if g.anyMayLimit = tableMayPlayLimit(t); g.anyMayLimit {
		g.anyHist = true // histCast records the may-play count (value_turnhistory.go)
	}
	g.partyRole = partyRoles(t)
	g.outlawSub = outlawSubs(t) // filter_valueheads.go
	g.anyRemSac = tableRemembersOp(t, kop.OpSacrifice) || tableRemembersIfPaidSac(t)
	g.anyUnion = tableUnion(t)
	g.anyLinkRet = tableLinkRet(t)
	g.anyLinkCtr = tableLinkCtr(t)
	prepStaticClasses(t, g)
	prepHexOwn(t, g) // kw_hexproofcolor.go
	for i := range t.Cards {
		c := &t.Cards[i]
		pc := &p.cards[i]
		if c.Leyline {
			g.leyline = true
		}
		if c.SelfFlash {
			p.anySelfFlash = true
		}
		if len(c.Faces) > 0 {
			g.anyFaces = true
			g.anyPrepare = g.anyPrepare || c.FaceMode == kop.FacePrepare
			g.anySplit = g.anySplit || c.FaceMode == kop.FaceSplit || c.FaceMode == kop.FaceRoom
			p.anyRoom = p.anyRoom || c.FaceMode == kop.FaceRoom
		}
		if c.Daybound || c.Nightbound {
			p.anyDayNight, g.anyHist = true, true // the spell count rides the history bank
		}
		if c.Landwalk != 0 || landwalkGrantIn(c.Abilities) {
			g.anyLandwalk = true
		}
		if c.CastBF != 0 {
			g.anyCastBF = true // the cast-time battlefield count (value_castbattlefield.go)
		}
		if c.HasWarp {
			g.anyWarp = true
		}
		if c.Impending > 0 {
			g.anyImpending = true
			p.cards[i].altCost = true // the hand walk's one-byte gate (offerAltHand)
		}
		if c.HasEvoke {
			g.anyEvoke = true
			p.cards[i].altCost = true // the hand walk's one-byte gate (offerAltHand)
		}
		if c.HasDash {
			g.anyDash = true
			p.cards[i].altCost = true // the hand walk's one-byte gate (offerAltHand)
		}
		if c.HasPlot {
			g.anyPlot = true
		}
		if c.RemPersist {
			g.anyRemPersist = true
		}
		if c.ExiledWith {
			if g.exwRec == nil {
				g.exwRec = make([]uint8, len(t.Cards))
			}
			g.anyExW, g.exwRec[i] = true, exwRecRead
			if c.ExiledWithRev {
				g.exwRec[i] |= exwRecRev
			}
		}
		if c.Types&kop.TBattle != 0 {
			g.anyBattle = true
		}
		if c.HasSneak {
			g.anySneak = true
		}
		if c.HasWebSling {
			g.anyWebSling = true
		}
		if c.HasMayFlash {
			g.anyMayFlash = true
		}
		if c.HasBestow {
			g.anyBestow = true
		}
		if c.OptCost.Flags != 0 || c.OptBehold || c.OptWaterbend != 0 {
			g.anyOptCost = true
		}
		if c.KickerCost.Flags != 0 {
			g.anyKickCost = true
		}
		g.anyBeam = g.anyBeam || c.BeamMeUp
		if c.TapPays() || c.Harmonize {
			g.anyConvoke = true
			g.anyHarmonize = g.anyHarmonize || c.Harmonize
		}
		if c.Delve {
			g.anyDelve = true
		}
		if c.Cascade != 0 {
			g.anyCascade = true
		}
		if c.MorphFamily != 0 || cardMintsFaceDown(c) {
			g.anyFaceDown = true
		}
		if c.Evasion != 0 {
			g.anyEvasion = true
		}
		if c.AloneAtk {
			g.anyAloneAtk = true
		}
		if c.AloneBlk {
			g.anyAloneBlk = true
		}
		if c.MinBlockers != 0 {
			g.anyMinBlk = true
		}
		if c.ExtraBlocks != 0 {
			g.anyExtraBlock = true
		}
		if c.HasMadness {
			g.anyMadness = true
		}
		if c.HasMiracle {
			g.anyMiracle = true
		}
		firstMana := true
		graveAdded := false // graveTrig lists a card once, though it may carry several graveyard triggers
		for j := range c.Abilities {
			a := &c.Abilities[j]
			g.prepAddManaRestr(a) // mana_resolverestriction.go
			for ti := 0; ti < int(a.NTargets); ti++ {
				if k := a.Targets[ti].Kind; k == kop.TgtStackObj || k == kop.TgtStackAb || k == kop.TgtStackObjTrig {
					g.anyStackTgt = true // tgt_stackability.go
				} else if copyAbKind(k) {
					g.anyStackTgt, g.anyCopyAb = true, true // copy_ability.go
				}
			}
			switch a.Kind {
			case kop.AbTriggered:
				if a.Trig.Event == kop.TrigManaExpend || a.Trig.Event == kop.TrigSpellCast && a.Trig.Flags2&kop.Tf2NthSpell != 0 {
					g.anyHist = true
				}
				if a.Trig.Flags2&kop.Tf2Batch != 0 {
					g.anyBatch = true
				}
				if (a.Trig.Event == kop.TrigDamageDone || a.Trig.Event == kop.TrigDamageDealtOnce) && (a.Trig.Filter2 != 0 || a.Trig.Flags2&(kop.Tf2PerReferent|kop.Tf2ToYou) != 0) {
					g.anyDmgRef = true
				}
				if a.Trig.Event == kop.TrigProliferate {
					g.anyProlif = true
				}
				if a.Trig.Event == kop.TrigFacesDilemma {
					g.anyDilemma = true
				}
				if a.Trig.Event == kop.TrigScry || a.Trig.Event == kop.TrigSurveil {
					g.anyScryTrig = true
				}
				if a.Trig.Event == kop.TrigLandPlayed {
					g.anyLandPlayed = true
				}
				if czPairLine(a) {
					g.anyCzPair = true
					if a.Trig.Flags2&kop.Tf2LeavesGrave != 0 {
						g.anyLeavesGrave = true
					}
				}
				if a.Trig.Event == kop.TrigSearchedLibrary || a.Trig.Event == kop.TrigForage || a.Trig.Event == kop.TrigDiscover || a.Trig.Event == kop.TrigManifestDread {
					g.anyKwAct = true
				}
				if a.Trig.Event == kop.TrigExplores || a.Trig.Event == kop.TrigCollectEvidence {
					g.anyExplEvTrig = true
				}
				if a.Trig.Event == kop.TrigTokenCreated {
					g.anyTokCr = true
				}
				if a.Trig.Event == kop.TrigBecomeMonstrous {
					g.anyMonstrosity = true
				}
				if a.Trig.Event == kop.TrigTransformed {
					g.anyTransformed = true
				}
				if a.Trig.Event == kop.TrigBecomesTarget &&
					(a.Trig.Flags&kop.TfSelf == 0 ||
						a.Trig.Whose == kop.WhoseOpp && a.Trig.N != kop.TrigNValiant) {
					// A BecomesTarget watcher, or the self form "whenever
					// this becomes the target of a spell or ability an
					// opponent controls" (TfSelf + WhoseOpp;
					// trig_becomestarget.go). Both must route
					// through btWatchers so the opponent-source gate applies
					// to the self line too.
					g.btWatch = true
					if a.Trig.Flags&kop.TfSelf == 0 && a.Trig.Flags2&kop.Tf2SpellToo != 0 {
						g.btSpell = true
					}
				}
				if a.AFlags&kop.AfLatent != 0 {
					// latent: reached by an op or a grant, never subscribed
					continue
				}
				if a.Trig.Event == kop.TrigCommitCrime {
					g.crimeOn = true
				}
				if a.Trig.Event == kop.TrigFullyUnlock {
					p.anyFullyUnlock = true // face_fullyunlock.go
				}
				if a.Trig.Flags2&(kop.Tf2CastFromHand|kop.Tf2Convoked) != 0 {
					g.anyCastProv = true
				}
				if a.Trig.Zone == kop.ZoneGraveyard {
					if !graveAdded {
						g.graveTrig = append(g.graveTrig, CardID(i))
						graveAdded = true
					}
					if a.Trig.Event == kop.TrigDrawn {
						g.graveDrawn = true
					}
					if a.Trig.Event == kop.TrigDamageDone {
						g.graveDmg = true
					}
					if a.Trig.Event == kop.TrigDies {
						g.graveDies = true
					}
					if a.Trig.Event == kop.TrigSacrificed {
						g.graveSac = true
					}
					if a.Trig.Event == kop.TrigETB {
						g.graveETB = true // face_fullyunlock.go
					}
				}
			case kop.AbReplacement:
				switch a.Repl.Kind {
				case kop.ReplEntersProgram:
					g.setFlag(CardID(i), gfEnterRepl)
					if len(a.Prog) > 0 && a.Prog[0].Op == kop.OpUnlessPaid && (int(a.Prog[0].C&0xF) == kop.PayLife || revealElection(&a.Prog[0])) {
						g.setFlag(CardID(i), gfShockEntry)
					}
				case kop.ReplOthersEnterProgram:
					g.setFlag(CardID(i), gfOthersRep)
					pc.hasStat = true // scanned through statM
				case kop.ReplCantLoseGame, kop.ReplOppsCantWinGame:
					g.setFlag(CardID(i), gfLifeRepl)
					pc.hasStat = true
				case kop.ReplDoubleDamageToOpps:
					g.replBits[i] |= rbDblDmg
					pc.hasStat = true
				case kop.ReplDamagePlus:
					g.replBits[i] |= rbPlusDmg
					pc.hasStat = true
				case kop.ReplPreventNoncombatToOthers:
					g.replBits[i] |= rbPrevNC
					pc.hasStat = true
				case kop.ReplPreventMillOpps:
					g.replBits[i] |= rbPrevMill
					pc.hasStat = true
				case kop.ReplPreventDamageCounters:
					g.replBits[i] |= rbPrevCtrSelf
					pc.hasStat = true
				case kop.ReplDamageHealFirst:
					g.replBits[i] |= rbHealSelf
					pc.hasStat = true
				case kop.ReplDoubleTokens, kop.ReplRewriteTokens, kop.ReplAddToken, kop.ReplCopyFirstToken:
					g.replBits[i] |= rbDblTok
					pc.hasStat = true
				case kop.ReplDoubleCounters:
					g.replBits[i] |= rbDblCtr
					pc.hasStat = true
				case kop.ReplExileOnLeave:
					g.replBits[i] |= rbExileOnLeave
					pc.hasStat = true
					if g.exileOnLeave == nil {
						g.exileOnLeave = make([]kop.FilterID, len(t.Cards))
					}
					g.exileOnLeave[i] = a.Repl.Filter
					if a.Repl.C == 1 {
						if g.exileAnyC == nil {
							g.exileAnyC = make([]bool, len(t.Cards))
						}
						g.exileAnyC[i], g.anyExileAny = true, true
					}
				case kop.ReplDoubleCountersOn:
					g.replBits[i] |= rbDblCtrOn | ctrWildBits(a)
					pc.hasStat = true
				case kop.ReplAddCountersOn:
					g.replBits[i] |= rbPlusCtrOn | ctrWildBits(a)
					pc.hasStat = true
				case kop.ReplDoubleCountersYou:
					g.replBits[i] |= rbDblCtrYou
					pc.hasStat = true
				case kop.ReplLifeDouble:
					g.replBits[i] |= rbDblLife
					pc.hasStat = true
				case kop.ReplLifePlus:
					g.replBits[i] |= rbPlusLife
					pc.hasStat = true
				case kop.ReplDrawMore:
					g.replBits[i] |= rbDrawMore
					pc.hasStat = true
				case kop.ReplLifeLossDouble:
					g.replBits[i] |= rbDblLoss
					pc.hasStat = true
				case kop.ReplDestroyHeal:
					g.replBits[i] |= rbDestroyHeal
					pc.hasStat = true
					if g.destroyHeal == nil {
						g.destroyHeal = make([]kop.FilterID, len(t.Cards))
					}
					g.destroyHeal[i] = a.Repl.Filter
				case kop.ReplDiscardOppToBattlefield:
					g.replBits[i] |= rbDiscETB
				case kop.ReplPayBeforeETB:
					g.replBits[i] |= rbPayETB
				case kop.ReplGraveToLibrary:
					g.replBits[i] |= rbGraveLib
				case kop.ReplMayNotUntap:
					g.untapRepl = true // the untap step runs untapStepGen, which asks (repl_untapchoice.go)
				case kop.ReplOthersDoesntUntap:
					g.untapRepl = true
					pc.hasStat = true // scanned through statM
				case kop.ReplAnyUntap:
					g.untapRepl = true
					g.anyUntapBlock = true
					pc.hasStat = true // scanned through statM
				case kop.ReplOnePoison:
					pc.hasStat = true // scanned through statM (trig_leavereturn.go)
				case kop.ReplCantTurnUp:
					g.anyTurnUpRepl = true
					pc.hasStat = true // scanned through statM (face_turnfaceup.go)
				case kop.ReplTurnUpProgram:
					g.anyTurnUpRepl = true
				}
			case kop.AbStatic:
				switch a.Static.Kind {
				case kop.StatCDAPT:
					g.setFlag(CardID(i), gfCDAPT)
					g.smallFor(CardID(i)).cdaAb = int16(j)
				case kop.StatOverride:
					g.setFlag(CardID(i), gfOvrStat)
					g.anyOvrStat = true
					g.fxNeed |= fxNeedOvr
					if !filterHostOnly(t, a.Static.Filter) {
						g.setFlag(CardID(i), gfGlobOvr)
						g.fxNeed |= fxNeedGlobal
						if o := a.Static.Override; o > 0 && t.Overrides[o-1].Grant != [2]uint8{} {
							g.globGrant = true
						}
					}
					if o := a.Static.Override; o > 0 && t.Overrides[o-1].LoseAbilities {
						g.setFlag(CardID(i), gfLoseStat)
						g.anyLoseStat = true
					}
				case kop.StatMayPlayFrom:
					g.anyMayPlay = true
				case kop.StatCanAttackDefender:
					g.anyCanAtkDef = true
				case kop.StatPanharmonicon:
					g.anyPanharm = true
				case kop.StatReduceAbility:
					g.anyAbReduce = true
				case kop.StatContinuous:
					if a.Static.Ward != 0 || a.Static.WardLife != 0 {
						g.anyWardGrant = true
					}
					if a.Static.Kw&kop.KwProwess != 0 {
						g.anyProwessGrant = true
					}
				case kop.StatPlayerHexproof:
					g.replBits[i] |= rbPlHex
					if a.Static.A == 1 {
						g.replBits[i] |= rbPlProt
					}
				case kop.StatNoMaxHandSize:
					g.anyHandSize = true
				case kop.StatCantBeBeamedUp:
					g.anyBeamBar = true
					pc.hasStat = true // scanned through statM (kw_beammeup.go)
				case kop.StatBeamGrant:
					g.anyBeam = true
					pc.hasStat = true // scanned through statM (kw_beammeup.go)
				case kop.StatFbGrant:
					g.anyFbGrant = true
					pc.hasStat = true // scanned through statM (kw_fbgrant.go)
				case kop.StatWarpGrant:
					if !g.anyWarpGrant {
						g.warpGrantCost = a.Static.GrantCost
					}
					g.anyWarpGrant, g.anyWarp = true, true
					pc.hasStat = true // scanned through statM (kw_warpgrant.go)
				case kop.StatCantPutCounter:
					g.anyCtrBlock = true
					pc.hasStat = true // scanned through statM (ctr_cantput.go)
				}
				if a.Static.Grant != 0 {
					g.anyGrant = true
					// a grant not limited to the source's host is globalGrantsGen's
					// (gfGlobGrant: exactly the statics its walk reports); a non-mana
					// one turns the walk on. An attached-only grant (Fishing Pole's
					// "equipped creature has ...") is the grantM walk's: turning
					// globGrant on for it made every grantedAbilitiesN call walk every
					// static source on a pool table.
					if !filterAttachedOnly(t, a.Static.Filter) {
						if a.Static.Kind == kop.StatContinuous {
							g.setFlag(CardID(i), gfGlobGrant)
							if int(a.Static.Grant) <= len(t.Cards[i].Abilities) && t.Cards[i].Abilities[a.Static.Grant-1].Kind == kop.AbTriggered {
								g.globTrigGrant = true
							}
						}
						if int(a.Static.Grant) <= len(t.Cards[i].Abilities) && t.Cards[i].Abilities[a.Static.Grant-1].Kind != kop.AbMana {
							g.globGrant = true
						}
					}
				}
			case kop.AbActivated:
				if a.ActZone == kop.ZoneGraveyard {
					g.anyGraveAct = true
				}
				if a.ActZone == kop.ZoneHand {
					g.anyHandAct = true
				}
				if a.AFlags&kop.AfNinjutsu != 0 {
					g.anyNinjutsu = true
				}
				if a.Cost.Flags&kop.CostPowerUp != 0 {
					g.anyPowerUp = true
				}
				if a.OwnReduce != (kop.ValueRef{}) {
					g.anyOwnReduce = true
				}
			case kop.AbMana:
				if _, _, ok := manaCtrRider(a); ok || kop.ManaRiderAb(a) {
					g.anyManaRider, g.anyTapsDefer = true, true // a planner-deferred source (mana_rider.go)
				}
				if a.ManaDeferred && a.AFlags&kop.AfLatent == 0 {
					// a scripted-prefix gap beside the restricted ones (mana_deferred.go),
					// and no filter: gapOthers must see it so filtOnly is false
					g.smallFor(CardID(i)).gapOther = int8(j)
					g.setFlag(CardID(i), gfDeferred|gfGapOther)
					g.anyDeferred, g.anyRestrScript, g.anyWindowGap = true, true, true
				}
				if kop.DiscardHandManaAb(a) {
					g.setFlag(CardID(i), gfDiscHand) // a prefix-search gap alone, never a planner unit (mana_discardhand.go)
					g.anyDiscHand = true
				}
				if a.AFlags&kop.AfManaEachColor != 0 && a.AFlags&kop.AfLatent == 0 && g.eachCol(CardID(i)) < 0 {
					g.smallFor(CardID(i)).eachCol = int8(j)
					g.setFlag(CardID(i), gfEachCol)
					g.anyEachCol, g.anyWindowGap = true, true
				}
				if a.ManaRestrict != 0 {
					g.regRestrVar(a)
					g.addRestrFV(a.ManaRestrict, g.restrVarOf(a))
				}
				if firstMana && a.AFlags&kop.AfLatent == 0 {
					firstMana = false
					if a.Cost.Flags&kop.CostOncePerGame != 0 {
						g.anyManaOnce = true
					}
					if restrWideAnyMana(a) {
						// a census gap the prefix search branches on (mana_window.go restrWideAnyMana)
						g.smallFor(CardID(i)).anyR = int8(j)
						g.setFlag(CardID(i), gfAnyR)
						g.anyAnyR, g.anyMultiR = true, true
					} else if restrTapMana(a) {
						g.smallFor(CardID(i)).restr = int8(j)
						g.setFlag(CardID(i), gfRestr)
						g.anyRestr, g.anyWindowGap = true, true
						if !restrSrcAb(a) {
							g.setFlag(CardID(i), gfRestrScript) // a planner unit is no script gap (mana_restrictedsource.go)
							g.anyRestrScript = true
						}
					} else if kop.FilterManaAb(a) {
						g.prepFiltFirst(CardID(i), j, a) // a filter first and only (mana_filterfirst.go)
					}
				} else if restrAnyTapMana(a) && !restrSrcAb(a) && (!restrWideAnyMana(a) || wideBesideAny(&t.Cards[i], a)) && g.anyR(CardID(i)) < 0 && firstManaAb(&t.Cards[i]) >= 0 {
					g.smallFor(CardID(i)).anyR = int8(j)
					g.setFlag(CardID(i), gfAnyR)
					g.anyAnyR = true
					g.anyMultiR = g.anyMultiR || restrWideAnyMana(a) // (Hermitic Herbalist: mana_restrictedcombo.go)
				} else if filterTapMana(&t.Cards[i], a) && g.filt(CardID(i)) < 0 {
					g.smallFor(CardID(i)).filt = int8(j)
					g.setFlag(CardID(i), gfFilt)
					g.anyFilt = true
					if _, pip := fixedPipFilt(a); pip {
						g.anyFiltPip = true
					}
				}
				if int(g.filt(CardID(i))) != j && censusGapMana(a) && !lifePainLandAb(&t.Cards[i], a) &&
					!kop.ChosenSecondManaAb(&t.Cards[i], firstManaAb(&t.Cards[i]), j) && g.gapOther(CardID(i)) < 0 {
					// (Quirion Elves' chosen {T} is folded into the source's
					// unit by prepMultiMana, not a census gap: registering it as
					// one would keep a filter beside it off the filtScript path)
					g.smallFor(CardID(i)).gapOther = int8(j)
					g.setFlag(CardID(i), gfGapOther)
				}
				if a.AFlags&kop.AfLatent != 0 {
					var gp pcard
					gp.manaAb, gp.manaCol, gp.manaN = int8(j), a.ManaColors, max(a.ManaN, 1)
					gp.manaTap = a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0
					prepGrantCost(&gp, a) // {T} + pay N life, sacrifice this alone (mana_grantcost.go)
					self := a.AFlags&kop.AfManaReflect != 0 && a.ManaAmong == 0
					if self {
						g.prepSelfGrant(gp) // mana_owncolours.go
					}
					g.grantSelf = append(g.grantSelf, self)
					g.grantPC = append(g.grantPC, gp)
					g.grantRestr = append(g.grantRestr, a.ManaRestrict)
					g.grantOf = append(g.grantOf, grantRef{CardID(i), uint8(j)})
				}
				// Mana abilities the autopay planner can't run (chosen colour,
				// dynamic amount, a spend restriction, a sacrifice cost, one the
				// card only grants: Imprisoned in the Moon) are
				// not autopay sources.
				if int(pc.manaAb) == j && a.AFlags&kop.AfManaChosen != 0 && a.AFlags&(kop.AfManaDynamic|kop.AfLatent) == 0 && a.ManaRestrict == 0 {
					// gorge's planner prices a chosen-colour source as a
					// one-colour choice (rules payment_plan.go's reflected
					// alternative): an autopay source of that colour
					pc.manaChosen = true
				} else {
					if int(pc.manaAb) == j && a.AFlags&kop.AfLatent == 0 && a.ManaRestrict == 0 && a.AFlags&kop.AfManaDynamic != 0 && a.Cost.Flags == kop.CostTap && !g.has(CardID(i), gfManaVar) {
						// a dynamic amount: one variant per amount (manaPCGen)
						g.setFlag(CardID(i), gfManaVar)
						g.smallFor(CardID(i)).manaVar = int16(len(g.varPC))
						for n := 0; n <= maxDynMana; n++ {
							vp := pcard{manaAb: int8(j), manaCol: a.ManaColors, manaN: uint8(n), manaTap: true, manaDyn: true, manaOne: manaOneColour(a, uint8(n))}
							if n == 0 {
								vp.manaAb = -1
							}
							g.varPC = append(g.varPC, vp)
						}
					}
					if int(pc.manaAb) == j && (a.AFlags&(kop.AfManaChosen|kop.AfManaDynamic|kop.AfLatent) != 0 || a.ManaRestrict != 0 || a.ManaColors == 0 || a.ManaDeferred) {
						pc.manaAb = -1
					}
				}
			}
		}
		prepManaDynPair(g, c, CardID(i), pc)
		prepManaGate(g, c, CardID(i), pc)
		prepManaRefl(g, c, CardID(i), pc)
		prepManaSplit(g, c, CardID(i), pc)
		g.prepRestrSrc(c, CardID(i), pc)
		// latent triggered abilities are not subscriptions
		pc.trigMask = 0
		for j := range c.Abilities {
			a := &c.Abilities[j]
			if a.Kind == kop.AbTriggered && a.AFlags&kop.AfLatent != 0 && a.Trig.Event == kop.TrigDies && a.Trig.Zone != kop.ZoneCommand {
				g.anyLatentDies = true // a granted dies trigger (trig_diesleaves.go)
			}
			if a.Kind == kop.AbTriggered && a.AFlags&kop.AfLatent == 0 && a.Trig.Zone != kop.ZoneGraveyard && a.Trig.Zone != kop.ZoneCommand {
				pc.trigMask |= 1 << a.Trig.Event
				if a.Trig.Event == kop.TrigDamageDealtOnce {
					pc.trigMask |= 1 << kop.TrigDamageDone // the damage paths fire both
				}
			}
		}
		for j := range c.Abilities {
			a := &c.Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.Grant != 0 {
				if ga := &c.Abilities[a.Static.Grant-1]; ga.Kind == kop.AbTriggered {
					// the source subscribes for its host (trigSubs adds the host)
					pc.trigMask |= 1 << ga.Trig.Event
					g.grantTrigEv |= 1 << ga.Trig.Event
					g.meleeGrant = g.meleeGrant || ga.MeleeGrant
				}
			}
		}
		pc.trigMask &= 1<<nSubEvents - 1 // TrigGiveGift: no subscription (kw_gift.go)
	}
	// a multi-face card shares one card text for the
	// exiled-with association (gorge's exileProvenanceNeeded scans every
	// face), so a face whose other face reads the link still records the
	// exile: Jade Seedstones' front pays a craft cost that exiles a creature,
	// and its back face reads ExiledWith$CardManaCost.
	for i := range t.Cards {
		fs := t.Cards[i].Faces
		if len(fs) == 0 {
			continue
		}
		bits := uint8(0)
		if g.exwRec != nil {
			bits |= g.exwRec[i]
		}
		for _, f := range fs {
			if int(f) < len(g.exwRec) {
				bits |= g.exwRec[f]
			}
		}
		if bits == 0 {
			continue
		}
		if g.exwRec == nil {
			g.exwRec = make([]uint8, len(t.Cards))
		}
		g.anyExW = true
		g.exwRec[i] |= bits
		for _, f := range fs {
			g.exwRec[f] |= bits
		}
	}
	anyTaps, manaGrant, manaTapsPrinted := false, false, false
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			anyTaps = anyTaps || a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps
			g.anyTapAll = g.anyTapAll || a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps && a.Trig.Flags2&kop.Tf2Batch != 0
			g.tapsWhose = g.tapsWhose || a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps && a.Trig.Whose != kop.WhoseAny
			if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps {
				g.anySelfTaps = g.anySelfTaps || a.Trig.Flags&kop.TfSelf != 0
				switch a.Trig.TapPlan {
				case kop.TapPlanDefer:
					g.setFlag(CardID(i), gfTapsDefer)
					g.anyTapsDefer = true
				case kop.TapPlanPain:
					g.setFlag(CardID(i), gfTapsPain)
					g.anyTapsPain = true
				}
			}
			g.anySelfDisc = g.anySelfDisc || a.Kind == kop.AbTriggered && selfDiscLine(a)
			g.anyTWTap = g.anyTWTap || a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps && a.Trig.N == kop.TrigNTeamworkTap
			if a.Kind == kop.AbTriggered && a.Trig.Flags3&kop.Tf3GameOnce != 0 {
				if j >= 0x3E {
					panic("kernel: GameActivationLimit on ability index >= 62") // usedRec.ab packs 0x40|ab
				}
				g.anyGameOnce = true
			}
			manaGrant = manaGrant || a.Kind == kop.AbMana && a.AFlags&kop.AfLatent != 0
			// A printed mana ability whose activation cost taps ({T}: Add …,
			// tap X creatures for mana) is a mana tap the TrigTaps lines must
			// fire on; without this the gate prices only granted mana and
			// ordinary sources, so Hawkeye's Bow never triggers on a planned
			// tap for a cast.
			manaTapsPrinted = manaTapsPrinted || a.Kind == kop.AbMana && a.Cost.Flags&(kop.CostTap|kop.CostTapX) != 0
		}
	}
	g.manaTaps = anyTaps && (manaGrant || manaTapsPrinted || g.anyTapAll)
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if a := &t.Emblems[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatNoMaxHandSize {
				g.anyHandSize = true
			}
		}
		if t.Emblems[i].UntilEOT {
			g.anyEOTEmb = true
		}
		if t.Emblems[i].UntilNextTurn {
			g.anyNTEmb = true
		}
		if t.Emblems[i].UntilEndOfNextTurn {
			g.anyNTEndEmb = true
		}
		for j := range t.Emblems[i].Abilities {
			if a := &t.Emblems[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCanAttackDefender {
				g.anyCanAtkDef = true // an Effect-delivered grant: prepared like a printed one
			}
			if a := &t.Emblems[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatMayPlayFrom {
				g.anyMayPlay = true // an Effect-delivered library-top permission (cast_staticpaylife.go)
			}
			if a := &t.Emblems[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.Kw&kop.KwProwess != 0 {
				g.anyProwessGrant = true
			} else if a.Kind == kop.AbTriggered {
				switch a.Trig.Event {
				case kop.TrigETB, kop.TrigDies, kop.TrigLeaves, kop.TrigSacrificed:
					g.embObjEv |= 1 << a.Trig.Event
				case kop.TrigDamageDone, kop.TrigAttackersDeclared, kop.TrigAttacks, kop.TrigLifeGained:
					g.embCombatEv |= 1 << a.Trig.Event
				case kop.TrigDamageDealtOnce:
					g.embCombatEv |= 1 << kop.TrigDamageDone // the same hook (fireEmblemsDmg)
				}
			}
		}
	}
	for i := range t.Cards {
		if t.Cards[i].Keywords&kop.KwWard != 0 {
			g.anyWard = true
		}
		if t.Cards[i].Keywords&kop.KwProtectionAll != 0 {
			g.replBits[i] |= rbProtAll
		}
		if t.Cards[i].Keywords&protColKw != 0 {
			g.replBits[i] |= rbProtCol
		}
		if t.Cards[i].ProtFrom != 0 {
			g.replBits[i] |= rbProtCol
			g.anyProtF = true
		}
		if t.Cards[i].HexColors != 0 {
			g.anyHexCol = true
		}
		if t.Cards[i].ProtColors != 0 {
			g.replBits[i] |= rbProtCol
			g.anyProtCol = true
		}
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kw&protColKw != 0 {
				g.replBits[i] |= rbProtCol
			}
			for ti := 0; ti < int(a.NTargets); ti++ {
				if a.Targets[ti].Divide != 0 {
					g.setFlag(CardID(i), gfDivCard)
				}
			}
		}
	}
	prepProtGrant(t, g) // kw_protection.go
	for _, b := range g.replBits {
		g.anyRepl |= b
	}
	prepEntryMerge(t, g) // ctr_entryperkind.go
	prepBecomeCopy(t, g) // ctr_entryperkind.go
	if p.anyRedNC || g.anyRepl&rbPlProt != 0 {
		g.anyRepl |= rbPlusDmg // open every replDamage site; no card carries the bit, so replPlus finds nothing
	}
	// a filter a reducer takes to nothing is the source's own mana (mana_filterfree.go)
	prepFiltFree(t, g, p)
	g.anyScriptGap = g.anyEachCol || g.anyRestrScript || g.anyAnyR || g.anyFilt || p.tapMana // (a tap-another-permanent mana gap: mana_tapanother.go)
	// a Diamond beside another census-gap kind (mana_diamondmix.go)
	prepDiamondMix(t, g, p)
	g.anyPrevFrom = grantsKw(t, kop.KwPreventCombatDamageFrom) || g.anyRepl&rbPrevCombatBy != 0
	prepReplMult(t, g)
	prepDmgMark(t, g) // repl_dmgmark.go
	prepInfect(t, g)
	prepTwoSeat(t, g) // stallclass.go
	prepFbStamp(t, g) // kw_fbgrant.go
	prepPoison(t, g)
	prepLayerWalk(t, g)
	prepChooseType(t, g) // act_choosetype.go
	prepCastBound(t, g)  // trig_castbound.go
	prepL6(t, g)
	prepCtrl(t, g)
	prepTokMana(t, g)
	prepMonstrosity(t, g)
	prepDiscAll(t, g)
	prepAbilityCast(t, g)
	prepGoad(t, g)           // act_goad.go
	prepArena(t, g, &p.cold) // arena.go
	prepSuspect(g)           // act_suspect.go; reads g.ix
	prepPermMark(g)          // filter_permmarks.go; reads g.ix
	prepTurnTallies(t, p, g) // value_turntallies.go; reads g.ix
	prepAddPhase(t, g)       // act_addphase.go
	prepSkipPhase(t, g)      // act_skipphase.go
	prepAirbend(t, g)
	prepMayWhileSrc(t, g) // cast_whilesource.go
	prepInit(t, g)        // act_initiative.go
	prepExileFD(t, g)     // zone_exilefacedown.go
	prepEmpower(t, g)
	g.tyCombat = overridesUncreature(t)
	g.lossSlow = g.anyLoseStat // fxNeedL6 alone: loss6 reads only the records (loss6Rec, kperf2), which genLost gates itself
	prepFaceDown(g)            // face_facedown.go
	prepLandGrant(t, p, g)     // static_landtype.go
	for i := range t.Overrides {
		o := &t.Overrides[i]
		if o.AddTypes&kop.TLegendary != 0 || o.HasTypes && o.SetTypes&kop.TLegendary != 0 {
			g.legendGrant = true
			break
		}
	}
	g.tyPost = g.anyBestow || g.anyImpending
	prepEquipAnim(t, g) // static_equipanimated.go
	prepTrigAtk(t, g)   // trig_attackers.go
	prepBatchPool(t, g) // trig_batchmembers.go
	g.anyCopyElect = g.anyCopyElect || p.casualty
	prepCold(g, &p.cold)
	return g
}

// Capacities of the generated-table state.
const (
	maxRem    = 64 // remembered / chosen objects of one resolution (12 before 2026-11-03: Sins of the Father's chapter III remembers a whole graveyard)
	maxOvr    = 8  // distinct override records on one permanent (identical ones merge: ovRec.n)
	maxMay    = 16 // play-from-zone permissions (8 before kcaps2x: StallMayPlay was hit, validaggs)
	maxDProg  = 6  // delayed programs
	maxEmb    = 4  // emblems per player in State; a table that repeats them keeps maxEmbCold more (static_emblemcold.go)
	maxDmgMap = 8  // damage-map entries
	maxXTgt   = 7  // objects of multi-object target slots beyond the first
	maxUsed   = 16 // once-per-turn records
	// maxSacOrd bounds one effect's sacrifice batch. The FDN and BLB
	// 1500-pair lockstep slices reach 12 (measured
	// 2026-10-22); 16 leaves headroom without paying for a whole board.
	maxSacOrd = 16
)

// ovRec is an override record applied to a permanent: Table.Overrides[idx]
// from the card src (whose abilities its Grant indexes name).
type ovRec struct {
	idx uint8 // Table.Overrides index + 1 (0 = empty)
	// fl: bit 7 = until end of turn; bits 5-6 = ovNT/ovNTYou (ovNTYou alone: ovCopyGrant);
	// bits 0-4 = further identical
	// applications merged into this record (Soulstone Sanctuary activated
	// again: gorge lists its keyword once per application)
	fl  uint8
	src CardID
	// ts is the record's layer timestamp (State.tick at its first
	// application): a "loses all abilities" effect newer than it removes
	// the abilities it grants (CR 613.7, gorge grantLostTo).
	ts uint16
	// p, t are the record's set P/T values, computed at application (a
	// value-driven Animate: Power$/Toughness$ read once at resolution).
	p, t int16
}

const ovEOT = 0x80

// ovNT: the record lasts until its controller's next turn begins (CR 613,
// Duration$ UntilYourNextTurn); ovNTYou is that controller.
// ovReps is the merge count's mask.
const (
	ovNT    = 0x40
	ovNTYou = 0x20
	ovReps  = 0x1F
	// ovCopyGrant (ovNTYou without ovNT, a combination no next-turn record
	// has) marks the record of an as-enters copy's AddAbilities$ (Mercurial
	// Pretender): the ability is a copiable value in gorge (CR 707.9b), so a
	// copy of the copy inherits the record (copyGrantsAdd), and the card the
	// ability indexes is src, not the copy's own printed card.
	ovCopyGrant = ovNTYou
)

// copyGrant reports whether r is an as-enters copy's grant record.
func (r ovRec) copyGrant() bool { return r.fl&(ovNT|ovCopyGrant) == ovCopyGrant }

func (r ovRec) eot() bool { return r.fl&ovEOT != 0 }

// reps is how many applications the record stands for.
func (r ovRec) reps() int { return 1 + int(r.fl&ovReps) }

// fxRec is one record of the effect pool (CR 613): a one-shot
// continuous effect locked onto the permanent at sl when it resolved (CR
// 611.2c names the object, never a filter). Records are appended in tick
// order, so pool order is timestamp order and nothing is ever sorted;
// removal shifts the tail down. Battlefield statics are not copied in:
// they are read live, stamped with their source's layerTs.
type fxRec struct {
	sl   Slot
	kind uint8 // fxOverride, fxLoseAll, fxCopyExc
	// fxOverride: the override record. fxLoseAll: r.ts is the latest
	// one-shot loss's timestamp and r.fl's ovEOT bit is set while every
	// loss applied is until end of turn (a permanent one clears it).
	r ovRec
}

const (
	fxOverride  uint8 = iota // a Table.Overrides record (applyOverride)
	fxLoseAll                // OpLoseAbilities' one-shot "loses all abilities" (layer 6)
	fxKwGrant                // a promoted until-end-of-turn keyword grant's timestamp (static_layer6.go kwGrantFx)
	fxCopyExc                // a copy's exception override (layer 1, CR 707.9b; copy_exceptions.go)
	fxControl                // a layer-2 control record (CR 613.1b; static_control.go)
	fxCopyEOT                // a permanent's until-end-of-turn copies: r.src the newest clone's card, r.idx its exception override, r.p its template, r.ts its timestamp, settled at cleanup (copy_clone.go)
	fxCopyPerm               // a permanent's newest permanent copy marker: r.p the source object, r.idx the exception override, r.ts its timestamp (copy_clone.go)
	fxOnceStash              // the permanent's printed abilities were activated once-only before a copy layer began, which the layer's end returns (copy_clone.go onceBasisBegin)
	fxCopyHeld               // a token copy's own copy exception (r.idx), held aside while an until-end-of-turn copy overrides it (copy_clone.go cloneHold)
	fxCopyMint               // a token copy's own copy (r.src the card it was minted as), held aside likewise (copy_clone.go cloneHold)
	fxAegisBase              // the copy a bearer wore (r.src its card) when an Aegis stack took over with no copy marker of its own, which shows again when the last Aegis copy ends (copy_aegisunder.go)
	fxAegisHeld              // a copy exception of the copy an Aegis stack covered (r.idx), held aside while the Aegis copy shows: no marker can rebuild it (copy_aegisunder.go aegisHold)
	fxCopySelf               // a self-copy layer's inherited copy exception (r.idx, r.ts the layer's timestamp), held beside the layer because its template is the becomer itself (copy_clone.go cloneSelfHeldPush)
)

// maxFx bounds the effect records live at once (all permanents); a full
// pool stalls the game (StallTables). 64 since 2026-11-03: a gifted Dawn's
// Truce grants hexproof and then indestructible to every permanent its
// controller has, a record each (kwGrantFx), so seventeen permanents filled 32.
const maxFx = 64

// maxXTPool bounds the stack entries with a multi-object target slot or a
// damage division alive at once.
const maxXTPool = 4

// mayPlay is a permission to play one card from a zone.
type mayPlay struct {
	obj   Obj
	ver   uint8
	ctrl  uint8
	until uint8 // DurEOT / DurYourNextTurn
	kind  uint8 // kop.MayPlayKinds riders (free, mana of any type)
	turn  uint16
}

// dprog is a delayed program: at the next end step run ability ab of card
// with RefDelayedRemembered = objs.
type dprog struct {
	card CardID
	ab   uint8
	ctrl uint8
	src  Obj
	objs [4]Obj // two with DpGroup; up to four with DpMany (trig_delayedmany.go)
	vers [4]uint8
	n    uint8
	ext  uint8 // dpExtBeginCombat/dpExtYourEndStep (trig_delaybegincombat.go, trig_delayedtiming.go): n's timing bits are full
	sv   uint8 // the source's ver at registration (CR 603.7c): a delayed body's Self names that object, not the one at fire
}

// usedRec marks a once-per-turn use (a trigger, a MayPlay static).
type usedRec struct {
	obj Obj
	ab  uint8
	ver uint8 // the object's ver (CR 400.7: a new object has used nothing)
}

// xTargets are the objects of a stack entry's multi-object target slots
// beyond the first (which sits in tgt[slot]), and the division of damage.
type xTargets struct {
	n    uint8
	slot [maxXTgt]uint8
	code [maxXTgt]uint8
	ver  [maxXTgt]uint8
	// crim is CR 700.13's pick-time verdicts for the pool's extra objects
	// (act_commitcrime.go crimeAskDone): bit j says extra j's pick was a
	// crime when it was made, bit 7 (crimRec) that verdicts were recorded.
	crim uint8
	div  [maxXTgt + 1]uint8 // damage assigned per object of the divided slot, in pick order
}

// genState is the generated-table part of State.
type genState struct {
	graveKept bool // removePerm: this move to a graveyard takes no "from anywhere" redirect
	// cskip: the combat-phase skips (act_skipphase.go). Bit p: player p skips the
	// combat phases of their next turn; bit 4+p: player p is skipping the combat
	// phases of the turn being played. Fills the padding byte after graveKept.
	cskip     uint8
	fx        [maxFx]fxRec
	nfx       uint8
	xtp       [maxXTPool]xTargets // stackEntry.xi - 1 indexes it
	atkHi     uint8               // the highest attacker group ever taken (the hash walks 1..atkHi)
	tli       uint8               // LKI index of a target the resolving entry moved off the battlefield; bit 7: a filter is reading it (zone_lki.go)
	energy    [MaxSeats]uint8     // each player's energy counters (CR 122.1; saturates at 255; act_energy.go)
	roll      uint8               // the last OpRollDie result (ExRollResult; act_energy.go)
	drawFirst bool                // the turn-based draw of the draw step is pending and no card has been drawn in it yet (repl_draw.go; transient, false between decisions)
	conn      uint8               // OpConnive's nonland hand count before its discard, +1 (0: no connive pending; padding; trig_delayedtiming.go)
	held      uint8               // entry stamps (mod 8) of until-leaves returns whose enters check a departure frame holds (zone_untilleavesbatch.go; 0 outside a frame; fills the padding after conn)

	rem     [maxRem]Obj // the resolution's remembered set
	nrem    uint8
	chosen  [maxRem]Obj
	nchosen uint8
	remCnt  uint16 // OpRemoveCounterRem's count
	availX  uint8  // transient availFor mode while scriptOffers re-walks priority (script_gen.go)
	noGrant bool   // transient: an activated ability's prefix search prices without granted mana (its grant prefix is its own search; script_gen.go)
	roleOut uint8  // a Role token may be on the battlefield: the SBA scans for CR 704.5z (static_roletokens.go)
	digLo   uint8  // a DuDugMark DigUntil's first exile index (act_discoverbottom.go; the padding before excess)
	// raiseO / raiseB: a may-play surcharge being offered / owed by the cast
	// being built (cast_whileexiled.go)
	raiseO, raiseB uint8
	excess         int16
	lostAmt        int16
	tokMade        uint8
	tokDef         uint8 // the CR 508.4 defender the TokenAttacking$ True ask was answered with (a player or planeswalker/battle target byte), valid while tokDefSet; transient within one mint (filter_combatwords.go)
	tokDefSet      bool
	entHost        uint8 // host slot + 1 of an Aura about to enter attached: enter attaches it before its enters check (zone_handmoveaura.go; transient, 0 between moves)

	dmg  [maxDmgMap]struct{ code, amt uint8 }
	ndmg uint8

	may  [maxMay]mayPlay
	nmay uint8
	// parad[p] bit i: p resolved a Paradigm spell with the table's i-th
	// Paradigm name (CardDef.Paradigm-1); never reset (kw_paradigm.go)
	parad [MaxSeats]uint8
	sidN  uint8 // the last stack identity given (tgt_stackability.go)

	dp    [maxDProg]dprog
	dpLKI [maxDProg]uint8 // each program's LKI pool index (zone_lki.go)
	ndp   uint8
	stkCt uint8 // a countered stack ability's controller+1, for its "its controller" (tgt_stackability.go)

	emb  [MaxSeats][maxEmb]uint8  // emblem index+1
	embC [MaxSeats][maxEmb]CardID // the card that created it (its latent abilities run the emblem's triggers)
	embO [MaxSeats][maxEmb]Obj    // that card's object (the source of the emblem's triggers)
	embN [MaxSeats][maxEmb]uint8  // the Effect's frozen SetChosenNumber$
	embT [MaxSeats][maxEmb]uint16 // the emblem's CR 613.7 layer timestamp, set at creation (static_layer6.go)
	nemb [MaxSeats]uint8
	qgO  [qgN]uint8      // exile grants (Q, Stacking the Deck): the granted exiled object+1, 0 = free (cast_exilegrant.go)
	qgV  [qgN]uint8      // each grant's object version: stale once the card left exile
	qgH  [qgN]uint8      // each grant's host (the card whose ability was granted) object+1
	pgT  [pgN]uint8      // Panharmonicon grants: the granted object+1, compacted (act_grantpanharm.go)
	pgH  [pgN]uint8      // each grant's host (the tapped source) object+1
	nlA  [MaxSeats]uint8 // locked permanents, object+1, compacted (act_lockpermanent.go; fills the padding before lki)
	// the untap step suspended on a keep-tapped ask (repl_untapchoice.go):
	// untapUp the slots untapped so far, untapNext the scan index of the
	// asked permanent, untapSl its slot, untapAns its answer (0 unanswered,
	// 1 untap, 2 keep tapped), untapOn the step being in progress
	untapUp   slotMask
	untapNext uint8
	untapSl   Slot
	untapAns  uint8
	untapOn   bool

	lki [maxLKI]lkiRec // the shared LKI pool (zone_lki.go)
	// lkiPad: the 192-bit SubMask grew each LKI record by 8 bytes (31 x 8 = 248); these 8 carry
	// that growth to 256, a multiple of 64, so perms keeps its cache-line phase in both size classes.
	lkiPad [8]uint8

	used  [maxUsed]usedRec // once-per-turn triggers / statics used this turn
	nused uint8

	died     uint8           // creatures put into a graveyard from the battlefield this turn
	attacked [MaxSeats]uint8 // creatures each player attacked with this turn
	oppAtt   uint8           // 1 once a declaration this turn named a player as defender (prep.anyOppsAtt only; value_oppsattacked.go)
	isCast   [MaxSeats]uint8 // instants and sorceries each player cast this turn (Tf2CastSnapshot)
	// the turn history bank (value_turnhistory.go): per player, counters kgen's
	// readers name by slot (histSpells: spells cast), and the mana spent
	// casting spells (expend). Kept only when genPrep.anyHist.
	hist [MaxSeats][3]uint8
	// actRing: per activator, the first actRingN abilities activated this turn, each as the source Obj+1 (0 = empty), actN the counts. Written only when genPrep.actRingOn; in no hash (cast_activationledger.go).
	actRing [MaxSeats][actRingN]uint8
	actN    [MaxSeats]uint8

	// pending as-enters choices (colour / creature type) and the leyline ask
	ask  [4]askRec
	nask uint8
	// askPrio is the priority holder when the first queued as-enters ask
	// was queued (gorge asks it mid-resolution, the holder unchanged);
	// priSave (0x80 | holder) the holder to restore once the asks are done
	askPrio, priSave uint8
	holdPrio         uint8           // the priority holder when the free casts were picked
	ctlRet           uint8           // control state to return to after the asks
	pre              preAsk          // an as-enters choice asked while the permanent spell is on the stack
	ley              [MaxSeats]uint8 // leyline asks: starting player done, Leylines declined (nextLeyline)
	leyP             uint8
	leyOn            bool
	leyDn            bool
	crime            uint8 // CR 700.13: bit p, player p committed a crime this turn; 0 unless genPrep.crimeOn

	chPick [6]struct {
		sl Slot
		m  uint8
	} // per restricted charm source: m bits 0-3 picked modes, bit 7 ThisGame scope (modal_charmpick.go)
	nchPick    uint8           // entries of chPick in use
	trigTurn   uint16          // triggered abilities resolved this turn (StallLoop)
	dilemmas   [MaxSeats]uint8 // the dilemmas each player faced this turn (trig_dilemma.go)
	on         bool            // the State plays a generated table
	tapBatch   uint8           // trig_tapall.go: the open tap batch's first pending index + 1 (0 = none)
	batchLatch uint8           // trig_oneormore.go: bit 7 a leave to a graveyard is being checked; bits 0-4 the open batch's first pending index + 1 (0 = none). Was pendOver (unused), so State is unchanged
	tyM        slotMask        // slots whose perm.gty holds override-derived types (ovGenTy)
	stalled    uint8           // why the game was stopped at the kernel's capacity (a draw); 0 = it wasn't
	noTgtRed   bool            // cost computation without target-dependent reductions
	mayPlayAny bool            // the may-play cast being priced is under the any-type rider
	probe      probe           // the spell / ability a priority feasibility check is for
	tgtSelf    uint8           // the graveyard card a feasibility check is for, plus one (0: none): it leaves the graveyard for the stack when cast, so it is no target of its own cast
	inLoop     bool            // executing one object of a multi-object target slot
	each       eachSusp        // an each-target loop parked on a copy token's as-enters ask (copy_eachtarget.go)
	tokFrz     slotMask        // the permanents whose CreateToken replacements an each-target copy loop reads, fixed when it began (tokSrcM)
	tokFrzOn   bool            // tokFrz is in force
	sweep      sweepSusp       // a copy-all sweep parked on a copy's as-enters ask (copy_allpark.go)
	mt         multiTgt        // the multi-object target slot being chosen
	lastCK     kop.CounterKind
	disc       Obj             // the card being discarded (TrigDiscarded)
	nlB        [MaxSeats]uint8 // locked permanents 3 and 4 (nlA; fills the padding before kicked)

	redNC [MaxSeats]uint8 // ExRedNCDmg: the noncombat damage each player's red sources dealt this turn, saturating (prep.anyRedNC only; value_damagetally.go; fills the pad before kicked)
	// grantSrc is the granting permanent plus one (0 = none) while a grant
	// walk (grantedAbilitiesN) reports one ability to its callback; it fills
	// the pad after redNC.
	grantSrc uint8
	cast     slotMask // slots whose permanent was cast by its controller
	zoneTurn objBits  // objects put into their current zone this turn
	// dmgTurn is gorge Object.WasDealtDamageThisTurn (CR 120.3a): it survives a
	// zone change and a regeneration (zones.go enterNF clears it only on a real
	// battlefield entry), the marked damage does not.
	dmgTurn     objBits // objects dealt positive damage this turn
	enteredN    uint8   // the live prefix of State.cold.entered
	enteredFull bool
	enteredOn   bool // the table has an ExThisTurnEntered expression
	// (act_initiative.go; the padding before mo)
	// the initiative's holder+1 (0 nobody), each player's venture marker
	// (room index+1, 0 no dungeon) and dungeon object
	initH   uint8
	room    [MaxSeats]uint8
	dObj    [MaxSeats]Obj
	retMiss bool             // the last OpReturnEx returned nothing (its StaticEffect rider then applies to nothing)
	ewDone  bool             // the last return applied its OvEnterWith override as the object entered (zone_assimilate.go)
	ewPend  bool             // a return's OvEnterWith override waits for its entrant: enterGen applies it before the as-enters replacements (zone_assimilate.go)
	retAt   uint8            // the remembered object a many-object return is moving, + 1 (0 = none; returnrem.go)
	retTS   uint16           // the entry stamp a many-object return started at: its entrants are the ones after it
	batchTS uint16           // the entry stamp the open batch began at: a watcher entered after it looks back to the board before the batch (trig_oneormore.go)
	spent   [MaxSeats]uint16 // the history bank (value_turnhistory.go): mana each player spent casting spells this turn (expend)
	sacM    slotMask         // permanents picked for an effect's sacrifice, sacrificed together once all are picked
	sacOrd  [maxSacOrd]Slot  // sacM's slots in sacrifice order (gorge zone_bulk.go effSacrifice: player by player, each in answer order)
	nsac    uint8
	// an Aura or Equipment whose host left stays attached to it, as gorge
	// shows it, until the next state-based check; lingerM is the live set
	linger   [maxLinger]lingerRec
	nlinger  uint8
	poison   [MaxSeats]uint8 // each player's poison counters (CR 122.1f; infect and toxic, kw_infect.go; padding before lingerM)
	poolArt  [MaxSeats]uint8 // per player, the pool slots holding mana an artifact produced (mana_artifact.go; padding)
	defSrc   [MaxSeats]uint8 // per player, the slot + 1 of the deferred source whose {R} floats in the pool (mana_deferred.go; padding)
	speed    uint8           // both seats' speed (3 bits each) and the once-per-turn speed trigger latch (bit 6; kw_speed.go; padding)
	noPoison uint8           // bit p: player p can't get additional poison counters this turn (Melira's replacement; trig_leavereturn.go; padding)
	plusCtr  [MaxSeats]uint8 // per player, how many Brad Boimler counter-plus-one effects are active this turn (ctr_pluseot.go; padding)
	plusSrc  [MaxSeats]Obj   // per player, the Boimler that made them (NoObj: more than one; the order ask's source, repl_ctrorder.go; padding)
	cast1    uint8           // bit p: seat p has cast a spell this game, recorded only for a table reading ExFirstCast (cast_firstcast.go; padding)
	ctrAdder uint8           // transient: the player putting the counters being placed, +1 (0: the resolving / building controller; trig_leavereturn.go; padding)
	// ctrSettle is the entrant whose entry counters are being placed, +1
	// (0: none). While set, the AddCounter matcher skips that permanent as a
	// source unless its recipient filter is FSelf (CR 614.12, gorge's
	// ownEntryCounterExcluded); the entrant's other statics stay live. Set
	// only across one placeEntryCounters call (turn_gen.go).
	ctrSettle uint8
	// this turn's counter placements (ExCtrAdded; ctr_addedthisturn.go).
	// pl: bit p set when player p put a counter on a creature; e: per object whose card
	// reads a Self form (gfCtrAdd), fl bit p any kind by p, bit 2+p +1/+1
	// by p, bit 4+p lore by p. A fifth object stalls (StallCtrAdded).
	ctrAdd struct {
		pl, n uint8
		e     [4]struct {
			o  Obj
			fl uint8
		}
	}
	// prevCol: Ext PreventColor's this-turn records, one per resolution (a
	// gorge Effect): the card that made it and the colour chosen (col, a
	// ColorMask bit; 0 = free) whose sources' damage it prevents
	// (cast_altfree.go; padding before lingerM)
	prevCol [3]struct {
		obj Obj
		col kop.ColorMask
		// sc, ctl: a scoped record (Ext PreventDamageScope) prevents the
		// damage its controller ctl (a player) or the creatures ctl controls
		// would take; sc holds the kop.PrevScope* bits and col is 0. A record
		// with sc&prevNextSc is an Ext PreventNext shield: ctl the recipient's
		// damage code, obj the effect's source, col the damage left (repl_prevnext.go)
		sc, ctl uint8
	}
	lingerM slotMask
	// declM: the slots declared as attackers earlier this turn (Count$AttackersDeclared
	// counts each creature once across extra combats). perm.flags has no free bit, so
	// the mark lives here; it ends with the permanent (removePerm) and with the turn.
	declM    slotMask
	replaced Obj // the object an as-enters replacement program is modifying

	// OpCastFree: cards still castable this resolution
	free  [8]Obj
	nfree uint8
	// the cards OpCastFree accepted, frozen: a mode's index ranks in this
	// list (gorge's modes decision is built once over the castable
	// candidates), the ones already cast keeping their places
	freeAll  [8]Obj
	nfreeAll uint8
	freeQ    [8]Obj // chosen to be cast after the resolution
	nfq      uint8
	freeP    uint8 // who casts the queued cards
	hold     bool  // an sfHold entry is on the stack

	// OpChooseBranch's answer
	branch uint8
	// OpRepeatEachPlayer: the bound player, the one still to come, the body
	eachPl  uint8
	repNext uint8 // 255 = none
	repPC   uint8
	repEnd  uint8
	// resolution decisions in progress
	pickLeft       uint8                   // picks still owed
	pickP          uint8                   // the player picking
	pickZ          uint8                   // destination zone / flags of the pick op
	pickF          uint16                  // the pick op's flags
	winSrc         Slot                    // the tap-another source whose pick a payment window waits on (mana_windowtapx.go)
	winMode        uint8                   // the payment that window serves (winNone: none)
	winCol         uint8                   // the colour a cast window's paused tap-another source makes once the pick is answered (mana_window.go)
	winFirst       uint8                   // a two-permanent tap-another source's first window pick, slot+1 (0: none yet; mana_windowtapx.go)
	winHeld        [kop.NumManaKinds]uint8 // the restricted mana that walk kept out of the pool so far
	nlook          uint8                   // cards looked at (dig / peek / reveal): the live prefix of armRes.look (scratch.go)
	attLKI         [MaxSeats]attLKIRec     // the last departed attachments' hosts (CR 608.2h; fills the pad before evSel)
	evSel          uint64                  // ward evidence picks (bit per graveyard index; MaxDeck 60; kw_wardcost.go)
	endTurn        bool                    // OpEndTurn: skip to cleanup after this resolution
	fog            bool                    // OpFog: all combat damage this turn is prevented (padding)
	beamPick       bool                    // offerBeamCasts prices a granted beam me up cast's return (kw_beammeup.go)
	altCast        bool                    // the cast beginCast starts is for the card's own alternative mana cost (castAltTgt; cast_altfree.go; padding)
	beamGrant      bool                    // the beam me up cast beginCast starts is the granted instance of a card that prints the keyword (kw_beammeup.go; padding)
	impCast        bool                    // the cast beginCast starts is for the card's impending cost (castImpTgt; kw_impending.go; padding)
	warpGrant      bool                    // the warp cast beginCast starts pays a battlefield grant's instance (kw_warpgrant.go; padding)
	bargainCast    bool                    // the permission cast beginCast starts is bargained: the pool it checks pays the bargained price (offerBargainPlay; kw_bargain.go; padding)
	noUntapL       uint8                   // OpSkipUntap C=1: bit p, player p's lands don't untap during p's next untap step; bit 4+p their creatures (SkipUntapPlayerCL; padding)
	yourTurns      [MaxSeats]uint8         // ExYourTurns: the turns each player began as the active player, saturating (prep.anyYourTurns only; padding)
	extraTurnBegun uint8                   // ExYourExtraTurns: bit p, player p has begun an extra turn this game (CR 500.7; value_exprs.go)
	dayNight       uint8                   // CR 731: bits 0-1 the designation (dnDay, dnNight), bits 2-3 the last turn's active player's spells, at most 2 (kw_daynight.go; padding)
	links          [16]struct {
		src  Obj
		sver uint8
		obj  Obj
		ts   uint16 // the tick the Aegis copy went live (aegisApply; zone_exilethenattach.go)
	} // exiled until the source leaves (OpExileUntilLeaves)
	nlinks   uint8
	nexw     uint8
	linkHide uint16 // bit i: links[i] is a Hideaway link (the card stays exiled face down; kw_hideaway.go; padding)
	linkAeg  uint16 // bit i: links[i] is an attachment whose copy of its exiled card is live (Assimilation Aegis; zone_exilethenattach.go; padding)
	linkHand uint16 // bit i: links[i] returns its card to its owner's hand, not the battlefield (a hand exile until the source leaves; act_mayask.go)
	// linkPad keeps perms' cache-line phase after links grew from 8 to 16
	// (linkPadN, seats_default.go and seats_mp.go)
	linkPad [linkPadN]uint8

	// retHeld lists, in departure order, the sources that left in the open
	// departure frame with links to return (holdReturns); bit i of retCtrl is
	// retHeld[i]'s last controller. A ninth source stalls (StallLinks).
	retHeld  [8]Obj
	nretHeld uint8
	retCtrl  uint8
	// retHold: the returns batch is parked on a host ask, so its frame's
	// close waits for the answer (zone_untilleavesbatch.go); retOld and
	// retStamp are the latch batchClose restores and the frame's opening
	// entry stamp, kept for that deferred close.
	retHold  bool
	retOld   uint8
	retStamp uint16
	// entExc: the copy exceptions of the token entering next (entExcOn | the
	// template's slot<<8 | the op's exception). Its entry's replacements read
	// the token with them (CR 707.9, 614.12).
	entExc   uint32
	enterCtr struct {
		n     uint8
		kind  kop.CounterKind
		tap   bool
		ctrl  bool
		kind2 uint8 // a second kind to enter with, n of it, plus one (0: none; a Return's WithCountersType$ pair)
	} // counters/tapped for the next enter (OpReturnEx)
	// entrySelf is the slot+1 of the permanent whose entry counters are being
	// placed (0: none). Its own general AddCounter replacement does not apply
	// to them (CR 614.12; gorge ownEntryCounterExcluded). Set for the whole
	// enterNF fold, which places the printed, requested, own/foreign-body and
	// effect-borne entry counters.
	entrySelf uint8
	// tokPT: the P/T set the token entering next takes with its entry
	// (act_tokenpt.go): on, the override index, the evaluated P/T
	tokPT struct {
		on   bool
		idx  uint16
		p, t int16
	}
	lookSel uint32  // picked looked-at cards (bit per index; 32 bits hold look's 24; fills the pad before ro)
	ro      replOrd // a combat damage pass paused on gorge's replacement-order ask: the header (repl_gen.go); the body is scr's armCombat
	// scr: the mutually exclusive decision scratch, one region read through
	// typed accessors: the combat damage pass (armCombat), the build in
	// progress (build), a resolution's look / pileOrd / dmgSeqOrd (armRes).
	// The discriminants stay outside it (scratch.go).
	scr [scrWords]uint64
	nc  ncDmg  // a noncombat damage event parked on that ask (ro.on false; repl_gen.go)
	tok tokOrd // a CreateToken op deciding a contested creation on gorge's CR 616.1 token order ask: ro carries the candidates (repl_tokorder.go)
	// a multi-event damage op paused on one of those asks: its recipients are
	// frozen at the op in gorge's sweep order (effDamageAll: seat 0's
	// battlefield in entry order, then seat 1's; the players arm last, bit 0
	// the active player, bit 1 the other, as the op deals them), dmgSeqI the
	// next entry still to deal
	dmgSeqOn bool
	dmgSeqN  uint8
	dmgSeqI  uint8
	dmgSeqPl uint8
	// plusBatchAsked: a damage pass parked on a pure plus-N order ask
	// (trig_batches.go replPlusBatch, and the direct replDamage sites'
	// replPlusSrcPass: OpDealDamageFrom's player mode, OpResolveDamageMap and
	// the one-target divided batch) is being re-executed, so the additions
	// are applied in any order and the ask is not posed again. The parked
	// hit's landed amount rides h.n, which is what the emitting effect's
	// EmitDamage returns (gorge returns the post-replacement Damage event).
	plusBatchAsked bool
	// colG: Ext ColorGrant's until-end-of-turn records (Skrelv;
	// static_colourgrant.go): the object (obj, ver), the
	// chosen colour (col, a ColorMask bit; 0 = free) and cgHex / cgNoBlock /
	// the toxic amount (fl >> 2)
	colG [maxColG]colGrant

	// cdh: the combat hits on players by sources whose card reads
	// CtrlHitBySrc (genPrep.cdRec): o the source, pl bit p = player p
	// was hit by it this turn (value_turntallies.go).
	// cdLeg bit p = player p was dealt combat damage this turn by a source
	// matching Creature.Legendary.
	cdh [4]struct {
		o  Obj
		pl uint8
	}
	ncdh  uint8
	cdLeg uint8

	// xph: the pending extra phases (act_addphase.go): at bits
	// 0-3 the splice point, 4-5 the kind (combat, upkeep, end), 6 a phase of the
	// entry is running; n the grants still pending. phCnt: bits 0-3 the combats
	// begun this turn, 4-7 the end steps begun (saturating at 15).
	xph   [4]struct{ at, n uint8 }
	nxph  uint8
	phCnt uint8

	marks srcMarks // DamagedBy / CrewedThisTurn source marks this turn (filter_combatwords.go)

	ctrCell    slotMask // the put action's recipients so far (trig_countercell.go)
	ctrCellHit bool     // the fire in progress is a later kind of one action on one recipient
	ctrEntry   bool     // the fire in progress is the entrant's own counters: their putter is its controller (CR 122.6a)

	ctrOver  [maxCtrOver]ctrOverEntry // counter kinds past the general slots, sorted (ctr_sparse.go)
	nctrOver uint8

	// Reserved tail: State bytes set aside so later work never edits a struct
	// definition. Nothing reads or writes these yet. The rule: new State bytes
	// come out of rsv, the mech arena, a cold array or a scr overlay; never a
	// mid-struct insertion, which moves every later field. rsv is named, not _,
	// so SameGame's == compares it. Together the block grows State by 80 bytes
	// (statelayout_test.go pins the size and perms' cache-line phase).
	imp     [maxImp]Obj
	nimp    uint8
	linkImp uint8
	embF    [MaxSeats]uint8 // bit i: player p's emblem i (UntilEndOfNextTurn) has begun a turn since creation (static_emblemdurations.go)
	embR    [MaxSeats][maxEmb]Obj
	nseat   uint8                             // seats in this game, set by InitN; read only when MaxSeats > 2 (nSeats)
	poolTre [MaxSeats][kop.NumManaKinds]uint8 // per player and colour, the floating units a Treasure produced (mana_artifact.go artTag/treSync)
	blkDef  uint8                             // the defending player declaring blockers; read only when MaxSeats > 2 (blockDef)
	ul      [maxUntapLock]untapLock           // source-bound untap locks (act_untaplock.go)
	nul     uint8
	dmk     [dmkBytes]uint8 // per-player damage-doubling marks, a bit per (controller, player) (repl_dmgmark.go)
	// tgtSpent: the resolving spell's target spell's cast spend + 1,
	// saturating at 255 (0 = no snapshot; resolve.go snapTgtSpent). Read by
	// ExTgtSpent after the counter removed the target's stack entry (EOE
	// Unravel; gorge targetManaSpentLKI).
	tgtSpent uint8
	// turnF: per player this turn's facts (static_turnbits.go). Bit p: player p
	// flipped a coin. Bit 2+p: player p activated an Exhaust ability. Bits
	// 4-7 are free. Cleared at the turn change. Taken from rsv.
	turnF uint8
	// dmgAmt: the positive damage each reader card's object (CardDef.DmgAmt)
	// received this turn, o = Obj+1 (0 empty) and n saturating at 255
	// (value_dmgamount.go). Written only under genPrep.anyDmgAmt; cleared at the
	// turn change. Taken from rsv.
	dmgAmt [dmgAmtN]struct{ o, n uint8 }
	rsv    [rsvN]uint8
	// embX: an emblem's remembered slots 2 and 3, object + 1 (static_emblemrem.go); embR is slot 1, embN slot 0. Taken from rsv.
	embX [MaxSeats][maxEmb][embRemSlots - 2]uint8
	// atkDef is the defending seat + 1 of the attack event whose triggers are
	// being checked (fireAttacksGen), 0 outside it: gorge binds a trigger's
	// DefendingPlayer from that event, and an IsPresent$ checked as the
	// trigger fires reads it (defendingSeatF2). Set only when MaxSeats > 2.
	// Taken from rsv.
	atkDef uint8
}

const (
	maxImp      = 16
	turnFBytes  = 1 // genState.turnF, taken from rsv
	dmgAmtN     = 4 // genState.dmgAmt: reader-card records (value_dmgamount.go)
	dmgAmtBytes = dmgAmtN * 2
	// rsvN pads the reserved block to 80 bytes (genState's tail padding
	// absorbs the rest of the alignment, including tgtSpent): 64, plus the 16
	// that carry State's growth for MaxObj 236 to a multiple of 64 so perms
	// keeps its phase.
	rsvN = max(0, 80-maxImp-2-MaxSeats-MaxSeats*maxEmb-2-MaxSeats*kop.NumManaKinds-ulBytes-dmkBytes-MaxSeats*maxEmb*(embRemSlots-2)-1-turnFBytes-dmgAmtBytes)
)

// exhRec is one Ext ExHaste record: attacker (obj, ver).
// kind: 0 empty, 1 "except by creatures with haste", 0x80|cmp<<4|N the
// power form (static_cantblockexcept.go).
type exhRec struct {
	obj  Obj
	ver  uint8
	kind uint8
}

// wardAmt is a card's ward trigger amount: {N} as N, "pay N life" as -N.
func wardAmt(c *kop.CardDef) int16 {
	if c.WardLifePower {
		return wardPowBit // kw_wardcost.go
	}
	if c.WardSac != 0 {
		return wardSacBit | 1 // kw_wardcost.go
	}
	if c.WardDiscard != 0 {
		return wardDiscBit | int16(c.WardDiscard) // kw_wardcost.go
	}
	if c.WardDiscOrN != 0 {
		return wardAltBits | int16(c.WardDiscOrN) // kw_wardcost.go
	}
	if c.WardWaterbend != 0 {
		return wardWBBit | int16(c.WardWaterbend) // cost_unlesswaterbend.go
	}
	if c.WardPoison != 0 {
		return wardPoisBit | int16(c.WardPoison) // cost_restokens.go
	}
	if c.WardEvidence != 0 {
		return wardEvBit | int16(c.WardEvidence) // kw_wardcost.go
	}
	if c.WardLife != 0 && c.WardN != 0 {
		return wardMLBit | int16(c.WardN) | int16(c.WardLife)<<5 // cost_unlesstapward.go
	}
	if c.WardLife != 0 {
		return -int16(c.WardLife)
	}
	return int16(c.WardN)
}

// nearCapacity reports whether the battlefield or the object table is
// about to overflow the kernel's fixed capacity (gorge stops such a game at
// its object cap as a "bigboard" stall). A phased-out permanent keeps its
// slot off every mask (kw_phasing.go), so a phasing table measures the slots
// an entrant can actually take: enterNF's freeSlots branch.
func (s *State) nearCapacity(k *prep) bool {
	if s.objTableFull() {
		return true
	}
	if k != nil && k.phasing {
		return s.freeSlots().count() <= 4
	}
	return s.occ.count() >= MaxPerm-4
}

// Stall reasons (StallReason). Most are a capacity; StallClassOf says which are
// rule gaps (stallclass.go).
const (
	StallNone    = 0
	StallBoard   = 1 // the battlefield or object table is full
	StallPending = 2 // more simultaneous triggers than MaxPending
	StallStack   = 3 // the stack is at MaxStack
	StallTables  = 4 // the override-record or multi-target pool is full
	StallLoop    = 5 // more than maxTrigTurn triggers resolved in one turn: an endless mandatory loop
	// One reason per bounded pool. Append
	// only; never renumber.
	StallDelayed  = 6  // MaxDelayed delayed triggers (State.dly)
	StallRemember = 7  // maxRem remembered or chosen objects of one resolution
	StallMayPlay  = 8  // maxMay play-from-zone permissions
	StallDProg    = 9  // maxDProg delayed programs
	StallEmblem   = 10 // embCap emblems of one player
	StallDmgMap   = 11 // maxDmgMap damage-map entries
	StallXTgt     = 12 // maxXTgt extra objects of multi-object target slots
	StallUsed     = 13 // maxUsed once-per-turn records
	StallKwRepX   = 14 // maxKwRepX repeated-keyword-grant records
	StallLinks    = 15 // gen.links exile-until-leaves links
	StallFree     = 16 // gen.free / gen.freeQ cast-without-paying candidates and queue
	StallAsk      = 17 // gen.ask queued as-enters choices
	StallCounters = 18 // maxCtrOver counter kinds past the permanents' general slots (genState.ctrOver)
	StallLKI      = 19 // maxLKI distinct last-known-information snapshots (genState.lki)
	// 20 was a Taps trigger in a table with a granted mana ability; a granted ability's tap now fires Taps triggers like any other
	StallInfect    = 21 // infect or toxic in a table with a damage replacement-order ask or a counter-added trigger (gen infectStall; kw_infect.go)
	StallReplPlus  = 22 // a damage plus-N replacement and a non-commuting damage replacement match one event: gorge's CR 616.1 order ask, whose answer changes the amount (trig_oneormore.go; two plus-N additions commute and are asked as a no-op)
	StallLifeRepl  = 25 // a life-gain doubler and a life-gain plus-N in one table: gorge's CR 616.1 order ask (they do not commute; act_discover.go)
	StallProlif    = 26 // a proliferate gorge orders differently: more than 16 picks, or a recipient with two counter kinds in a table with a counter-added trigger (act_proliferate.go)
	StallTapPick   = 27 // a tap-any-number cost (CostTapPower, Crew / Saddle) picked past build.picks' 24 (12 before kcaps2x; kw_crewsaddle.go)
	StallRegen     = 28 // a ninth regeneration shield on one permanent: pfRegenX plus the arena's mechRegen bit hold eight (4 before kcaps2x; repl_regenerate.go; kverify-c4)
	StallLook      = 29 // a dig window wider than genState.look's 24 (12 before kcaps2x; Ulamog's "exile the top twenty"; Hatchery Spider's undergrowth; selfcast): the window used to truncate silently
	StallCtrOrder  = 30 // an OpPutCountersFrom of two or more counter kinds in a table with a counter-added trigger: gorge fires per kind in the source's insertion order, the kernel walks kinds ascending (ctr_clear.go; eachfromsource)
	StallGrants    = 31 // more than maxGrants granted mana abilities on one permanent (mana_grantedoffers.go grantIdxs)
	StallShield    = 32 // a SHIELD counter and another damage replacement match one event: gorge asks the CR 616.1 order, the kernel stalls (ctr_shield.go)
	StallActs      = 33 // 127 non-mana activations of one object in a turn: object.acts keeps the count in seven bits beside the owner
	StallReplCtr   = 34 // a filtered counter doubler and a filtered plus/minus replacement match one event, or a plus and a minus replacement do: they do not commute; only a two-candidate doubler+plus on an OpPutCounter is asked (repl_ctrorder.go)
	StallReplLeave = 35 // two or more replacements compete for one death (including two dies-to-exile sources or two turn-scoped exile marks on one creature): gorge asks the CR 616.1 order, the kernel stalls (repl_counters.go)
	StallLinger    = 36 // more than maxLinger attachments lose their host in one state-based pass (genState.linger; static_lingerattach.go)
	StallSac       = 37 // one effect sacrifices more than maxSacOrd permanents (genState.sacOrd)
	StallEntered   = 38 // more than maxEntered zone entries in one turn (genState.entered)
	StallConvoke   = 39 // more convoke picks than the cast build holds, or more convoke options than a decision holds (kw_convoke.go)
	StallCastRing  = 40 // a player pushes more than castRingN spells in one turn (State.cold.castRing)
	StallPT        = 41 // a power or toughness pump leaves the int16 end-of-turn range, or needs a fifth of the maxPtX slots (eotAdd, OpPumpAllEach; filter_defined.go)
	StallActRing   = 42 // a player activates more than actRingN abilities in one turn (genState.actRing)
	StallCharmOpts = 43 // a multi-pick charm's combined target ask has more than MaxOptions options
	StallCopyFace  = 44 // a permanent-spell copy of a two-faced card (CR 707.10g; copy_permanentspell.go)
	StallRefLatch  = 45 // a combat damage batch has more than maxRefLatch (listener, referent) per-referent damage triggers (trig_damagedonce.go refLatch)
	// 46 was a twobrid cost beside a cost reduction or increase; the cost statics now compose on the announced faces (mana_twobrid.go foldTb)
	StallZone          = 47  // a hand, graveyard, exile or library array is full: tokens that left the battlefield together wait there for the next state-based check (zones.go placeFrom)
	StallOptions       = 48  // a combat declaration has more attack or block pairs than MaxOptions (combat.go optionsFull)
	StallRoleMulti     = 49  // an AttachedTo$ token mint would create a number of tokens other than one (static_roletokens.go)
	StallSrcMark       = 50  // more than maxSrcMark DamagedBy / CrewedThisTurn sources marked permanents this turn (genState.marks; filter_combatwords.go)
	StallDefenderPW    = 51  // a TriggeredDefender damage whose attacker attacks a planeswalker or battle (or left): gorge damages the defending seat instead (OpAtkPWGuard; filter_combatwords.go); past two seats, a trigger with no bound defending player read for an entering attacker's defender (enterDefenderFallback) or a defending player's permanent (defendingSeatF2)
	StallTokAtkAsk     = 52  // a TokenAttacking$ True mint while the opponent has a planeswalker or battle: gorge asks the CR 508.4 defender (filter_combatwords.go)
	StallDmgLatch      = 53  // a ninth DamageDealtOnce listener of one source in one combat damage batch (triggers_gen.go dmgLatch)
	StallReplCands     = 54  // a fifth competing replacement candidate for one event or untap (replOrd.cand; repl_gen.go, act_energy.go)
	StallEnterDefender = 55  // no longer raised: an Attacking$ True entry with two or more CR 508.4 defenders asks raGenEnterDefender (zone_changezoneparams.go)
	StallPrevCol       = 56  // a fourth Ext PreventColor record in one turn (genState.prevCol; cast_altfree.go)
	StallColGrant      = 57  // more than maxColG Ext ColorGrant records in one turn (genState.colG; static_colourgrant.go)
	StallRestrX        = 58  // a reflected search's restricted mana past the batches State.rb holds (mana_reflected.go)
	StallEndureN       = 59  // an Endure Spirit mint a CreateToken replacement multiplies (act_endure.go)
	StallLinkCtr       = 60  // an until-leaves return rider's counters on a card that does not enter inline (a returning Aura asked its host, an as-enters choice asked first; zone_untilleaves.go)
	StallNumTimes      = 61  // an open Announce$ NumTimes (no AnnounceMax$) whose eighth repetition is still payable (cast_announcetimes.go)
	StallAegis         = 62  // an Assimilation Aegis copy that cannot be kept (a second exiled card, a bearer already a copy; zone_exilethenattach.go)
	StallSacOther      = 63  // a Mobilize group sacrifice whose member another player controls (gorge checks no controller; kw_mobilize.go)
	StallObsSrc        = 70  // a second observed mana source (Spell.ManaFromCard.StrictlySelf reader) tapped by one cast (mana_spendtrigger.go)
	StallCopyCast      = 88  // a Play CopyCard$ cast of a multi-face card other than an Adventure card (cast_copycast.go)
	StallDynMana       = 89  // a dynamic mana ability's amount above maxDynMana, its autopay variants (gen.go manaPCGen; value_gravecounts.go)
	StallExiledWith    = 96  // more than maxExw live exiled-with links (zone_exiledwith.go)
	StallBreach        = 94  // more picks than the chosen set holds (Breach the Multiverse; act_millthenreturn.go) (reinstate maxDynMana 40 + loud StallDynMana on the rebased tree (The Core's dynamic amount reached 21 in the gravecounts lockstep: gorge produced 21, kernel truncated to 20); NumStall 128 untouched, the name registers cold-side)
	StallNetLock       = 85  // a ninth permanent locked by Braided Net at once (nlA, nlB, nlC; act_lockpermanent.go)
	StallExileGrant    = 138 // a ninth live exile grant (Q, Stacking the Deck; genState.qgO; cast_exilegrant.go)
	StallPanharmGrant  = 142 // a fifth Panharmonicon grant (Lwaxana Troi) live at once (genState.pgT; act_grantpanharm.go)
	StallNetMana       = 86  // a Braided Net lock on a card with a mana ability that does not tap it (act_lockpermanent.go)
	StallCharmPick     = 99  // a seventh restricted charm source holds a pick log at once (ChoiceRestriction$; modal_charmpick.go)
	NumStall           = 192 // fixed capacity of the reason tables (StallName), not last+1: a new reason takes the next free value below it and registers its name in its own file; at most 255 (reasons are uint8)
)

// StallName is reason r's short name ("" for StallNone).
func StallName(r uint8) string {
	if int(r) < len(stallNames) && (r == StallNone || stallNames[r] != "") {
		return stallNames[r]
	}
	return "unknown"
}

// stallNames is filled by per-reason init registrations (an unregistered
// slot is "").
var stallNames [NumStall]string

// capFull records that an insert found its pool full: the game stalls with
// reason why instead of dropping the insert. A generated table stops at its
// next step (stepGen), never mid-resolution; the core stops now. Cold: only a
// full pool reaches it.
//
//go:noinline
func (s *State) capFull(why uint8) {
	if s.g.stalled == 0 {
		s.g.stalled = why
	}
	if !s.g.on {
		s.stall(why)
	}
}

// maxTrigTurn is the triggered abilities one turn may resolve before the
// game is called a draw as an endless loop (CR 104.4b).
const maxTrigTurn = 2000

// stall ends the game as a draw at the kernel's capacity.
func (s *State) stall(why uint8) {
	if s.g.stalled == 0 {
		s.g.stalled = why
	}
	s.over = true
	s.win = -1
	s.ctl = ctlOver
}

// stallLatched ends the game as a draw when a stall was latched (capFull)
// while the options of the decision just built were being generated.
func (s *State) stallLatched() {
	if s.g.stalled != 0 && !s.over {
		s.stall(s.g.stalled)
	}
}

// Stalled reports whether the game was stopped at the kernel's capacity.
func (s *State) Stalled() bool { return s.g.stalled != 0 }

// StallReason is why the game was stopped at capacity (StallNone if not).
func (s *State) StallReason() uint8 { return s.g.stalled }

// manaPCGen is the planner's view of slot sl as a mana source on a
// generated table: a mana ability granted to it (Imprisoned in the Moon),
// none when an override took its own away, else its card's.
func (s *State) manaPCGen(k *prep, sl Slot) *pcard {
	if k.gen.anyCantAct && s.actForbidden(k, sl, true) {
		return &noManaPC // a CantBeActivated lock, a granted ability's too (cast_cantactivate.go)
	}
	if g := k.gen; len(g.grantPC) > 0 && s.grantMay(k, sl) {
		// a granted ability beside the card's own usable one: the own one
		// is the planner's (mana_grantedoffers.go, grantAlt)
		if found := s.grantIdx(k, sl); found >= 0 && !s.ownManaUsable(k, sl) {
			if g.blgOn {
				s.blgStall(k, sl, s.cardAt(sl))
			}
			return s.grantView(g, int16(found), sl)
		}
	}
	c := s.cardAt(sl)
	pc := &k.cards[c]
	vi := int16(-1)
	if k.gen.flags[c]&gfManaVar != 0 {
		vi = k.gen.manaVar(c)
	}
	if (pc.manaAb >= 0 || vi >= 0) && s.genLost(k, sl) {
		return &noManaPC // genLost covers a face-down permanent (CR 708.2)
	}
	if vi >= 0 {
		if k.gen.blgOn {
			s.blgStall(k, sl, c)
		}
		if k.gen.anyCondSplit && k.gen.splitAb(c) >= 0 {
			return s.condSplitView(k, sl, c)
		}
		if k.gen.flags[c]&gfManaGate != 0 {
			return s.manaGateView(k, sl, c, vi)
		}
		if k.gen.flags[c]&gfManaDynPair != 0 {
			return s.dynPairView(k, sl, c, vi)
		}
		if k.gen.flags[c]&gfManaRefl != 0 {
			return s.manaReflView(k, sl, c, vi)
		}
		cd := &k.t.Cards[c]
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if a.Kind != kop.AbMana {
				continue
			}
			if a.AFlags&kop.AfManaChosen != 0 {
				ch := s.perms[sl].chosen
				if ch > 4 {
					return &noManaPC // no colour chosen yet
				}
				return &k.gen.varPC[int(vi)+int(ch)]
			}
			n := s.dynManaN(k, a, sl)
			return &k.gen.varPC[int(vi)+min(max(n, 0), maxDynMana)]
		}
	}
	if k.gen.blgOn {
		return s.blgView(k, sl, c, pc) // a granted basic land type's intrinsic abilities (static_landtype.go)
	}
	return pc
}

// maxDynMana caps a dynamic mana ability's amount as an autopay source; a
// larger amount stalls StallDynMana. The amount indexes varPC, one variant
// per amount, and a pool colour is a byte in State (poolAdd saturates there).
// A self-pumping source (Improvised Arsenal copies on the tapping creature)
// grows without bound, so 255 is the declared ceiling and a larger amount is
// a counted capacity stall, not a bigger table.
const maxDynMana = 255

// dynManaN evaluates a dynamic mana ability's amount and stalls when it is
// above maxDynMana: a larger amount is never silently truncated, whichever
// pool asks for it (gen.go's autopay variants, mana_restrictedsource.go's
// restricted-source planner). Every consumer goes through here.
func (s *State) dynManaN(k *prep, a *kop.Ability, sl Slot) int {
	n := s.evalV(k, a.ManaV, evalCtx{src: s.perms[sl].obj, srcSl: sl, ctrl: s.perms[sl].ctrl})
	if n > maxDynMana {
		s.capFull(StallDynMana) // never a silent truncation
	}
	return n
}

// eachOvRec calls f for sl's override records in application order. It
// walks the pool directly instead of materialising an array: a slot with
// no records costs one mask test, and the hot layer walks never build a
// zeroed [maxOvr]ovRec just to copy it back out.
func (s *State) eachOvRec(sl Slot, f func(r ovRec)) {
	if s.fxM.has(sl) {
		s.eachOvRecIn(sl, f) // the mask test inlines; a slot with no record costs no call
	}
}

// eachOvRecIn is eachOvRec past its mask test (kept out of line so
// eachOvRec itself inlines).
//
//go:noinline
func (s *State) eachOvRecIn(sl Slot, f func(r ovRec)) {
	n := 0
	for i := 0; i < int(s.g.nfx) && n < maxOvr; i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxOverride {
			f(r.r)
			n++
		}
	}
}

// ovRecs is the override records on slot sl, in application order.
func (s *State) ovRecs(sl Slot) (out [maxOvr]ovRec) {
	if !s.fxM.has(sl) {
		return
	}
	n := 0
	for i := 0; i < int(s.g.nfx) && n < maxOvr; i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxOverride {
			out[n] = r.r
			n++
		}
	}
	return
}

// hasOvRec reports whether slot sl has an override record (the position
// hashes mix them in; a slot holding only a loss record hashes as before
// the pool, whose loss timestamp was never hashed).
func (s *State) hasOvRec(sl Slot) bool {
	if !s.fxM.has(sl) {
		return false
	}
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxOverride {
			return true
		}
	}
	return false
}

// dropFx removes the effect records drop rejects and recomputes fxM.
func (s *State) dropFx(drop func(i int) bool) {
	n := 0
	s.fxM = slotMask{}
	for i := 0; i < int(s.g.nfx); i++ {
		// the predicate reads record i in place by index: a pointer into s
		// passed to it (or a copy's address) would move every State whose
		// path clears effects (clearFx) to the heap
		if r := &s.g.fx[i]; drop(i) {
			if r.kind == fxOverride {
				s.touchSl(r.sl)
			}
			continue
		}
		s.g.fx[n] = s.g.fx[i]
		s.fxM.set(s.g.fx[n].sl)
		n++
	}
	for i := n; i < int(s.g.nfx); i++ {
		s.g.fx[i] = fxRec{}
	}
	s.g.nfx = uint8(n)
}

// clearFx removes slot sl's effect records (it left, or a new object
// entered it: CR 400.7).
func (s *State) clearFx(sl Slot) {
	if s.fxM.has(sl) {
		s.dropFx(func(i int) bool { return s.g.fx[i].sl == sl })
	}
}

// addOvRec adds an override record to slot sl: merge collapses an identical
// record in place (keyword repeats), otherwise a repeated P/T setter moves to
// the newest position. A grant merges only with the newest record, as the order
// of the records is the order of what they grant (gorge lists its grants by
// timestamp). False when the pool is full.
func (s *State) addOvRec(sl Slot, r ovRec, merge, grants bool) bool {
	s.touchSl(sl)
	if s.fxM.has(sl) {
		for i := 0; i < int(s.g.nfx); i++ {
			if p := &s.g.fx[i]; p.sl == sl && p.kind == fxOverride && p.r.idx == r.idx && p.r.src == r.src && p.r.p == r.p && p.r.t == r.t && p.r.fl&^ovReps == r.fl&^ovReps {
				if p.r.fl&ovReps == ovReps || grants && (s.newerLayer6Fx(sl, i) || s.statM.any() && s.newerLayer6Static(p.r.ts)) {
					continue // a further identical record carries the rest
				}
				if merge {
					p.r.fl++
					return true
				}
				// A P/T record's position is its application order: a
				// repeated identical setter becomes the newest record,
				// or an older different setter would win again.
				r.fl = p.r.fl + 1
				copy(s.g.fx[i:], s.g.fx[i+1:s.g.nfx])
				s.g.fx[s.g.nfx-1] = fxRec{}
				s.g.nfx--
				s.g.fx[s.g.nfx] = fxRec{sl: sl, kind: fxOverride, r: r}
				s.g.nfx++
				s.fxM.set(sl)
				return true
			}
		}
	}
	if cur := s.ovRecs(sl); cur[maxOvr-1].idx != 0 {
		return false // full: the caller stalls (StallTables), never evicts
	}
	if int(s.g.nfx) >= maxFx {
		return false
	}
	s.g.fx[s.g.nfx] = fxRec{sl: sl, kind: fxOverride, r: r}
	s.g.nfx++
	s.fxM.set(sl)
	return true
}

// newerLayer6Fx reports an effect record on sl after pool position i that
// can reorder a layer-6 effect: a keyword or ability grant or removal, a loss
// of abilities, or any record that is not an override. A layer-6 effect is
// ordered by timestamp among layer-6 effects only (CR 613.1f, 613.7), so a
// newer record of another layer (a P/T, colour or type setter) lets a repeat
// of an older keyword record merge into it.
func (s *State) newerLayer6Fx(sl Slot, i int) bool {
	for j := i + 1; j < int(s.g.nfx); j++ {
		p := &s.g.fx[j]
		if p.sl != sl {
			continue
		}
		if p.kind != fxOverride {
			return true
		}
		if ov := &s.k().t.Overrides[p.r.idx-1]; ov.Grant[0] != 0 || ov.Grant[1] != 0 || ov.AddKw != 0 || ov.RemoveKw != 0 || ov.LoseAbilities || ov.CantAttack {
			return true
		}
	}
	return false
}

// newerLayer6Static reports a battlefield Continuous static that grants or
// removes keywords and is newer than timestamp ts. Its layer-6 effect sits
// between an older record and a repeat of it (CR 613.7), so the repeat takes
// a timestamp of its own: Abigale's second loss of abilities, after Chronicle
// of Victory entered, takes that grant away again. The static's filter is not
// read (a spare record is only a spare record).
func (s *State) newerLayer6Static(ts uint16) bool {
	k := s.k()
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		if s.layerTs(src) <= ts {
			continue
		}
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			if a := &cd.Abilities[i]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && (a.Static.Kw != 0 || a.Static.Override > 0 || a.Static.Grant != 0) {
				return true
			}
		}
	}
	return false
}

// setLossFx records a one-shot "loses all abilities" on sl at timestamp ts
// (until end of turn when eot); false when the pool is full. One record
// per slot carries the latest loss's timestamp, read only while sl's
// ovLoseAll or ovLoseAllEOT flag is set.
func (s *State) setLossFx(sl Slot, ts uint16, eot bool) bool {
	if s.fxM.has(sl) {
		for i := 0; i < int(s.g.nfx); i++ {
			if p := &s.g.fx[i]; p.sl == sl && p.kind == fxLoseAll {
				p.r.ts = ts
				if !eot {
					p.r.fl = 0
				}
				return true
			}
		}
	}
	if int(s.g.nfx) >= maxFx {
		return false
	}
	r := fxRec{sl: sl, kind: fxLoseAll, r: ovRec{ts: ts}}
	if eot {
		r.r.fl = ovEOT
	}
	s.g.fx[s.g.nfx] = r
	s.g.nfx++
	s.fxM.set(sl)
	return true
}

// ovSlots is the slots with an override record (fxM less the slots that
// hold only a loss record).
func (s *State) ovSlots() (m slotMask) {
	if s.g.nfx == 0 {
		return
	}
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.kind == fxOverride {
			m.set(r.sl)
		}
	}
	return
}

// lossFxTs is the timestamp of the latest one-shot loss on sl (0 if none).
func (s *State) lossFxTs(sl Slot) uint16 {
	if s.fxM.has(sl) {
		for i := 0; i < int(s.g.nfx); i++ {
			if p := &s.g.fx[i]; p.sl == sl && p.kind == fxLoseAll {
				return p.r.ts
			}
		}
	}
	return 0
}

var noXT xTargets

// xtOf is entry e's multi-object targets (read-only when it has none).
func (s *State) xtOf(e *stackEntry) *xTargets {
	if e.xi == 0 {
		return &noXT
	}
	return &s.g.xtp[e.xi-1]
}

// xtMut is entry e's multi-object targets for writing, taking a pool entry
// no live stack entry uses; nil when the pool is full (the game stalls).
func (s *State) xtMut(e *stackEntry) *xTargets {
	if e.xi != 0 {
		return &s.g.xtp[e.xi-1]
	}
	for i := uint8(1); i <= maxXTPool; i++ {
		if s.xtUsed(i) {
			continue
		}
		e.xi = i
		s.g.xtp[i-1] = xTargets{}
		return &s.g.xtp[i-1]
	}
	s.stall(StallTables)
	return nil
}

func (s *State) xtUsed(i uint8) bool {
	if s.ctl == ctlBuild && s.bld().e.xi == i || s.res.active && s.res.e.xi == i {
		return true
	}
	for j := 0; j < int(s.sp); j++ {
		if s.stack[j].xi == i {
			return true
		}
	}
	return false
}

// refreshTy recomputes the override-derived types of the slots an override
// record or an attached override static touches, into perm.gty with
// ovGenTy set (the core's typesAt reads them without a call), and clears the flag
// where no override applies any more. It runs before every step and every
// resolution op and after the changes that move overrides.
func (s *State) refreshTy(k *prep) {
	m := s.ovSlots()
	recs := m
	if k.gen.cpxTy {
		m = m.or(s.copyExcTySlots(k)) // a copy exception's added types ride gty
	}
	var walk slotMask // slots two order-sensitive effects reach (CR 613 walk)
	if k.gen.anyOvrStat {
		var one slotMask
		for sm := s.statM; sm.any(); sm = sm.rest() {
			src := sm.first()
			if a := s.perms[src].attached; a != NoSlot && k.gen.has(s.cardAt(src), gfOvrStat) {
				if one.has(a) || recs.has(a) {
					walk = walk.with(a)
				}
				one = one.with(a)
				m = m.with(a)
			}
		}
	}
	if k.gen.fxNeed&fxNeedSubSet != 0 {
		walk = walk.or(m) // a subtype grant beside a setter: every overridden slot walks (CR 613.1d)
	}
	if k.gen.fxNeed&(fxNeedGlobal|fxNeedSubGrant) != 0 && s.walkAllOn(k) {
		m = s.occ // a global override static or a subtype lord: every permanent walks
		walk = s.occ
	}
	if k.gen.tyPost {
		m = m.or(s.tyPostSlots(k)) // attached bestow cards, impending-dormant ones (kw_impending.go)
	}
	m = m.and(s.occ)
	if k.gen.anyFaceDown {
		// a face-down permanent's types ride on ovGenTy and its
		// characteristics come from the walk (face_facedown.go)
		fd := s.faceDownSlots()
		m = m.or(fd)
		walk = walk.or(fd)
	}
	for c := s.g.tyM.andNot(m); c.any(); c = c.rest() {
		sl := c.first()
		if s.perms[sl].ovr.flags&ovGenTy != 0 {
			s.perms[sl].ovr.flags &^= ovGenTy
			s.touchSl(sl)
		}
		s.perms[sl].ovr.walk = 0
	}
	s.g.tyM = m
	for ; m.any(); m = m.rest() {
		sl := m.first()
		p := &s.perms[sl]
		f0, t0 := p.ovr.flags, p.gty
		p.ovr.flags &^= ovGenTy
		p.ovr.walk = 0
		if walk.has(sl) {
			p.ovr.walk = walkOn
		}
		fd := p.gty & (gtyFaceDown | gtyProvMask)
		p.gty = s.typesGen(k, sl)&^(gtyFaceDown|gtyProvMask) | fd
		p.ovr.flags |= ovGenTy
		if p.ovr.flags != f0 || p.gty != t0 {
			s.touchSl(sl)
		}
	}
}

// askRec is a queued as-enters choice for the permanent at sl, or (sl
// NoSlot) a returning Aura's host ask for the exiled object ab, posed over
// the permanents entered by stamp ts (queueAskRet).
type askRec struct {
	sl        Slot
	ver, kind uint8
	card      CardID
	ab, pc    uint8
	ts        uint16
}

// preAsk is an as-enters colour / type choice gorge asks while the
// permanent spell is still on the stack (Heraldic Banner): state 1 posed,
// 2 answered (val is perm.chosen, applied when the program reaches the
// choice as the permanent enters).
// preReturn is preAsk.state for a card waiting in exile to return
// (leaveGen); 1 and 2 are a pre-ask posed and answered.
const preReturn uint8 = 3

// preReturnOp is preAsk.state for a card a resolution's return op parked with its
// as-enters choice: the answer runs the op again.
const preReturnOp uint8 = 4

// preReturnPay is preReturn for a card a cost's sacrifice set aside (the
// source of its exile left as the payment's pick): gorge asks the choice at
// once, before the rest of the resolution, and the answer enters the card
// and finishes the payment (payReturnPark, raGenRetChoice).
const preReturnPay uint8 = 7

// preDigEnchant is preAsk.state for an Aura a battlefield dig took, waiting
// in the library for its controller's choice among two or more hosts;
// preDigEnchantDone once answered (val is the host's slot).
const (
	preDigEnchant     uint8 = 5
	preDigEnchantDone uint8 = 6
)

type preAsk struct {
	obj   Obj
	kind  uint8
	state uint8
	val   uint8
	land  bool // a land being played (asked while it is still in the hand)
	// link: the clone park is an until-leaves link return (returnLinked), not
	// a resolution's return op; the answer enters the card from exile instead
	// of rerunning the op (copy_cloneenter.go finishCloneReturn)
	link bool
	// ordPick[:ordN] are the replacement sources the player named, in answer
	// order, to a CR 616.1 entry order ask; ordSet marks that the ask was
	// posed. The as-enters copy and the other sources compete over the entry
	// and the ask repeats until the unnamed rest commutes (repl_entryclone.go)
	ordPick [7]Obj
	ordN    uint8
	ordSet  bool
	// ordMid: gorge moved the permanent in after the first answer, with every
	// counter body folded into the move, and asks the rest with it on the
	// battlefield; the entrant's own line (the copy) is still to apply
	ordMid bool
	// ask2/val2: the copy template's own as-enters choice, asked after the
	// clone election (copy_templatechoice.go); ask2 is its kind, val2 the answer
	ask2 uint8
	val2 uint8
}

// preEnterAsk poses e's as-enters choice before the permanent spell
// resolves onto the battlefield; false when there is none (or it was
// answered: the resolution proceeds).
func (s *State) preEnterAsk(k *prep, e *stackEntry) bool {
	return s.preAskFor(k, e.obj, e.card, false)
}

// preLandAsk poses a played land's as-enters choice (Secluded Courtyard)
// while the land is still in the hand, as gorge asks it: the land play
// completes once it is answered (applyGen).
func (s *State) preLandAsk(k *prep, o Obj) bool {
	return s.preAskFor(k, o, s.objs[o].card, true)
}

func (s *State) preAskFor(k *prep, o Obj, c CardID, land bool) bool {
	if s.g.pre.state == 2 && s.g.pre.obj == o {
		return false
	}
	kind := s.enterChoiceKind(k, c)
	if kind == 0 && !land && k.gen.has(c, gfEnterRepl) {
		kind = s.cloneAskKind(k, c) // only a cast spell is asked (gorge FlagManaSpent)
	}
	if kind == 0 {
		return false
	}
	s.g.pre = preAsk{obj: o, kind: kind, state: 1, land: land}
	if !land {
		s.res.active = false
	}
	s.g.ctlRet = ctlPriority
	s.ctl = ctlGenAsk
	return true
}

// enterChoiceKind is the as-enters choice card c's enters replacement
// opens with (akColor, akType, akNumber, akName, akPayLife), or 0.
func (s *State) enterChoiceKind(k *prep, c CardID) uint8 {
	if k.gen == nil || !k.gen.has(c, gfEnterRepl) {
		return 0
	}
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplEntersProgram || len(a.Prog) == 0 {
			continue
		}
		if a.Repl.C == kop.ReplAfterEnter {
			// an R: line (ReplacementResult$ Updated) poses its choice once
			// the permanent is on the battlefield, not while it is still a
			// spell on the stack: the queued as-enters ask (runReplProgAs)
			// poses it, and a shock land's pay-life election is one of these
			continue
		}
		switch a.Prog[0].Op {
		case kop.OpChooseColor:
			return akColor
		case kop.OpChooseType:
			return akType
		case kop.OpExt:
			switch a.Prog[0].Ext() {
			case kop.ExtChooseNumber:
				return akNumber
			case kop.ExtChooseName:
				return nameAskKind(&a.Prog[0])
			}
		case kop.OpUnlessPaid:
			// the shock lands' as-enters pay-life election
			if s.shockAmount(k, a.Prog) > 0 {
				return akPayLife
			}
		}
	}
	return 0
}

// ceaseTokensGen is CR 704.5d on a generated table: a token that left the
// battlefield sits in its new zone until the state-based actions (gorge
// shows Uncharted Voyage's token in the library during the surveil), then
// ceases to exist; so does a spell copy that left the stack (CR 704.5e).
func (s *State) ceaseTokensGen() {
	// tokens and copies are the object table's suffix: the deck cards come
	// first (Init) and every later object is minted by newToken with
	// ofToken or ofCopy, which nothing clears; walk only that suffix
	i0 := int(s.nobj)
	for i0 > 0 && s.objs[i0-1].of&(ofToken|ofCopy) != 0 {
		i0--
	}
	k := s.k()
	for i := i0; i < int(s.nobj); i++ {
		ob := &s.objs[i]
		if ob.of&(ofToken|ofCopy) == 0 {
			continue
		}
		if s.mObj(k, mechSBAChain, Obj(i)) {
			continue // gorge's sbaChain: it ceases at the pass that destroys nothing (sbaChainEnd)
		}
		switch ob.zone {
		case kop.ZoneLibrary, kop.ZoneHand, kop.ZoneGraveyard, kop.ZoneExile:
			if ob.zone == kop.ZoneExile && ob.of&ofCopy != 0 {
				if k.gen != nil && k.gen.anyPrepare { // CR 722.3c: a linked prepared copy waits in exile
					if _, ok := s.preparedCopyLive(k, Obj(i)); ok {
						continue
					}
				}
			}
			if ob.zone == kop.ZoneGraveyard {
				s.ceaseQuiet(Obj(i)) // nothing has left the graveyard (CR 111.8; face_prepare.go)
				continue
			}
			s.detach(Obj(i))
		}
	}
}

// nonCreatureSubtype names the land, artifact, enchantment, spell and planeswalker
// subtypes kgen's filter vocabulary accepts (CR 205.3g-205.3k).
var nonCreatureSubtype = map[string]bool{
	"Plains": true, "Island": true, "Swamp": true, "Mountain": true, "Forest": true,
	"Desert": true, "Gate": true, "Cave": true, "Sphere": true, "Town": true, "Locus": true, "Lair": true,
	"Mine": true, "Power-Plant": true, "Tower": true, "Urza's": true,
	"Equipment": true, "Treasure": true, "Food": true, "Clue": true, "Vehicle": true, "Blood": true,
	"Gold": true, "Map": true, "Powerstone": true, "Incubator": true, "Fortification": true, "Junk": true,
	"Spacecraft": true, "Lander": true, "Communicator": true, "Planet": true,
	"Aura": true, "Saga": true, "Shrine": true, "Curse": true, "Cartouche": true, "Class": true,
	"Room": true, "Rune": true, "Background": true, "Role": true, "Case": true, "Shard": true,
	"Adventure": true, "Arcane": true, "Lesson": true, "Trap": true, "Omen": true,
	"Ajani": true, "Liliana": true, "Vivien": true, "Chandra": true, "Kaito": true, "Kiora": true,
	"Jace": true, "Garruk": true, "Elspeth": true, "Nissa": true, "Gideon": true, "Teferi": true,
}
