package oracle

import "testing"

// Fact or Fiction's unchosen pile goes to the graveyard one card at a time
// (CR 700.3), and with two Rest in Peace on the battlefield each move has two
// replacements to order (CR 616.1): gorge asks the card's owner for every
// card, and the kernel asks it too instead of stalling at replleave.
func TestPileSplitTwoRestInPeaceAsksEachCardLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "pile_split_two_rest_in_peace_asks_each_card")
}
