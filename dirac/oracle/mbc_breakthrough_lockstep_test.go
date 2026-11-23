package oracle

import "testing"

// Breakthrough ({X}{U}) cast with Island and Gemstone Caverns available: the
// kernel offers X=0 and X=1, gorge offered only X=0 (decision 60, tail-04-c000
// pair 3 at main 9b3d0f9f8d).
func TestMBCBreakthroughReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "breakthrough_x_offer_with_gemstone_caverns")
}
