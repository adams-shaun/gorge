package oracle

import "testing"

// Kor Haven's damage prevention and Sphere of Reason's: gorge posed a choice
// between two replacement effects (decision 906) where the kernel offered only
// priority.
func TestMBCKorHavenReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kor_haven_replacement_choice")
}
