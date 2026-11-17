package oracle

import "testing"

// TestMBCDrMbcAaabReprosLockstep replays a dr-mbc-aaab game in which two
// damage replacement effects match one damage event. gorge asks its controller
// for the replacement order; dirac must model the same choice instead of
// stalling at prevnext.
func TestMBCDrMbcAaabReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "prevnext_shield_order")
}
