package oracle

import "testing"

// CR 605.1a, 601.2g: an Eldrazi Scion token's "Sacrifice this creature: Add
// {C}" is a last-resort unit of gorge's payment-planner census (pay
// PlanLastResortCostOK), not a census gap. Beside Shimmering Grotto's
// "{1}, {T}: Add one mana of any color" the board is still filter-only, so
// gorge's prefix search finds Ipnu Rivulet's {1}{U} activation (a Swamp
// funds the Grotto's {U}, the other pays the {1}). The kernel once counted
// the Scion as a non-filter gap, ran no filter search, and offered only pass
// (tail-10 pair 189, decision 304).
func TestMBCIpnuRivuletReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sac_only_mana_beside_filter")
}
