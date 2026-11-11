package kop

// FilterID indexes Table.Filters. 0 is the empty filter (matches anything in
// the default zone); Table.Filters[0] must be the zero Filter.
type FilterID uint16

// TypeMask is a card's types (plus the two subtypes the kernel treats as
// types because rules read them: Aura and Equipment).
type TypeMask uint16

const (
	TLand TypeMask = 1 << iota
	TCreature
	TArtifact
	TEnchantment
	TPlaneswalker
	TInstant
	TSorcery
	TBattle
	TKindred
	TAura      // enchantment subtype Aura (attaches on resolution)
	TEquipment // artifact subtype Equipment (Equip ability)
	TBasic     // supertype basic
	TLegendary // supertype legendary
	TToken     // the object is a token (set on token CardDefs)
	TVehicle
)

// ColorMask is WUBRG in bits 0..4. ColorColorless (bit 5) marks a colourless
// mana symbol / production. ColorAny marks "one mana of any colour".
type ColorMask uint8

const (
	ColW ColorMask = 1 << iota
	ColU
	ColB
	ColR
	ColG
	ColC             // colourless mana ({C})
	ColAny ColorMask = 0x1f
)

// CtrlSel selects by controller relative to the source's controller.
type CtrlSel uint8

const (
	CtrlAny CtrlSel = iota
	CtrlYou
	CtrlOpp
)

// Pred is a filter's extra predicate opcode (applied with Filter.PredN).
type Pred uint8

const (
	PredNone         Pred = iota
	PredPowerLE           // power <= N
	PredPowerGE           // power >= N
	PredToughnessLE       // toughness <= N
	PredCMCLE             // mana value <= N
	PredCMCGE             // mana value >= N
	PredHasKeyword        // has keyword bit N
	PredHasCounter        // has a counter of kind N
	PredSubtype           // has subtype index N (Table.Subtypes)
	PredNamed             // is card id N
	PredWithPowerLEX      // power <= X
)

// FilterFlags are boolean conditions of a Filter.
type FilterFlags uint16

const (
	FOther    FilterFlags = 1 << iota // not the source itself
	FNontoken                         // not a token
	FToken                            // a token
	FTapped
	FUntapped
	FAttacking
	FBlocking
	FAttackingOrBlocking
	FSelf   // only the source itself
	FPlayer // a target spec: players allowed
	FCreatureOrPW
	FNotAttacking
	FSpell         // matches spells on the stack
	FAttachedBySrc // the permanent the source Aura/Equipment is attached to
	FExiledWith    // exiled with the reading source (gorge ExiledWithSource): a resolving entry's, or a static's own (kernel zone_exiledwith.go)
	// FNotAttachedBySrc: '!EnchantedBy' / '!EquippedBy': not the permanent the
	// source is attached to; true for all when the source is unattached or
	// gone (gorge !attachedBy). Bit 15, the LAST free FilterFlags bit
	// (kernel filter_notattached.go)
	FNotAttachedBySrc
)

