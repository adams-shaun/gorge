package oracle

import "testing"

// Relic Robber and Slumbering Dragon: two creatures attack the Dragon's
// controller, so the Dragon's "whenever a creature attacks you" trigger
// fires twice. Kernel posed a trigger_order decision where gorge went
// straight to priority.
func TestMBCRelicRobberReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "relic_robber_dragon_trigger_order")
}
