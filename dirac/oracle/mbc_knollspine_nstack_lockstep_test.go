package oracle

import "testing"

// Knollspine Invocation's "{X}, Discard a card with mana value X: Knollspine
// Invocation deals X damage to any target": the announced X is answered
// before the target. Gorge judges the paid-X discard at that X and reverses
// the activation before it asks a target (NStack 0); the kernel used to ask
// the target first and only discover the short discard at the cost stage,
// one decision later with the ability still on the stack (NStack 1).
// Recorded from the tail-53 focus pool (pair 4); its X = 1 has no card of
// mana value 1 in hand.
func TestMBCKnollspineNStackReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "knollspine_invocation_nstack")
}
