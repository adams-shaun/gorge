package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// activationHeadKeys are the parameters lowerAbility's AB arm reads off an
// activated ability's head before it lowers the chain (lower.go): the
// activation cost and its restrictions and gates, none of them the body's.
// A key the arm never reads (ActivationPhases$, Forecast$, ...) is not here:
// the real lowering leaves it unread at the head node too.
var activationHeadKeys = [...]string{
	"Activation", "ActivationLimit", "ActivationZone", "CantCopy", "CheckSVar",
	"Cost", "Exhaust", "GameActivationLimit", "IsPresent", "Planeswalker",
	"PlayerTurn", "PowerUp", "PresentCompare", "PresentDefined", "PresentPlayer",
	"PresentZone", "ReduceCost", "SVarCompare", "SorcerySpeed", "XColor",
}

// probeHeadParams is the head node's params for probeBody's read of a
// refused activated ability: the activation keys counted read, as the AB
// arm hands them to chain.
func probeHeadParams(sa *cards.SA) *params {
	p := newParams(sa.Params)
	p.consume(activationHeadKeys[:]...)
	return p
}
