package oracle

import "testing"

// A buyback cast is not a kicked spell: Mind Peel cast with its buyback cost
// paid doesn't trigger Risen Riptide's "whenever you cast a kicked spell".
// gorge flags the paid cost FlagBuyback, not FlagKicked, so the stack kept the
// priority pass; dirac rides buyback on the kicked flag and queued both
// Riptide triggers for ordering (decision 354, tail-24 pair 28).
func TestMBCMindPeelReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "buyback_cast_is_not_a_kicked_spell")
}
