package oracle

import "testing"

// Anje's Ravager's attack trigger discards the hand; a madness card in it
// (Anje's Ravager itself) is exiled, and the owner is asked to cast it for its
// madness cost. The two engines once differed on whether "yes" was offered
// (decision 946 of tail-41-c001 pair 76).
func TestMBCAnjeSRavagerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "madness_cast_offer_ravager_hand_discard")
}
