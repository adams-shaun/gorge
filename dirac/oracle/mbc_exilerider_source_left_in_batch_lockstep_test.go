package oracle

import "testing"

// Two opposing Kalitas, Traitor of Ghet: each side's Kalitas and an enemy
// creature die in the same batch, and each death is replaced by the other
// Kalitas's dies-to-exile rider ("exile it instead ... create a 2/2 black
// Zombie"). A replacement's source may leave the battlefield earlier in the
// same batch than the death it replaces (CR 614.12: the replacements are read
// off the pre-batch board), and gorge still resolves that rider in full
// (tail-18-c000 pair 7, seed 325460 -> game seed 7938099482600704976,
// decision 418): the kernel once stalled on it instead (StallExileRider).
func TestMBCTail18ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "exilerider_source_left_in_batch")
}