// Filter is the object predicate for targets, *All ops, triggers and statics:
// type mask (any-of) + not-types + controller + colour mask + one predicate.
type Filter struct {
	Types     TypeMask // any of these (0 = any type)
	NotTypes  TypeMask
	Ctrl      CtrlSel
	Colors    ColorMask // any of these colours (0 = any)
	NotColors ColorMask
	Pred      Pred
	PredN     int16
	Flags     FilterFlags
	Zone      Zone // ZoneNone = battlefield (or the stack for FSpell)

	// Appended by dirac/kgen (table.go documents them).
	Owner  CtrlSel // owner relative to the source's controller
	Sub    uint8   // must have subtype index Sub-1 (0 = no requirement)
	NotSub uint8   // must not have subtype index NotSub-1
	// NotSubs are further subtypes the object must not have (index-1; 0 =
	// unused), filled only beside NotSub ("non-Vampire, non-Werewolf,
	// non-Zombie, non-Human, non-Bear")
	NotSubs [4]uint8
	Flags2  FilterFlags2 // more boolean conditions
	PredV   ValueRef     // the bound for the *V predicates
	Or      FilterID     // the next alternative: the object matches if this OR Filters[Or] matches (0 = none)
	// Each marks the head of an Or chain that is Forge's "EACH A & B" list
	// (a search or hand pick of one card per listed type): a card belongs to
	// the group of the FIRST chain element that matches it, and a pick takes
	// at most one card per group (gorge eachAlternatives / EachTypeGroups).
	// The match itself is the ordinary Or chain.
	Each bool
	// Special marks a RestrictValid$ Static.<tail> alternative: the CR 116.2
	// special actions the clause admits. A clause with Special set never
	// matches a spell or an activated ability (those match the spell half);
	// the restricted-mana pool matches it against a special action's paid
	// object. 0 for every ordinary card filter.
	Special SpecialMask
	// PickSumMax caps the total printed mana value of the cards a pick has
	// chosen: a candidate fits only while the picked cards' values plus its
	// own stay at most N (WithTotalCMC$). VConst with N = 0 is the zero
	// Filter's "no cap"; kgen refuses a zero budget, so a cap is positive.
	// Table data read by State.pickFits (dirac/zone_each.go), not State.
	PickSumMax ValueRef
	// PickDistinctPower requires a picked card's printed power to differ
	// from every card already picked (WithDifferentPowers$). False is the
	// zero Filter's "no requirement".
	PickDistinctPower bool
}

// SpecialMask is a set of CR 116.2 special actions a RestrictValid$ Static.*
// clause admits (gorge pay.Special).
type SpecialMask uint8

const (
	SpmMorphUp SpecialMask = 1 << iota // Morph and Megamorph (Forge isMorphUp)
	SpmDisguiseUp
	SpmManifestUp
	SpmCloakUp
	SpmUnlock
	// SpmActivated marks an undotted RestrictValid$ Activated on a resolving
	// DB$ Mana: the clause admits every activated-ability payment and nothing
	// else (gorge restrictValidTermMatches); never a CR 116.2 special action.
	SpmActivated
)

// SpmTurnFaceUp is Forge's isTurnFaceUp: every turn-up procedure.
const SpmTurnFaceUp = SpmMorphUp | SpmDisguiseUp | SpmManifestUp | SpmCloakUp

// HybTwobrid marks Hyb[0] when the cost carries twobrid pips (ManaCost.Tb).
// It is no colour mask: ManaCost.Resolve never sees it.
const HybTwobrid ColorMask = 0x80

// HybPhyrexian marks Hyb[1] beside HybTwobrid when ManaCost.Tb's pips are
// Phyrexian ({W/P}, CR 107.4f): each is paid with its colour
// or two life, and counts one toward the mana value (CR 202.3g).
const HybPhyrexian ColorMask = 0x40

// FilterFlags2 (Filter.Flags2).
type FilterFlags2 uint32

