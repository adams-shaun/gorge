package oracle

import "testing"

// CR 702.58a: Graft N is gorge kwGraft's entering-counters replacement and
// enters trigger moving a +1/+1 counter onto another creature (kgen
// expandedKeywords), not a battlefield characteristic, so the chars projection
// lists it in gorgeKwElsewhere like Modular and Persist. Before it did, Vigean
// Graftmage entering the battlefield was a projection error (gorge keyword
// "Graft:2" has no kernel bit; dr-mbc-bbaa pair 1, seed 777540, decision 227),
// the class that held back Llanowar Reborn and the other Graft carriers. The
// recording is pool "custom" (a table built from its own decks): a synth
// pool's membership moves whenever the lowered set grows.
func TestMBCLlanowarRebornReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "graft_has_no_chars_bit")
}
