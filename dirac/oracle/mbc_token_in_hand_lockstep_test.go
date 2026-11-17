package oracle

import "testing"

// CR 111.1 and CR 111.7: a token returned to its owner's hand by Flood of
// Tears is not a card (and ceases to exist at the next state-based check), so
// "put a permanent card from your hand onto the battlefield" can't pick it.
// Gorge's hand walk offered the token beside the real cards; dirac did not
// (decision 662 of 662, tail-12 pair 91).
func TestMBCFloodOfTearsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "token_in_hand_not_a_card")
}
