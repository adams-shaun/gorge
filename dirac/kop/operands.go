package kop

// C of an ExtAddPhase: the extra phase's kind (ExtraPhase$ Combat,
// Upkeep, End of Turn) in bits 0-1, its splice point (AfterPhase$, the LAST
// step of the named phase, a Step) in bits 4-7. The omitted AfterPhase$ (the
// step the grant resolves in) is not lowered: its carrier, Full Throttle,
// stays refused for its Effect.
const (
	AddPhaseCombat = 0 // BeginCombat..EndCombat
	AddPhaseUpkeep = 1 // Upkeep alone
	AddPhaseEnd    = 2 // End alone
)

// AtEOT$ on a ChangeZoneAll sweep onto the battlefield.
// OpChangeZoneAllEx with MvEntryRiders carries A = counter count | kind<<8
// (kgen's riders). The counter kind stays below EntryKindMask+1, so A's top
// two bits are free: EntryAtEOT registers one next-end-step promise per
// entrant (gorge scheduleAtEOT over the moved set) and EntryAtEOTExile makes
// it an exile rather than a sacrifice.
const (
	EntryKindMask   = 0x3f
	EntryAtEOT      = 1 << 15 // as a uint16 image of A.N; the int16 sign bit
	EntryAtEOTExile = 1 << 14
)

// a counter kind that outgrows EntryKindMask fails this build, not a sweep
var _ [EntryKindMask + 1 - int(NumCounterKinds2)]struct{}

// ExtCloneCopy's B.N flags.
const (
	CloneEOT       = 1 << 0 // Duration$ UntilEndOfTurn (the copy ends at cleanup)
	CloneExclTmpl  = 1 << 1 // NotDefinedTargeted: the template is not a becomer
	CloneTmplPick  = 1 << 2 // the template is the Choices$ pick (chosen[0]), else target slot (B.N>>CloneTmplShift)
	ClonePick      = 1 << 3 // the Choices$ pick itself (F the candidates)
	CloneTmplShift = 4      // bits 4-5: the template's target slot
	CloneBatch     = 1 << 6 // the Choices$ pool is the trigger's batch members in exile (with ClonePick and CloneTmplPick; the copy is of the picked card, not a permanent)
)

// BecomeCopyCtr is OpBecomeCopy's B.N bit (a different op from ExtCloneCopy,
// so its flag space is its own): the copied template's counters join the
// entering copy's counters as it enters (gorge's RememberCloneOrigin$ True +
// PutCounter EachFromSource$ Remembered rider, CR 707.9b/614.12). No other
// B.N bit is used by OpBecomeCopy.
const BecomeCopyCtr = 1 << 0

// MayGuardCloneCopy: the OpMayAsk guard of an Optional$ Clone: the next op is
// an ExtCloneCopy that has a live template and at least one becomer (gorge
// effClone's pairs, computed before the may-copy election).
const MayGuardCloneCopy = 8

// MayGuardStackSpell: the OpMayAsk guard of an optional move of a targeted
// spell off the stack (a ChangeZone Origin$ Stack Optional$ True; gorge
// changeZoneOptionalConfirm): the next op is an OpCounter whose Obj is still a
// spell on the stack, else nothing is asked and the move is skipped.
const MayGuardStackSpell = 9

// MayGuardPut: the OpMayAsk guard of an Optional$ PutCounter with
// RememberPut$ True (gorge putCounterOptionalDeclined): the next op is an
// OpPutCounter whose Defined target is live (a dead source has no slot), else
// nothing is asked and the counter is not put.
const MayGuardPut = 10

// MayGuardAnimateAll: the OpMayAsk guard of Animate over Defined$ Valid: the
// next op is an OpOverrideAll whose F has at least one battlefield hit (gorge
// animateWouldChange), else nothing is asked.
const MayGuardAnimateAll = 7

// LibThird is OpToLibrary's C for LibraryPosition$ 2: the object goes into
// its owner's library third from the top, or at the bottom when fewer than
// two other cards remain (gorge libraryOrderPlacementAt: the offset is
// min(2, the cards left); Braided Quipu).
const LibThird = 6

// an OpChooseMove of one card from a hand onto the
// battlefield whose filter may admit an Aura (CR 303.4f/g).
const (
	// SrAuraEntry (OpChooseMove B.N, read from the instruction, never ORed
	// into pickF): the picked card, when it is an Aura, enchants what its
	// controller chooses as it enters, stays in the hand with no legal
	// object or player (gorge auraEntryGate). B.N bit 3 is clear of
	// SrForgetChanged/SrHandGrave/UntilLeaves (0..2) and of the entry
	// counter kind (4..11).
	SrAuraEntry = 1 << 3
)

// LibChoiceSecond is OpToLibraryChoice's C for AlternativeDecider$ with
// LibraryPosition$ 1 (Temporal Cleansing, Lost Days): the owner's
// primary answer puts the object second from the top, min(1, the other
// cards) above it (gorge libraryOrderPlacementAt), not on the top.
const LibChoiceSecond = 1

