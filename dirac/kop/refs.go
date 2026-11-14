package kop

// Ref selects the object or player an Instr acts on. Append-only.
//
// Every Ref has an explicit value: a new one takes the next free value
// (35 up, below the high block that starts at 241); values are pinned by
// testdata/ordinals.tsv, so declarations may sit in any file or order.
type Ref uint8

const (
	RefNone             Ref = 0
	RefTarget0          Ref = 1  // the ability's first chosen target
	RefTarget1          Ref = 2  // second target
	RefTarget2          Ref = 3  // third target
	RefSelf             Ref = 4  // the source object (the spell itself, or the permanent the ability is on)
	RefController       Ref = 5  // the source's controller (player)
	RefOpponent         Ref = 6  // the controller's opponent (2-player)
	RefEachPlayer       Ref = 7  // every player, the resolving controller first (gorge AliveFrom(controller))
	RefEachOpponent     Ref = 8  // every opponent of the controller
	RefTriggerObj       Ref = 9  // the object that caused the trigger (entering creature, attacker, ...)
	RefTriggerPlayer    Ref = 10 // the player the trigger event names (drew, gained life, ...)
	RefAttached         Ref = 11 // the permanent the source Aura/Equipment is attached to
	RefLastCreated      Ref = 12 // the token(s) the previous CreateToken created
	RefChosen           Ref = 13 // the object chosen by the previous ChooseCard
	RefTargetController Ref = 14 // the controller of target 0
	RefDefendingPlayer  Ref = 15 // the defending player (combat triggers)
	RefEach             Ref = 16 // the object bound by the enclosing RepeatEach
	RefTargetOwner      Ref = 17 // the owner of target 0
)

// two player Refs, explicit constants clear of the
// generated Ref enum (kernel filter_defined.go).
const (
	// RefOppNonRemCtrl is gorge's Defined$ OppNonRememberedController: the
	// controller's opponent unless some remembered object's controller (its
	// RememberLKI$ capture off the battlefield, CR 608.2h) is that opponent.
	RefOppNonRemCtrl Ref = 245
	// RefSrcOwner is gorge's Defined$ CardOwner: the resolving source
	// object's owner (CR 108.3).
	RefSrcOwner Ref = 246
)

// More Refs, continuing the low values.
const (
	RefRemembered           Ref = 18 // the resolution's remembered objects (an Exile/Dig with remember)
	RefDelayedRemembered    Ref = 19 // the objects a delayed program was registered with
	RefReplaced             Ref = 20 // the object a replacement program is modifying (it is entering)
	RefTriggerSource        Ref = 21 // the source of the damage that caused the trigger
	RefSacrificed           Ref = 22 // the object sacrificed to pay this ability's cost (last known)
	RefHost                 Ref = 23 // the permanent that granted this ability (an Equipment's granted ability)
	RefTargetSlot           Ref = 24 // reserved
	RefTargets01            Ref = 25 // every legal target of slots 0 and 1, in slot order (gorge's chain target union, Defined$ Targeted; union)
	RefTargets012           Ref = 26 // every legal target of slots 0, 1 and 2
	RefTargets12            Ref = 27 // every legal target of slots 1 and 2 (a charm mode's chain)
	RefRememberedController Ref = 28 // a player ref: the first remembered object's controller (the RememberLKI$ captured one off the battlefield; gorge rememberedControllersLKI)
	RefDiscarded            Ref = 29 // the card the resolving spell's Discard<1/...> cost discarded (stackEntry.trigObj, Grab the Prize)
	RefLibTop               Ref = 30 // the top card of the resolving controller's library (gorge definedSpecTopOrBottomOfLibrary); none when it is empty
	RefTriggerAttackers     Ref = 31 // the attacker group of the AttackersDeclared event that raised this trigger (gorge definedSpecTriggeredAttackers); only OpPumpUntilEOT / OpPutCounter / OpGrantKeyword read it
	RefTargets0123          Ref = 32 // every legal target of slots 0 to 3
	RefTriggerTgtCtrl       Ref = 33 // a player ref: the controller of the triggering referent, read live at resolution (gorge TriggeredTargetController: a referent that left the battlefield reads its owner, CR 603.7a)
	// RefRememberedOwner is a player ref: the first remembered object's
	// OWNER (CR 108.3: an object's owner never changes; gorge definedSpecRememberedOwner
	// over the source's persistent Remembered). Unidentified Hovership's
	// leave trigger manifests dread for the owner of the card its ETB exiled
	// (the RemPersist seed); none when the remembered set is empty.
	RefRememberedOwner Ref = 34
	// RefPromised is a player ref: the opponent the gift was promised to
	// (CR 702.174a; gorge Defined$ Promised, Object.GiftPromisedTo): the
	// spell's election, or a permanent's gift trigger's queue-time
	// snapshot. None when no gift was promised.
	RefPromised Ref = 36
	// RefAttachedCtrl is a player ref: the controller of the permanent the
	// source Aura/Equipment is attached to, read live at resolution (the
	// last known attachment when the source has left). Contaminated Bond's
	// "its controller loses 3 life" for the enchanted creature that blocked
	// (gorge Defined$ TriggeredBlockerController over a Card.AttachedBy
	// Blocks trigger).
	RefAttachedCtrl Ref = 37
	// RefResChosenPlayer is a player ref: the resolution's chosen player, the
	// RANDOM pick OpChoosePlayer recorded (distinct from land's
	// RefChosenPlayerRes = 41, the controller-asked ExtChoosePlayerRes pick) (gorge Ctx.Chosen's player entry, Forge's
	// chosen player field). 255 when that resolution made no pick (gorge
	// effChoosePlayer's random branch with an empty pool records nothing, so
	// a following ChosenPlayer destination walk attaches nothing).
	RefResChosenPlayer Ref = 42
	// RefTriggerObjOwner is a player ref: the owner of the trigger's object
	// (CR 108.3: an object's owner never changes, so the read is the same
	// whether the object is still where it was or has since moved). Gorge
	// Defined$ TriggeredCardOwner; Brainstealer Dragon's "they lose life
	// equal to its mana value". None when the trigger binds no object.
	RefTriggerObjOwner Ref = 40
)

// RefTgtCtrl1 / RefTgtCtrl2 are RefTargetController for target slot 1 / 2
// (a charm mode's own slot): the player a player target names, else the
// target's controller as it last existed (kernel playerRef). High,
// clear of the appended Ref enum.
const (
	RefTgtCtrl1 Ref = 241
	RefTgtCtrl2 Ref = 242
)

// RefTarget3 / RefTarget4 are the fourth and fifth target slots
// (a four-mode Command's last modes). High, clear of the appended Ref enum;
// kernel tgtSlot (tgt_fiveslots.go) maps every RefTarget to its slot.
const (
	RefTarget3 Ref = 243
	RefTarget4 Ref = 244
)
