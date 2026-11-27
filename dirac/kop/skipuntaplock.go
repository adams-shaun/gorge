package kop

// OpSkipUntap C values past the controller's next-untap-step forms (C 0 the
// permanent Obj, C 1 the controller's lands): a lock on the permanent Obj
// that lasts for as long as the resolving ability's source stays on the
// battlefield under the player who resolved it (gorge Effect Duration$
// AsLongAsControl with an Event$ Untap | Layer$ CantHappen |
// ValidCard$ Card.IsRemembered replacement and ForgetOnMoved$ Battlefield;
// CR 611.2b). It ends for good when the source changes controller and when
// Obj leaves the battlefield.
const (
	SkipUntapLockStep = 2 // Obj doesn't untap during its controller's untap step (ValidStepTurnToController$ You: Ty Lee, Chi Blocker)
	SkipUntapLockAny  = 3 // Obj can't become untapped at all, in the untap step or by an effect (no ValidStepTurnToController$: Spider-Woman, Secret Agent)

	// SkipUntapLockHostStep is the untap-step lock of SkipUntapLockStep that lasts for as
	// long as the source is on the battlefield under any controller (gorge
	// Duration$ AsLongAsInPlay or UntilHostLeavesPlay: Somnophore, Tidebinder
	// Mage, Origin of Iron Man). It ends only when the source or Obj leaves.
	SkipUntapLockHostStep = 5

	// SkipUntapLockUntapStep is the untap-step lock of SkipUntapLockHostStep that
	// lasts only for as long as the source remains tapped (gorge Duration$
	// UntilUntaps: Amber Prison, Ice Floe, Rust Tick). The source's first untap
	// ends it for good, as does its departure; a retapped source never revives
	// it (CR 611.2b).
	SkipUntapLockUntapStep = 6
)

// SkipUntapPlayerCL is the OpSkipUntap C value for the player Obj's creatures
// and lands (gorge Effect with an UntapDenial restriction, Affected$
// Creature.RememberedPlayerCtrl,Land.RememberedPlayerCtrl; Exhaustion): the
// permanents of those types that player controls at their next untap step stay
// tapped through it, whoever cast the effect.
const SkipUntapPlayerCL = 4

// SkipUntapPlayerCreatures is the OpSkipUntap C value for the player Obj's
// creatures alone (gorge Effect with an Event$ Untap | Layer$ CantHappen |
// ValidCard$ Creature | ValidStepTurnToController$ Player.IsRemembered
// replacement; Blinding Beam): the creatures that player controls at their next
// untap step stay tapped through it, lands unaffected.
const SkipUntapPlayerCreatures = 7