// an effect's OptionalDecider$ (Draw, DealDamage) rides an
// OpMayAsk whose Obj, when set, names the deciding player. Appended after
// MayGuardLibraryTop (4).

// MayGuardDraw: the OpMayAsk guard of an optional draw: the next Draw's
// amount is above 0 and the drawing player's library holds a card (gorge
// effDraw draw.go:80-96 canDraw; an empty library asks nothing and draws
// nothing, :137-144: the kernel skips the Draw unasked).
const MayGuardDraw = 5

// MayGuardDamage: the OpMayAsk guard of an optional damage: the next
// DealDamage's amount is above 0 and its recipient is a player or a
// permanent still on the battlefield (gorge effDealDamage
// damage_deal.go:105, damageHasRecipient :32-43).
const MayGuardDamage = 6

// DurWhileSrcMay is an OpMayPlay duration (Effect
// Duration$ AsLongAsControl): the permission lasts for as long as the
// effect's source is on the battlefield (gorge continuousLive's default
// arm). The record's turn field holds the source: Obj | source ver<<8.
const DurWhileSrcMay = 11

// DurWhileValid is an OpMayPlay duration: the permission lasts for as long as the
// card stays in its zone and applies while the controller controls a permanent
// matching the instruction's filter (Effect MayPlay with CheckSVar$ over
// Count$Valid; Possibility Technician's Kavu). The record's turn field holds the
// filter.
const DurWhileValid = 12

// MvPermOrSpell (OpBounce C; the MvFromBattlefield bit, no new value): the
// object is a TgtPermOrSpell target, bounced from the battlefield or, a
// spell, moved off the stack to its owner's hand by the countered-spell move
// (a flashback-family cast is exiled; "can't be countered" does not stop it).
const MvPermOrSpell = MvFromBattlefield

// MayGuardSuspect: the OpMayAsk guard of a suspect election: some object the
// next ExtSuspect names would actually flip (gorge alterAttributeWouldChange),
// else nothing is asked.
const MayGuardSuspect = 3

// MayGuardLibraryTop: the OpMayAsk guard of Defined$ TopOfLibrary Optional$
// True: the controller's library holds a card (gorge moveDefinedLibraryObjects
// consumes an empty one with no ask, unlike a mill's mill_optional). Appended
// after MayGuardSuspect (3).
const MayGuardLibraryTop = 4

// the packed constants of the unread-params family's ops.

// OpMayAsk guards (C): when gorge poses its yes/no ask.
const (
	MayGuardMill   = iota // the next op's amount (A) is above 0 (gorge effMill: count > 0)
	MayGuardAttach        // the next op's Obj is a battlefield permanent other than the source (gorge effAttach: a legal Defined$ destination)
	MayGuardMove          // the next op's Obj is still the live battlefield source (gorge changeZoneOptionalConfirm asks per object the move loop keeps)
)

// Chooser is who picks for OpDiscard (C, low bits).
const (
	ChooseSelf   = iota // the discarding player chooses
	ChooseRandom        // at random
	ChooseOpp           // the discarding player's opponent chooses (reveal)
)

// Optional-ask bits (OpDiscard/OpSacrifice C, above the Chooser). The ask
// pauses the op; the cold optask code (act_optionalask.go) re-reads C to
// classify. OptAsk: "may" asks exist for this op (Optional$ True). OptAny:
// the pick may take any eligible count (AnyNumber$): Min 0, Max eligible,
// no yes/no election. OptAll: the pick takes the whole hand (Mode$ Hand);
// a decline discards nothing, an acceptance discards every card.
const (
	OptAsk int16 = 1 << 2 // 4
	OptAny       = 1 << 3 // 8
	OptAll       = 1 << 4 // 16
)

// Duration of an OpOverride / OpMayPlay.
const (
	DurEOT          = 0 // until end of turn
	DurPermanent    = 1 // for as long as the object stays in its zone
	DurYourNextTurn = 2 // until the end of the controller's next turn
	// appended for CR 613 layer 2 (OpGainControl / OpOverride)
	DurWhileSource        = 3  // for as long as the source stays on the battlefield (LoseControl$ LeavesPlay)
	DurWhileControlSource = 4  // for as long as the controller controls the source (LeavesPlay,LoseControl)
	DurUntilYourNextTurn  = 5  // until the controller's next turn begins (Duration$ UntilYourNextTurn: it ends at the preceding cleanup)
	DurWarp               = 6  // a warp exile's recast (CR 702.185a): for as long as the card remains exiled, and only on a later turn
	DurPlot               = 7  // the plotted designation (CR 702.170d): for as long as the card remains exiled, cast free at sorcery speed only on a later turn
	DurPrepared           = 8  // the prepared designation (CR 722.3a/c): obj/ver = the exile copy, turn = source Obj | source ver<<8; live while the source is on the battlefield with that ver
	DurDefeated           = 9  // a defeated battle in exile awaiting its owner's CR 310.11 transformed-cast ask: obj/ver = the exiled card, ctrl = its owner; consumed by the ask
	DurAirbend            = 10 // an airbent card (CR 701.65a): its owner may cast it from exile for {2} for as long as obj/ver stays exiled
)

