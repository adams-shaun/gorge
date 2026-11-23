package oracle

import "testing"

// TestMBCBlackMarketReprosLockstep: Black Market's "whenever a creature dies"
// triggers for two creatures that die together (a Sakashima's Student and its
// Illusion token, lethal damage in combat) are two pending triggers of one
// controller, so CR 603.3b has gorge ask the order. The kernel put no ordering
// round to the controller (decision 1264 of 1264: gorge trigger_order, kernel
// priority).
func TestMBCBlackMarketReprosLockstep(t *testing.T) {
	for _, name := range []string{
		"black_market_simultaneous_death_triggers",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stdReproNamed(t, name)
		})
	}
}
