package oracle

import "testing"

// Child of Gaea's modes decision (tail-12-c000 pair 6, decision 533 of 533):
// gorge offered mode 0 alone where the kernel also offered mode 1. A later
// main no longer parts on this game; the recording pins that it replays clean.
func TestMBCChildOfGaeaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "child_of_gaea_mode_offer")
}
