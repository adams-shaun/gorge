package kop

// Op is one primitive of the kernel's card-effect vocabulary. A card's
// resolving program (Ability.Prog) is a flat slice of Instr, each naming one
// Op with a fixed operand layout:
//
//	Obj  Ref       who/what the op acts on (a target slot, the source, a player selector, ...)
//	A, B ValueRef  amounts (constant | X | count-of-filter | last-damage | ...)
//	C    int16     an op-specific constant (keyword mask, counter kind, token card id, zone, skip count)
//	F    FilterID  a filter into Table.Filters for the *All / Search / choice ops (0 = none)
//
// The enum is APPEND-ONLY: the generator (dirac/kgen) and the hand-written
// tables encode ops by ordinal. The iota list below is frozen at OpCount; a
// new op is an explicit constant (see OpCount), with a
// one-line semantic comment in the same form.
// The interpreter (package dirac, interp.go) must handle every op below
// OpCount: an op it does not implement panics naming the op, never a silent
// no-op.
type Op uint8

const (
	OpNop Op = iota // does nothing (padding; a generator placeholder never emits it)

	// --- single-object / single-player primitives ---
	OpDealDamage       // Obj (object or player) is dealt A damage by the source; sets LastDamage
	OpDraw             // player Obj draws A cards
	OpDestroy          // destroy permanent Obj (indestructible / regeneration respected; C&1: it can't be regenerated; C&2: RememberDestroyed$, a destroyed target is remembered; C&4: RememberLKI$, a remembered read of the destroyed target takes its last-known controller)
	OpExile            // exile object Obj (from any zone; a token ceases to exist)
	OpBounce           // return object Obj to its owner's hand
	OpToLibrary        // put object Obj into its owner's library: C=0 top, C=1 bottom, C=2 shuffled in
	OpPumpUntilEOT     // creature Obj gets +A/+B until end of turn
	OpGrantKeyword     // creature Obj gains keywords C (KwMask low 16 bits; B = high 16 bits) until end of turn
	OpPutCounter       // put A counters of kind C on permanent Obj
	OpRemoveCounter    // remove A counters of kind C from permanent Obj
	OpCreateToken      // player Obj creates A tokens of card C (a token CardDef in the Table); B = TokenRider flags
	OpGainLife         // player Obj gains A life
	OpLoseLife         // player Obj loses A life
	OpDiscard          // player Obj discards A cards; C = Chooser (who picks: self/random/opponent); F = card filter; B.N&1: the discarded cards are remembered
	OpScry             // player Obj scries A
	OpSurveil          // player Obj surveils A
	OpMill             // player Obj mills A; C = MoveFlags (kgen: MvRemember adds the milled cards to the remembered set)
	OpFight            // creature Obj fights creature in Ref C (each deals damage equal to its power to the other); A == Const(1) (ExcessSVar$): records ExExcessDamage for the hit dealt to the C creature
	OpTap              // tap permanent Obj
	OpUntap            // untap permanent Obj
	OpSacrifice        // player Obj sacrifices A permanents matching F (the player chooses)
	OpCounter          // counter spell/ability Obj on the stack; C (CtrDest*): where the countered spell card goes instead of its graveyard
	OpAttach           // attach the source (Aura/Equipment) to permanent Obj; C&MvRemember: the attached source joins the remembered set (RememberAttached$)
	OpDig              // player Obj looks at the top A cards, puts B matching F into zone C (ZoneHand default), rest on the bottom
	OpSearch           // player Obj searches library for up to A cards matching F, puts them into zone C, shuffles
	OpCharm            // mode branch: if mode bit C was NOT chosen, skip the next A instructions
	OpDamageAll        // each creature (and player if B.N=1 via flags) matching F is dealt A damage by the source; B.N bit 2: and each opponent of the controller, bit 4: each permanent matching F, not only creatures
	OpDestroyAll       // destroy each permanent matching F; C&1: the ones put into a graveyard are remembered; C&2: they can't be regenerated
	OpPumpAll          // each creature matching F gets +A/+B until end of turn; keywords C granted too
	OpChangeZoneAll    // move every object matching F (zone in the filter) to zone C
	OpAnimate          // permanent Obj becomes an A/B creature with keywords C until end of turn (override record)
	OpGainControlEOT   // the source's controller gains control of Obj until end of turn (untap, haste)
	OpSetPT            // override record: Obj's base P/T become A/B (C=1: until EOT, 0: while source stays)
	OpSetTypes         // override record: Obj's type mask becomes C (A=1 adds rather than sets)
	OpLoseAbilities    // override record: Obj loses all abilities (C=1: until EOT)
	OpCopyAsToken      // player in Ref C creates A tokens that are copies of Obj (override record copy-of)
	OpDelayedTrigger   // register a delayed trigger: at the next end step (A=DelayWhen) do op C to Obj (DelayedAct)
	OpWard             // Ward A: counter the triggering spell/ability Obj unless its controller pays {A}
	OpIfKicked         // branch: if the spell was NOT kicked, skip the next A instructions
	OpChooseColor      // decision: controller chooses a colour; stored as the resolution's Chosen
	OpChooseType       // decision: controller chooses a creature type among the Table's subtypes; stored as Chosen
	OpChooseCard       // decision: player Obj chooses a card matching F (zone in the filter); stored as Chosen. C 4/5: the object-side Attach pick (the next AttachObj's destination narrows the candidates; no ask for none, or one when C = 4; C = 5 adds Done)
	OpGenericChoice    // decision: controller picks one of A branches; skip to the chosen branch (branch lengths in C-indexed table)
	OpRepeat           // run the next B instructions A times
	OpRepeatEach       // run the next B instructions once for each object matching F (Ref Each binds the current one)
	OpEndTurn          // end the turn (exile the stack, skip to cleanup)
	OpAddTurn          // player Obj takes an extra turn after this one
	OpAddMana          // player Obj adds A mana of colour mask C (C=ColorAny: one colour of the player's choice)
	OpReturn           // return object Obj (graveyard/exile) to zone C under its owner's control (ZoneBattlefield: reanimate); B.N, 1<<zone bits, the zones it may come from
	OpSkipIf           // branch: if condition C (Cond) with operand A is FALSE, skip the next B instructions
	OpBite             // creature Obj deals damage equal to its power to Ref C (one-sided fight)
	OpLoseGame         // player Obj loses the game
	OpShuffle          // player Obj shuffles their library
	OpPutOnBattlefield // put object Obj (any zone) onto the battlefield under the controller's control; C = EnterFlags
	OpExileUntilLeaves // exile Obj until the source leaves the battlefield (O-Ring); return handled by the kernel
	OpRegenerate       // the next time Obj would be destroyed this turn, regenerate it instead
	OpPreventDamage    // prevent the next A damage that would be dealt to Obj this turn
	OpGrantAbility     // Obj gains ability index C of the source's card until end of turn (B=1: permanently)
	OpProliferate      // the controller proliferates A times (CR 701.34a; gorge effProliferate): each time chooses any number of permanents and players with a counter (ChGenProlif, closed by Done) and gives each another counter of each kind it has, then fires TrigProliferate
	OpTransform        // Obj turns to its card's next face (gorge effSetState Mode$ Transform: FlipFace to (FaceIdx+1)%faces; same object, counters, damage, attachments and summoning sickness stay; gorge flips in any zone; dirac/act_transform.go)

	// --- appended by dirac/kgen for the FDN pool (operand layouts in table.go) ---
	OpSkipUnless        // if Table.Conds[C] is FALSE, skip the next A.N instructions
	OpBranch            // GenericChoice branch C: if branch C was NOT chosen, skip the next A.N instructions
	OpRepeatEachPlayer  // run the next A.N instructions once per player in Obj (C&1: controller first, gorge AliveFrom; C=0: APNAP); RefEach binds the player
	OpUnlessPaid        // player Obj may pay cost C&0xF (PayKind) of A (B.N: packed pips, F: filter); if paid skip the next C>>4 instructions
	OpIfPaid            // player Obj may pay cost C&0xF of A (B.N pips, F filter); if NOT paid skip the next C>>4 instructions
	OpDealDamageEx      // Obj is dealt A damage by the source; C = DamageFlags (track excess / exile if it dies this turn)
	OpDealDamageDivided // A damage divided as the controller chose at targeting among target slot Obj's objects
	OpResolveDamageMap  // damage-map end: the DealDamage instrs since the program start (or the last map end) were one simultaneous event
	OpExileEx           // exile Obj; C = MoveFlags (MvRemember); with A counters of kind B.N when A > 0
	OpReturnEx          // return Obj (graveyard/exile) to zone C&7; C>>3 = MoveFlags (tapped, under your control); A counters of kind B.N (B.N RetFromLibrary: Origin$ Library, a library card only; RetShuffleOwed: then its owner's shuffle); MvForgetChanged drops the moved object from the remembered set
	OpToLibraryChoice   // Obj's owner puts it on the top (C = LibChoiceSecond: second from the top) or the bottom of its library (the owner chooses)
	OpSearchEx          // player Obj searches their library for up to A cards matching F to zone C&7; C>>3 = SearchFlags; shuffles; B.N SrForgetChanged drops each moved card from the remembered set
	OpChooseMove        // player Obj chooses A cards matching F (F.Zone) and moves them to zone C&7 (C>>3 = SearchFlags: mandatory)
	OpChangeZoneAllEx   // move every object matching F (zone in the filter) to zone C&7; C>>3 = MoveFlags; Obj (a player) restricts to that player's cards; MvEntryRiders: each battlefield entrant enters with A.N&255 counters of kind (A.N>>8)&EntryKindMask and gets override uint16(B.N)-1; A bit 15 (EntryAtEOT, bit 14 EntryAtEOTExile) registers its next-end-step sacrifice (exile) promise (operands.go)
	OpDigEx             // player(s) Obj look at the top A cards; up to B (-1 all, -2 any number) matching F go to zone C&7; C>>3 = DigFlags (rest zone, order, optional, reveal, remember)
	OpDigUntil          // player(s) Obj reveal from the top until A cards matching F; found to zone C&7, the rest per C>>3 = DigUntilFlags
	OpPeek              // player Obj looks at the top A cards; they become the remembered set
	OpTwoPiles          // the controller separates the remembered cards into two piles; an opponent picks one; the chosen pile goes to zone C, the other to the graveyard; B.N = TpOppSeparates (the opponent separates, the controller picks)|TpUnchosenPileOrder (the unchosen pile in its own order)
	OpChooseEachType    // each player in Obj chooses one permanent they control of each permanent type; the chosen set is RefChosen
	OpSacrificeAll      // every permanent matching F is sacrificed by its controller
	OpCastFree          // the controller may cast any number of the cards in Obj (zone C) without paying their mana costs
	OpMayPlay           // the controller may play card Obj from zone C until A.N (Duration)
	OpCreateEmblem      // the controller gets Table.Emblems[C]
	OpOverride          // Obj gets Table.Overrides[C] for duration A.N (Duration); Obj may be RefRemembered for the object a ReturnEx just moved
	OpReflexiveTrigger  // "when you do": put this card's ability C on the stack A times (its targets are chosen then); Obj a target ref: its object and ver captured as the entry's trigObj/trigAmt (RememberObjects$ Targeted)
	OpDelayedProgram    // at the beginning of the next end step run this card's ability C with RefDelayedRemembered = Obj (as it is then)
	OpClearRemembered   // forget the resolution's remembered objects
	OpClearChosen       // forget the chosen card(s)
	OpDoubleCounters    // double the counters of kind C on each object in Obj (C = 255: every kind)
	OpRemoveCounterRem  // remove A counters of kind C from Obj; the number removed becomes the remembered count
	OpEnterTapped       // replacement program: the entering object enters tapped
	OpEnterWithCounters // replacement program: the entering object enters with A counters of kind C
	OpCopyAsTokenEx     // the controller creates A tokens that are copies of Obj; B.N = CopyFlags (the non-characteristic riders); C = the copy exceptions' Table.Overrides index+1 (0: none; CR 707.9b)
	OpCreateTokenEx     // player Obj creates A tokens of card C (pool token index, remapped per matchup); B.N = TokenFlags
	OpChooseBranch      // player Obj chooses one of A.N branches (the OpBranch blocks that follow); branch A.N (the fallback) runs when none can be chosen
	OpChooseTargets     // the controller chooses target slot C now (a TgfAtResolution slot: one object, or none for an up-to slot)
	OpRepeatWhile       // run the next B.N instructions at most A times, again only while Table.Conds[C] holds after a pass
	OpCopySpell         // copy the spell Obj (RefTriggerObj: the spell whose cast triggered this) A times; C&1: each copy's controller may choose new targets as it resolves (CR 707.10c)

	OpPutCounterAll    // each battlefield permanent matching F gets A counters of kind C (gorge effPutCounterAll; battlefield order, seat 0 first)
	OpUntapAll         // each tapped battlefield permanent matching F untaps (a stun counter is removed instead; gorge effUntapAll)
	OpDealDamageFrom   // creature Obj deals A damage to the object in Ref C (DamageSource$ with an amount other than its power)
	OpGiveGift         // the controller gives a gift (CR 702.174b): fires TrigGiveGift; the gift's effects follow as ordinary ops
	OpClassLevelUp     // the source Class becomes level A (CR 716.2a; gorge effClassLevelUp): its CtrLevel count is set to A-1 and a "becomes level" trigger (TrigCounterAddedOnce, N = CtrLevel) fires
	OpGainEnergy       // player(s) Obj get A energy counters (CR 122.1; gorge PutCounter CounterType$ ENERGY on a player)
	OpOverrideAll      // each battlefield permanent matching F gets Table.Overrides[C] for duration A.N (gorge effAnimateAll; seat 0 first, entry order)
	OpRollDie          // the controller rolls a die with A sides (one rng.IntN(A) draw); the result, die + B.N, is ExRollResult (gorge effRollDice, one die)
	OpBecomeCopy       // as-enters replacement program: the entering permanent enters as a copy of the battlefield permanent its controller chose as it entered (the clone ask over F's matches; CR 707.2, 614.1c); C>>1 = the copy exceptions' Table.Overrides index+1 (0: none; CR 707.9b); C&1: optional (it may enter as itself); A>0: the Table.Overrides index+1 of an ability grant applied to the copy from the entering card (AddAbilities$, Mercurial Pretender). Nothing without an answer (it entered without being cast).
	OpTapAll           // each untapped battlefield permanent matching F becomes tapped (gorge effTapAll; seat 0 first, entry order; Taps triggers fire per permanent)
	OpSeek             // player Obj puts A random cards matching F from their library into their hand (gorge effSeek: library order from the top, one rng IntN(eligible) draw per card, without replacement; moved after all are chosen)
	OpAmass            // player Obj amasses A (gorge effAmass): their first battlefield permanent with the printed Army subtype, or if none each Army token of card C created (pool token index, remapped per matchup), gets A +1/+1 counters and Table.Overrides[B.N&0xfff - 1] permanently (the "also a <Type>"); B.N bit 15: RememberAmass$ (the amassing Army is remembered)
	OpConnive          // permanent Obj connives A (CR 701.50a; gorge effConnive): on the battlefield, its controller draws A then discards A (asked when the hand holds more); the next op, OpConniveCounters, puts a +1/+1 counter on it per nonland card discarded
	OpConniveCounters  // the connive's counters (after OpConnive's discard): one +1/+1 counter on Obj per nonland card that left its controller's hand
	OpGainControl      // the new controller (Ref C; RefNone = the source's controller) gains control of Obj for duration A.N (DurEOT / DurPermanent / DurYourNextTurn / DurWhileSource / DurWhileControlSource): a layer-2 control record (CR 613.1b, 613.7); B.N = GcUntap|GcHasteEOT riders
	OpSkipNextUntap    // permanent Obj doesn't untap during its controller's next untap step (CR 611.2b; gorge CantUntapNextStep: a one-shot consumed at that step)
	OpAtEOT            // permanent Obj (on the battlefield) is sacrificed (C=0) or exiled (C=1) at the beginning of the next end step (an AtEOT$ rider: gorge scheduleAtEOT)
	OpAttachObj        // the battlefield permanent in Ref C becomes attached to the battlefield permanent Obj (gorge effAttach Object$: nothing when either is gone or they are the same; legality is the attachment SBAs')
	OpTapOrUntap       // the controller chooses to tap or untap the battlefield permanent Obj (gorge effTapOrUntap; asked only while it is on the battlefield; the untap removes a stun counter instead)
	OpExplore          // permanent Obj explores (CR 701.44a; gorge effExplore): its controller reveals their library's top card; a land goes to their hand; otherwise a +1/+1 counter on Obj, then the controller chooses the card into the graveyard or back on top. Nothing when Obj left the battlefield or the library is empty
	OpPoison           // player(s) Obj get A poison counters (CR 122.1f; gorge effPoison: one PlayerCounterChange POISON per player)
	OpFog              // all combat damage that would be dealt this turn is prevented (CR 615; gorge effFog: the combat damage passes deal nothing this turn)
	OpSetLife          // each player in Obj (RefEachPlayer: controller first) has their life total become A, read once first (CR 119.5; gorge effSetLife: one life change of the difference, a gain or a loss)
	OpSkipUntap        // a "doesn't untap during your next untap step" effect the controller makes (gorge Effect Duration$ UntilTheEndOfYourNextUntap, an Untap replacement): C=0 the permanent Obj, C=1 the lands the controller controls; it ends with the controller's next untap step
	OpDiscover         // player Obj discovers A (CR 701.57a; gorge effDiscover): C=0 exiles their library's cards from the top until a nonland card with mana value A or less, puts the others on the bottom in a random order drawn from the engine rng (gorge's Fisher-Yates), and makes the found card the remembered set (the following OpCastFree offers it); C=1 (the tail, gorge effDiscoverBottom) puts the remembered card still in exile into its owner's hand
	OpMoveCounter      // move A counters of kind B.N (A < 0: all of them) from the battlefield permanent in Ref C onto the battlefield permanent Obj (CR 122.6; gorge effMoveCounter: one removal, then one counter change on the destination, its counter replacements applying); nothing when either is gone or they are the same
	OpRearrange        // the controller looks at the top A cards of player Obj's library and puts them back in any order (gorge effRearrangeTopOfLibrary: one KArrange ordering every card; ChArrange picks, the first the new top, no Done); C&1 (MayShuffle$): then the controller is asked whether that library is shuffled (ChGenConfirm yes/no)
	OpBolster          // the controller bolsters A (CR 701.39a; gorge putCounterBolster): A counters of kind C on a creature they control with the least current toughness (a tie: ChGenBolster ask; none or A <= 0: nothing)
	OpSaddle           // permanent Obj becomes saddled until end of turn (CR 702.171b; gorge effAlterAttribute "Saddled": a battlefield designation, gone at the next turn or when it leaves)
	OpClearCounters    // every counter of every kind is removed from battlefield permanent Obj (gorge effRemoveCounter CounterType$ All CounterNum$ All: one removal per kind present; a Class level is not a counter)
	OpRemoveCounterAll // each battlefield permanent matching F loses A counters of kind C, or (B.N&1, AllCounters$) every counter of that kind it has (gorge effRemoveCounterAll; battlefield order, seat 0 first)
	OpPutCountersFrom  // battlefield permanent Obj gets, for each counter kind the object in Ref C had, that count times A counters of the kind (gorge putCounterEachFromSource, CounterType$ EachFromSource: the source's live positive counters, else the resolving entry's LKI snapshot of it)
	OpSupport          // the controller supports A (CR 701.41a; gorge putCounterSupport): one counter of kind C&0xFF on each of up to A creatures (C&0x100: other than the source), an any-number pick (ChGenBolster, gorge's "counter_pick", closed by Done or the A-th pick); none eligible or A <= 0: nothing
	OpUnlessSwitched   // player Obj may pay cost C&0xF of A (B.N pips, F filter); if NOT paid skip the next C>>4 instructions; always an unless choice (ChGenPayMode), never a trigger's own cost
	OpMonstrosity      // battlefield permanent Obj becomes monstrous (CR 701.37): if it is not already, put A +1/+1 counters on it through OpPutCounter's path, set its monstrous bit and fire TrigBecomeMonstrous with amount A; A <= 0 does nothing (gorge effPutCounter's MonstrositySet arm)
	OpChangeSpeed      // each player in Obj changes speed (CR 702.179; gorge effChangeSpeed): C=1 +1 (from 0 it starts engines: B.N=1 only when speed is 0), C=2 -1 (never below 1; B.N=2: only if the player's speed is greater than every other player's); the 0..4 bounds hold
	OpWarpExile        // the delayed warp trigger (CR 702.185a): exile battlefield permanent Obj (no LKI rider); when it reached exile this way its owner gets a DurWarp recast record (dirac/kw_warp.go)
	OpMayShuffleMove   // player Obj (the effect's controller; RefNone when A.N=0) may (A.N=1; else must) put the legal chosen cards of target slot C on top of their owners' libraries in pick order, then shuffles each distinct owner's library once; no chosen card: nothing, and no ask (gorge objectPathMayShuffle + objectPathShuffleOwners, CR 701.24c)
	OpEmpower          // player Obj empowers A (CR 701.71a; gorge effEmpower): one Jace token they control (Table.Overrides[B.N&0xfff - 1] grants the Type$ subtype and the "<Type$> Token" name) gets A loyalty counters; with no such token, create each token of card C (pool token index, remapped per matchup), grant it that override, then put the counters on the minted token (a second mint asks); A <= 0 still creates the token but places nothing
	OpIncubate         // player Obj incubates A (CR 701.53a-b; gorge effIncubate): B times (min 1) create the Incubator token C under Obj (token doublers apply) and put A +1/+1 counters on each mint; A < 0 creates nothing, A = 0 mints with no counter event
	OpEarthbend        // earthbend A (CR 701.66; gorge effEarthbend): if battlefield permanent Obj is still there, it becomes a 0/0 Creature with haste for as long as it stays (override C, DurPermanent), gets A +1/+1 counters (A <= 0: none) and a leave-keyed delayed record (State.dly, when = dlyLeave)
	OpEarthbendReturn  // the earthbend return trigger (internal, never in a Prog): when the land goes to a graveyard or exile, return it from either zone to the battlefield tapped under its owner's control
	OpManifest         // player Obj manifests A cards from the top of their library (CR 701.40a; gorge effManifest): each is the real card moved face down, provenance marked manifested; B.N = CfManifestRemembered (manifest each remembered card instead; Obj/A unused)
	OpCloak            // player Obj cloaks A cards from the top of their library (CR 701.58a; gorge effCloak): face down with ward {2}, provenance marked cloaked; B.N = CfTapped|CfShuffle (no-op)|CfRemember|CfChosen|CfCloakCtrl|CfCloakRemembered (cloak each remembered card)
	OpManifestDread    // player Obj manifests dread A times (CR 701.62a; gorge effManifestDread): look at the top two, manifest one, the other to the graveyard; B.N = CfRemember

	// A: Defined$ Valid <filter> mass subjects.

	// OpPumpAllEach: each creature matching F gets +A/+B until end of turn
	// and keywords C (gorge effPump over Defined$ Valid <filter>, the set read
	// once at resolution, CR 611.2c). Obj&PumpEachPower: A is the creature's
	// OWN power instead; Obj&PumpEachTough: B its own toughness (NumAtt$ /
	// NumDef$ Double, read per creature).
	OpPumpAllEach
	// OpDoubleCountersAll: OpDoubleCounters (kind C, 255 = every kind) on
	// each battlefield permanent matching F, in battlefield order (seat 0
	// first; gorge effMultiplyCounter over Defined$ Valid <filter>).
	OpDoubleCountersAll
	// OpPrepare: the battlefield permanent Obj becomes prepared (CR 722.3a;
	// gorge effAlterAttribute Prepared, events apply_player.go): if it is a
	// preparation front (FacePrepare, Face 0) and not already prepared, its
	// exile copy is minted at the end of its controller's exile (CR 722.3c,
	// gorge grantPreparedCopy). dirac/face_prepare.go. C=1: unprepare
	// (Activate$ False; gorge Amount -1): the permanent's prepared designation
	// is cleared and its copy unlinked (kernel face_prepare.go).
	OpPrepare
	OpDemonstrate // Demonstrate (CR 702.144a; gorge effDemonstrate): the controller of the spell Obj (RefTriggerObj) is asked yes/no; on yes they copy it and each opponent still in the game copies it, every copy keeping the original's targets (gorge's StackCopy); nothing when the spell left the stack

	OpNinjutsu // the K:Ninjutsu ability's resolution (CR 702.49a-b; gorge kwNinjutsu's ChangeZone): if the source card Obj is still in its owner's hand it enters the battlefield, then a Tap ("entered tapped"), then attacking the defender captured on the entry (trigObj) when that is still legal; never declared (no Attacks trigger); C=1: a mana-only hand ability's plain put-this-card-onto-the-battlefield (Talon Gates of Madara), no tap and no attack

	// OpAtkPWGuard: the Attacks trigger's attacker (RefTriggerObj) must be
	// attacking a player: attacking a planeswalker or battle, or gone, the
	// game stalls at its next step (StallDefenderPW). gorge's
	// TriggeredDefender reads the defending seat only (its planeswalker
	// arm, Valid Planeswalker.TriggeredDefender, matches nothing), so the
	// kernel copies it only where it is right (Raid Bombardment).
	OpAtkPWGuard

	// OpMayAsk: the resolving controller is asked yes/no (gorge's yes/no
	// KChoose: mill_optional, attach_optional) whether to do the next
	// instruction; a no skips A instructions. C is the guard under which
	// gorge poses the ask (MayGuard*); when it fails nothing is asked and
	// the next instruction runs (and does nothing). dirac/act_mayask.go.
	OpMayAsk
	// OpToLibraryPair: battlefield permanent Obj (the target) and then the
	// source (ThisDefinedAndTgts$ Self) go to the top of their owners'
	// libraries, then each distinct owner's library is shuffled once, the
	// target's owner first (gorge's object path + objectPathShuffleOwners;
	// Floodpits Drowner). dirac/act_mayask.go.
	OpToLibraryPair
	// OpLookAck: the resolving controller acknowledges a private look at
	// player Obj's hand (gorge poseLookAck's one-option look_ack, posed by
	// effRevealHand's Look$ arm only when the hand holds a card); no other
	// effect. C=1: the look is at the top A cards of player Obj's library
	// (PeekAndReveal NoReveal$ True without RememberPeeked$: Mishra's
	// Bauble; posed when the library holds a card).
	// dirac/act_mayask.go.
	OpLookAck
	// OpDiscardUnless: player Obj discards A cards unless they discard one
	// card matching F (gorge effDiscard UnlessType$; Thirst for Discovery).
	// dirac/act_mayask.go.
	OpDiscardUnless
	// OpUntapChoose: untap A battlefield permanents matching F, any
	// controller (gorge effUntap's UntapType$ + Amount$ chooser); C=1 up to
	// A (UntapUpTo$). dirac/act_mayask.go.
	OpUntapChoose
	// OpBlight: player Obj blights A (CR 701.68): that player chooses a
	// creature they control and puts A -1/-1 counters on it; none is a
	// no-op, one is unasked (gorge effBlight).
	// dirac/act_blight.go.
	OpBlight

	// OpCount ends the frozen iota list (the ops above are 0..155). It is
	// a fixed value: do not append above it. A new op is an explicit
	// constant with a value in [OpCount,
	// OpHighFirst) and an init naming it (opNames[OpX] = "X").
	OpCount Op = 156
)

