package oracle

import "testing"

// Nantuko Monastery's "Threshold — {G}{W}: becomes a 4/4 ..." is offered at
// priority only with seven or more cards in its controller's graveyard
// (tail-15-c000 pair 40, decision 833: gorge offered the activation, dirac
// did not).
func TestMBCNantukoMonasteryThresholdReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "nantuko_monastery_threshold_activation")
}