// MoveFlags (OpExileEx C, OpReturnEx C>>3, OpChangeZoneAllEx C>>3).
const (
	MvRemember         = 1 << iota // the moved objects become the remembered set
	MvTapped                       // enters tapped
	MvUnderYourControl             // enters under the controller's control (else its owner's)
	MvGraveExile                   // graveexile: the source zones are Graveyard then Exile (OpChangeZoneAllEx), not Filter.Zone
	MvTransformed                  // the card enters transformed (CR 712.14a)
	MvFromBattlefield              // the exile happens only if the object is still on the battlefield (ChangeZone Origin$ Battlefield)
	MvForgetOther                  // ForgetOtherRemembered$: resolve Obj (RefRemembered: the whole set), forget the remembered set, then move (MvRemember re-adds the moved objects)
	MvLKI                          // RememberLKI$ on a targeted move (OpExileEx/OpBounce C): the remembered target is read with the controller its LKI snapshot holds (kernel g.tli bit tliRemLKI)
	MvMayMill                      // OpMill C: Optional$, each acting player answers yes/no before milling (gorge mill_optional)
	MvAttacking                    // OpReturnEx C>>3: the permanent enters attacking (Attacking$ True; CR 508.4)
	MvAlsoSelf                     // OpReturnEx C>>3: ThisDefinedAndTgts$ Self, the source returns too, after the target, as one entry batch
	MvForgetChanged                // ForgetChanged$ True (OpReturnEx C>>3, OpExileEx C, OpBounce C): each object that moved leaves the resolution's remembered set, order kept (gorge eventForgetChanged); bit 11 is the last positive bit of OpReturnEx's int16 C>>3
	MvEntryRiders                  // OpChangeZoneAllEx C>>3, onto the battlefield: each entrant gets A.N&255 counters of kind A.N>>8 (A.N 0 = none) and override uint16(B.N)-1 (B.N 0 = none) before its enters checks fire (WithCountersType$ / StaticEffect$; CR 122.6, 603.6b); bit 12 (ChangeZoneAllEx's C sign bit, kept readable by the arithmetic C>>3 shift; no other op reads it)
	MvFromGraveyard    = 1 << 13   // OpExile/OpExileEx C: a trigger object's graveyard-origin exile moves only if the card is still in the graveyard (bits 11 and 12 belong to the shifted OpChangeZoneAllEx/OpReturnEx space, so this unshifted flag takes 13)
)

// SurveilRememberKept (OpSurveil C): the cards the surveil leaves on top of the
// library join the remembered set (RememberKept$), beside MvRemember's cards put
// into the graveyard.
const SurveilRememberKept = 1 << 1

// DrawRememberAll (OpDraw C): RememberDrawn$ AllReplaced records every card
// the draw and its replacement body moved into the drawing player's hand,
// not only the pre-draw top card (gorge recordDrawn; CR 121.6c, 614.1).
const DrawRememberAll = 1 << 1

// SearchFlags (OpSearchEx / OpChooseMove C>>3).
const (
	SrTapped           = 1 << iota // the found cards enter tapped
	SrThenTop                      // shuffle, then put the found card on top
	SrMandatory                    // must find a card if able
	SrShuffleOptional              // retired: kgen never sets it; the bit stays reserved
	SrConfirm                      // Optional$ confirmation (yes/no) before the pick: OpChooseMove from a hand, or an OpSearchEx library search (the fetching player answers)
	SrRemember                     // the moved cards join the remembered set (RememberChanged$; a search onto the battlefield or into a hand, or an OpChooseMove)
	SrChooserYou                   // OpChooseMove from player Obj's hand: the resolving controller chooses (gorge Chooser$ You), not the hand's owner
	SrReveal                       // OpChooseMove from a hand to zone none: a reveal pick (gorge effReveal's reveal_pick), nothing moves; SrConfirm then asks only when a card qualifies
	SrOwnZone                      // OpChooseMove over the battlefield: only player Obj's permanents (gorge effHiddenPick with DefinedPlayer$), not every player's
	SrGraveExile                   // OpChooseMove (graveexile): the pool is every player's Graveyard then Exile (gorge effHiddenPick gameWide); Filter.Zone is ZoneNone
	SrForgetOther                  // OpChooseMove (graveexile): ForgetOtherRemembered$ clears the remembered set before the moves (and on an empty pool)
	SrUnderYourControl             // OpChooseMove (GainControl$): the picked cards enter under the fetching controller, else (SrGraveExile) under their owner; a plain Graveyard pick with it is game-wide
)