// opNames is filled by the literal below for the frozen list and by per-op
// init registrations (opNames[OpX] = "X") for later ones.
var opNames = [OpHighFirst]string{
	OpNop: "Nop", OpDealDamage: "DealDamage", OpDraw: "Draw", OpDestroy: "Destroy", OpExile: "Exile",
	OpBounce: "Bounce", OpToLibrary: "ToLibrary", OpPumpUntilEOT: "PumpUntilEOT", OpGrantKeyword: "GrantKeyword",
	OpPutCounter: "PutCounter", OpRemoveCounter: "RemoveCounter", OpCreateToken: "CreateToken",
	OpGainLife: "GainLife", OpLoseLife: "LoseLife", OpDiscard: "Discard", OpScry: "Scry", OpSurveil: "Surveil",
	OpMill: "Mill", OpFight: "Fight", OpTap: "Tap", OpUntap: "Untap", OpSacrifice: "Sacrifice",
	OpCounter: "Counter", OpAttach: "Attach", OpDig: "Dig", OpSearch: "Search", OpCharm: "Charm",
	OpDamageAll: "DamageAll", OpDestroyAll: "DestroyAll", OpPumpAll: "PumpAll", OpChangeZoneAll: "ChangeZoneAll",
	OpAnimate: "Animate", OpGainControlEOT: "GainControlEOT", OpSetPT: "SetPT", OpSetTypes: "SetTypes",
	OpLoseAbilities: "LoseAbilities", OpCopyAsToken: "CopyAsToken", OpDelayedTrigger: "DelayedTrigger",
	OpWard: "Ward", OpIfKicked: "IfKicked", OpChooseColor: "ChooseColor", OpChooseType: "ChooseType",
	OpChooseCard: "ChooseCard", OpGenericChoice: "GenericChoice", OpRepeat: "Repeat", OpRepeatEach: "RepeatEach",
	OpEndTurn: "EndTurn", OpAddTurn: "AddTurn", OpAddMana: "AddMana", OpReturn: "Return", OpSkipIf: "SkipIf",
	OpBite: "Bite", OpLoseGame: "LoseGame", OpShuffle: "Shuffle", OpPutOnBattlefield: "PutOnBattlefield",
	OpExileUntilLeaves: "ExileUntilLeaves", OpRegenerate: "Regenerate", OpPreventDamage: "PreventDamage",
	OpGrantAbility: "GrantAbility", OpProliferate: "Proliferate", OpTransform: "Transform",
	OpSkipUnless: "SkipUnless", OpBranch: "Branch", OpRepeatEachPlayer: "RepeatEachPlayer",
	OpUnlessPaid: "UnlessPaid", OpIfPaid: "IfPaid", OpDealDamageEx: "DealDamageEx",
	OpDealDamageDivided: "DealDamageDivided", OpResolveDamageMap: "ResolveDamageMap", OpExileEx: "ExileEx",
	OpReturnEx: "ReturnEx", OpToLibraryChoice: "ToLibraryChoice", OpSearchEx: "SearchEx",
	OpChooseMove: "ChooseMove", OpChangeZoneAllEx: "ChangeZoneAllEx", OpDigEx: "DigEx", OpDigUntil: "DigUntil",
	OpPeek: "Peek", OpTwoPiles: "TwoPiles", OpChooseEachType: "ChooseEachType", OpSacrificeAll: "SacrificeAll",
	OpCastFree: "CastFree", OpMayPlay: "MayPlay", OpCreateEmblem: "CreateEmblem", OpOverride: "Override",
	OpReflexiveTrigger: "ReflexiveTrigger", OpDelayedProgram: "DelayedProgram",
	OpClearRemembered: "ClearRemembered", OpClearChosen: "ClearChosen", OpDoubleCounters: "DoubleCounters",
	OpRemoveCounterRem: "RemoveCounterRem", OpEnterTapped: "EnterTapped", OpEnterWithCounters: "EnterWithCounters",
	OpCopyAsTokenEx: "CopyAsTokenEx", OpCreateTokenEx: "CreateTokenEx", OpChooseBranch: "ChooseBranch",
	OpChooseTargets: "ChooseTargets", OpRepeatWhile: "RepeatWhile", OpCopySpell: "CopySpell",
	OpPutCounterAll: "PutCounterAll", OpUntapAll: "UntapAll", OpDealDamageFrom: "DealDamageFrom", OpGiveGift: "GiveGift",
	OpClassLevelUp: "ClassLevelUp", OpGainEnergy: "GainEnergy", OpOverrideAll: "OverrideAll", OpRollDie: "RollDie", OpBecomeCopy: "BecomeCopy",
	OpTapAll: "TapAll", OpSeek: "Seek", OpAmass: "Amass", OpConnive: "Connive", OpConniveCounters: "ConniveCounters", OpGainControl: "GainControl", OpSkipNextUntap: "SkipNextUntap", OpAtEOT: "AtEOT", OpAttachObj: "AttachObj", OpTapOrUntap: "TapOrUntap", OpExplore: "Explore", OpPoison: "Poison", OpFog: "Fog", OpSetLife: "SetLife", OpSkipUntap: "SkipUntap", OpDiscover: "Discover", OpMoveCounter: "MoveCounter", OpRearrange: "Rearrange",
	OpChangeSpeed: "ChangeSpeed", OpBolster: "Bolster", OpSaddle: "Saddle", OpPutCountersFrom: "PutCountersFrom", OpClearCounters: "ClearCounters", OpRemoveCounterAll: "RemoveCounterAll", OpSupport: "Support", OpUnlessSwitched: "UnlessSwitched", OpMonstrosity: "Monstrosity",
	OpWarpExile: "WarpExile", OpMayShuffleMove: "MayShuffleMove", OpEmpower: "Empower", OpIncubate: "Incubate", OpEarthbend: "Earthbend", OpEarthbendReturn: "EarthbendReturn",
	OpManifest: "Manifest", OpCloak: "Cloak", OpManifestDread: "ManifestDread",
	OpPumpAllEach: "PumpAllEach", OpDoubleCountersAll: "DoubleCountersAll", OpPrepare: "Prepare", OpDemonstrate: "Demonstrate", OpNinjutsu: "Ninjutsu",
	OpAtkPWGuard: "AtkPWGuard",
	OpMayAsk:     "MayAsk", OpToLibraryPair: "ToLibraryPair", OpLookAck: "LookAck", OpDiscardUnless: "DiscardUnless", OpUntapChoose: "UntapChoose", OpBlight: "Blight",
}

