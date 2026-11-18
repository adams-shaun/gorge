package oracle

import "testing"

// CR 702.193b: Goliath, Mass Manipulator's Power-up ability ({4}{G}) costs
// less by his mana cost ({1}{G}) on the turn he entered. gorge offered the
// activation at decision 661 of tail-19 pair 29 (seed 207208, autopay) while
// dirac did not.
func TestMBCGoliathMassManipulatorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "powerup_goliath_entry_turn_offer")
}