// Search flags carried in Instr.B (a VConst) of OpSearchEx / OpChooseMove,
// not in C: C>>3 has no room left (bit 11 is the last positive bit of the
// int16). startSearch ORs them into the pick's flags (genState.pickF).
const (
	SrAttacking      = 1 << 12 // a card put onto the battlefield enters attacking (Attacking$ True; the hand pick of Kinscaer Sentry)
	SrDifferentNames = 1 << 13 // DifferentNames$ True: no two found cards share a name, and the count is capped at the distinct names (gorge zone_search.go Group = name)
	SrRandom         = 1 << 15 // OpChooseMove from a graveyard (AtRandom$ True): the engine rng picks the card(s), nobody is asked (Tersa Lightshatter)
	SrForgetChanged  = 1 << 1  // ForgetChanged$ True on a library search (OpSearchEx): each found card that moved leaves the remembered set (gorge zone_search eventForgetChanged). B bit 1 is read from the instruction (moveFound / pickLeaves), never ORed into pickF; bit 0 is the UntilLeaves hand-exile link
	SrHandGrave      = 1 << 2  // OpChooseMove: the pool is the fetching player's Hand then Graveyard (gorge effSearchLibrary over Origin$ Hand,Graveyard); read from the instruction's B, never ORed into pickF. B bits 4..11 hold the entry counter kind+1 (WithCountersType$, amount 1; 0 = none)
	SrGameGrave      = 1 << 3  // OpChooseMove from a graveyard with no fetch player: the pool is every player's graveyard in seat order (gorge effHiddenPick gameWide), not player Obj's; picked cards enter their owner's zone (gorge zoneOwner). B bit 3 is free on an OpChooseMove (bits 4..11 are the entry counter kind, and this bit is read in startSearch, not by a move scan)
	SrNoShuffle      = 1 << 14 // Shuffle$ False: the library is not shuffled after the moves; a library destination puts the found cards on top in pick order, remembered with SrRemember (Flourishing Bloom-Kin)
	SrImprint        = 1 << 8  // OpChooseMove from the Library (ChangeZone Imprint$ True under a DigUntil RememberRevealed$): each moved card joins the chosen set (imprint). Read from the instruction's B, never ORed into pickF (bit 8 there is SrOwnZone); bits 4..11 are the graveyard-pick counter kind, which a library pick never reads
	SrShareLandType  = 1 << 9  // OpSearchEx only: ShareLandType$ True -- every found card shares a land type with the others, so a later pick is offered only the cards sharing a printed subtype with every earlier one (gorge rules.validateSearch through effects.SharedLandTypes; kgen lowers it only over a basic-land filter). Read from the instruction's B, never ORed into pickF
	SrAltOrigin      = 1 << 4  // OpSearchEx only (bits 4..11 are OpChooseMove's entry counter kind, unread on a library search): OriginAlternative$ -- the searcher first elects which of the library and the alternative zone to search (gorge originAlternativeZones), the pool is the elected zones' cards (library first), the shuffle is owed only when the library was elected. B bits 5..7 (SrAltZoneShift) hold the alternative zone (Graveyard or Hand); SrAltGraveHand there names the Graveyard,Hand pair, so the election offers three zones; read from the instruction, never ORed into pickF
)

// SrAltZoneShift is the shift of SrAltOrigin's alternative zone in B.N.
const SrAltZoneShift = 5

// SrAltGraveHand is the SrAltOrigin zone code for the OriginAlternative$
// pair Graveyard then Hand (Agency Outfitter). The library is never an
// alternative zone, so its code is free to name the pair; the election then
// offers the library, the graveyard and the hand.
const SrAltGraveHand = int(ZoneLibrary)

// RetAttachTo (OpReturnEx B.N, A counters absent):
// AttachedTo$ -- the returned permanent enters attached to the permanent the
// Ref in B.N bits 8..10 names (RefSelf, or a target slot; the low byte stays
// clear, as RetFromLibrary's does, so the counter-kind prep scans read kind 0). An Aura whose named host
// is no longer a legal object to enchant stays in its zone (CR 303.4g, gorge
// markChangeZoneAttach / auraEntryGate); an Equipment enters, then attaches
// when the host is still a creature on the battlefield (changeZoneAttachedTo).
// With the instruction's F set the bearer is a filter, not a Ref (gorge,
// One Last Job's Creature.YouCtrl): the returned permanent's
// controller chooses among the legal permanents F matches (an Aura's also bound
// by its Enchant restriction), with no ask for exactly one.
const RetAttachTo = 1 << 11

// RetAttachShift is the shift of RetAttachTo's bearer Ref in B.N.
const RetAttachShift = 8

// OpReturnEx B.N flag (A counters absent): Origin$ Library, the move takes a
// library card only (every other OpReturnEx skips a library object), then
// the owner's library is shuffled when the script owes it (RetShuffleOwed).
const RetFromLibrary = 1 << 14

// BncFromLibrary is the OpBounce C flag (above the MoveFlags bits a Bounce
// reads) for Origin$ Library: a Defined$ library fetch takes a library card
// only, and a fetch that moved a card shuffles its owner's library when the
// script owes it (BncShuffleOwed; gorge zone_search.go
// moveDefinedLibraryObjects).
const BncFromLibrary = 1 << 12