// String names the op (reports and panics only; never on the hot path).
func (o Op) String() string {
	if o >= OpHighFirst && o < OpHighEnd {
		return highOpNames[o-OpHighFirst] // ops.go
	}
	if int(o) < len(opNames) && opNames[o] != "" {
		return opNames[o]
	}
	return "Op(?)"
}

// Instr is one instruction of an op program.
type Instr struct {
	Op  Op
	Obj Ref
	A   ValueRef
	B   ValueRef
	C   int16
	F   FilterID
	// W is an op's player-valued side input, the Ref the player reads
	// (0 = none): OpTapOrUntap's Tapper$ provenance read at the tap (the
	// election ask is the controller's, the tap's is the ref's, gorge
	// effTapOrUntap's definedPlayerIDs read), OpAttach's PLAYER destination
	// (gorge effAttach's "attach to player" event, Enchant:Player
	// destinations). 0 keeps every op's playerless default.
	W Ref
}

// High ops: ops new families add are numbered from
// OpHighFirst up, clear of the iota list above OpCount that sibling
// families append to, so parallel branches never renumber each other. Every
// loop over the ops walks [0, OpCount) and then [OpHighFirst, OpHighEnd).
const (
	// OpGoad (CR 701.15): the battlefield permanent Obj -- or,
	// with Obj RefNone, every battlefield permanent matching F (gorge's
	// Defined$ Valid sweep, seat 0 first) -- is goaded by the resolving
	// controller until that player's next turn (gorge effGoad, events
	// foldGoad / expireTurnGoads; the default UntilYourNextTurn duration
	// only). A goaded creature attacks each combat if able, and a player
	// other than its goaders if able (rules/combat act_goad.go).
	// dirac/act_goad.go.
	OpGoad Op = 250

	// OpTakeInitiative (CR 726): player Obj (the resolving
	// controller) takes the initiative (one holder, CR 726.3), then
	// ventures into the dungeon card C (pool index: Undercity, CR 726.2):
	// with no dungeon it enters at the topmost room; else the marker moves
	// along the room's arrow (two arrows: a room ask); from the bottommost
	// room the dungeon is completed and a new one begins (CR 701.49a-c).
	// The room entered queues its ability (CR 309.4c). gorge
	// effTakeInitiative / effVenture. dirac/act_initiative.go.
	OpTakeInitiative Op = 251

	// OpDigCounters: A > 0: the card the next OpDigEx puts
	// onto the battlefield enters with A counters of kind C (Dig's
	// WithCounters$ / WithCountersAmount$, a one-card take only: the
	// Undercity's Throne of the Dead Three); A = 0 clears it (kgen emits
	// the clear right after the Dig, so an empty take leaks nothing).
	OpDigCounters Op = 252

	// OpUnearth (CR 702.84a): after its unearth ability's
	// return, the source gains haste until end of turn and is exiled at the
	// beginning of the next end step (a delayed record whose act is
	// OpUnearth); while that record is live, the permanent leaving the
	// battlefield goes to exile instead (gorge rules/unearth.go).
	// dirac/cast_altfree.go.
	OpUnearth Op = 253

	// OpExt: the extension op, the last high slot.
	// A.N holds its sub-op (ExtOp, A.K = VConst; the field no ext user
	// reads otherwise); the rest of the instruction is the sub-op's own
	// (ext.go). New families take an Ext sub-op from 1 up rather than an
	// op: the op space is full. The kernel dispatches it in a cold file
	// (kernel extop.go execExt).
	OpExt Op = 254

	// OpHighEnd is one past the last high op.
	OpHighEnd Op = 255
)

