package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() {
	// AssignNoCombatDamage: CR 510.1's "assigns no combat damage" (Ophidian,
	// the Laccolith family, Master of Cruelties), read by combatDamageAmount,
	// the one assignment-amount read every combat damage site makes.
	effects.RegisterNonAPI("stat:" + effects.ModeAssignNoCombatDamage)
}

// assignsNoCombatDamage reports whether an AssignNoCombatDamage static
// applies to creature id: a printed S: static through the assignment-static
// walk (ValidCard$ resolved against the candidate, the shared gates first),
// or the continuous restriction an Effect registered (Ophidian's
// Card.EffectSource, the Laccolith family's Card.IsRemembered), resolved by
// restrictionApplies against the registration's own capture.
func (e *Engine) assignsNoCombatDamage(id state.ObjID) bool {
	for _, sv := range e.assignmentStatics("AssignNoCombatDamage") {
		if !e.restrictionGateHolds(sv, id) || !e.checkSVarHolds(sv) {
			continue
		}
		if e.matchesSpec(sv.ParamStr(cards.PKValidCard), id, e.assignmentStaticSpecCtx(sv)) {
			return true
		}
	}
	for i, active := 0, e.active(); i < len(active); i++ {
		ce := &active[i]
		if ce.Restriction == effects.ModeAssignNoCombatDamage && e.restrictionApplies(ce, id) {
			return true
		}
	}
	return false
}
