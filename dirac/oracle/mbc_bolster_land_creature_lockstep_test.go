package oracle

import "testing"

// CR 701.39a with CR 613.1d: bolster chooses among the creatures its
// controller has, read after the layers. Dragonscale General's end-step
// bolster runs with two lands Toph, Earthbending Master earthbent into 1/1
// land creatures beside Master Trinketeer, Teshar and two Servos (all
// toughness 2), so the election is between the two lands. gorge once read
// the printed type line, skipped the lands and offered the 2-toughness
// creatures (decision 868 of tail-30 pair 7).
func TestMBCMasterTrinketeerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bolster_elects_among_land_creatures")
}
