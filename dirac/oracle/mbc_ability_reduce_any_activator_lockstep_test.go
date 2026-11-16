package oracle

import "testing"

// CR 601.2f, 602.2b: Heartstone ("Activated abilities of creatures cost {1}
// less to activate", ReduceCost with no Activator$) lowers the activated
// abilities of every creature, whoever activates them. Two Heartstones of one
// player cut the opponent's Flamewave Invoker from {7}{R} to {5}{R}, which the
// opponent's four lands and Remote Farm pay. The kernel priced only the
// reducer controller's own activations and never offered the activation
// (decision 320, tail-06 focus game).
func TestMBCFlamewaveInvokerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ability_cost_reduction_without_activator_reaches_opponent")
}