const (
	F2Kicked           FilterFlags2 = 1 << iota // the object (spell or permanent) was kicked
	F2CastByYou                                 // the permanent was cast by its controller
	F2EnteredThisTurn                           // put into its current zone this turn
	F2ChosenType                                // has the source's chosen creature type
	F2ChosenColor                               // has the source's chosen colour
	F2Remembered                                // in the remembered set
	F2NotRemembered                             // not in the remembered set
	F2ChosenCard                                // is a chosen card
	F2NotChosenCard                             // is not a chosen card
	F2AttachedToT0                              // attached to target 0's object
	F2TargetedBy                                // targeted by the resolving ability
	F2SameNameAsSource                          // has the source's name (same card id)
	F2WithFlying                                // has flying
	F2WithoutFlying                             // doesn't have flying
	F2Emblem                                    // an emblem (sources)
	F2Opponent                                  // players: an opponent of the controller
	F2You                                       // players: the controller
	F2Basic                                     // has the supertype basic
	F2SharesColorRem                            // shares a colour with another object in the remembered set (cards off the battlefield only)
	F2NotTriggerObj                             // is not the resolving (or building) ability's triggering object
	F2WasDamaged                                // a permanent with damage marked this turn (gorge wasDealtDamageThisTurn)
	F2Legendary                                 // has the supertype legendary (with a type head: Creature.Legendary)
	F2Saddled                                   // a battlefield permanent that is saddled (CR 702.171b; gorge IsSaddled: SaddledTurn == the turn)
	F2BasePermanent                             // the filter's Permanent base: on the battlefield it reads the live permanent; off it a permanent card
	F2Sneaked                                   // the spell or permanent was cast for its sneak cost (the battlefield mark is the arena's mechSneaked)
	F2Unblocked                                 // an attacking permanent with no blocker recorded (CR 509.1h; gorge "unblocked"; battlefield only)
	F2BlockedBySrc                              // an attacking creature the source is blocking; with the source gone (sacrificed), any attacking creature: the target was chosen while it blocked (gorge blockedBySourceLKI)
	F2DamagedBy                                 // a permanent the source dealt damage to this turn (gorge DamagedBy over DamageTakenThisTurnBy)
	F2CrewedSrc                                 // a creature that crewed the source this turn (gorge CrewedThisTurn)
	F2Warped                                    // a battlefield permanent cast for its warp cost, its delayed exile still pending (gorge CastSa Spell.Warp)
	F2TgtPlayerCtrl                             // a permanent whose controller is a player target of the current stack entry (gorge TargetedPlayerCtrl; battlefield only)
	F2DefendingCtrl                             // a permanent the defending player controls: in two-player combat, not the active player (gorge ControlledBy TriggeredDefendingPlayer in an Attacks trigger; battlefield only)
)

// Appended predicates (Filter.Pred).
const (
	PredCMCEQ            Pred = PredWithPowerLEX + 1 + iota // mana value == PredN
	PredCMCLEV                                              // mana value <= the value PredV (== when PredN is 1)
	PredCountersGE                                          // at least PredN>>8 counters of kind PredN&0xff
	PredPowerGEV                                            // a battlefield permanent's power >= the value PredV; PredN 1: a spell's power == PredV; PredN 2: a card in a graveyard whose printed power <= PredV (dirac/filter_ptvalue.go); PredN 3: a battlefield permanent's power == PredV (Amalia's powerEQ20)
	PredToughnessGE                                         // toughness >= PredN
	PredHasXCost                                            // the printed mana cost contains {X}
	PredPowerLTToughness                                    // power < toughness
	PredMultiColor                                          // two or more colours (gorge wordMultiColor)
	PredHistoric                                            // artifact, legendary or Saga (gorge historicMatches); PredN = the Saga subtype index+1, 0 if the table has none
	PredEquipped                                            // a battlefield permanent with an Equipment attached (gorge equipped)
	PredEnchanted                                           // a battlefield permanent with an Aura attached (gorge enchanted)
	PredAttached                                            // attached to the source (gorge objectAttachedToSource)
	PredActiveCtrl                                          // controlled (off the battlefield: owned) by the active player (gorge ActivePlayerCtrl)
	PredTypesAll                                            // has EVERY card type in the mask PredN (a Forge "Artifact.Creature" / "Land.Artifact" clause: the type head AND a disjoint type property; typesall); derived types on the battlefield, printed off it
	PredModified                                            // modified (CR 700.9): a battlefield permanent with a counter, an Equipment attached, or an Aura its controller controls attached (gorge modifiedPermanent)
	PredHasCounters                                         // HasCounters: a battlefield permanent with one or more counters of any kind, the regeneration shield and the deathtouch mark included (gorge wordHasCounters/hasCounters)
	PredMonstrous                                           // the battlefield permanent is monstrous (CR 701.37b; gorge IsMonstrous: Object.Monstrous)
	PredNoCounters                                          // !HasCounters: no counter of any kind, the regeneration shield and the deathtouch mark included (PredHasCounters negated)
	PredLacksKeyword                                        // !with<Keyword>: does not have keyword bit PredN (PredHasKeyword negated)
	PredOutlaw                                              // an Assassin, Mercenary, Pirate, Rogue or Warlock (gorge outlawMatches); PredN 1 negates (!Outlaw)
	PredMonoColor                                           // exactly one colour (gorge wordMonoColor)
	// PredSameNameT0: the resolving ability's target 0, or an object
	// sharing its name: a battlefield permanent (a named, face-up one; gorge
	// "TargetedCard.Self,Permanent.NotDefinedTargeted+sharesNameWith
	// Targeted": Maelstrom Pulse) or a card whose target is a graveyard card
	// (the Card.NotDefinedTargeted spelling: Rat King, Verminister)
	PredSameNameT0
	// PredCountersLT: fewer than PredN>>8 counters of kind PredN&0xff
	// (gorge counters_LT/LE/EQ0: Reckoner Bankbuster's counters_EQ0_CHARGE
	// is LT 1; off the battlefield a card has none but a stash mark;
	// )
	PredCountersLT
	// PredToughnessLEV: a battlefield permanent whose current toughness
	// is <= the value PredV (gorge toughnessLE<X>: Zero Point Ballad's
	// "creatures with toughness X or less"; battlefield only). PredN 1: a
	// spell's toughness == PredV (Talion's toughnessEQChosen;
	// dirac/filter_ptvalue.go). PredN 3: a battlefield permanent's toughness
	// is >= PredV, a card off it by its printed or last known toughness
	// (Hulkling's toughnessGTZ lowered as >= Z+1; dirac/filter_ptvalue.go)
	PredToughnessLEV
)

