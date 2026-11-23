package oracle

import "testing"

// Scourge Devil's unearth (CR 702.84a) gives it haste until it leaves; the
// two engines must agree on its keyword set through the unearth round trip
// (dr-mbc-babb pair 124, decision 471: gorge kept 0, dirac showed 256).
func TestMBCScourgeDevilReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "scourge_devil_unearth_keywords")
}
