package oracle

import "testing"

// TestMBCMinamoSchoolAtWaterSEdgeReprosLockstep: at a priority decision (pool
// dr-mbc-abbb, pair 1, seed 609670) the kernel offered Minamo, School at
// Water's Edge's "{U}, {T}: Untap target legendary permanent", payable only
// through Starfleet Academy's "Federation — {5}, {T}" filter, while gorge's
// prefix search ran out of its 160-node budget first. Three Tree City's
// "{2}, {T}" filter resolves to zero there (no creature of the chosen type),
// so it is no census gap, but gorge's funding pass still asks which of its
// abilities to activate whenever the pool pays {2}, and branches the dead
// filter's colour ask: nodes the kernel's funding pass did not spend
// (dirac/mana_filtzerofund.go).
func TestMBCMinamoSchoolAtWaterSEdgeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "act_script_funds_nonmana_activation")
}
