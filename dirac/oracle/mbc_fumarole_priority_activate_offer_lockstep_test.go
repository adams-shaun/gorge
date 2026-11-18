package oracle

import "testing"

// At a priority decision gorge offered Wandering Fumarole's {4}{U/R}
// activation as a play and the kernel withheld it (tail-03 pair 92,
// seed 827267, decision 527, surface full). Recorded as
// fumarole_priority_activate_offer.
func TestMBCWanderingFumaroleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "fumarole_priority_activate_offer")
}
