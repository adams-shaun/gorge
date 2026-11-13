package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
)

// manaGrantConditionHolds reports whether a Continuous static's own existence
// condition lets its AddAbility$ mana member join the authoritative mana
// membership (appendAvailableManaAbilitiesGate's direct scan). The scan is the
// one home both the pass-scoped offer walk and the fresh activation/payment
// readers go through, so gating here keeps them in agreement; ignorePayable
// skips payability only and never reaches this gate.
//
// A "Max speed --" grant (CR 702.179e, Condition$ MaxSpeed) exists only while
// the static's CONTROLLER has max speed. This scan reads the condition itself
// rather than going through the layer walk, which deliberately does not emit a
// MaxSpeed AddAbility$ (engine/rules/speed.go's maxSpeedAbilities owns the `granted`
// priority offer). An IsPresent$ / IsPresent2$ gate (Forge's
// StaticAbility.checkConditions: Joraga Treespeaker's "LEVEL 1-4" band,
// counters_GE1_LEVEL+counters_LE4_LEVEL) is read through presentGate, the one
// reader the layer walk's continuousGateHolds uses. Every other Condition$
// keeps the scan's pre-existing behaviour: not read here.
func (e *Engine) manaGrantConditionHolds(sv staticView) bool {
	if spec, ok := sv.Param(cards.PKIsPresent); ok && !e.presentGate(sv, spec) {
		return false
	}
	if spec, ok := sv.Param(cards.PKIsPresent2); ok && !e.presentGate(sv, spec) {
		return false
	}
	if !isMaxSpeedCondition(sv.ParamStr(cards.PKCondition)) {
		return true
	}
	return int(sv.Controller) < len(e.G.Players) && e.G.Players[sv.Controller].Speed >= maxSpeed
}
