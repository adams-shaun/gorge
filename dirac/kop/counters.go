package kop

// CounterKind names a counter type. Append-only.
type CounterKind uint8

const (
	CtrP1P1 CounterKind = iota // +1/+1
	CtrM1M1                    // -1/-1
	CtrLoyalty
	CtrStun
	CtrShield
	CtrFlying
	CtrOther
	NumCounterKinds
)

// Kind classes of ExCtrAdded's X.N.
const (
	CtrAddedKindAny  = 0 // any counter kind
	CtrAddedKindP1P1 = 1 // +1/+1 counters
	CtrAddedKindLore = 2 // lore counters
)

// Object forms of ExCtrAdded's X.N (shifted left by CtrAddedFormShift).
const (
	CtrAddedFormCreature = 0 // a counter on a creature
	CtrAddedFormSelf     = 1 // Card.Self: a counter on the reading source
	CtrAddedFormShift    = 2
	CtrAddedKindMask     = 3
)

// OpPutCounter and OpPutCounterAll carry PutCtrCellFirst in B on the first
// placement of an action that lands several kinds (a CounterTypes$ list, a
// PutCounterAll second sweep) and PutCtrCellNext on the later ones.
const (
	PutCtrCellFirst = 3
	PutCtrCellNext  = 4
	// PutCtrRememberPut is OpPutCounterAll's B for a RememberPut$ rewrite: a
	// recipient is remembered only when placeCounters landed counters on it,
	// after replacements (gorge counters_put.go rememberPlaced's placed list,
	// the OpPutCounter B=2 arm; the overload each-variant of PutCounter,
	// kw_overload.go).
	PutCtrRememberPut = 2
)

// PutCtrRemAmount is OpPutCounterAll's B for RememberAmount$ True on a
// single sweep: the counters that landed across the sweep join the
// resolution's remembered count (gorge effPutCounterAll appends each
// recipient once per landed counter; Guinan, Ancient Advisor).
const PutCtrRemAmount = 5

// CtrRemMV is OpCounter's B bit 0: RememberCounteredCMC$ True. The countered
// spell's mana value (announced X included) joins resCtx.remLK when the spell
// leaves the stack, the number Count$RememberedNumber (ExRemLKIPT) then reads.
const CtrRemMV = 1

// CtrRemember is OpCounter's B bit 1: RememberChanged$ True on a stack move
// (Brazen Bluff's optional exile): the moved spell joins the resolution's
// remembered set just before it leaves the stack.
const CtrRemember = 2

// DivMaxV is TargetSpec.Divide's sentinel for a slot (TgfBoundsV, 0..X targets)
// whose divided total is the slot's own evaluated maximum: "distribute X
// +1/+1 counters among any number of target creatures" with X the spell's
// paid {X} (Elusive Otter's Grove's Bounty). Divide is a
// count, not an enum list; 254 is no literal total (kgen lowers 2..8).
const DivMaxV = 254

// DivLKIP1P1 is TargetSpec.Divide's sentinel for a slot whose target count
// (up to) and divided total are both the triggering source's last-known
// +1/+1 counters (Quirion Beastcaller).
const DivLKIP1P1 = 255

// DivValue is TargetSpec.Divide's sentinel for a slot whose divided total is
// TargetSpec.DivideV, a value evaluated where the spell or ability is put on
// the stack (gorge divisionTotalFor; CR 601.2d, 603.3d). Ureni, the Song
// Unending divides its X damage, X the lands its controller controls; Rolling
// Thunder divides its paid X. The slot's target count is clamped to the total
// (each target takes at least 1, gorge cast_targets.go), whatever its own
// maximum (an all-candidates count, Shatterskull Smashing's two).
const DivValue = 253

// CtrMode is Gate.Ctr's sentinel for a Siege's mode gate: the ability exists
// only while its source permanent's chosen mode index is Gate.N (a granted
// trigger is checked when its event happens only). Not a counter kind.
const CtrMode CounterKind = 253

