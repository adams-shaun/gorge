package oracle

import "testing"

// CR 603.5 with CR 603.10a: Fecundity's "that creature's controller may draw
// a card" is answered by the dying creature's last-known controller, not the
// Fecundity controller. Fecundity's controller (seat 0) sees three of the
// opponent's and its own creatures die at once; each trigger's ask goes to
// the seat that controlled its creature (decision 628: seat 1's Lifecraft
// Cavalry), where the kernel once asked seat 0 for every one.
func TestMBCFecundityReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "optional_dies_trigger_decider_is_the_creatures_controller")
}