// BncShuffleOwed / RetShuffleOwed (gorge zone_search.go
// moveDefinedLibraryObjects): a Defined$ library fetch is not a search, so
// it shuffles its owner's library only when the script owes one (an
// explicit Shuffle$ True, or a preceding library search that deferred its
// shuffle with Shuffle$ False / NoShuffle$ True). kgen sets the bit
// statically; without it the fetch moves the card and shuffles nothing.
// BncShuffleOwed is OpBounce's C, RetShuffleOwed OpReturnEx's B.N.
const (
	BncShuffleOwed = 1 << 14
	// BncDelayedMany marks a delayed program's body (Defined$
	// DelayTriggerRememberedLKI, Origin$ Battlefield) as the many-object
	// return: every object the firing entry names that is still on the
	// battlefield moves to its owner's hand (trig_delayedmany.go; DpMany).
	BncDelayedMany = 1 << 13
	RetShuffleOwed = 1 << 13
)

// RetCounterPairShift: an OpReturnEx whose WithCountersType$ names two kinds
// (Perennation) carries its constant A as count | (second kind + 1)<<8 (read
// as unsigned) and the first kind in B.N, as for one kind. Both enter with the
// permanent, each of count, first kind first (CR 122.6, 614.1c).
const RetCounterPairShift = 8

// RetUnderOpponent is OpReturnEx's B.N flag for GainControl$ ChosenPlayer
// after a one-opponent ChoosePlayer (Zuko, Conflicted: "return it to the
// battlefield under an opponent's control"; CR 110.2a): the returned object
// enters under the resolving controller's opponent. Never with entry
// counters (A>0 reads B.N as a counter kind).
const RetUnderOpponent = 1 << 12

// DigFlags (OpDigEx C>>3): bits 0..2 the rest zone, then flags.
const (
	DigRestShift    = 0       // the rest go to zone (C>>3)&7 (ZoneLibrary: bottom unless DigRestTop)
	DigOptional     = 1 << 3  // the player may take fewer (none)
	DigRandomOrder  = 1 << 4  // the rest go in a random order
	DigReveal       = 1 << 5  // the taken cards are revealed
	DigRemember     = 1 << 6  // the moved cards become the remembered set
	DigTakeToTop    = 1 << 7  // the taken card goes on top of the library (zone Library)
	DigRestTop      = 1 << 8  // the rest go on top
	DigRevealToCtrl = 1 << 9  // ForceRevealToController (no rules effect in 2-player)
	DigTapped       = 1 << 10 // a card taken onto the battlefield enters tapped (Tapped$)
	// DigFaceDown: a taken card put into exile is exiled face down and the
	// resolving controller may look at it (ExileFaceDown$ True with
	// WithMayLook$ True; gorge applyMoveFaceDown, zone_change.go:1286-1335,
	// and the fold in apply_zone.go:222-240). C is int16: the zone takes bits
	// 0-2 and the flags shift by 3, so bit 11 here is bit 14 of C.
	DigFaceDown = 1 << 11
	// DigExilerOther (bit 15 of C, the sign bit; every reader masks the
	// shifted flags): the cards exiled face down are exiled by the
	// controller's opponent (DefinedExiler$ Targeted over Defined$ You), so
	// the exiler's seat, not the controller's, is the card's looker. A blind
	// face-down exile (no WithMayLook$) rides DigFaceDown with this bit or
	// without it for the controller as exiler (Vulcan Mind-Meld).
	DigExilerOther = 1 << 12
)

// DigUntilFlags (OpDigUntil C>>3): bits 0..2 the revealed rest's zone.
const (
	DuRestBottom        = 1 << 3 // the revealed rest go to the library bottom
	DuRandomOrder       = 1 << 4 // ... in a random order
	DuFoundWithRevealed = 1 << 5 // the found card goes with the revealed rest
	DuFoundTapped       = 1 << 6 // a found card put onto the battlefield enters tapped (Tapped$)
	DuRememberFound     = 1 << 7 // the found cards replace the remembered set (RememberFound$)
	DuDugMark           = 1 << 8 // mark where the resolving controller's exile starts the dug cards (OpDiscover C=DiscoverDugBottom; Jodah)
	DuFoundAttacking    = 1 << 9 // a found card put onto the battlefield enters attacking (Attacking$ True; gorge effects/diguntil.go:176-184)
	// DuRememberRevealed (A=3): digRememberRevealed appends every scanned
	// card — found and revealed, in scan order — to the remembered set, per
	// player (gorge RememberRevealed$ True: gorge appends the whole reveal at
	// the end of the player's walk; the scan order is the same list).
	DuRememberRevealed = 1 << 10
	// DuMinTotalCMC: the scan reads its bound from B (A$ MinTotalCMC) and
	// stops once the revealed matches' mana values sum to at least that
	// bound (gorge digUntil's MinTotalCMC$; the amount bound is ignored
	// then, and a card beyond want still gets found).
	DuMinTotalCMC = 1 << 11
	// DuImprintFound (A=4): digImprintFound dedupe-appends the found cards to
	// the chosen set (gorge ImprintFound$ True emits one seek-found Imprint
	// on the source after the player walk; the kernel accumulates per
	// player, which only the later Effect reads).
	DuImprintFound = 1 << 12
)

