package oracle

import "testing"

// Spiked Ripsaw's equip ability is offered at priority by gorge and not by
// dirac (tail-71-c000 pair 25, decision 607).
func TestMBCSpikedRipsawReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spiked_ripsaw_equip_offer")
}
