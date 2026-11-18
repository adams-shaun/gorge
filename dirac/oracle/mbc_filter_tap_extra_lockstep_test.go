package oracle

import "testing"

// CR 106.12a, 605.1b: a Wolfwillow Haven on a Desolate Mire ({1},{T}: Add {W}{B})
// adds {G} when the Mire taps for mana, so the controller's {4}{W}{W} Farewell is
// affordable from four lands and the Mire. gorge's potential bound folds the tap's
// extra for every ability that taps (potential.go addPotentialTapExtra), a filter's
// included; the kernel folded it only for the printed ability and left Farewell out
// of the cast offers (tail-75 pair 2, decision 439).
func TestMBCFarewellReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_land_tap_extra_potential")
}
