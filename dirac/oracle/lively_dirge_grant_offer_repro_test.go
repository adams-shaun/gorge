package oracle

import "testing"

// TestReproLivelyDirgeGrantOffer replays dr-mbc-bbbb pair 13: p0 holds
// Lively Dirge ({1}{B}, Spree) on a board with Abundant Growth's any-colour
// grant on an untapped Forest. gorge's potential walk lists the cast, but its
// planner declines the face ("shape:modal_cost") and prices the printed cost
// only when a legal mode adds nothing (potentialModalCastFreeMode); both of
// Lively Dirge's modes cost mana, so gorge never offers it. The kernel must
// mirror that per-play gate rather than lower the cast from the grant board.
func TestReproLivelyDirgeGrantOffer(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lively_dirge_grant_offer")
}
