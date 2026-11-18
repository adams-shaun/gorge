package oracle

import "testing"

// CR 601.2g, 702.27a: Whispers of the Muse's buyback cast ({1}{U} plus {5}
// is six mana) is a potential play when five lands and a Lotus Blossom with
// a petal counter ("{T}, sacrifice: add X mana of any one color") can pay it.
// gorge's potential planner withholds a play only on a proof, and Lotus is a
// gap source its census never plans, so the offer stays; the kernel's kicked
// arm read the census without Lotus and left the buyback cast out (decision
// 253, tail-74 focus game).
func TestMBCWhispersOfTheMuseReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "buyback_potential_counts_dynamic_sac_mana")
}
