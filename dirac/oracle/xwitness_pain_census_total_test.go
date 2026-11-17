package oracle

import "testing"

// Mana Confluence at a Blaze cast: the {X} witness's census-total bound must
// count the pain-only sources painTight holds out of the census (gorge's
// paymentPlanCensusTotal counts every census source's largest alternative),
// or the witness announces one X less than gorge's and the cast taps one
// Confluence fewer.
func TestManaconfluenceProjectionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "xwitness_census_total_omits_pain_units")
}
