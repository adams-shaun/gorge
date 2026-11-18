package oracle

import "testing"

// Lonely Sandbar's cycling ({U}) is offered from hand at a priority decision
// by gorge but not by the kernel (decision 140 of the recording, tail-45
// autopay pair 198).
func TestMBCLonelySandbarReprosLockstep(t *testing.T) {
	stdReproNamed(t, "lonely_sandbar_cycling_offer")
}
