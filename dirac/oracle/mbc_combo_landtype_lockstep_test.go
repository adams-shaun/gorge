package oracle

import "testing"

// Stirring Wildwood ({T}: Add {G} or {W}) under Yavimaya, Cradle of Growth
// (every land is a Forest) has the granted Forest's deterministic {G} ability
// beside its printed Combo one, since gorge's dedup keeps a printed ability
// only when its Produced$ is exactly the colour. gorge's payment census lists
// such a source with the fixed-production ones at its battlefield position, so
// the activation's {1}{G}{W} taps Forest, Plains and a second Forest; the
// kernel listed it with the choice-shaped sources, last, and tapped the
// Wildwood and two other Forests (tail-11-c000 pair 5, decision 522).
func TestMBCGrimBackwoodsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "combo_land_granted_type_census_order")
}
