package oracle

import "testing"

// Pyretic Charge (plot {3}{R}) in tail-26: at decision 264 of the loop's pair 12
// (seed 175926, autopay) gorge offered the plotted card's free cast from exile
// and the kernel did not. The recording replays the pair in lockstep.
func TestMBCPyreticChargeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "pyretic_charge_plotted_cast_options")
}
