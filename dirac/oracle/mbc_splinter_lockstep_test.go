package oracle

import "testing"

// CR 603.2d: Teysa Karlov doubles a triggered ability that a creature dying
// causes, and Splinter, the Mentor's own "leaves the battlefield" trigger is
// one when Splinter dies. gorge queues the trigger twice and asks the order
// of both; dirac once offered only one (pair 1 of tail-15 seed 242677,
// decision 308).
func TestMBCSplinterTheMentorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "teysa_doubles_dying_leaves_trigger")
}
