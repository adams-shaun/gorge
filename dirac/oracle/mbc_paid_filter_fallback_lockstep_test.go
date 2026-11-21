package oracle

import "testing"

// CR 605.3b, 602.2b: Minamo, School at Water's Edge's "{U}, {T}: Untap target
// legendary permanent" beside Starfleet Academy's "{5}, {T}: Add X mana of one
// color" (X the creature types among the player's creatures) and Three Tree
// City, whose chosen type matches no creature. The only {U} is the filter's.
// Gorge's prefix search spends its 160 nodes without finding that line and its
// planner never returns a Plan through a paid filter, so gorge does not offer
// the activation; the kernel's capped fallback walk found the line and offered
// it (decision 553 of 553; dr-mbc-abbb pair 23).
func TestMBCMinamoSchoolAtWaterSEdgeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "paid_filter_fallback_is_no_planner_witness")
}
