package oracle

import "testing"

// CR 702.43a: Modular N is gorge kwModular's entering-counters replacement and
// dies trigger (kgen expandedKeywords), not a battlefield characteristic, so the
// chars projection lists it in gorgeKwElsewhere like Persist and Undying. Before
// it did, Arcbound Mouser entering the battlefield was a projection error
// (gorge keyword "Modular:1" has no kernel bit). The recording is pool "custom"
// (a table built from its own decks): a tail-NN bucket moves whenever the
// lowered set grows, and the replay then fails with a card "not in the pool"
// before it plays a decision.
func TestMBCArcboundMouserReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "modular_has_no_chars_bit")
}
