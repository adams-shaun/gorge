package oracle

import "testing"

// CR 702.26b and CR 702.51a: a phased-out permanent is treated as though it
// does not exist, so it is no creature to tap for convoke. In this recording
// Clever Concealment phased out p1's Court Homunculus; a second Clever
// Concealment then had only three creatures to convoke, and gorge still
// credited the Homunculus, pricing the cast at nothing for the empty floating
// pool and offering the cast where the kernel offered only pass. The
// recording is on a custom pool, the table its own decks need: the tail
// buckets a synthetic pool deals re-cut whenever the lowered card set moves.
func TestMBCCleverConcealmentReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "clever_concealment_options")
}