// PredAdventureCard: an adventurer card (CR 715.1; gorge wordAdventureCard,
// Forge's AdventureCard): the card's layout, so it holds in every zone, on the
// stack as the Adventure half is cast, in exile on an adventure, and for the
// permanent on the battlefield.
const PredAdventureCard Pred = 253

// OvChosenColor, carried in an OpOverride's B field (which OpOverride does
// not otherwise read; 0 is the zero Const there), marks the chosen-colour
// animation family (Puca's Eye, Mondo Gecko): the
// instruction's C names the FIRST of five consecutive Overrides records
// differing only in SetColors -- W, U, B, R, G in that order, one record
// per colour, in C + chosen - 1 order. At resolution the kernel applies the
// record whose colour the controller chose earlier in the same resolution
// (ExtChooseColorRes's res.chosen, 1..5), baking the colour into the
// applied record: gorge effChooseColor + combatfx.go animateGrant read the
// source's ChosenColor AT RESOLUTION, and OverwriteColors$ True replaces
// the object's colours (layer 5, CR 613.1e).
const OvChosenColor = 1

// OvEnterWith, in an OpOverride's B field: the override belongs to the OpReturn
// just before it and applies as the card enters, ahead of the enters triggers
// (assimilate, CR 613-layer characteristics seen as it enters).
const OvEnterWith = 2

// The exiled-with source link, numbered explicitly clear of the
// iota lists that sibling families append to.

// PredExiledWith: the card was exiled with the reading ability's source (gorge
// filter ExiledWithSource, Forge Card.ExiledWithSource): the reverse link
// Object.ExiledWith held live for the source's incarnation, or the source's
// forward ExiledCards list (kernel zone_exiledwith.go). Resolution-time
// readers only: the source is the resolving entry's. A filter reads the same
// link as the FExiledWith flag, which takes its source from
// the matcher's context (a static's own object, else the resolving entry);
// the predicate survives as the synthetic Defined$ ExiledWith sweep's form
// (PredN 1). Omenpath Journey, Ghost Vacuum, Northampton Farm.
const PredExiledWith Pred = 249

