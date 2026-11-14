package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
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
// counters_GE1_LEVEL+counters_LE4_LEVEL) is read through presentGatesHold, which
// mirrors the layer walk's continuousGateHolds reads; present is the
// engine's presentGate. Every other Condition$ keeps the scan's pre-existing
// behaviour: not read here.
func manaGrantConditionHolds(g *state.Game, sv staticView, present func(staticView, string) bool) bool {
	if !presentGatesHold(sv, present) {
		return false
	}
	if !isMaxSpeedCondition(sv.ParamStr(cards.PKCondition)) {
		return true
	}
	return int(sv.Controller) < len(g.Players) && g.Players[sv.Controller].Speed >= maxSpeed
}

// presentGatesHold reports whether a static's IsPresent$ and IsPresent2$
// gates (Forge's StaticAbility.checkConditions) both hold, each read through
// present. An absent gate holds.
func presentGatesHold(sv staticView, present func(staticView, string) bool) bool {
	for _, k := range [...]cards.ParamKey{cards.PKIsPresent, cards.PKIsPresent2} {
		if spec, ok := sv.Param(k); ok && !present(sv, spec) {
			return false
		}
	}
	return true
}
