package oracle

import "testing"

// Valiant Knight's {3}{W}{W} activation is offered by one engine only at
// decision 335 of tail-41-c000 (pair 3, seed 925744, autopay).
func TestMBCValiantKnightReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "valiant_knight_activate_affordability")
}
