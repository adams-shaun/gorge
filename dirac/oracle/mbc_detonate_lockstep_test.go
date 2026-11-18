package oracle

import "testing"

// Detonate ({X}{R}, destroy target artifact with mana value X): the kernel
// offered the cast at decision 253 of tail-33-c000 pair 3 while gorge did not.
func TestMBCDetonateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "detonate_artifact_target_options")
}
