package oracle

import "testing"

// CR 702.26b: a phased-out permanent is treated as though it does not exist,
// so it is no creature to tap for convoke (CR 702.51a). Clever Concealment
// phased out p1's Court Homunculus; a second Clever Concealment then had only
// three creatures to convoke, and gorge still credited the Homunculus, pricing
// the cast at nothing for the empty floating pool.
// The recording is on a custom pool, the table its own decks need: the tail
// buckets a synthetic pool deals re-cut whenever the lowered card set moves.
func TestConvokeSkipsAPhasedOutCreatureLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "convoke_skips_phased_out_creature")
}