const (
	StDynamic              StaticFlags = 1 << iota // VA/VB carry the amounts
	StOncePerTurn                                  // MayPlayFrom: one card per turn through this static
	StAnyManaType                                  // MayPlayFrom: mana of any type may be spent
	StWithoutManaCost                              // MayPlayFrom: without paying the mana cost
	StFunctionsInGraveyard                         // the static works while the source is in the graveyard
	StRemove6Counters                              // MayPlayFrom: also remove Static.A counters from among creatures you control (the name predates Dawnhand Dissident's three)
	StTargetsFilter2                               // ReduceCost, spell or activated ability: only if it targets an object matching Filter2
	StRaiseYou                                     // IncreaseCost (kgen): only spells the static's controller casts (Activator$ You)
	StRaiseOpp                                     // IncreaseCost (kgen): only spells the controller's opponents cast (Activator$ Opponent); on a ReduceCost, StReduceAny: every caster's spells (no Activator$)
	StYouHexproof                                  // Continuous (kgen, shield): the static's controller has hexproof too (Affected$ You,...; read by playerHexproof under Cond2)
	StFunctionsInExile                             // the static works while the source is in exile
	StPanhETB                                      // Panharmonicon (kgen): echoes only a trigger caused by a permanent entering the battlefield that matches Filter2 (ValidMode$ ChangesZone,ChangesZoneAll | ValidCause$ | Destination$ Battlefield)
)

// StAlsoSideboard is a MayPlayFrom static whose permission also covers the
// owner's sideboard ("from outside the game", CR 400.11: The Holodeck's
// Sideboard,Graveyard). StaticFlags is full at 16 bits, so it aliases
// StYouHexproof, a Continuous-only flag: a static's StFlags are kind-specific,
// and neither kind reads the other's bit (as StTapOnC aliases StOncePerTurn).
const StAlsoSideboard = StYouHexproof

// StFreeOnly is a MayPlayFrom static that only waives the mana cost and does
// not itself open a zone ("MayPlayDontGrantZonePermissions$ True",
// Weftwalking; gorge rules/mayplay.go mayPlayStatic: grants = !DontGrant,
// free = WithoutManaCost). It aliases StRaiseYou, an IncreaseCost-only flag.
const StFreeOnly = StRaiseYou

// StHandOpp is StRaiseOpp read on a StatNoMaxHandSize: the static sets its
// controller's opponents' maximum hand size (Winter, Misanthropic Guide's
// Affected$ Opponent), not its controller's. StaticFlags is full at 16 bits;
// NoMaxHandSize reads no IncreaseCost flag.
const StHandOpp = StRaiseOpp

// StReduceAny is StRaiseOpp read on a StatReduceCost: the reduction applies to
// the spells of either player (a ReduceCost with no Activator$, Helm of
// Awakening), where a ReduceCost with Activator$ You reduces its controller's.
const StReduceAny = StRaiseOpp

// OpCounter C: Destination$ of the countered spell card (0 graveyard).
const (
	CtrDestExile     = 1
	CtrDestHand      = 2
	CtrDestLibTop    = 3
	CtrDestLibBottom = 4
	// CtrDestMove: not a counter but a move off the stack (gorge
	// ChangeZone Origin$ Stack, Hullbreaker Horror's "return target
	// spell to its owner's hand"): a spell that can't be countered still
	// goes
	CtrDestMove = 8
)

// Appended counter kinds (CounterKind is append-only; NumCounterKinds stays
// the size of the kernel's per-permanent counter array, so these sit above
// it and the kernel must widen that array to NumCounterKinds2 to use them).
const (
	CtrIncubation CounterKind = NumCounterKinds + iota
	CtrBait
	CtrSoul
	CtrFellowship
	CtrRevival
	CtrStash
	NumCounterKinds2
)

// CtrLevel is a Class's level (CR 716) kept in a permanent's
// sparse counter slots as level-1 (no slot at level 1). gorge's level is a
// designation, not a counter (Object.ClassLevelValue; CR 716.4's
// classLevel_ reads it, never a counter census), so the kind sits ABOVE
// NumCounterKinds2: every per-kind array and every counter walk bounded by
// NumCounterKinds2 (proliferate, doubling, projections, gorge's counter
// list) never sees it, and kgen never lowers a counter op of this kind.
const CtrLevel CounterKind = 14

// CtrLevel's value is fixed: NumCounterKinds2 is frozen (the kernel
// feature layout sizes its counter arrays by it) and the Class tables name
// the level kind 14 (NumCounterKinds2 + 1).
var _ = [1]struct{}{}[CtrLevel-NumCounterKinds2-1]