// CopyFlags (OpCopyAsTokenEx B.N). The copy's characteristic exceptions
// (CR 707.9b: set P/T, added keywords, an added subtype) are an exception
// override in Instr.C, not flags; CpHaste, CpOneOne and CpFlying are
// retired (no table emits them) and keep their
// bits only because the enum is append-only.
const (
	CpHaste      = 1 << iota // retired: an exception override's AddKw
	CpSacAtEnd               // sacrifice it at the beginning of the next end step
	CpRemember               // the tokens become the remembered set
	CpOneOne                 // retired: an exception override's set P/T (Offspring)
	CpExileAtEnd             // exile it at the beginning of the next end step (AtEOT$ Exile: Stormsplitter)
	CpFlying                 // retired: an exception override's AddKw (Mockingbird)
	// CpSacAtEndDly: sacrifice it at the beginning of the next end step
	// through an untracked one-shot delayed registration (AtEOT$ Sacrifice,
	// gorge scheduleAtEOT's __kwEncoreSacrifice: the trigger is still put on
	// the stack when the token already left, and does nothing; CR 603.7c;
	// Reflection of Kiki-Jiki). CpSacAtEnd is AtEOTTrig$, a
	// trigger the token itself carries.
	CpSacAtEndDly
	CpTapped // the token enters tapped (TokenTapped$ True; gorge copypermanent.go CopyTokenTapped)
)

// CopyPumpShift: OpCopyAsTokenEx B.N >> 8 (as uint16) names the keyword of
// PumpKeywords$: the log2 of its KwMask bit, plus one; 0 =
// none. Each created token gains it as a permanent keyword override record
// (a Table.Overrides record holding only that AddKw, which kgen interns),
// never as a copy exception, so a copy of the token lacks it.
const CopyPumpShift = 8

// CopyPumpEOT: OpCopyAsTokenEx B.N bit 14, beside a PumpKeywords$ keyword:
// the grant lasts until end of turn (PumpDuration$ EOT), not permanently.
const CopyPumpEOT = 1 << 14

// CopyNonRemCtrl: OpCopyAsTokenEx B.N bit 15, as a uint16 image (the int16
// sign bit): the tokens are created for the opponent of the copied
// remembered permanent's controller, its last-known controller once it left
// the battlefield (Controller$ NonRememberedController, two-player).
const CopyNonRemCtrl = 1 << 15

// TokenFlags (OpCreateTokenEx B.N).
const (
	TokTapped      = 1 << iota // created tapped
	TokRemember                // the created tokens are counted (ExTokensCreated)
	TokRememberObj             // each created token joins the resolution's remembered set (gorge RememberTokens$: appended to Ctx.Remembered)
	TokSacAtEnd                // each created token is sacrificed at the beginning of the next end step (AtEOT$ Sacrifice)
	TokExileAtEnd              // ... exiled (AtEOT$ Exile)
	TokAttach                  // the token enters attached to the object Ref (B.N >> 8): RefTarget0..2 or RefSelf; withheld when that object is not on the battlefield (CR 303.4g, gorge auraTokenWithheld; static_roletokens.go)
	TokAttacking               // the token enters attacking the controller's opponent (TokenAttacking$); TokAttackAsk: gorge's CR 508.4 ask when the opponent has a planeswalker or battle (the kernel stalls)
	TokAttackAsk               // with TokAttacking: TokenAttacking$ True (the defender is asked among two or more candidates)
)

// TokAttachDest is TokAttach's destination Ref in B.N (the high byte).
func TokAttachDest(n int16) Ref { return Ref(uint8(n >> 8)) }

// TokAttachN is TokAttach's B.N: flags low byte, destination Ref high byte.
func TokAttachN(r Ref) int16 { return int16(r) << 8 }

// TokAtEOTDestroy and TokAtEOTSacCombat: both AtEOT bits set (TokSacAtEnd
// and TokExileAtEnd): an extended rider, its kind in the high byte (never
// with TokAttach). Destroy: at the beginning of the next end step, destroy
// the token (AtEOT$ Destroy); SacCombat: at this turn's end of combat,
// sacrifice it (AtEOT$ SacrificeCombat).
const (
	TokAtEOTDestroy   = 1
	TokAtEOTSacCombat = 2
)

// LibTopShuffle is OpToLibrary's C for gorge's object-path Shuffle$ True:
// the object goes on top of its owner's library, which is then shuffled.
const LibTopShuffle = 4

// LibTopShuffleGrave is LibTopShuffle for a trigger object's Origin$ Graveyard
// move (Lembas' "its owner shuffles it into their library"): the card moves,
// and the library is shuffled, only if it is still in the graveyard. Exiled in
// response, it does neither (gorge's Origin$ precondition on the object path).
const LibTopShuffleGrave = 8

