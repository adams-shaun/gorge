package oracle

import "testing"

// Incubation Druid's reflected Amount$ is a census gap in gorge's planner, and
// gorge's ability offer pays an activation from the planner alone: a board whose
// only {G} source is a counter-bearing Druid (its Forests tapped) beside a
// Cascade Bluffs offers Whip Silk's "{G}: Return Whip Silk to its owner's hand"
// in the kernel (the filter script's search tapped the Druid) but not in gorge
// (decision 480 of tail-55-c000 pair 44).
func TestMBCWhipSilkReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "whip_silk_druid_script_pays")
}
