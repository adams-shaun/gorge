package kgen

import (
	"strings"
)

// asgnGrant lowers an Effect-delivered combat-damage-assignment static over
// the ability's remembered object (ps is the Effect node, sp the static body):
// Mode$ CombatDamageToughness (Plagon, Lord of the Beach: "assigns combat
// damage equal to its toughness rather than its power this turn", CR 510.1)
// and Mode$ AssignCombatDamageAsUnblocked (Outmaneuver: "assign their combat
// damage this turn as though they weren't blocked", CR 510.1c-d), both to the
// kernel's ExtAsgnGrant record on the object (dirac/combat_asgngrant.go).
//
// gorge registers either as a continuous effect scoped to Card.IsRemembered
// that lasts the turn (an absent Duration$ is this turn, effEffect's
// absentDurationMeansThisTurn and effectUntilEOT); a record tied to the
// object's version ends when the permanent leaves (gorge's ForgetOnMoved$
// Battlefield sweep, which the CombatDamageToughness arm registers; the
// as-unblocked arm reads no ForgetOnMoved$ but its restriction resolves
// Card.IsRemembered against a battlefield attacker, so a new object is never
// covered either way). An Optional$ True body is the controller's election
// (gorge asUnblockedNeeding), which the kernel has no ask for: refused.
func (lc *lowerCtx) asgnGrant(ps, sp *params, mode string, obj lRef) error {
	if obj.Kind != rTarget && obj.Kind != rRemembered {
		return refuse("Effect %s over %s", mode, obj)
	}
	if v := strings.TrimSpace(sp.str("ValidCard")); v != "Card.IsRemembered" {
		return refuse("Effect %s ValidCard$ %q", mode, v)
	}
	sp.consume("ValidCard")
	c := 0
	if mode == "AssignCombatDamageAsUnblocked" {
		c = 1
	} else if strings.TrimSpace(ps.str("ForgetOnMoved")) == "Battlefield" {
		ps.consume("ForgetOnMoved")
	}
	lc.emit(lInstr{Op: "AsgnGrant", Obj: obj, C: c, Note: "until end of turn the object assigns its combat damage by toughness (C 0) / as though unblocked (C 1)"})
	return nil
}