// Unfeaturized counter kinds (kcounters): named counters with no inherent
// rules meaning, numbered ABOVE CtrLevel so the featurized range
// [0, NumCounterKinds2) and FeatCtrDeathtouched stay byte-identical (trained
// models consume that layout; featview.go's seeCtr and project.go's
// Counters skip every kind >= NumCounterKinds2). The kernel keeps them in
// the same sparse perm slots (ck is a uint8); ProjObj.CountersX carries
// them to the oracle, which compares each kind against gorge. Append-only.
const (
	CtrCharge CounterKind = CtrLevel + 1 + iota
	CtrOil
	CtrSpore
	CtrQuest
	CtrVerse
	CtrGrowth
	CtrFuse
	CtrStory
	// CR 122.1b keyword counters. Flying is the featurized CtrFlying;
	// Decayed and Exalted have no kop.Kw bit, so kgen refuses those kinds.
	CtrFirstStrike
	CtrDoubleStrike
	CtrDeathtouch
	CtrHaste
	CtrHexproof
	CtrIndestructible
	CtrLifelink
	CtrMenace
	CtrReach
	CtrShadow
	CtrTrample
	CtrVigilance
	// CtrSupply is Supply, a named counter with no inherent rules meaning
	// (Stocking the Pantry).
	CtrSupply
	// CtrLore is a Saga's lore counter (CR 702.151, kw:Chapter; kw_saga.go).
	CtrLore
	// CtrLandmark is Treasure Map's landmark counter, a named counter with no
	// inherent rules meaning (CR 122.1; the card's own branch counts it).
	CtrLandmark
	// CtrDread is a named counter with no inherent rules meaning (Grasping Shadows).
	CtrDread
	// CtrDefense is a Battle's defense counter (CR 310.4; gorge "DEFENSE").
	CtrDefense
	// CtrChorus is a named counter with no inherent rules meaning (Malcolm,
	// Alluring Scoundrel's chorus counters).
	CtrChorus
	// CtrM0M1 is a -0/-1 counter (CR 122.1a; Wall of Roots' mana cost,
	// ): each lowers the toughness by one (the kernel's ptGen).
	CtrM0M1
	// CtrFinality is a finality counter (CR 122.1: a creature with one that
	// would be put into a graveyard from the battlefield is exiled instead;
	// gorge finalityReplacementApplies; Zoraline).
	CtrFinality
	// CtrTime is a time counter (CR 122.1; Impending's, CR 702.176a: a
	// permanent cast impended is not a creature while it has one; kernel
	// kw_impending.go).
	CtrTime
	// CtrValor is a valor counter, a named counter with no inherent rules
	// meaning (Intrepid Adversary's valor counters).
	CtrValor
	// CtrNet is a net counter, a named counter with no inherent rules meaning
	// (Braided Net's etbCounter and its {T}, remove-a-net-counter cost;
	// ).
	CtrNet
	// CtrPlan is a plan counter, a named counter with no inherent rules
	// meaning (the MSH Plan enchantments; "when the Nth plan counter is put
	// on this enchantment").
	CtrPlan
	// CtrInvasion is an invasion counter, a named counter with no inherent
	// rules meaning (Alien Invasion's Count$CardCounters.INVASION; CR 122.1).
	CtrInvasion
	// CtrLoot is a loot counter, a named counter with no inherent rules
	// meaning (Bandit's Haul's SubCounter<2/LOOT>; CR 122.1).
	CtrLoot
	// CtrRev is a rev counter, a named counter with no inherent rules meaning
	// (Chainsaw's Count$CardCounters.REV; CR 122.1).
	CtrRev
	// CtrPage is a page counter, a named counter with no inherent rules
	// meaning (Diary of Dreams' Count$CardCounters.PAGE; CR 122.1).
	CtrPage
	// CtrIngenuity is an ingenuity counter, a named counter with no inherent
	// rules meaning (Lady Octopus's Count$CardCounters.INGENUITY; CR 122.1).
	CtrIngenuity
	// CtrFilm is a film counter, a named counter with no inherent rules
	// meaning (Peter Parker's Camera's etbCounter and SubCounter<1/FILM>;
	// CR 122.1).
	CtrFilm
	// CtrFire is a fire counter, a named counter with no inherent rules
	// meaning (War Balloon's counters_GE3_FIRE; CR 122.1).
	CtrFire
	// CtrHone is a hone counter (CR 122.1j): on an Equipment it gives +1/+0
	// to the creature the Equipment is attached to (Dwalin, Weaponmaster;
	// kernel honePower, ctr_hone.go).
	CtrHone
	// CtrCoin is a coin counter, a named counter with no inherent rules
	// meaning (Wishing Well's Count$CardCounters.COIN; CR 122.1;
	// ).
	CtrCoin
	// CtrDivinity is a divinity counter, a named counter with no inherent
	// rules meaning (Myojin of Night's Reach's counters_GE1_DIVINITY;
	// CR 122.1).
	CtrDivinity
	// CtrImpostor is an impostor counter, a named counter with no inherent
	// rules meaning (Illicit Masquerade's counters_GE1_IMPOSTOR; CR 122.1;
	// ).
	CtrImpostor
	// CtrBore is a bore counter, a named counter with no inherent rules
	// meaning (Brass's Tunnel-Grinder's Count$CardCounters.BORE; CR 122.1;
	// ).
	CtrBore
	// CtrPoint is a point counter, a named counter with no inherent rules
	// meaning (Contested Game Ball's five-point sacrifice; CR 122.1;
	// ).
	CtrPoint
	// CtrBloodstain is Blood Spatter Analysis's named counter (CR 122.1;
	// ).
	CtrBloodstain
	// CtrPossession is Unwilling Vessel's named counter, a counter with no
	// inherent rules meaning (CR 122.1).
	CtrPossession
	// CtrPromotion is Beckett Mariner's named counter (CR 122.1).
	CtrPromotion
	// CtrPeace is Peace in Our Time's named counter (CR 122.1; its twelve-counter win).
	CtrPeace
	// CtrWish is Wishclaw Talisman's named counter (CR 122.1).
	CtrWish
	// CtrSkewer is Rotisserie Elemental's named counter (CR 122.1).
	CtrSkewer
	// CtrNest is Twitching Doll's named counter (CR 122.1).
	CtrNest
	// CtrAge is an age counter (CR 702.24a: cumulative upkeep puts one on the
	// permanent each upkeep and charges the cost once per counter; Mystic
	// Remora). A named counter with no other rules meaning (CR 122.1).
	CtrAge
	// CtrLuck is Gemstone Caverns' luck counter (CR 122.1): a named counter
	// whose presence its mana ability reads (SplitGateCounter).
	CtrLuck
	// CtrDepletion is a depletion counter (Peat Bog's "remove a depletion
	// counter: add {B}{B}"): a named counter with no inherent rules meaning
	// (CR 122.1) whose count its mana ability's cost and rider read.
	CtrDepletion
	// CtrMining is a mining counter (Gemstone Mine): a named counter with no
	// inherent rules meaning (CR 122.1).
	CtrMining
	// CtrFade is a fade counter (CR 702.32a: fading enters with N and removes
	// one each upkeep, sacrificing when it can't; Blastoderm). A named
	// counter with no other rules meaning (CR 122.1).
	CtrFade
	// CtrBurden is The One Ring's burden counter: a named counter with no
	// inherent rules meaning (CR 122.1); its count drives the upkeep life
	// loss and the draw rider (Count$CardCounters.BURDEN).
	CtrBurden
	// CtrStorage is a storage counter (Molten Slagheap and kin): a named
	// counter with no inherent rules meaning (CR 122.1) that its own mana
	// ability removes.
	CtrStorage
	NumCounterKinds3
)

