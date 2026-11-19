package oracle

import "testing"

// Lavaclaw Reaches' {1}{B}{R} animation, with Energy Refractor's "{2}: Add one
// mana of any color" the controller's only black source (tail-02 pair 2,
// decision 1163 of 1163): gorge and dirac must agree on whether it is offered.
func TestMBCLavaclawReachesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lavaclaw_animate_fee_filter_black")
}
