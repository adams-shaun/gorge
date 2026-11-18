package oracle

import "testing"

// tail-67-c001 pair 124: Hall's kernel and gorge projections disagree on
// projection Stack[1].Kind at decision 759.
func TestMBCSimicKeyruneReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "projection_stack_kind_hall")
}
