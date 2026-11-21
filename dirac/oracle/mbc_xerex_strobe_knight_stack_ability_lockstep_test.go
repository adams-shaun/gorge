package oracle

import "testing"

// CR 707.2/707.9b: Cryptoplasm's upkeep copy became a copy of Battle Angels
// of Tyr and kept its own upkeep trigger (GainThisAbility$ True). Attacking
// fires the copied card's Myriad keyword trigger off the copy, and the stack
// entry must project the same ability identity on both engines (the class the
// lockstep calls projection Stack[0].Ability).
func TestMBCXerexStrobeKnightReprosLockstep(t *testing.T) {
	stdReproNamed(t, "xerex_strobe_knight_stack_ability")
}