// PredTriggerObj: the object IS the resolving ability's triggering object
// (kernel filter_predicates.go isTriggerObj, F2NotTriggerObj's positive).
// It is not a Forge filter word: kgen builds it for a Hidden$ ChangeZone's
// ChooseFromDefined$ TriggeredCards pool (gorge chooseFromDefinedPool over
// the trigger's capture), whose eligible cards are the marker's objects
// still in the origin zone. Explicit high ordinal, filters.go style; 240 is
// free beside the 241-254 block. Paranormal Analyst.
const PredTriggerObj Pred = 240

// PredFaceDown: the filter word faceDown, a face-down
// BATTLEFIELD permanent (gorge wordFaceDown: AsFaceDown || Object.FaceDown on
// the battlefield). A face-down spell or exile card never matches. Explicit
// high ordinal, filters.go style; took 246.
const PredFaceDown Pred = 245

// PredWord7: five filter words, sub-kind in PredN's low byte:
// 1 noName, 2 greatestPower, 3 attackingAlone, 4 inZone (zone in PredN>>8).
const PredWord7 Pred = 241

const (
	W7NoName         int16 = 1
	W7GreatestPower  int16 = 2
	W7AttackingAlone int16 = 3
	W7InZone         int16 = 4
)

// six more PredWord7 sub-kinds (PredN's low byte; no new
// Pred). 5 faceUp, 6 IsNotChosenType, 7 "attacking Opponent", 8 "EnchantedBy
// Aura.YouCtrl", 9 "doesNotShareNameWith OtherYourBattlefield", 10
// "sharesNameWith Valid Permanent.Legendary" (gorge cuts the spec at '+': a
// trailing +YouCtrl is the outer filter's, not the inner's). Append-only.
const (
	W7FaceUp           int16 = 5
	W7NotChosenType    int16 = 6
	W7AttackingOpp     int16 = 7
	W7EnchantedByYours int16 = 8
	W7NoNameTwin       int16 = 9
	W7SharesNameLegend int16 = 10
)

// PredWord7 sub-kinds 11-13 (PredN's low byte; no new
// Pred). 11 withWarp (gorge's printed-keyword read, effects/filter.go
// keywordInCtx: the card's printed CardDef.HasWarp in any zone; a
// battlefield permanent that is face down or lost all abilities has none;
// Blade of the Swarm), 12 greatestPowerControlledByTargeted (a
// battlefield creature controlled by a player target of the current stack
// entry that no other such creature out-powers; ties match; no player
// target matches nothing), 13 CrewedBySourceThisTurn (the filter's source
// crewed the candidate Vehicle this turn; no source matches nothing).
// Append-only.
const (
	W7WithWarp         int16 = 11
	W7GreatestPowerTgt int16 = 12
	W7CrewedBySrc      int16 = 13
	// W7CostBlue is ManaCostPartialBlue: the printed mana cost has a blue
	// symbol, a hybrid or Phyrexian one included (never face down: CR 708.2).
	W7CostBlue int16 = 14
)

// W7NoAbilities (PredWord7 sub-kind 15): the card's printed face has no
// ability of any kind (CardDef.NoAbilities; gorge noAbilitiesPermanent).
// Read off the battlefield only: kgen refuses it on a permanent, where a
// granted or lost ability would change the answer.
const W7NoAbilities int16 = 15

// W7IsCommander (PredWord7 sub-kind 16): the object is one of a seat's
// commanders (gorge's IsCommander word, state.Game.CommanderOf, CR 903.3).
// On the battlefield it holds for a seat's commander permanent in a Commander
// game and is false in every other game; off the battlefield it is false for
// every card (the word sees only a CardID there).
const W7IsCommander int16 = 16

// W7BottomLibrary and W7NotBottomLibrary (PredWord7 sub-kinds 17 and 18): the
// object is, or is not, the last card of its owner's library (gorge
// wordBottomLibrary; Jace, Reality Sculptor's "all but the bottom card"). A
// card in any other zone is not the bottom of a library.
const (
	W7BottomLibrary    int16 = 17
	W7NotBottomLibrary int16 = 18
)

