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
// PredProg (a manifest entry) reads PredN as a FilterProgID: the filter's
// predicate is a program of literals, the conjuncts the one slot cannot hold
// (filterprog.go).
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
	// CtrlTgt, when non-zero, requires the candidate's controller to be the
	// controller of target slot CtrlTgt-1 of the current stack entry: a
	// player target is that player, a stack ability its controller, an object
	// target the controller of its permanent or, once it has left the
	// battlefield, its owner (gorge controlReferentPlayers
	// TargetedOrController). Battlefield only; the card matchers fail
	// closed on it, but for CtrlTgtOwner.
	CtrlTgt uint8
}

// CtrlTgtOwner marks a Filter.CtrlTgt read on the card's OWNER, off the
// battlefield (gorge controlReferentPlayers OwnedBy TargetedController: the
// card's owner is the controller of the target, a card target off the
// battlefield read as its owner): the card matcher compares the owner with
// slot (CtrlTgt&^CtrlTgtOwner)-1; the perm matcher, whose slot it never names,
// fails closed. Nezumi Graverobber.
const CtrlTgtOwner = 0x80

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
	// SpmNonSpell marks a resolving DB$ Mana's CantCastNonArtifactSpells: the
	// batch's other clauses are the artifact spells it may pay, and this one
	// adds every payment that is not a spell (gorge restrictValidTermMatches);
	// never a CR 116.2 special action.
	SpmNonSpell
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
	PredPowerGEV                                            // a battlefield permanent's power >= the value PredV; PredN 1: a spell's power == PredV; PredN 2: power <= PredV, a battlefield permanent's current power (Legacy's Allure's powerLEX) or a card off it by its printed or last known power (Ruthless Technomancer's graveyard powerLEX; dirac/filter_ptvalue.go); PredN 3: a battlefield permanent's power == PredV (Amalia's powerEQ20)
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

// OvChosenType, in an OpOverride's B field: the chosen basic land type
// animation (Navigator's Compass: "target land you control becomes the basic
// land type of your choice in addition to its other types"). C names the
// FIRST of five consecutive Overrides records differing only in AddSub, one
// basic land type each in gorge's option order (Forest, Island, Mountain,
// Plains, Swamp: effects type_choices.go chooseBasicLandTypes). At resolution
// the kernel applies the record whose AddSub holds the type the controller
// chose earlier in the same resolution (OpChooseType with B =
// ChooseTypeBasicLand, res.chosen); gorge combatfx.go animateGrant reads the
// source's ChosenType AT RESOLUTION.
const OvChosenType = 3

// ChooseTypeBasicLand, in a mid-resolution OpChooseType's B field: the ask
// offers the five basic land types (gorge effChooseType Type$ Basic Land,
// CR 205.3i) rather than the creature-type vocabulary. The answer is the
// chosen type's matchup subtype index.
const ChooseTypeBasicLand = 2

// ChooseTypeRandomDeck, in a mid-resolution OpChooseType's B field: the
// AtRandom$ Type$ CreatureInTargetedDeck shape (Aswan Jaguar's enters
// trigger, "choose a random creature type from those in target opponent's
// deck"). gorge effChooseType's AtRandom arm poses no ask: the option list is
// the sorted creature subtypes the resolution's player target's library
// prints, one word is taken whole and two or more draw the engine rng once
// (gorge labels[h.Rand(len(labels))]); the answer is the source's chosen type
// (dirac act_randomdecktype.go). C names the target slot the deck belongs to
// (a kop RefTarget: ValidTgts$ Opponent, the resolution's chosen target).
const ChooseTypeRandomDeck = 3

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
// filter's source; AttToYouCtrl, controlled by the evaluating controller;
// AttToSrcBearer, the bearer is the one the filter's own source is attached
// to (gorge's attachedBy, the EnchantedBy and EquippedBy qualifiers' one
// body, With Great Power and Baki's Curse). Nothing off the battlefield is
// attached.
const W7AttachedTo int16 = 19

