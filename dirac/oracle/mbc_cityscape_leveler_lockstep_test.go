package oracle

import "testing"

// Cityscape Leveler's unearth {8} offered at a priority decision: dirac listed
// activate(#55{Cityscape Leveler}) where gorge did not (tail-41 pair 76,
// decision 354, p1 holding Blinkmoth Well and two castable creatures).
func TestMBCCityscapeLevelerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unearth_cityscape_leveler_offer")
}