// W7AttachedTo (PredWord7 sub-kind 19): the object is attached to a
// battlefield permanent (gorge's two-token "AttachedTo <class>[.<qual>]",
// effects/filter_attachment.go attachedToArg) of the class PredN's high byte
// names (AttToCreature, AttToLand; neither: any permanent, the Card and
// Permanent classes) and that meets its qualifier: AttToOther, not the
// filter's source; AttToYouCtrl, controlled by the evaluating controller.
// Nothing off the battlefield is attached.
const W7AttachedTo int16 = 19

// W7AttachedTo's PredN high-byte bits (PredN>>8).
const (
	AttToCreature = 1
	AttToLand     = 2
	AttToOther    = 4
	AttToYouCtrl  = 8
)

// W7NoNameTwinGrave (PredWord7 sub-kind 20): the pair "doesNotShareNameWith
// YourGraveyard+doesNotShareNameWith OtherYourBattlefield" read as one word
// (Guardian Project): no other permanent the object's controller controls
// shares its name, and no card in the evaluating controller's graveyard does
// (gorge doesNotShareNameWithMatches, both keys; CR 201.2). A card in that
// graveyard shares a name with itself.
const W7NoNameTwinGrave int16 = 20

// W7SharesCrTrigObj (PredWord7 sub-kind 21): the object shares a creature type
// with the triggering object of the ability being resolved or put on the
// stack (gorge's "sharesCreatureTypeWith TriggeredAttacker": the creature an
// Attacks trigger fired for, CR 603.2c). Both sides read their current
// creature types (CR 613.1d); one with every creature type shares with any
// object that has a creature type (CR 702.73a). An unbound referent matches
// nothing.
const W7SharesCrTrigObj int16 = 21

// W7WithDisguise (PredWord7 sub-kind 22): the card was printed with Disguise
// (CardDef.MorphFamily == MorphDisguise; gorge's exact keyword word
// withDisguise, objectHasKeyword). A battlefield permanent that is face down
// or lost all abilities has none; off the battlefield the printed card
// counts in any zone. Expose the Culprit.
const W7WithDisguise int16 = 22

// W7FaceUpDisguise (PredWord7 sub-kind 23): the pair faceUp and withDisguise
// in one clause (a clause holds one comparison): not face down and printed
// with Disguise, in any zone.
const W7FaceUpDisguise int16 = 23

// W7EnteredFromBF (PredWord7 sub-kind 24): the card is in a graveyard, or
// another zone off the battlefield, because it was put there from the
// battlefield this turn (gorge's ThisTurnEnteredFrom_Battlefield: the
// object's latest arrival into its current zone, read off the entered
// ledger). Read off the battlefield only: kgen refuses it on a permanent.
const W7EnteredFromBF int16 = 24

// W7Imprinted (PredWord7 sub-kind 25): the object is in the resolution's
// imprinted set (gorge's IsImprinted; genState.imp, written by an Imprint$
// writer earlier in the same chain). kgen lowers it only inside such a chain.
const W7Imprinted int16 = 25

// W7WithFlashback (PredWord7 sub-kind 26): the card was printed with
// Flashback (CardDef.Keywords&KwFlashback; gorge's exact keyword word
// withFlashback, objectHasKeyword reads the printed face). A battlefield
// permanent that is face down or lost all abilities has none; off the
// battlefield the printed card counts in any zone. Sorceress's Schemes,
// Runic Repetition.
const W7WithFlashback int16 = 26

// W7LacksIslandwalk (PredWord7 sub-kind 27): the negated exact-keyword word
// !hasKeywordLandwalk:Island; the kernel reads the Island subtype off the
// matchup table (genPrep.blgSub; a matchup without one has no islandwalk). A battlefield permanent has islandwalk when its printed
// CardDef.Landwalk, unless it lost its abilities, or a Continuous static's
// granted Static.Landwalk, is a land filter naming that subtype (an Or-chain
// link counts), as gorge's keywordPredicateMatches reads a derived
// Landwalk:Island; the predicate holds when it has none. Off the battlefield
// the printed Landwalk alone counts. Stormtide Leviathan.
const W7LacksIslandwalk int16 = 27