// OpHighFirst is the first high op.
const OpHighFirst Op = 250

var highOpNames = [OpHighEnd - OpHighFirst]string{
	OpGoad - OpHighFirst: "Goad", OpTakeInitiative - OpHighFirst: "TakeInitiative", OpDigCounters - OpHighFirst: "DigCounters",
	OpUnearth - OpHighFirst: "Unearth", OpExt - OpHighFirst: "Ext",
}

// Known reports whether o is a defined op: below OpCount, a registered op in
// [OpCount, OpHighFirst) (opNames[o] set by an init) or a high op.
func (o Op) Known() bool {
	return o < OpCount || o >= OpHighFirst && o < OpHighEnd || o < OpHighFirst && opNames[o] != ""
}

// RollDieCoin marks Instr.C of an OpRollDie as a coin flip (gorge's
// effects.FlipCoin, one rng draw where heads is 1), not a die roll: the
// kernel records the flip on the turn facts and applies a FlipCoinMod static
// (dirac/static_turnbits.go). A die roll leaves C 0.
const RollDieCoin int16 = 1

// RollDieCoinBatch and RollDieCoinMark are bits beside RollDieCoin for a
// multi-coin flip (gorge effFlipCoin's Amount$, one batch that reads
// FlipCoinForced once): a Batch flip leaves the flipper's turn bit alone, so
// every flip in the batch reads the same FlipCoinMod answer, and the Mark
// that closes the batch draws nothing and keeps the roll, only setting the
// turn bit.
const (
	RollDieCoinBatch int16 = 2
	RollDieCoinMark  int16 = 4
)
