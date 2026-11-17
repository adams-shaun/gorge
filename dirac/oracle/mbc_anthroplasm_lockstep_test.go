package oracle

import "testing"

// Anthroplasm's {X},{T} ability: the X choices gorge and dirac offer must agree
// (gorge offered 0..10, the kernel 0..9 in the recorded game).
func TestMBCAnthroplasmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "anthroplasm_x_activation_options")
}
