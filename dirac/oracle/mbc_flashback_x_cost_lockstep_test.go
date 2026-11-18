package oracle

import "testing"

// Devil's Play's flashback cost {X}{R}{R}{R} is offered by one engine only at
// decision 236 of tail-08-c000 pair 3 (seed 22846, full surface).
func TestMBCDevilSPlayReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "flashback_x_cost_offer")
}
