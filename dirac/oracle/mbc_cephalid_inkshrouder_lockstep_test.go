package oracle

import "testing"

// CR 702.26b: a phased-out permanent is treated as though it does not exist,
// so it can't be attacked (CR 506.3, 508.1b). Kaito Shizuki phases out at its
// controller's end step the turn it enters, and stays out through the
// opponent's turn; gorge still offered it as a planeswalker to attack beside
// the player (decision 616 of tail-27 pair 6, Cephalid Inkshrouder attacking).
// The recording replays on custom: pool tail-27 has since re-dealt around a
// newer corpus and none of its current decks hold the game's cards.
func TestMBCCephalidInkshrouderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phased_out_planeswalker_attack_target")
}
