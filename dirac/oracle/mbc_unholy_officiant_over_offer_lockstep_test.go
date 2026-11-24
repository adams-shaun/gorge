package oracle

import "testing"

// Unholy Officiant's {4}{W} +1/+1 counter ability with a sole life-only pain
// unit (Treasonous Ogre) and an Artifact Unknown Shores filter among p0's
// lands: the kernel offered the activation at priority and gorge did not
// (decision 967, tail-51-c000 pair 33).
func TestMBCUnholyOfficiantOverOfferReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_pain_filter_over_offers_activation")
}
