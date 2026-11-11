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
)

// SkipUntapPlayerCL is the OpSkipUntap C value for the player Obj's creatures
// and lands (gorge Effect with an UntapDenial restriction, Affected$
// Creature.RememberedPlayerCtrl,Land.RememberedPlayerCtrl; Exhaustion): the
// permanents of those types that player controls at their next untap step stay
// tapped through it, whoever cast the effect.
const SkipUntapPlayerCL = 4
