package oracle

import "testing"

// CR 702.81a with CR 107.3: Six gives a graveyard Lifeblood Hydra ({X}{G}{G}{G})
// retrace, and the cast pays X, the printed cost and the discarded land. Gorge
// offers that cast; the kernel once withheld a granted retrace on any {X} card
// and ended the game in the unmodelled retracegrant stall (tail-58 pair 6,
// autopay, mulligans 1).
func TestMBCTail58ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "retrace_grant_x_cost")
}