// PredPermMark, four per-permanent marks the filter words
// attackedThisTurn, dealtDamageThisTurn, dealtDamagetoAny and
// dealtCombatDamagetoAny read (gorge filter.go AttackedTurn; filter_word.go
// DamageDealtThisTurn, DealtDamageToAnyGame, DealtCombatDamageToAnyGame).
// PredN bits 0-3 name the mark (PermMark*), bit 4 (PermMarkNot) negates it.
// Each mark is an arena slot bit; a card off the battlefield has none, so
// the negated word is true there.
const PredPermMark Pred = 244

const (
	PermMarkAttacked    = 0 // declared as an attacker this turn (CR 508.1; cleared as the turn changes)
	PermMarkDealtTurn   = 1 // dealt damage this turn (CR 120.1; cleared as the turn changes)
	PermMarkDealtAny    = 2 // dealt damage since it entered (game-long)
	PermMarkDealtCombat = 3 // dealt combat damage since it entered (game-long)
	PermMarkNot         = 16
)

// PredTypesAllCMCLE: an all-of card-type conjunction AND a constant
// mana-value bound. PredN is exactly PredTypesAll's mask (the object has
// EVERY type in it); PredV is a Const and the object's mana value is at most
// PredV.N (Resistance Is Futile's "Artifact.Creature" with cmcLE3). The
// battlefield arm reads the derived types and current mana value; the card
// arm the printed types and printed mana value (a split card off the stack
// sums its halves, CR 709.4b; a face-down card has none, CR 708.4).
// Explicit high ordinal, filters.go style; 242 was free beside PredWord7
// (241) and PredPowerGTBase (243).
const PredTypesAllCMCLE Pred = 242

// PredPowerGTBase: the filter word powerGTbasePower, a
// BATTLEFIELD permanent whose current power is greater than its base power
// (layer 7b, before 7c effects and 7d counters; CR 613.4; gorge
// filter_value.go numericPred). Nothing off the battlefield matches.
// Explicit high ordinal, filters.go style; 244 is PredPermMark.
const PredPowerGTBase Pred = 243

// PredNotF2 (high predicate kind): the '!' negation of one
// Flags2 word (gorge matchPredicate negates any positive word): PredN is the
// negated F2 bit's index. kgen lowers !targetedBy and !ThisTurnEntered to it;
// FilterFlags2 is full, so they cannot be bits of their own (kernel filter_negations.go).
const PredNotF2 Pred = 246

// CtrlEachPl is the battlefield Filter.Ctrl "controlled by the player of the
// OpRepeatEachPlayer in progress" (gorge's RememberedPlayerCtrl read inside a
// RepeatEach player body, effects/filter.go). It rides
// a Ctrl value because FilterFlags2 is full. Appended after CtrlOpp.
const CtrlEachPl CtrlSel = 3

// High predicates, numbered from 250 clear of the iota list in
// table.go that sibling families append to. Both are battlefield-only
// and read on the cold predicate path (kernel predPermCold).
const (
	// PredTotalPT: the permanent's current power plus toughness passes the
	// DmgGate-encoded comparison PredN (gorge totalPT_<cmp><n>, Cut Down's
	// totalPT_LE5).
	PredTotalPT Pred = 250
	// PredBlocked: an attacking creature with a blocker recorded, even one
	// that has since left combat (gorge "blocked", CR 509.1h; Tactical
	// Advantage).
	PredBlocked Pred = 251
	// PredSolved: PredN 1, a solved Case; PredN 0, anything not solved
	// (gorge IsSolved / !IsSolved: Object.Solved, CR 719.3b). The
	// designation rides OpSaddle with C = 1 (kernel kw_crewsaddle.go).
	PredSolved Pred = 252
)

