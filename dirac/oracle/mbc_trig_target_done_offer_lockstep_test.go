package oracle

import "testing"

// A convoked spell's up-to target ask (Wheel and Deal, "any number of target
// opponents", cast with Flying Dolphin-Fish and Phantasmal Sphere convoked)
// prices its empty answer net of the announced picks: gorge offers done, dirac
// priced the full cost against the lands alone and withheld it (options
// options, lead gorge; kfuzz implicates the convoked Phantasmal Sphere).
// Synthetic pool tail-32, pair 2, surface full, seed 112351.
func TestMBCPhantasmalSphereReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trig_target_done_offer")
}
