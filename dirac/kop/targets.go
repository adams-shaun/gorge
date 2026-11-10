package kop

// TargetKind is what a target slot accepts.
type TargetKind uint8

const (
	TgtNone   TargetKind = iota
	TgtObject            // a permanent matching Filter
	TgtPlayer            // a player (Filter.Ctrl selects you/opponent)
	TgtAny               // any target: creature, planeswalker, battle or player (CR 115.4)
	TgtSpell             // a spell on the stack matching Filter
	TgtCard              // a card in a non-battlefield zone matching Filter (Filter.Zone)
	// TgtStackObj: a spell matching Filter, or an activated or triggered
	// ability, on the stack (gorge TargetType$ Spell,Activated,Triggered;
	// kernel tgt_stackability.go): a spell by its object, an
	// ability as StackTarget0 + its stack index.
	TgtStackObj
	// TgtStackAb: an activated or triggered ability on the stack only
	// (TargetType$ Activated,Triggered: Stifle), named as TgtStackObj names one.
	TgtStackAb
)

// TargetSpec is one target slot of an ability.
type TargetSpec struct {
	Kind   TargetKind
	Filter FilterID
	UpTo   bool // "up to one": may be left unchosen

	// Appended by dirac/kgen: a slot that takes several objects.
	Min, Max  uint8       // 0,0 = exactly one (UpTo: zero or one); else Min..Max distinct objects
	MinKicked uint8       // when kicked: Min/Max become MinKicked/MaxKicked (TgfKickedRange)
	MaxKicked uint8       // 255 = any number
	TFlags    TargetFlags // distinct-from-earlier-slots, same/different controllers, ...
	Divide    uint8       // damage divided among the chosen objects (assigned at targeting)
	// DivideV is the divided total when Divide == DivValue: a value evaluated
	// as the slot is announced (gorge divisionTotalFor).
	DivideV ValueRef
	// ParentPlus1 is the 1-based slot of the player target whose graveyard
	// this card slot is restricted to (kgen's TargetsWithDefinedController$
	// ParentTarget); 0 = unrestricted.
	ParentPlus1 uint8
	MaxMV       uint8 // TgfMaxTotalMV: the slot's total mana value cap
	// MinV, MaxV: the bounds when TFlags has TgfBoundsV (a constant side holds
	// kop.Const); Min/Max are unused then.
	MinV, MaxV ValueRef
}

// MaxTargets is the number of target slots an ability may have.
const MaxTargets = 5

// TargetFlags (TargetSpec.TFlags).
const (
	TgfDistinct       TargetFlags = 1 << iota // distinct from every earlier slot's objects
	TgfSameController                         // all the slot's objects share a controller (graveyard: owner)
	TgfDiffController                         // the slot's objects have different controllers
	TgfKickedRange                            // MinKicked/MaxKicked apply when kicked; with Min|Max == 0 and MinKicked > 0 the slot takes no object unless kicked (Tear Asunder's second slot)
	TgfAttachedToT0                           // the object must be attached to target 0's object
	TgfAtResolution                           // chosen mid-resolution at the program's OpChooseTargets, not as the spell is cast (gorge's deferred chain-link targets)
	TgfDecl                                   // a spell's copy target declaration: declared by its root node (or a chosen mode's head), asked again for a copy whose controller may choose new targets (CR 707.10c)
	TgfGiftRange                              // Gift: the slot takes Min..Max objects, MinKicked..MaxKicked when the gift was promised, both literal (0..0 = no target)
	TgfDeclSub                                // a SubAbility$ link's copy target declaration (CR 707.10c, gorge copyChainDeclarations): asked after the root's, in chain order, for a copy whose controller may choose new targets
	TgfBoundsV                                // MinV/MaxV are the slot's bounds, evaluated as the slot is announced (gorge resolvedTargetBounds: TargetMin$/TargetMax$ X, a count, the trigger amount)
	TgfSharedCrType                           // all the slot's objects share a creature type (TargetsWithSameCreatureType$)
	TgfDistinctNames                          // no two of the slot's objects share a name (TargetsWithDifferentNames$)
	TgfMaxTotalMV                             // the objects' total mana value is at most TargetSpec.MaxMV (MaxTotalTargetCMC$)
)

// TargetFlags qualifies a target slot.
type TargetFlags uint16

// TgtCopyAb: an activated or triggered ability on the stack that the chooser
// controls, whose source's card types match Filter (gorge TargetType$
// Activated.YouCtrl,Triggered.YouCtrl; the ability's types are its source's,
// CR 113.7a). Named as TgtStackAb names one (StackTarget0 + its stack index).
const TgtCopyAb TargetKind = 236

// TgtCopyTrig is TgtCopyAb for a triggered ability only (TargetType$
// Triggered.YouCtrl).
const TgtCopyTrig TargetKind = 237

// Counter target shapes and riders.

// TgtStackObjTrig: a spell matching Filter or a TRIGGERED ability on the
// stack (TargetType$ Instant,Sorcery,Triggered; Spider-Sense). An activated
// ability is never a candidate.
const TgtStackObjTrig TargetKind = 238 // a spell matching Filter or a TRIGGERED ability on the stack (TargetType$ Instant,Sorcery,Triggered)

// TgtSpellTgting: a spell whose own object targets include a battlefield
// object matching Filter for the chooser (TargetValidTargeting$; the offer
// only, the resolution rechecks a plain spell target).
const TgtSpellTgting TargetKind = 239 // a spell whose own object targets include a battlefield object matching Filter for the chooser (TargetValidTargeting$; offer only)

// TgtPermOrSpell: a permanent matching Filter, or any
// spell on the stack other than the source (gorge ValidTgts$
// <perm>,Card.inZoneStack with Origin$ Battlefield,Stack and TgtZone$
// Stack,Battlefield: Fatehold Charm, Jeskai Revelation).
const TgtPermOrSpell TargetKind = 235

// TgtPermOrGraveCard: a permanent matching Filter (the TgtObject arm), or a
// card in a graveyard matching Filter (the TgtCard arm): gorge ValidTgts$
// <type> over Origin$ Battlefield,Graveyard (Aurelia's Vindicator).
const TgtPermOrGraveCard TargetKind = 240

// TgtGraveExileCard: a card in a graveyard or in exile matching Filter's Or
// chain, each chain clause carrying its own Zone (gorge changeZoneTargetZones
// over Origin$ Graveyard,Exile: Sorceress's Schemes). The graveyards are
// walked before exile, so a chain names its graveyard clauses first.
const TgtGraveExileCard TargetKind = 241

// TgfSingleTgt (TargetSpec.TFlags): a TgtStackObj slot admits only a spell or
// an ability whose targets are exactly one distinct target (gorge
// TargetType$ SpellAbility.singleTarget, the singleTarget property).
const TgfSingleTgt TargetFlags = 1 << 13
