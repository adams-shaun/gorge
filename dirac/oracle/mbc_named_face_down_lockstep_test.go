package oracle

import "testing"

// A named<X> filter whose X no object of the matchup prints (Rock Lobster's
// "creatures named Scissors Lizard can't attack or block") matches nothing.
// The kernel read the absent name as NoCard, which is also the name of a
// face-down permanent (CR 708.2), so a manifested Swamp counted as named and
// Rock Lobster stopped it attacking; gorge offered it (decision 912, pair 105
// of the tail-14 autopay pool).
func TestMBCTail14ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "named_absent_card_face_down")
}
