package oracle

import "testing"

// Monk of the Open Hand's cast is offered at a priority decision by gorge and
// withheld by dirac (decision 140, dr tail-03-c000 pair 9).
func TestMBCMonkOfTheOpenHandReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "monk_of_the_open_hand_cast_offer")
}
