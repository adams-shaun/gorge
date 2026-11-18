package oracle

import "testing"

// A "search that player's library" choice must offer the cards of the library
// the rule names. In this recorded game the two engines disagreed at decision
// 275 about whose library a search choice lists: gorge offered one player's
// creature cards, the kernel the other player's.
func TestMBCASpeakeasyServerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "search_offer_owner")
}
