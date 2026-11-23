package oracle

import "testing"

// CR 106.1, 605.3a, 601.2g: Crossroads Candleguide's "{2}: Add one mana of any
// color" is a repeatable fee-only mana ability with no {T}, so two activations
// paid from the five other untapped sources make {U}{U}, and Sunscape Master's
// "{U}{U}, {T}: Return target creature to its owner's hand" is payable. Gorge's
// potentialPrefixScript used to spend its 160-clone budget on that filter's
// fee branches before it funded the blue, so the activation was priced
// census_incomplete and not offered while the kernel's mirrored search offered
// it (tail-26 pair 49, decision 598). The fee-only prune's cut now applies
// only once the pips are complete, so both engines fund {U}{U} through the
// Candleguide and the recorded game replays without a divergence.
func TestMBCSunscapeMasterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "candleguide_fee_filter_prefix_budget")
}
