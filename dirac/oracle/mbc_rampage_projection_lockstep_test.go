package oracle

import "testing"

// CR 702.23a: Rampage N is gorge kwRampage's becomes-blocked trigger (kgen
// expandedKeywords), not a battlefield characteristic, so the chars projection
// lists it in gorgeKwElsewhere like Persist and Modular. Before it did, Horrible
// Hordes on the battlefield was a projection error (gorge keyword "Rampage:1"
// has no kernel bit). The recording is pool "custom" (a table built from its own
// decks): a tail-NN bucket moves whenever the lowered set grows, and the replay
// then fails with a card "not in the pool" before it plays a decision.
func TestMBCRampageProjectionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "rampage_has_no_chars_bit")
}
