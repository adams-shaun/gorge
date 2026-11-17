package oracle

import "testing"

// Knollspine Invocation ({X}, Discard a card with mana value X) paid at X=4
// with Deep-Slumber Titan (mana value 4) and a Mountain in hand (tail-53
// pair 7, decision 431): gorge read the discard filter's X as 0 and offered
// only the Mountain; the kernel, and CR 602.2b with 118.8 / 107.3, offer the
// card whose mana value is the announced X.
func TestMBCDeepSlumberTitanReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "discard_cost_paid_x")
}