// LibTopShuffleSelf is OpToLibrary's C for LibTopShuffle with
// ThisDefinedAndTgts$ Self: the target, then the source, go on top of their
// owners' libraries (one departure batch); each owner's library is then
// shuffled once, in that order (Floodpits Drowner).
const LibTopShuffleSelf = 5

// LibBottomShuffle is OpToLibrary's C for LibTopShuffle with
// LibraryPosition$ -1: the object goes on the bottom of its owner's library,
// which is then shuffled (the placement is the shuffle's input, so it is
// not the top's). Origin$ Battlefield only: an object no longer on the
// battlefield does not move, but the library is still shuffled.
const LibBottomShuffle = 7

// LibBottomRememberGrave is OpToLibrary's C for a trigger object's
// Origin$ Graveyard move to the bottom of its owner's library with
// RememberChanged$ True (The Spot, Living Portal's "put him on the bottom of
// his owner's library. If you do, ..."): a card no longer in the graveyard
// does not move (gorge's Origin$ precondition), and a card that reached the
// library joins the remembered set.
const LibBottomRememberGrave = 9

// LibTopGrave and LibBottomGrave are OpToLibrary's C for a trigger object's
// Origin$ Graveyard move to the top (LibraryPosition$ absent or 0) or the
// bottom (-1) of its owner's library: a card no longer in the graveyard (put
// into a hand in response, so a new object, CR 400.7) does not move (gorge's
// Origin$ precondition on the object path). Fell Horseman.
const (
	LibTopGrave    = 10
	LibBottomGrave = 11
)

// Delayed program timings (OpDelayedProgram A; 0 = the next end step).
const (
	DelayUpkeep     = 1 // the next upkeep of any player (gorge Phase$ Upkeep, ValidPlayer$ Player)
	DelayYourUpkeep = 2 // the controller's next upkeep (ValidPlayer$ You)
	DelayEndCombat  = 3 // the next end of combat step (gorge Phase$ EndCombat, ValidPlayer$ Player)
	// DelaySpellCast is the controller's next cast of a spell matching the
	// latent ability's Trig.Filter this turn (gorge Mode$ SpellCast,
	// ThisTurn$ True); one-shot
	DelaySpellCast = 4
	// DelayDiesThisTurn: the remembered object is put into a graveyard from
	// the battlefield this turn (gorge ChangesZone + IsTriggerRemembered +
	// ThisTurn$; one-shot)
	DelayDiesThisTurn = 5
	// DelayBeginCombat: the next beginning of combat step this turn (gorge
	// Phase$ BeginCombat, ThisTurn$ True; one-shot, dropped at the turn change)
	DelayBeginCombat = 6
	// DelayYourEndStep: the controller's next end step (gorge Phase$ End of
	// Turn, ValidPlayer$ You; another player's end step leaves it pending)
	DelayYourEndStep = 7
)

// DpGroup is OpDelayedProgram B's group flag: the program remembers the
// resolution's whole remembered set (at most two objects), not one object
// (Mobilize's RememberChain$ False delayed trigger).
const DpGroup = 1

// DpTrack is OpDelayedProgram B's incarnation flag: the program belongs to
// the resolving source's current object only and is dropped, not run, when
// that object has left (gorge's tracked __kwDecayed registration, CR 400.7 /
// 702.147a; Tainted Adversary's token).
const DpTrack = 2

// DpMany is OpDelayedProgram B's many-object flag (a value, not a mask: 3
// also has the DpGroup and DpTrack bits): the program remembers the
// resolution's whole remembered set, up to four objects, and its body is
// the many-object return (BncDelayedMany). Random Encounter's milled
// creatures (ChangeZoneAll ForgetOtherRemembered$).
const DpMany = 3

// TrigEvent / Step values the generator uses that the core's enums do not name
// separately: ETB-of-self and dies-of-self use TrigETB / TrigDies with
// TfSelf.

// OpPeek's C bits (0 = the private look: a look_ack, then the looked-at
// cards become the remembered set).
const (
	PeekPublic    = 1 << iota // a public reveal: no look_ack (gorge effReveal's public PeekAndReveal)
	PeekRemember              // with PeekPublic: the revealed cards join the remembered set (RememberRevealed$)
	PeekMayReveal             // the library's owner may reveal the top card if it matches F (RevealOptional$ + RevealValid$ + RememberRevealed$): ask yes/no, a yes remembers it
)

// OpGainControl riders (B.N).
const (
	GcUntap    = 1 << iota // untap it (Untap$ True; a stun counter is removed instead)
	GcHasteEOT             // it gains haste until end of turn (AddKWs$ Haste on an until-end-of-turn steal)
	GcRemember             // the gained permanent joins the remembered set (RememberControlled$; gorge effGainControl)
)

