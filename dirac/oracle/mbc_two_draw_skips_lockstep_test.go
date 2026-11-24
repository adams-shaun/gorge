package oracle

import "testing"

// CR 616.1 with CR 614.1b: two Null Profusions on the active player's side
// each skip the draw step, and applying either skips it whole, so the order
// choice changes nothing. gorge posed it anyway (decision 630, "replacement",
// with the step still at upkeep) while the kernel composes the skips in scan
// order and had entered the first main phase.
func TestMBCBrotherhoodPatriarchReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "two_draw_skips_order_ask")
}
