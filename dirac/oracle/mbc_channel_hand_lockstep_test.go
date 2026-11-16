package oracle

import "testing"

// Colossal Skyturtle's channel abilities (CR 702.xxx-style activation from hand)
// were offered by only one engine at a priority decision (dr-mbc-abba pair 141).
func TestMBCColossalSkyturtleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "channel_from_hand_offered")
}
