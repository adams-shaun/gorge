package oracle

import "testing"

// Organic Extinction ({8}{W}{W}, Improvise) is offered by gorge's planImprovise
// route but was left off dirac's priority menu (tail-65 pair 155, decision 674
// of 674, evidence loop f538fd8534).
func TestMBCOrganicExtinctionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "organic_extinction_improvise_offer")
}
