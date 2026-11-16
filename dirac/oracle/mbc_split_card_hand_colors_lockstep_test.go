package oracle

import "testing"

// CR 709.4 with CR 709.4b: in every zone but the stack and the battlefield a
// split card has the characteristics of both its halves combined, so Leave //
// Chance in a hand is a white and a red card. Cave-In's alternative cost
// exiles a red card from the hand: gorge offered the combined Leave // Chance
// as fodder and the kernel, reading only the front half's white, did not. The
// recording is the first divergence of tail-73 on the Cave-In and Leave focus,
// pair 28: options at decision 3, gorge leading, on the autopay surface.
func TestMBCLeaveReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_split_card_hand_colors")
}
