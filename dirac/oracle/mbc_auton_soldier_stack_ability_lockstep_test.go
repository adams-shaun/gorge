package oracle

import "testing"

// CR 707.9b: Auton Soldier enters as a copy of a creature "except it has
// myriad". Attacking fires that granted Myriad trigger, and the stack entry
// must project the same ability identity on both engines (the class the
// lockstep calls projection Stack[0].Ability).
func TestMBCAutonSoldierReprosLockstep(t *testing.T) {
	stdReproNamed(t, "auton_soldier_stack_ability")
}
