package oracle

import "testing"

// CR 605.3a, 119.4: Treasonous Ogre's "Pay 3 life: Add {R}" is no {T} cost, so
// it can be activated again for each payment. Edgar Markov ({3}{R}{W}{B}) beside
// Artifact Unknown Shores' "{1}, {T}: any colour", three Swamps, a Mountain and
// the Ogre is affordable: the Ogre's first {R} pays the Shores' fee for {W}, the
// lands pay five of the six pips and the Ogre pays the last. gorge offers the
// cast; the kernel offered the plain cast without the Ogre's second activation
// (decision 229 of the pair 144 game, pool tail-51).
func TestMBCEdgarMarkovReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_mana_funds_filter_fee_and_plan")
}
