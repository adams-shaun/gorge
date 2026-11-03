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
	castTapOn       bool  // castRingOn, or some card carries a Tf2ManaFromSelf reader: the planner's tap sites record a Treasure or observed source (noteTapSrc)
	castRingOn      bool  // some expression reads the ring (ExThisTurnCastSpec): the push records it
	embSubGrant     bool  // an emblem has a Continuous subtype grant (walked on every walk slot)
	anyTypeValid    bool  // some card's as-enters type ask is narrowed by ValidTypes$ (CardDef.TypeValid; act_choosetype.go)
	anyAddChosen    bool  // some static is AddType$ ChosenType (kop.Static.AddChosen; act_choosetype.go)
	chosenCardTgt   bool  // some card target's or spell-cast trigger's filter reads the source permanent's chosen type (act_choosetype.go)
	anyLoseRec      bool  // some override record removes abilities
	tyCombat        bool  // some override sets types or makes a creature: a combatant can stop being one (CR 506.4 removeNoncreatureCombatants; kperf2)
	l6RecOnly       bool  // fxNeedL6 comes only from an effect's keyword removal (Debuff): a slot without an fx record keeps the fixed-order layer 6 (l6On; kperf2)
	lossSlow        bool  // anyLoseStat: genLost always asks ovrLoses (kperf-blb, kperf2)
	ctrlSBA         bool  // the state-based check settles control (a control static, a source-bound OpGainControl)
	anyNTOv         bool  // an OpOverride / OpOverrideAll lasts until its controller's next turn
	legendGrant     bool  // some Override adds or sets the Legendary supertype (legendRule reads derived types)
	anyMayPlay      bool
	anyMayRaise     bool    // some may-play static owes a non-mana RaiseCost$ (cost_mayplayraise.go)
	anyMayKind      bool    // some OpMayPlay grants a record with a kind rider (cast_maypermission.go)
	anyMayLimit     bool    // some OpMayPlay record carries a MayPlayLimit cap (cast_maypermission.go)
	sCls            []uint8 // per card: its static classes (static_classgates.go scls*)
	anyGraveAct     bool
	anyHandAct      bool // an activated ability works from the hand (cycling; repl_regenerate.go)
	anyRemPersist   bool // a card's enters trigger remembers what its leaves trigger reads (CardDef.RemPersist; zone_rememberpersist.go)
	anyNinjutsu     bool // a card has a K:Ninjutsu ability (kop.AfNinjutsu; kw_ninjutsu.go)
	anyPowerUp      bool // a card has a printed PowerUp$ ability (kw_powerup.go)
	anyStackTgt     bool // some target slot is a TgtStackObj (tgt_stackability.go)
	anyCopyAb       bool // some target slot is a TgtCopyAb or TgtCopyTrig (copy_ability.go)
	anyBestow       bool // a card has printed bestow (CardDef.HasBestow; kw_bestow.go)
	anyKickCost     bool // a card has a non-mana kicker cost (CardDef.KickerCost; kw_kicker.go)
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
	anyLandwalk     bool // a card has printed landwalk (repl_regenerate.go)
	anyEvasion      bool // a card has a printed CardDef.Evasion keyword (act_remember.go)
	anyTokEndCombat bool // some CreateTokenEx carries the AtEOT$ SacrificeCombat rider (trig_tokenateot.go)
	anyProwessGrant bool // some Continuous static grants prowess: onCast counts one trigger per static (kw_prowess.go)

	anyMinBlk                bool // a card has a printed MinMaxBlocker floor (CardDef.MinBlockers; combat.go)
	anyUntapOther            bool // a card has a StatUntapOther static (static_untapother.go)
	anyTapExtra              bool // a card has a StatTapExtra static (mana_tapextra.go)
	anyMaxBlk                bool // a card has a MinMaxBlocker Max$ 1 static (kop.StatMaxBlockers; combat_maxblockers.go)
	anyCantGain, anyCantDraw bool // StatCantGainLife / StatCantDraw statics (static_cantdraw.go)
	anyCdmgTough             bool // a card has a StatCombatDmgToughness static (combat_toughnessassign.go)
	anyDisETB                bool // a card has a DisableTriggers enters static (kop.StatDisableETB; trig_disable.go)
	anyMadness               bool // a card has printed madness (CardDef.HasMadness; kw_madness.go)
	anyHexCol                bool // a card has printed hexproof from a colour (CardDef.HexColors; kw_hexproofcolor.go)
	anyProtCol               bool // a card has printed protection from a colour (CardDef.ProtColors; kw_protection.go)
	// grantTrigEv: events (bit per TrigEvent) of triggered abilities a
	// Continuous static grants to the permanent its source is attached to
	// (AddTrigger$): their listeners include those hosts (kw_protection.go trigSubs)
	grantTrigEv uint32
	// globTrigGrant: a Continuous static grants a triggered ability beyond its
	// source's host ("artifact creatures you control have ..."), so any
	// permanent can hold the trigger (trigger_granted.go grantTrigHosts)
	globTrigGrant bool
	anyProtF      bool // a card has printed K:Protection:<spec> (CardDef.ProtFrom; kw_protection.go)
	// manaTaps: the table has a Taps trigger (TrigTaps) and a granted mana
	// ability, which can tap a Taps listener for mana: the mana taps fire it
	// (manaTapped)
	manaTaps bool
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
	anyWitherGrant bool           // some table part grants kop.KwWither (kw_wither.go); sets anyInfect
	anyEOTEmb      bool           // some emblem ends at cleanup (kop.Emblem.UntilEOT; kw_protection.go)
	anyNTEmb       bool           // some emblem ends at its controller's next turn (kop.Emblem.UntilNextTurn; static_effectdelivered.go)
	embObjEv       uint32         // object events (bit per TrigEvent) some emblem trigger listens to (fireEmblemsObj)
	embCombatEv    uint32         // events (DamageDone, AttackersDeclared, LifeGained) some emblem trigger listens to (trig_effectuntileot.go)
	anyGrant       bool           // some override or static grants an ability
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
	anyFiltRepl    bool           // a filtered damage doubler or token multiplier, or an x3 one (repl_multipliers.go)
	untapRepl      bool           // a ReplOthersDoesntUntap or ReplAnyUntap card: the untap step runs untapStepGen (act_energy.go)
	anyUntapBlock  bool           // some card has ReplAnyUntap: every untap site consults State.untapBlocked (static_cantuntap.go)
	anyCtrBlock    bool           // some card has StatCantPutCounter: every counter placement consults State.putCounterBlocked (ctr_cantput.go)
	anyCastSpent   bool           // some SpellCast trigger carries Tf2CastSpent (opus)
	anySpentGate   bool           // some SpellCast trigger carries a Spell.ManaSpent gate in Trigger.N (trig_spentvalid.go)
	anySpent       bool           // some expression reads the mana spent to cast a spell (ExCastManaSpent; copy_asenters.go)
	anyBecomeCopy  bool           // some enters program is OpBecomeCopy (an as-enters clone): a permanent it made a copy is projected as its printed card, and a token copy of a clone is asked its election (ctr_entryperkind.go)
	anyEntryMerge  bool           // the table has an AddCounter replacement and more than one source of entry counters: enterGen merges them into one put per kind (ctr_entryperkind.go)
	anyEffEtbCtr   bool           // some instruction is ExtEffEtbCtr: enterGen puts a cast spell's pending extra counters on (ctr_effectentry.go)
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
	anyCantCast    bool           // some card has a StatCantBeCast (trig_delayedtiming.go castForbidden)
	anyCantAct     bool           // some card has a StatCantActivate (cast_cantactivate.go actForbidden)
	anyFaces       bool           // some card has faces (CardDef.Faces; face_multiface.go)
	anyWarp        bool           // some card has warp (CardDef.HasWarp; kw_warp.go)
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
	anyConvoke     bool           // some card has printed or granted convoke/improvise or harmonize (CardDef.Convoke/Improvise/Harmonize, StatStackKw Improvise; kw_convoke.go)
	anyBeam        bool           // some card has printed beam me up (CardDef.BeamMeUp; kw_beammeup.go)
	anyBeamBar     bool           // some card has a StatCantBeBeamedUp static
	anyHarmonize   bool           // some card has printed harmonize (CardDef.Harmonize; kw_harmonize.go)
	anyCascade     bool           // some card has printed cascade (CardDef.Cascade; kw_cascade.go)
	anyStackKw     bool           // some card has a StatStackKw: AffectedZone$ Stack keyword grant (static_stackkeyword.go)
	anyStorm       bool           // some card has a StatStackKw Storm grant (kw_storm.go)
	anySplitSecond bool           // some card has a StatStackKw Split second grant (kw_splitsecond.go)
	anyStackLife   bool           // some card has a StatStackKw Lifelink grant (static_stacklifelink.go)
	anySplit       bool           // some card is a split card (kop.FaceSplit: its mana value off the stack; face_castface.go)
	anyTwobrid     bool           // some card has a twobrid pip in its printed cost (kop.ManaCost.Tb; mana_twobrid.go)
	anyBeamGrant   bool           // some card has a StatBeamGrant static
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
	anyKwAct       bool           // a trigger is TrigSearchedLibrary, TrigForage or TrigDiscover
	anyDilemma     bool           // some table reads ExDilemmasFaced or TrigFacesDilemma (trig_dilemma.go)
	anyScryTally   bool           // some expr is ExScriedThisTurn (value_scrytally.go)
	anyTokCr       bool           // some trigger is TrigTokenCreated (act_proliferate.go)
	anyMonstrosity bool           // some program is OpMonstrosity or some trigger is TrigBecomeMonstrous (act_monstrosity.go)
	anyAirbend     bool           // some program is ExtAirbend (act_airbend.go)
	anyMayWhileSrc bool           // some OpMayPlay lasts DurWhileSrcMay (cast_whilesource.go)
	anyGoad        bool           // some program is OpGoad (act_goad.go)
	anyPermMark    bool           // the table gives some kop.PredPermMark mark an arena mask (filter_permmarks.go)
	anyCombatHit   bool           // some card records its combat hits on players (cdRec) or some expression reads the legendary-hit bit (value_turntallies.go)
	cdLegOn        bool           // some expression is ExOppsLostLife N 1: record cdLeg
	anyAddPhase    bool           // some program is ExtAddPhase, some trigger is Tf3FirstAttack or some expression is ExPhaseCount (act_addphase.go)
	anyAttackedObj bool           // the table gives mechAttackedObj an arena mask: finishAttacks marks the declared attackers
	anySuspect     bool           // the table gives mechSuspected an arena mask (act_suspect.go)
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
	anyLure        bool           // some card is a lure (CardDef.Lure; combat_lure.go)
	anyMustBlock   bool           // some card is printed must-be-blocked or the table gives mechMustBlocked an arena mask (combat_mustbeblocked.go)
	anyFaceExile   bool           // some Dig exiles face down (kop.DigFaceDown; zone_exilefacedown.go)
	anyExW         bool           // some card reads the exiled-with source link (CardDef.ExiledWith; zone_exiledwith.go)
	exwRec         []uint8        // per card: its resolutions record the cards they exile (anyExW only): exwRecRead, exwRecRev
	anyCloneCopy   bool           // some program is ExtCloneCopy: permanent copies record their source (fxCopyPerm; copy_clone.go)
	anyAegis       bool           // some program is ExtAegisClone, or some card has a TrigAttached ability (zone_exilethenattach.go)
	anyHideaway    bool           // some program is ExtHideaway (kw_hideaway.go)
	anyLoyaltyX    bool           // some ability costs a planeswalker's [-X] (CostLoyaltyX; cost_loyaltyx.go)
	anyLifeX       bool           // some spell or ability costs an announced PayLife<X> (Cost.N 0; cost_paylifex.go)
	anyCopyCast    bool           // some OpCastFree casts a copy (C bit 6; cast_copycast.go)
	anyNumTimes    bool           // some ability announces a pay-N-times count (Ext NumTimes; cast_announcetimes.go)
	anyExHaste     bool           // some ability can't be blocked except by creatures with haste (Ext ExHaste; static_cantblockexcept.go)
	anyColGrant    bool           // some ability grants by a chosen colour (Ext ColorGrant; static_colourgrant.go)
	anyDiscRef     bool           // some expression reads RefDiscarded (value_discardedref.go)
	anySpellCond   bool           // some spell ability carries a cast restriction in Trig.Cond (cast_spellcondition.go)
	anyEntUntap    bool           // some ReplOthersEnterProgram untaps the entrant (ExtEnterUntapped): a tap/untap pair asks the CR 616.1 order (repl_entryorder.go)
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
	anyWhileHost   bool           // some override or emblem ends when its host leaves (kop.Override/Emblem.WhileHost; filter_combatwords.go)
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
	anyExileAny  bool           // some ReplExileOnLeave has Repl.C 1: a move to a graveyard from any zone checks it (placeFrom)
	altFree      []uint8        // per card: 1 + the ability index of its own free StatAlternativeCost (nil when no card has one; cast_altfree.go)
	// eachCol: card i's "one mana of each colour among" ability (Bloom
	// Tender), -1 none: a source gorge's planner census misses, reached by
	// a potential play's scripted prefix (script_gen.go)
	anyEachCol bool
	anyRestr   bool // some card has restr (kperf-blb: the ward / unless window's gate)
	// anyRestrScript: some card has a restricted source gorge's census still
	// misses (a reflected one: Plaza of Heroes), a scripted-prefix gap
	// (gfRestrScript); anyRestrSrc: some card has a restricted source the
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
	// (player.poolR's kind bits), so restrKinds are encodable (Giada's Angel
	// spells, Secluded Courtyard's creature spells of the chosen type, a
	// Standard table's three restricted lands)
	restrF [restrKinds]kop.FilterID
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
	// anyWindowGap: some card has eachCol or restr (gorge's ward / unless
	// window activates them, mana.go windowTap)
	anyWindowGap bool
	// l6Stall: a global lose-all override static shares the table with a
	// subtype lord the layer-4 walk reads (the loss check would re-enter
	// the walk): the game stalls at its start (static_layer6.go prepL6)
	l6Stall bool
	// anyTriggerAttackers: some program reads RefTriggerAttackers
	// (trig_attackers.go)
	anyTriggerAttackers bool
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
)