// PredTrigPlayerCtrl: a battlefield permanent whose controller is the
// current trigger entry's trigger player, the player a damage trigger's
// damage was dealt to (gorge ControlledBy TriggeredTarget, CR 603.2):
// false with no trigger entry and off the battlefield. Explicit ordinal.
const PredTrigPlayerCtrl Pred = 239

// PredSuspected: PredN 1, a suspected battlefield permanent; PredN 0, anything
// else, off the battlefield too (gorge IsSuspected / !IsSuspected:
// Object.Suspected, false off the battlefield).
const PredSuspected Pred = 254

// The this-turn records, numbered explicitly clear of the
// iota lists that sibling families append to.

// CtrlHitBySrc is the battlefield Filter.Ctrl "controlled by a player who was
// dealt combat damage this turn by the evaluating ability's source" (gorge
// dealtCombatDamageBySource, Forge ControlledBy
// Player.wasDealtCombatDamageThisTurnBySource). It rides a Ctrl value, as
// CtrlEachPl does, because FilterFlags and FilterFlags2 are full, and it is
// not a Pred so it composes with the mana-value bound beside it (Steel
// Hellkite: Permanent.nonLand+cmcEQX+ControlledBy ...). Matched by the
// source's Obj, so the record outlives the source leaving and returning
// (kernel value_turntallies.go). Only a card marked CardDef.CombatHitRec
// records. Appended after CtrlEachPl.
const CtrlHitBySrc CtrlSel = 4

// PredWasCastEnter: the entering permanent is the spell that is resolving
// right now, read as it enters (gorge WasCast / ObjectWasCastFromGraveyard on
// the object a resolution puts onto the battlefield; kernel
// cast_wascastenter.go). PredN: the origin the cast must have (0 any,
// 1 a graveyard, 2 the permanent's owner's hand, cast by its controller,
// 3 a graveyard by the reading player: wasCastFromYourGraveyard),
// plus 4 for the negation ("!wasCast": not cast, or a copy, or no resolving
// spell). A battlefield read inside the enter only.
const PredWasCastEnter Pred = 248

// PredWasCastSpell: the cast provenance of a spell on the stack, or of the
// spell resolving right now (gorge WasCast, WasCastFromHand[ByYou],
// WasCastFromExile, CastSa Spell.MayPlaySource and the effects-side wasCast
// predicates; kernel cast_wascastspell.go). Numbered explicitly
// beside PredWasCastEnter. PredN low four bits are the
// origin the cast must have: 0 cast at all (a copy was never cast), 1 cast
// from a hand by any caster (wasCastFromYourHand), 2 cast from a hand by
// the reading player (wasCastFromYourHandByYou), 3 cast from exile
// (wasCastFromExile), 4 cast from anywhere but a hand
// (wasCast+!wasCastFromYourHand), 5 cast through a may-play permission
// (CastSa Spell.MayPlaySource, gorge's FlagMayPlay read);
// bit 4 (16) negates the whole read. A spell's origin zone is
// stackEntry.prov (genPrep.anyCastProv).
const PredWasCastSpell Pred = 247

// Player-set predicates (B): OpRepeatEachPlayer and the each-player
// OpDealDamage read an optional predicate from Instr.B.N (0 = none), with
// PlayerPredControlsCreature's creature filter in Instr.F. gorge definedSpec
// (Player.<q> / Opponent.<q>) builds the set once, before the effect acts.
const (
	PlayerPredControlsCreature = 1 // Player.controlsCreature.<spec>: the candidate controls a creature matching F
	PlayerPredMaxSpeed         = 2 // Player.MaxSpeed: speed 4
	PlayerPredCorrupted        = 3 // Player.IsCorrupted: three or more poison counters
	PlayerPredArtOrEnch        = 4 // TokenOwner$ Player.controlsEnchantment,controlsArtifact: the candidate controls an artifact or enchantment (any permanent)
	PlayerPredMostCreatures    = 5 // Player.withMostTypeCreature: the candidate controls at least as many creatures as the other player (a tie keeps both)
	PlayerPredNeg              = 0x100
)