// CtrAll is no counter kind: an ExCountersOn reading it sums every counter
// on the object (gorge Count$CardCounters.ALL, Object.Counter("ALL")).
// Never stored, put or removed (kernel refCounters).
const CtrAll CounterKind = 254

// NumCounterKindsX is the number of unfeaturized kinds (ProjObj.CountersX).
const NumCounterKindsX = int(NumCounterKinds3 - CtrCharge)

// CtrXNames is gorge's (Forge's CounterType) name of each unfeaturized
// kind, CtrXNames[k-CtrCharge].
var CtrXNames = [NumCounterKindsX]string{"CHARGE", "OIL", "SPORE", "QUEST", "VERSE", "GROWTH", "FUSE", "STORY",
	"First Strike", "Double Strike", "Deathtouch", "Haste", "Hexproof", "Indestructible", "Lifelink", "Menace",
	"Reach", "Shadow", "Trample", "Vigilance", "SUPPLY", "LORE", "LANDMARK", "DREAD", "DEFENSE", "CHORUS", "M0M1", "FINALITY", "TIME", "VALOR", "NET", "PLAN",
	"INVASION", "LOOT", "REV", "PAGE", "INGENUITY", "FILM", "FIRE", "HONE", "COIN",
	"DIVINITY", "IMPOSTOR", "BORE", "POINT", "BLOODSTAIN", "POSSESSION", "PROMOTION", "PEACE", "WISH", "SKEWER", "NEST", "AGE", "LUCK",
	"DEPLETION", "MINING", "FADE", "BURDEN", "STORAGE"}

// IsCounterKind reports whether c is a real counter kind (a featurized one
// or an unfeaturized one; CtrLevel and the gap at NumCounterKinds2 are not).
func IsCounterKind(c int) bool {
	return c < int(NumCounterKinds2) || c >= int(CtrCharge) && c < int(NumCounterKinds3)
}

// CtrMaxSpeed is Gate.Ctr's sentinel for a max-speed gate (CR 702.179e):
// the ability exists only while its source's controller has speed 4 (a
// granted trigger is checked when its event happens only). Not a counter
// kind.
const CtrMaxSpeed CounterKind = 255