// rbCtrOn is either filtered counter replacement.
const rbCtrOn = rbDblCtrOn | rbPlusCtrOn

type grantRef struct {
	card CardID
	ab   uint8
}

// noManaPC is the planner's view of a permanent whose mana ability an
// override took away.
var noManaPC = pcard{manaAb: -1}

// prepareGen precomputes a generated table (nil for a hand-written one).
func prepareGen(t *kop.Table, p *prep) *genPrep {
	if t.Exprs == nil && t.TypeVocab == nil && t.Conds == nil {
		return nil
	}
	enteredNeed := false
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
	for i, n := range t.Subtypes {
		if creatureTypeWords[n] {
			g.creatSub = g.creatSub.With(i)
		}
	}
	for i := range t.Overrides {
		if t.Overrides[i].LoseAbilities {
			g.anyLoseRec = true
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
	p.phasing = tablePhasing(t) // kw_phasing.go
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
		g.anySpent, g.anyHist = true, true // the Increment comparison reads the spell's recorded spend (kw_increment.go); histCast itself is anyHist-gated. Never anyCastSpent: no planner decline
	}
	prepCtrAdded(t, p, g) // ctr_addedthisturn.go
	if tableSpentGate(t) {
		g.anySpentGate, g.anySpent, g.anyHist = true, true, true // the Spell.ManaSpent gate reads the spell's recorded spend (trig_spentvalid.go); never anyCastSpent: no planner decline
	}
	prepSkipUntap(t, g)
	prepOpus(t, g)
	prepConverge(t, g)   // mana_converge.go
	prepEvoke(t, g)      // kw_evoke.go
	prepEffEtbCtr(t, g)  // ctr_effectentry.go
	prepStaticTrig(t, g) // trig_mayplaystatic.go
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
	g.anyTwobrid = tableTwobrid(t)
	g.anyBeamGrant = tableHasStatic(t, kop.StatBeamGrant)
	g.anyCantCast = tableHasStatic(t, kop.StatCantBeCast) || prepSpellCond(t, g)
	prepCastLimit(t, g) // static_castlimit.go
	prepCantAct(t, g)   // cast_cantactivate.go
	g.anyMaxBlk = tableHasStatic(t, kop.StatMaxBlockers)
	g.anyUntapOther = tableHasStatic(t, kop.StatUntapOther)
	g.anyTapExtra = tableHasStatic(t, kop.StatTapExtra) || p.manaEach // an all-at-once source's own extra (mana_eachcolour.go)
	g.anyDisETB = tableHasStatic(t, kop.StatDisableETB)
	g.anyCantGain = tableHasStatic(t, kop.StatCantGainLife)
	g.anyCantDraw = tableHasStatic(t, kop.StatCantDraw)
	g.anyDiscard = tableHasOp(t, kop.OpDiscard)
	g.anyRemDisc = tableRemembersDiscards(t)
	g.anySpellMana = prepDbMana(t) // mana_dbresolve.go
	g.anyMayKind = tableMayPlayKinds(t)
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
		if c.Landwalk != 0 {
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
		if c.HasBestow {
			g.anyBestow = true
		}
		if c.OptCost.Flags != 0 || c.OptBehold {
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
		if c.Cascade != 0 {
			g.anyCascade = true
		}
		if c.MorphFamily != 0 || cardMintsFaceDown(c) {
			g.anyFaceDown = true
		}
		if c.Evasion != 0 {
			g.anyEvasion = true
		}
		if c.MinBlockers != 0 {
			g.anyMinBlk = true
		}
		if c.HasMadness {
			g.anyMadness = true
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
				}
				if a.Trig.Event == kop.TrigSearchedLibrary || a.Trig.Event == kop.TrigForage || a.Trig.Event == kop.TrigDiscover {
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
				case kop.ReplDoubleTokens:
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
					g.replBits[i] |= rbDblCtrOn
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
				case kop.ReplGraveToLibrary:
					g.replBits[i] |= rbGraveLib
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
					if a.Static.Ward != 0 {
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
				if a.AFlags&kop.AfManaEachColor != 0 && a.AFlags&kop.AfLatent == 0 && g.eachCol(CardID(i)) < 0 {
					g.smallFor(CardID(i)).eachCol = int8(j)
					g.setFlag(CardID(i), gfEachCol)
					g.anyEachCol, g.anyWindowGap = true, true
				}
				if a.ManaRestrict != 0 {
					g.addRestrF(a.ManaRestrict)
				}
				if firstMana && a.AFlags&kop.AfLatent == 0 {
					firstMana = false
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
						g.prepFiltFirst(CardID(i), j) // a filter first and only (mana_filterfirst.go)
					}
				} else if restrAnyTapMana(a) && !restrSrcAb(a) && !restrWideAnyMana(a) && g.anyR(CardID(i)) < 0 && firstManaAb(&t.Cards[i]) >= 0 {
					g.smallFor(CardID(i)).anyR = int8(j)
					g.setFlag(CardID(i), gfAnyR)
					g.anyAnyR = true
				} else if filterTapMana(&t.Cards[i], a) && g.filt(CardID(i)) < 0 {
					g.smallFor(CardID(i)).filt = int8(j)
					g.setFlag(CardID(i), gfFilt)
					g.anyFilt = true
				}
				if int(g.filt(CardID(i))) != j && censusGapMana(a) && !lifePainLandAb(&t.Cards[i], a) && g.gapOther(CardID(i)) < 0 {
					g.smallFor(CardID(i)).gapOther = int8(j)
					g.setFlag(CardID(i), gfGapOther)
				}
				if a.AFlags&kop.AfLatent != 0 {
					var gp pcard
					gp.manaAb, gp.manaCol, gp.manaN = int8(j), a.ManaColors, max(a.ManaN, 1)
					gp.manaTap = a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0
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
					if int(pc.manaAb) == j && (a.AFlags&(kop.AfManaChosen|kop.AfManaDynamic|kop.AfLatent) != 0 || a.ManaRestrict != 0 || a.ManaColors == 0) {
						pc.manaAb = -1
					}
				}
			}
		}
		prepManaGate(g, c, CardID(i), pc)
		prepManaRefl(g, c, CardID(i), pc)
		g.prepRestrSrc(c, CardID(i), pc)
		// latent triggered abilities are not subscriptions
		pc.trigMask = 0
		for j := range c.Abilities {
			a := &c.Abilities[j]
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
	anyTaps, manaGrant := false, false
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			anyTaps = anyTaps || a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps
			g.tapsWhose = g.tapsWhose || a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps && a.Trig.Whose != kop.WhoseAny
			g.anySelfDisc = g.anySelfDisc || a.Kind == kop.AbTriggered && selfDiscLine(a)
			g.anyTWTap = g.anyTWTap || a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps && a.Trig.N == kop.TrigNTeamworkTap
			if a.Kind == kop.AbTriggered && a.Trig.Flags3&kop.Tf3GameOnce != 0 {
				if j >= 0x3E {
					panic("kernel: GameActivationLimit on ability index >= 62") // usedRec.ab packs 0x40|ab
				}
				g.anyGameOnce = true
			}
			manaGrant = manaGrant || a.Kind == kop.AbMana && a.AFlags&kop.AfLatent != 0
		}
	}
	g.manaTaps = anyTaps && manaGrant
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
		for j := range t.Emblems[i].Abilities {
			if a := &t.Emblems[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCanAttackDefender {
				g.anyCanAtkDef = true // an Effect-delivered grant: prepared like a printed one
			}
			if a := &t.Emblems[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.Kw&kop.KwProwess != 0 {
				g.anyProwessGrant = true
			} else if a.Kind == kop.AbTriggered {
				switch a.Trig.Event {
				case kop.TrigETB, kop.TrigDies, kop.TrigLeaves, kop.TrigSacrificed:
					g.embObjEv |= 1 << a.Trig.Event
				case kop.TrigDamageDone, kop.TrigAttackersDeclared, kop.TrigAttacks, kop.TrigLifeGained:
					g.embCombatEv |= 1 << a.Trig.Event
				}
			}
		}
	}
	for i := range t.Cards {
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
	g.anyPrevFrom = grantsKw(t, kop.KwPreventCombatDamageFrom)
	prepReplMult(t, g)
	prepInfect(t, g)
	prepPoison(t, g)
	prepLayerWalk(t, g)
	prepChooseType(t, g) // act_choosetype.go
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
	maxEmb    = 4  // emblems per player
	maxDmgMap = 8  // damage-map entries
	maxXTgt   = 7  // objects of multi-object target slots beyond the first
	maxUsed   = 8  // once-per-turn records
	// maxSacOrd bounds one effect's sacrifice batch. The FDN and BLB
	// 1500-pair lockstep slices reach 12 (measured
	// 2026-10-22); 16 leaves headroom without paying for a whole board.
	maxSacOrd = 16
)

// ovRec is an override record applied to a permanent: Table.Overrides[idx]
// from the card src (whose abilities its Grant indexes name).
type ovRec struct {
	idx uint8 // Table.Overrides index + 1 (0 = empty)
	// fl: bit 7 = until end of turn; bits 5-6 = ovNT/ovNTYou; bits 0-4 = further identical
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
)

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
	fxOverride uint8 = iota // a Table.Overrides record (applyOverride)
	fxLoseAll               // OpLoseAbilities' one-shot "loses all abilities" (layer 6)
	fxKwGrant               // a promoted until-end-of-turn keyword grant's timestamp (static_layer6.go kwGrantFx)
	fxCopyExc               // a copy's exception override (layer 1, CR 707.9b; copy_exceptions.go)
	fxControl               // a layer-2 control record (CR 613.1b; static_control.go)
	fxCopyEOT               // a permanent holding until-end-of-turn copies, settled at cleanup (copy_clone.go)
	fxCopyPerm              // a permanent's newest permanent copy marker: r.p the source object, r.idx the exception override (copy_clone.go)
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
	div  [maxXTgt + 1]uint8 // damage assigned per object of the divided slot, in pick order
}

// genState is the generated-table part of State.
type genState struct {
	graveKept bool // removePerm: this move to a graveyard takes no "from anywhere" redirect
	fx        [maxFx]fxRec
	nfx       uint8
	xtp       [maxXTPool]xTargets // stackEntry.xi - 1 indexes it
	atkHi     uint8               // the highest attacker group ever taken (the hash walks 1..atkHi)
	tli       uint8               // LKI index of a target the resolving entry moved off the battlefield; bit 7: a filter is reading it (zone_lki.go)
	energy    [2]uint8            // each player's energy counters (CR 122.1; saturates at 255; act_energy.go)
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
	roleOut uint8  // a Role token may be on the battlefield: the SBA scans for CR 704.5z (static_roletokens.go)
	digLo   uint8  // a DuDugMark DigUntil's first exile index (act_discoverbottom.go; the padding before excess)
	// raiseO / raiseB: a may-play surcharge being offered / owed by the cast
	// being built (cast_whileexiled.go)
	raiseO, raiseB uint8
	excess         int16
	lostAmt        int16
	tokMade        uint8
	entHost        uint8 // host slot + 1 of an Aura about to enter attached: enter attaches it before its enters check (zone_handmoveaura.go; transient, 0 between moves)

	dmg  [maxDmgMap]struct{ code, amt uint8 }
	ndmg uint8

	may  [maxMay]mayPlay
	nmay uint8
	// parad[p] bit i: p resolved a Paradigm spell with the table's i-th
	// Paradigm name (CardDef.Paradigm-1); never reset (kw_paradigm.go)
	parad [2]uint8
	sidN  uint8 // the last stack identity given (tgt_stackability.go)

	dp    [maxDProg]dprog
	dpLKI [maxDProg]uint8 // each program's LKI pool index (zone_lki.go)
	ndp   uint8
	stkCt uint8 // a countered stack ability's controller+1, for its "its controller" (tgt_stackability.go)

	emb  [2][maxEmb]uint8  // emblem index+1
	embC [2][maxEmb]CardID // the card that created it (its latent abilities run the emblem's triggers)
	embO [2][maxEmb]Obj    // that card's object (the source of the emblem's triggers)
	embN [2][maxEmb]uint8  // the Effect's frozen SetChosenNumber$
	nemb [2]uint8
	nlA  [2]uint8 // locked permanents, object+1, compacted (act_lockpermanent.go; fills the padding before lki)

	lki [maxLKI]lkiRec // the shared LKI pool (zone_lki.go)

	used  [maxUsed]usedRec // once-per-turn triggers / statics used this turn
	nused uint8

	died     uint8    // creatures put into a graveyard from the battlefield this turn
	attacked [2]uint8 // creatures each player attacked with this turn
	oppAtt   uint8    // 1 once a declaration this turn named a player as defender (prep.anyOppsAtt only; value_oppsattacked.go)
	isCast   [2]uint8 // instants and sorceries each player cast this turn (Tf2CastSnapshot)
	// the turn history bank (value_turnhistory.go): per player, counters kgen's
	// readers name by slot (histSpells: spells cast), and the mana spent
	// casting spells (expend). Kept only when genPrep.anyHist.
	hist [2][3]uint8
	// actRing: per activator, the first actRingN abilities activated this turn, each as the source Obj+1 (0 = empty), actN the counts. Written only when genPrep.actRingOn; in no hash (cast_activationledger.go).
	actRing [2][actRingN]uint8
	actN    [2]uint8

	// pending as-enters choices (colour / creature type) and the leyline ask
	ask  [4]askRec
	nask uint8
	// askPrio is the priority holder when the first queued as-enters ask
	// was queued (gorge asks it mid-resolution, the holder unchanged);
	// priSave (0x80 | holder) the holder to restore once the asks are done
	askPrio, priSave uint8
	holdPrio         uint8    // the priority holder when the free casts were picked
	ctlRet           uint8    // control state to return to after the asks
	pre              preAsk   // an as-enters choice asked while the permanent spell is on the stack
	ley              [2]uint8 // leyline asks: starting player done, Leylines declined (nextLeyline)
	leyP             uint8
	leyOn            bool
	leyDn            bool
	crime            uint8 // CR 700.13: bit p, player p committed a crime this turn; 0 unless genPrep.crimeOn

	chPick [6]struct {
		sl Slot
		m  uint8
	} // per restricted charm source: m bits 0-3 picked modes, bit 7 ThisGame scope (modal_charmpick.go)
	nchPick    uint8    // entries of chPick in use
	trigTurn   uint16   // triggered abilities resolved this turn (StallLoop)
	dilemmas   [2]uint8 // the dilemmas each player faced this turn (trig_dilemma.go)
	on         bool     // the State plays a generated table
	batchLatch uint8    // trig_oneormore.go: bit 7 a leave to a graveyard is being checked; bits 0-4 the open batch's first pending index + 1 (0 = none). Was pendOver (unused), so State is unchanged
	tyM        slotMask // slots whose perm.gty holds override-derived types (ovGenTy)
	stalled    uint8    // why the game was stopped at the kernel's capacity (a draw); 0 = it wasn't
	noTgtRed   bool     // cost computation without target-dependent reductions
	mayPlayAny bool     // the may-play cast being priced is under the any-type rider
	// poolRChosen is, per player, the creature type the source of poolR's mana chose.
	poolRChosen [2]uint8
	probe       probe    // the spell / ability a priority feasibility check is for
	tgtSelf     uint8    // the graveyard card a feasibility check is for, plus one (0: none): it leaves the graveyard for the stack when cast, so it is no target of its own cast
	inLoop      bool     // executing one object of a multi-object target slot
	mt          multiTgt // the multi-object target slot being chosen
	lastCK      kop.CounterKind
	disc        Obj      // the card being discarded (TrigDiscarded)
	nlB         [2]uint8 // locked permanents 3 and 4 (nlA; fills the padding before kicked)

	redNC [2]uint8 // ExRedNCDmg: the noncombat damage each player's red sources dealt this turn, saturating (prep.anyRedNC only; value_damagetally.go; fills the pad before kicked)
	// grantSrc is the granting permanent plus one (0 = none) while a grant
	// walk (grantedAbilitiesN) reports one ability to its callback; it fills
	// the pad after redNC.
	grantSrc    uint8
	cast        slotMask // slots whose permanent was cast by its controller
	zoneTurn    objBits  // objects put into their current zone this turn
	enteredN    uint8    // the live prefix of State.cold.entered
	enteredFull bool
	enteredOn   bool // the table has an ExThisTurnEntered expression
	// (act_initiative.go; the padding before mo)
	// the initiative's holder+1 (0 nobody), each player's venture marker
	// (room index+1, 0 no dungeon) and dungeon object
	initH   uint8
	room    [2]uint8
	dObj    [2]Obj
	retMiss bool            // the last OpReturnEx returned nothing (its StaticEffect rider then applies to nothing)
	retAt   uint8           // the remembered object a many-object return is moving, + 1 (0 = none; returnrem.go)
	retTS   uint16          // the entry stamp a many-object return started at: its entrants are the ones after it
	spent   [2]uint16       // the history bank (value_turnhistory.go): mana each player spent casting spells this turn (expend)
	sacM    slotMask        // permanents picked for an effect's sacrifice, sacrificed together once all are picked
	sacOrd  [maxSacOrd]Slot // sacM's slots in sacrifice order (gorge zone_bulk.go effSacrifice: player by player, each in answer order)
	nsac    uint8
	// an Aura or Equipment whose host left stays attached to it, as gorge
	// shows it, until the next state-based check; lingerM is the live set
	linger   [maxLinger]lingerRec
	nlinger  uint8
	poison   [2]uint8 // each player's poison counters (CR 122.1f; infect and toxic, kw_infect.go; padding before lingerM)
	poolArt  [2]uint8 // per player, the pool slots holding mana an artifact produced (mana_artifact.go; padding)
	speed    uint8    // both seats' speed (3 bits each) and the once-per-turn speed trigger latch (bit 6; kw_speed.go; padding)
	noPoison uint8    // bit p: player p can't get additional poison counters this turn (Melira's replacement; trig_leavereturn.go; padding)
	ctrAdder uint8    // transient: the player putting the counters being placed, +1 (0: the resolving / building controller; trig_leavereturn.go; padding)
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
		// would take; sc holds the kop.PrevScope* bits and col is 0
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
	pickLeft  uint8                   // picks still owed
	pickP     uint8                   // the player picking
	pickZ     uint8                   // destination zone / flags of the pick op
	pickF     uint16                  // the pick op's flags
	winSrc    Slot                    // the tap-another source whose pick a payment window waits on (mana_windowtapx.go)
	winMode   uint8                   // the payment that window serves (winNone: none)
	winCol    uint8                   // the colour a cast window's paused tap-another source makes once the pick is answered (mana_window.go)
	winHeld   [kop.NumManaKinds]uint8 // the restricted mana that walk kept out of the pool so far
	nlook     uint8                   // cards looked at (dig / peek / reveal): the live prefix of armRes.look (scratch.go)
	attLKI    [2]attLKIRec            // the last departed attachments' hosts (CR 608.2h; fills the pad before evSel)
	evSel     uint64                  // ward evidence picks (bit per graveyard index; MaxDeck 60; kw_wardcost.go)
	endTurn   bool                    // OpEndTurn: skip to cleanup after this resolution
	probeWin  bool                    // convokeTargetAbort's probe window: a tap-another source is listed and never waits on a pick (padding)
	fog       bool                    // OpFog: all combat damage this turn is prevented (padding)
	beamPick  bool                    // offerBeamCasts prices a granted beam me up cast's return (kw_beammeup.go)
	altCast   bool                    // the cast beginCast starts is for the card's own alternative mana cost (castAltTgt; cast_altfree.go; padding)
	beamGrant bool                    // the beam me up cast beginCast starts is the granted instance of a card that prints the keyword (kw_beammeup.go; padding)
	impCast   bool                    // the cast beginCast starts is for the card's impending cost (castImpTgt; kw_impending.go; padding)
	noUntapL  uint8                   // OpSkipUntap C=1: bit p, player p's lands don't untap during p's next untap step (padding)
	yourTurns [2]uint8                // ExYourTurns: the turns each player began as the active player, saturating (prep.anyYourTurns only; padding)
	dayNight  uint8                   // CR 731: bits 0-1 the designation (dnDay, dnNight), bits 2-3 the last turn's active player's spells, at most 2 (kw_daynight.go; padding)
	links     [8]struct {
		src  Obj
		sver uint8
		obj  Obj
	} // exiled until the source leaves (OpExileUntilLeaves)
	nlinks   uint8
	nexw     uint8
	linkHide uint8 // bit i: links[i] is a Hideaway link (the card stays exiled face down; kw_hideaway.go; padding)
	linkAeg  uint8 // bit i: links[i] is an attachment whose copy of its exiled card is live (Assimilation Aegis; zone_exilethenattach.go; padding)
	linkHand uint8 // bit i: links[i] returns its card to its owner's hand, not the battlefield (a hand exile until the source leaves; act_mayask.go)

	// retHeld lists, in departure order, the sources that left in the open
	// departure frame with links to return (holdReturns); bit i of retCtrl is
	// retHeld[i]'s last controller. Each holds a link, so links bounds the list.
	retHeld  [8]Obj
	nretHeld uint8
	retCtrl  uint8
	// entExc: the copy exceptions of the token entering next (entExcOn | the
	// template's slot<<8 | the op's exception). Its entry's replacements read
	// the token with them (CR 707.9, 614.12).
	entExc   uint32
	enterCtr struct {
		n    uint8
		kind kop.CounterKind
		tap  bool
		ctrl bool
	} // counters/tapped for the next enter (OpReturnEx)
	lookSel uint32  // picked looked-at cards (bit per index; 32 bits hold look's 24; fills the pad before ro)
	ro      replOrd // a combat damage pass paused on gorge's replacement-order ask: the header (repl_gen.go); the body is scr's armCombat
	// scr: the mutually exclusive decision scratch, one region read through
	// typed accessors: the combat damage pass (armCombat), the build in
	// progress (build), a resolution's look / pileOrd / dmgSeqOrd (armRes).
	// The discriminants stay outside it (scratch.go).
	scr [scrWords]uint64
	nc  ncDmg // a noncombat damage event parked on that ask (ro.on false; repl_gen.go)
	// a multi-event damage op paused on one of those asks: its recipients are
	// frozen at the op in gorge's sweep order (effDamageAll: seat 0's
	// battlefield in entry order, then seat 1's; the players arm last, bit 0
	// the active player, bit 1 the other, as the op deals them), dmgSeqI the
	// next entry still to deal
	dmgSeqOn bool
	dmgSeqN  uint8
	dmgSeqI  uint8
	dmgSeqPl uint8
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
}

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
// its object cap as a "bigboard" stall).
func (s *State) nearCapacity() bool {
	return s.occ.count() >= MaxPerm-4 || int(s.nobj) >= MaxObj
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
	StallEmblem   = 10 // maxEmb emblems of one player
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
	StallReplPlus  = 22 // a damage plus-N replacement and another damage replacement match one event: gorge's CR 616.1 order ask, whose answer changes the amount (trig_oneormore.go)
	StallLifeRepl  = 25 // a life-gain doubler and a life-gain plus-N in one table: gorge's CR 616.1 order ask (they do not commute; act_discover.go)
	StallProlif    = 26 // a proliferate gorge orders differently: more than 16 picks, or a recipient with two counter kinds in a table with a counter-added trigger (act_proliferate.go)
	StallTapPick   = 27 // a tap-any-number cost (CostTapPower, Crew / Saddle) picked past build.picks' 24 (12 before kcaps2x; kw_crewsaddle.go)
	StallRegen     = 28 // a ninth regeneration shield on one permanent: pfRegenX plus the arena's mechRegen bit hold eight (4 before kcaps2x; repl_regenerate.go; kverify-c4)
	StallLook      = 29 // a dig window wider than genState.look's 24 (12 before kcaps2x; Ulamog's "exile the top twenty"; Hatchery Spider's undergrowth; selfcast): the window used to truncate silently
	StallCtrOrder  = 30 // an OpPutCountersFrom of two or more counter kinds in a table with a counter-added trigger: gorge fires per kind in the source's insertion order, the kernel walks kinds ascending (ctr_clear.go; eachfromsource)
	StallGrants    = 31 // more than maxGrants granted mana abilities on one permanent (mana_grantedoffers.go grantIdxs)
	StallShield    = 32 // a SHIELD counter and another damage replacement match one event: gorge asks the CR 616.1 order, the kernel stalls (ctr_shield.go)
	StallActs      = 33 // 127 non-mana activations of one object in a turn: object.acts keeps the count in seven bits beside the owner
	StallReplCtr   = 34 // a filtered counter doubler and a filtered plus/minus replacement match one event, or a plus and a minus replacement do: they do not commute (repl_counters.go)
	StallReplLeave = 35 // two or more replacements compete for one death (including two dies-to-exile sources or two turn-scoped exile marks on one creature): gorge asks the CR 616.1 order, the kernel stalls (repl_counters.go)
	StallLinger    = 36 // more than maxLinger attachments lose their host in one state-based pass (genState.linger; static_lingerattach.go)
	StallSac       = 37 // one effect sacrifices more than maxSacOrd permanents (genState.sacOrd)
	StallEntered   = 38 // more than maxEntered zone entries in one turn (genState.entered)
	StallConvoke   = 39 // more convoke picks than the cast build holds, or more convoke options than a decision holds (kw_convoke.go)
	StallCastRing  = 40 // a player pushes more than castRingN spells in one turn (State.cold.castRing)
	StallPT        = 41 // a power or toughness pump leaves the int16 end-of-turn range, or needs a fifth of the maxPtX slots (eotAdd, OpPumpAllEach; filter_defined.go)
	StallActRing   = 42 // a player activates more than actRingN abilities in one turn (genState.actRing)
	StallCharmOpts = 43 // a multi-pick charm's combined target ask has more than MaxOptions options
	StallCopyFace  = 44 // a permanent-spell copy of a multi-face card (CR 707.10g; copy_permanentspell.go)
	StallRefLatch  = 45 // a combat damage batch has more than maxRefLatch (listener, referent) per-referent damage triggers (trig_damagedonce.go refLatch)
	// 46 was a twobrid cost beside a cost reduction or increase; the cost statics now compose on the announced faces (mana_twobrid.go foldTb)
	StallZone          = 47  // a hand, graveyard, exile or library array is full: tokens that left the battlefield together wait there for the next state-based check (zones.go placeFrom)
	StallOptions       = 48  // a combat declaration has more attack or block pairs than MaxOptions (combat.go optionsFull)
	StallRoleMulti     = 49  // an AttachedTo$ token mint would create a number of tokens other than one (static_roletokens.go)
	StallSrcMark       = 50  // more than maxSrcMark DamagedBy / CrewedThisTurn sources marked permanents this turn (genState.marks; filter_combatwords.go)
	StallDefenderPW    = 51  // a TriggeredDefender damage whose attacker attacks a planeswalker or battle (or left): gorge damages the defending seat instead (OpAtkPWGuard; filter_combatwords.go)
	StallTokAtkAsk     = 52  // a TokenAttacking$ True mint while the opponent has a planeswalker or battle: gorge asks the CR 508.4 defender (filter_combatwords.go)
	StallDmgLatch      = 53  // a ninth DamageDealtOnce listener of one source in one combat damage batch (triggers_gen.go dmgLatch)
	StallReplCands     = 54  // a fifth competing replacement candidate for one event or untap (replOrd.cand; repl_gen.go, act_energy.go)
	StallEnterDefender = 55  // an Attacking$ True move onto the battlefield while the opponent has a planeswalker or battle: gorge asks the CR 508.4 defender (zone_changezoneparams.go)
	StallPrevCol       = 56  // a fourth Ext PreventColor record in one turn (genState.prevCol; cast_altfree.go)
	StallColGrant      = 57  // more than maxColG Ext ColorGrant records in one turn (genState.colG; static_colourgrant.go)
	StallRestrX        = 58  // a restricted colour past 7 mana beside another (State.poolRx; mana_reflected.go)
	StallEndureN       = 59  // an Endure Spirit mint a CreateToken replacement multiplies (act_endure.go)
	StallLinkCtr       = 60  // an until-leaves return rider's counters on a card that does not enter inline (a returning Aura asked its host, an as-enters choice asked first; zone_untilleaves.go)
	StallNumTimes      = 61  // an open Announce$ NumTimes (no AnnounceMax$) whose eighth repetition is still payable (cast_announcetimes.go)
	StallAegis         = 62  // an Assimilation Aegis copy that cannot be kept (a second exiled card, a bearer already a copy; zone_exilethenattach.go)
	StallSacOther      = 63  // a Mobilize group sacrifice whose member another player controls (gorge checks no controller; kw_mobilize.go)
	StallObsSrc        = 70  // a second observed mana source (Spell.ManaFromCard.StrictlySelf reader) tapped by one cast (mana_spendtrigger.go)
	StallCopyCast      = 88  // a Play CopyCard$ cast of a multi-face card, or a copy cast reversed after its push (CR 733.1; cast_copycast.go)
	StallDynMana       = 89  // a dynamic mana ability's amount above maxDynMana, its autopay variants (gen.go manaPCGen; value_gravecounts.go)
	StallExiledWith    = 96  // more than maxExw live exiled-with links (zone_exiledwith.go)
	StallBreach        = 94  // more picks than the chosen set holds (Breach the Multiverse; act_millthenreturn.go) (reinstate maxDynMana 40 + loud StallDynMana on the rebased tree (The Core's dynamic amount reached 21 in the gravecounts lockstep: gorge produced 21, kernel truncated to 20); NumStall 128 untouched, the name registers cold-side)
	StallNetLock       = 85  // a ninth permanent locked by Braided Net at once (nlA, nlB, nlC; act_lockpermanent.go)
	StallNetMana       = 86  // a Braided Net lock on a card with a mana ability that does not tap it (act_lockpermanent.go)
	StallCharmPick     = 99  // a seventh restricted charm source holds a pick log at once (ChoiceRestriction$; modal_charmpick.go)
	NumStall           = 144 // capacity of the reason tables (StallName); a reason registers its name in its own file
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
		if k.gen.flags[c]&gfManaGate != 0 {
			return s.manaGateView(k, sl, c, vi)
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
// larger amount stalls StallDynMana. 40 (a whole 40-card deck: The Core's
// permanent cards in your graveyard reached 21).
const maxDynMana = 40

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
				if p.r.fl&ovReps == ovReps || grants && s.newerLayer6Fx(sl, i) {
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
// opens with (akColor, akType, akPayLife), or 0.
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
	for i := i0; i < int(s.nobj); i++ {
		ob := &s.objs[i]
		if ob.of&(ofToken|ofCopy) == 0 {
			continue
		}
		switch ob.zone {
		case kop.ZoneLibrary, kop.ZoneHand, kop.ZoneGraveyard, kop.ZoneExile:
			if ob.zone == kop.ZoneExile && ob.of&ofCopy != 0 {
				if k := s.k(); k.gen != nil && k.gen.anyPrepare { // CR 722.3c: a linked prepared copy waits in exile
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
