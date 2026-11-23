package kgen

import "testing"

// The static mode port of StatAssignCombatDamageAsUnblocked ("you may assign
// <this>'s combat damage as though it weren't blocked", CR 510.1c): the mode's
// census carriers lower AND BUILD. A card listed here unlocks exactly when the
// named static kind lowers beside everything else it prints, so the
// precondition (every listed card refuses only at the static today) is what
// the RED run pins. gorge registers the kind
// (engine/rules/statics_assignment.go); the kernel side is dirac's
// StatAssignCombatDamageAsUnblocked record (dirac/combat_asgngrant.go's
// asUnblocked election arm).
func TestCombatDamageAssignLower(t *testing.T) {
	for _, c := range lowerPoolBuilt(t, []string{
		"Thorn Elemental", "Rhox", "Siege Behemoth",
	}) {
		for _, r := range c.refusals {
			t.Errorf("%s: refused: %s", c.name, r.Reason)
		}
	}
}
