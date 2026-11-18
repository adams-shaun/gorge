package oracle

import "testing"

// Tolarian Terror (dr-mbc-baaa pair 166, decision 761): two abilities on the stack
// disagree on their order between gorge and dirac.
func TestMBCTolarianTerrorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tolarian_terror_ward_trigger_order")
}
