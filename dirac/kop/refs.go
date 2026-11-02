package kop

// Ref selects the object or player an Instr acts on. Append-only.
type Ref uint8

const (
	RefNone             Ref = iota
	RefTarget0              // the ability's first chosen target
	RefTarget1              // second target
	RefTarget2              // third target
	RefSelf                 // the source object (the spell itself, or the permanent the ability is on)
	RefController           // the source's controller (player)
	RefOpponent             // the controller's opponent (2-player)
	RefEachPlayer           // every player, the resolving controller first (gorge AliveFrom(controller))
	RefEachOpponent         // every opponent of the controller
	RefTriggerObj           // the object that caused the trigger (entering creature, attacker, ...)
	RefTriggerPlayer        // the player the trigger event names (drew, gained life, ...)
	RefAttached             // the permanent the source Aura/Equipment is attached to
	RefLastCreated          // the token(s) the previous CreateToken created
	RefChosen               // the object chosen by the previous ChooseCard
	RefTargetController     // the controller of target 0
	RefDefendingPlayer      // the defending player (combat triggers)
	RefEach                 // the object bound by the enclosing RepeatEach
	RefTargetOwner          // the owner of target 0
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

// More Refs, continuing the iota list.
const (
	RefRemembered           Ref = iota + RefTargetOwner + 1 // the resolution's remembered objects (an Exile/Dig with remember)
	RefDelayedRemembered                                    // the objects a delayed program was registered with
	RefReplaced                                             // the object a replacement program is modifying (it is entering)
	RefTriggerSource                                        // the source of the damage that caused the trigger
	RefSacrificed                                           // the object sacrificed to pay this ability's cost (last known)
	RefHost                                                 // the permanent that granted this ability (an Equipment's granted ability)
	RefTargetSlot                                           // reserved
	RefTargets01                                            // every legal target of slots 0 and 1, in slot order (gorge's chain target union, Defined$ Targeted; union)
	RefTargets012                                           // every legal target of slots 0, 1 and 2
	RefTargets12                                            // every legal target of slots 1 and 2 (a charm mode's chain)
	RefRememberedController                                 // a player ref: the first remembered object's controller (the RememberLKI$ captured one off the battlefield; gorge rememberedControllersLKI)
	RefDiscarded                                            // the card the resolving spell's Discard<1/...> cost discarded (stackEntry.trigObj, Grab the Prize)
	RefLibTop                                               // the top card of the resolving controller's library (gorge definedSpecTopOrBottomOfLibrary); none when it is empty
	RefTriggerAttackers                                     // the attacker group of the AttackersDeclared event that raised this trigger (gorge definedSpecTriggeredAttackers); only OpPumpUntilEOT / OpPutCounter / OpGrantKeyword read it
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
