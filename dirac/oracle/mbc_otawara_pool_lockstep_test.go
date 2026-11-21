package oracle

import "testing"

// Otawara, Soaring City's channel ability costs {1} less for each legendary
// creature its controller controls; the mana pool after paying it must match
// between gorge and dirac (dr-mbc-abbb pair 1, decision 451, Pool[0][1]).
func TestMBCMinamoSchoolAtWaterSEdgeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "otawara_cost_reduction_pool")
}
