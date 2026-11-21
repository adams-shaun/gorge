package oracle

import "testing"

// CR 118.5 / 117.3: Horrid Shadowspinner's "you may draw cards equal to its
// power" is a cost, and a cost of zero cards (its power shrunk to 0 by Elesh
// Norn, Grand Cenobite's -2/-2) is still payable. Gorge offered pay and
// decline; the kernel priced Draw<X/You> at X = 0 as unpayable and offered
// decline only (decision 633, tail-36 pair 38 seed 158018).
func TestMBCHorridShadowspinnerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "horrid_shadowspinner_zero_power_draw_cost")
}
