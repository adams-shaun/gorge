package oracle

import "testing"

// CR 118.3 with CR 603.7: Vigil for the Lost's "you may pay {X}" trigger offers
// the pay arm whenever the payer's mana can cover the announced X. The
// recording is the game where gorge offered trigger_cost_pay beside the decline
// and the kernel offered the decline alone (decision 841 of tail-42 pair 0).
func TestMBCVigilForTheLostReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_x_pay_offered_when_potential_covers_x")
}