// OpMayPlay B flags over the remembered set.
const (
	MayPlayExileOnMoved = 1  // the Effect ends on a remembered card's move: exact for one card only
	MayPlayNonLand      = 2  // remembered lands get no permission (Affected$ Card.IsRemembered+nonLand)
	MayPlayFree         = 4  // MayPlayWithoutManaCost$ True: the granted cast's mana cost is waived
	MayPlayAnyType      = 8  // MayPlayIgnoreType$ True: mana of any type can be spent to cast it
	MayPlayIgnoreColor  = 16 // MayPlayIgnoreColor$ True: any colour pays a coloured pip, {C} stays strict
	MayPlayLimitShift   = 5  // MayPlayLimit$ n rides the kind's high bits (1..7)
	MayPlayLimitMask    = 0xE0

	// MayPlayAdventure (Mosswood Dreadknight): the grant permits only
	// the Adventure face's cast (Affected$ Card.IsRemembered+Adventure). B
	// bit 8; the record keeps it as kind bit MayPlayAdvKind (bit 0, which the
	// ExileOnMoved/NonLand B bits never reach the record in).
	MayPlayAdventure = 256
	MayPlayAdvKind   = 1

	// MayPlayOwner (B bit 9): the permission is the remembered
	// card's owner's (MayPlayPlayer$ CardOwner), not the effect controller's;
	// a surcharge {B >> MayPlayRaiseShift} (1..7) is added to a spell the
	// owner casts that way when the owner is the controller's opponent
	// (RaiseCost$ Activator$ Opponent). Both live outside MayPlayKinds.
	MayPlayOwner      = 0x200
	MayPlayRaiseShift = 10

	// MayPlayInstSorc (Chandra, Hope's Beacon): remembered cards that
	// are neither an instant nor a sorcery get no permission (Affected$
	// Instant.IsRemembered,Sorcery.IsRemembered). B bit 13 (above MayPlayOwner's
	// bit 9 and the surcharge bits 10-12), grant-time only.
	MayPlayInstSorc = 0x2000

	// MayPlayOpponent (B bit 14): the permission is the effect controller's
	// opponent's (EffectOwner$ Targeted over an opponent target, Vulcan
	// Mind-Meld: the player who exiled each card may play it). Grant-time
	// only; the record's holder is that player.
	MayPlayOpponent = 0x4000

	// MayPlayPayLife (B bit 15, the int16 sign bit, so the constant is
	// negative; test it as B.N&MayPlayPayLife != 0): MayPlayAltManaCost$ PayLife<ConvertedManaCost>
	// (Inside Information): a spell cast through the permission pays life
	// equal to its mana value instead of its mana cost (CR 118.9; the cast's
	// other costs still apply). The record keeps it as MayPlayFree plus a
	// mark on its until byte (dirac cast_mayplaypaylife.go); it never
	// shares a grant with a cap.
	MayPlayPayLife = -0x8000

	// MayPlayKinds are the B flags a permission record keeps (mayPlay.kind).
	MayPlayKinds = MayPlayFree | MayPlayAnyType | MayPlayIgnoreColor | MayPlayLimitMask
)

// DiscoverDugBottom is OpDiscover's C for Jodah, the Unifier's tail
// (kernel act_discoverbottom.go): the cards a DuDugMark DigUntil exiled that
// are still in exile and not queued to be cast go to the library bottom in
// a random order (gorge ChangeZoneAll RandomOrder$: the exile scan order
// Fisher-Yates'd through the engine rng). High, clear of cascade's 2/3.
const DiscoverDugBottom = 240

// DiscoverRemBottom is OpDiscover's C for Cosmic Cube's tail (kernel
// act_discoverbottom.go remBottom): the remembered cards still in the
// resolving controller's library and not queued to be cast go to its bottom
// in a random order (gorge ChangeZoneAll RandomOrder$ over the library).
const DiscoverRemBottom = 241

// CzUntilLeaves is OpChangeZoneAllEx's B bit 0:
// Duration$ UntilHostLeavesPlay on a battlefield-to-exile sweep. Nothing
// moves when the source has left the battlefield; each exiled non-token
// permanent gets an until-host-leaves link (gorge recordExileReturnFor). B is
// otherwise the entry riders' override index, and an exile sweep carries none,
// so the bit is free there.
const CzUntilLeaves = 1

// CzBattleGrave is OpChangeZoneAllEx's B for a MvGraveExile sweep whose
// origin is Battlefield,Graveyard (Strategic Betrayal): the kernel scans the
// battlefield first, one simultaneous departure, then the graveyard alone.
// Gorge's effChangeZoneAll walks the origins zone-major, so the battlefield
// half precedes the graveyard half. Clear of CzUntilLeaves (bit 0) and of
// ZoneHand, which czallSecondZone reads as Graveyard,Hand.
const CzBattleGrave = 8

// RepeatEachSeatOrder is OpRepeatEachPlayer's C bit 1: the players run in
// seat order (seat 0 first) instead of the controller first (gorge
// tokenOwnersControllingAny walks AliveFrom(0)).
const RepeatEachSeatOrder = 2

// OpPumpAllEach's Obj bits: the amount read per creature from its own
// derived power / toughness (NumAtt$ Double / NumDef$ Double).
const (
	PumpEachPower = 1
	PumpEachTough = 2
)