// W7AttachedTo's PredN high-byte bits (PredN>>8).
const (
	AttToCreature  = 1
	AttToLand      = 2
	AttToOther     = 4
	AttToYouCtrl   = 8
	AttToSrcBearer = 16
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
//
// PredN>>8 names the referent: 0 the triggering object, 1 the evaluating
// controller's commanders (gorge's "sharesCreatureTypeWith Commander",
// sharesTypeReferentsCommander: Path of Ancestry's rider; any of them, in any
// zone): W7SharesCrCommanderN builds that PredN, Filter.SharesCrCommander reads it.
const W7SharesCrTrigObj int16 = 21

// W7SharesCrCommanderN is the PredN of W7SharesCrTrigObj's commander referent.
func W7SharesCrCommanderN() int { return int(W7SharesCrTrigObj) | 1<<8 }

// SharesCrCommander reports whether f is W7SharesCrTrigObj with the commander
// referent.
func (f *Filter) SharesCrCommander() bool {
	return f.PredN&0xff == W7SharesCrTrigObj && f.PredN>>8 == 1
}

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

// W7WithInfect (PredWord7 sub-kind 28): the card was printed with Infect
// (CardDef.Infect; gorge's exact keyword word withInfect, objectHasKeyword
// reads the printed face). A battlefield permanent that is face down or lost
// all abilities has none; off the battlefield the printed card counts in any
// zone. Corpse Cur, Hand of the Praetors, Burn the Impure.
const W7WithInfect int16 = 28

// W7AttackingEnchTrig (PredWord7 sub-kind 29): a creature the current
// trigger entry's trigger player controls that is attacking the PLAYER the
// evaluating source Aura is attached to (gorge's resolution-time read of the
// Commander curses' second-leg selector `TriggeredAttackingPlayer.Opponent+
// controlsCreature.attacking Player.EnchantedBy`: the controlsCreature clause
// holds when that player controls a creature attacking the source-relative
// Player.EnchantedBy seat). It fails closed with no source (the Aura left
// the battlefield), an unattached source, no trigger entry, a non-creature,
// a creature not attacking, and an attack on a planeswalker or battle.
const W7AttackingEnchTrig int16 = 29

// W7NotBlocking (PredWord7 sub-kind 30): gorge's '!' of the filter word
// blocking (effects/filter.go isBlocking negated): a battlefield permanent
// that is not blocking; a card off the battlefield blocks nothing, so the
// word holds there. FilterFlags has no free bit for it beside FNotAttacking.
// Tromokratis, The Muckslinger Gang.
const W7NotBlocking int16 = 30

// W7BlockingTrigObj (PredWord7 sub-kind 31): gorge's
// blockingTriggeredAttacker, a battlefield creature blocking the current
// entry's trigger object (the become-blocked trigger's attacker), that
// attacker still on the battlefield. No entry, no trigger object, an
// attacker gone and a card off the battlefield fail closed. kgen admits it
// only in an AttackerBlocked trigger's body. General Marhault Elsdragon.
const W7BlockingTrigObj int16 = 31

// W7AttackingSame (PredWord7 sub-kind 32): an attacking creature attacking
// the same defender (player, or planeswalker) the evaluating source attacks
// (gorge's attackingSame, Forge's combat.isAttacking(card,
// getDefenderByAttacker(source))). The source itself matches; a source that
// is not attacking, or gone (NoSlot), leaves the set empty; nothing off the
// battlefield attacks. Kitesail Skirmisher.
const W7AttackingSame int16 = 32

// W7BlockingSrc (PredWord7 sub-kind 33): gorge's blockingSource, a
// battlefield creature blocking the filter's source (the source's own
// BlockedBy list names it). No source on the battlefield and a card off the
// battlefield fail closed. Master of Arms, Cromat.
const W7BlockingSrc int16 = 33

// W7BlockedBySrc (PredWord7 sub-kind 34): gorge's blockedBySource, a
// battlefield creature the filter's source is blocking (the candidate's own
// BlockedBy list names the source). No source on the battlefield and a card
// off the battlefield fail closed. Cromat.
const W7BlockedBySrc int16 = 34

// W7EnteredFromLib (PredWord7 sub-kind 35): W7EnteredFromBF's library
// sibling, gorge's ThisTurnEnteredFrom_Library: the card was put into the
// zone it is in now from a library this turn (the object's latest arrival,
// read off the entered ledger). Read off the battlefield only: kgen refuses
// it on a permanent. The Weaver King, Captain N'ghathrod.
const W7EnteredFromLib int16 = 35

// W7EnchPlayerCtrl (PredWord7 sub-kind 36): a battlefield permanent controlled
// by the player the evaluating source Aura enchants (gorge wordEnchantedPlayerCtrl,
// Forge's EnchantedPlayerCtrl: source.getEntityAttachedTo() is a Player equal
// to the candidate's controller). The seat is the source's atchSeat; a source
// that enchants no player, or is gone (NoSlot), leaves the set empty. Read on
// the battlefield only. Trespasser's Curse.
const W7EnchPlayerCtrl int16 = 36

// W7Snow (PredWord7 sub-kind 37): the card has the printed Snow supertype
// (CardDef.Snow; gorge's hasType Snow reads the current face's printed type
// line). A face-down battlefield permanent answers only its face-down type
// words, so it is never snow; off the battlefield the printed card counts in
// any zone. No lowered card grants or removes Snow, so the printed type is
// exact. Skred, Abominable Treefolk, Priest of the Haunted Edge.
const W7Snow int16 = 37

// W7NonSnow (PredWord7 sub-kind 38): W7Snow's negation, gorge's nonSnow (the
// generic non<type> negation): every object that is not snow, a face-down
// permanent included. Chill to the Bone.
const W7NonSnow int16 = 38

// W7NotImprinted (PredWord7 sub-kind 39): W7Imprinted's negation, gorge's
// '!' of the word (contextPredicateBound refuses to invert only the unbound
// case, and the lowered shape always carries a bound source: the store read
// of a card that keeps its Imprinted list). Mirror of Life Trapping's
// TrigReturn pick (Permanent.ExiledWithSource+!IsImprinted).
const W7NotImprinted int16 = 39

// W7WithSkulk (PredWord7 sub-kind 40): the permanent has skulk, the keyword
// word withSkulk (gorge effects/filter.go keywordPredicates, as withInfect).
// The printed CardDef.Evasion EvSkulk (none face down or after a
// lose-all-abilities effect), or a granted one (a Continuous static's
// Evasion or an ExtKwMark KwMarkSkulk, kw_kwmark.go); off the battlefield the
// printed card in any zone. The Master, Mesmerist.
const W7WithSkulk int16 = 40

// W7WithDevoid (PredWord7 sub-kind 41): the card was printed with Devoid
// (CardDef.Devoid; gorge's exact keyword word withDevoid, objectHasKeyword
// reads the printed face). A battlefield permanent that is face down or lost
// all abilities has none; off the battlefield the printed card counts in any
// zone. A mana restriction's Spell.withDevoid reads it on the card paid for.
// Corrupted Crossroads.
const W7WithDevoid int16 = 41

// W7HasLandwalk (PredWord7 sub-kind 42): the permanent has the landwalk whose
// land filter is PredV's FilterID (the SharedKeywordsZone$ PumpAll lowering,
// dirac/filter_sharedkw.go). Read over the derived keywords: the printed
// CardDef.Landwalk unless it lost its abilities, a Continuous static's
// Static.Landwalk, or an override record's Override.Landwalk.
const W7HasLandwalk int16 = 42

// W7HasProtQuality (PredWord7 sub-kind 43): the permanent has the protection
// quality whose source filter is PredV's FilterID (the SharedKeywordsZone$
// PumpAll lowering, dirac/filter_sharedkw.go). Read over the derived
// keywords: the printed CardDef.ProtFrom unless it lost its abilities, a
// Continuous static's Static.ProtFrom, or an override record's
// Override.ProtFrom.
const W7HasProtQuality int16 = 43

// W7HasProtColor (PredWord7 sub-kind 44): the permanent has protection from
// any colour in PredV's colour mask (the SharedKeywordsZone$ PumpAll lowering,
// dirac/filter_sharedkw.go). Read over the derived keywords: printed
// ProtColors, the KwProtBlack/KwProtRed bits, a chosen-colour grant (colProtOf)
// and a Continuous static's or override record's colour ProtFrom clause.
const W7HasProtColor int16 = 44

// W7HasProtEachColor (PredWord7 sub-kind 45): the permanent has protection
// from every colour ("Protection from each color"), the SharedKeywordsZone$
// PumpAll lowering's family spelling (dirac/filter_sharedkw.go). Read over the
// same derived protection colours as W7HasProtColor, all five required.
const W7HasProtEachColor int16 = 45

// PredPowerLTToughness's PredN: the power/toughness relation it reads. 0 is
// power < toughness (gorge powerLTtoughness); PTRelEQ is gorge's equalPT
// (net power equal to net toughness) and PTRelNE its negation !equalPT, each
// lowered only over a creature-only type head (gorge reads no P/T of a
// non-creature without a printed one).
const (
	PTRelLT int16 = 0
	PTRelEQ int16 = 1
	PTRelNE int16 = 2
)

// PredStatBox's PredN: a power range and a toughness range, one 4-bit bound
// each (bits 0-3 power low, 4-7 power high, 8-11 toughness low, 12-15
// toughness high), every bound inclusive and PTBoxOpen meaning no bound.
const PTBoxOpen = 15

// PTBoxPack packs bounds b (power lo, hi, toughness lo, hi; each 0..14 or
// PTBoxOpen) into a PredStatBox PredN.
func PTBoxPack(b [4]int) int16 {
	return int16(uint16(b[0]&15 | b[1]&15<<4 | b[2]&15<<8 | b[3]&15<<12))
}

// PTBoxUnpack is PTBoxPack's inverse.
func PTBoxUnpack(n int16) [4]int {
	u := int(uint16(n))
	return [4]int{u & 15, u >> 4 & 15, u >> 8 & 15, u >> 12 & 15}
}

// PTBoxHolds reports whether power pw and toughness t lie in box n.
func PTBoxHolds(n int16, pw, t int) bool {
	b := PTBoxUnpack(n)
	return (b[0] == PTBoxOpen || pw >= b[0]) && (b[1] == PTBoxOpen || pw <= b[1]) &&
		(b[2] == PTBoxOpen || t >= b[2]) && (b[3] == PTBoxOpen || t <= b[3])
}

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

// PredHasManaAbility: the object has a mana ability (gorge FaceHasManaAbility:
// an activated CR 605.1a ability, the intrinsic basic-land ability included,
// or a CR 605.1b triggered mana ability). PredN 0 = has one, 1 = !hasManaAbility.
// The card read is CardDef.HasManaAbility, shared with the gorge predicate;
// a face-down permanent or card matches nothing (CR 708.2). Read on the
// battlefield and off it (Moonsilver Key searches a library). Explicit high
// ordinal, filters.go style; 255 is free.
const PredHasManaAbility Pred = 255

// PredTrigPlayerCtrl: a battlefield permanent whose controller is the
// current trigger entry's trigger player, the player a damage trigger's
// damage was dealt to (gorge ControlledBy TriggeredTarget, CR 603.2):
// false with no trigger entry and off the battlefield. Explicit ordinal.
const PredTrigPlayerCtrl Pred = 239

// PredAttackingYou: an attacking battlefield permanent whose defending player
// is the evaluating controller: the attacked player, the controller of the
// attacked planeswalker or the protector of the attacked battle (gorge
// attackingYou: o.Attacking == you; CR 506.2, 508.1b). At any seat count;
// false off the battlefield. Explicit ordinal.
const PredAttackingYou Pred = 235

// PredSuspected: PredN 1, a suspected battlefield permanent; PredN 0, anything
// else, off the battlefield too (gorge IsSuspected / !IsSuspected:
// Object.Suspected, false off the battlefield).
const PredSuspected Pred = 254

// PredRenowned: PredN 1, a renowned battlefield permanent; PredN 0, anything
// else, off the battlefield too (gorge IsRenowned / !IsRenowned:
// Object.Renowned, which the move off the battlefield clears; CR 702.112b).
// The designation is the arena's mechRenowned, set by OpExt ExtRenown
// (dirac/act_renown.go). Explicit ordinal: the free one below PredEscaped.
const PredRenowned Pred = 236

// PredEscaped: PredN 1, a battlefield permanent whose spell was cast for its
// escape cost; PredN 0, anything else, off the battlefield too (gorge
// filter.go's 'escaped' predicate / '!escaped' over
// CastFlags&FlagEscaped, which the move off the battlefield resets). The
// mark is the arena's mechEscaped (kw_escape.go), the shape PredSuspected
// rides; a stack read needs no per-permanent storage (the SpellCast self
// trigger's filter, selfCastFilterOK).
// 237: land's PredHasManaAbility took 255 in the merge; this is the free
// explicit ordinal below the 239+ family and PredSameNameRem (238).
const PredEscaped Pred = 237

// PredSameNameRem: the object shares a printed name with an object in the
// resolution's remembered set (gorge's filter word "sharesNameWith
// Remembered", read by Tainted Pact's Card.sharesNameWith Remembered gate).
// A face-down object has no name (CR 708.2) and matches nothing.
// 238: land's PredHasManaAbility took 255 in the merge; this is the free
// explicit ordinal below the 239+ family.
const PredSameNameRem Pred = 238

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
// (CastSa Spell.MayPlaySource, gorge's FlagMayPlay read), 6 cast for its
// mayhem cost, 7 cast from a graveyard by the reading player
// (wasCastFromYourGraveyard, read only by a mana restriction: Lord of the
// Forsaken), 8 cast from a graveyard, any caster (wasCastFromGraveyard,
// gorge ObjectWasCastFromGraveyard's !IsCopy included: a prepared copy
// reads false; Ash Zealot, Laquatus's Disdain), 9 cast from a library by
// the reading player (wasCastFromYourLibrary, gorge castOriginTokens:
// Melek, Izzet Paragon, Wickerwing Effigy);
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
	PlayerPredHandEmpty        = 6 // Player.Opponent+HasCardsInHand_Card_EQ0: the candidate's hand holds no card
	PlayerPredNeg              = 0x100
)
