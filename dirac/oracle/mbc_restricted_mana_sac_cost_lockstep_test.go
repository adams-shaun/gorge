package oracle

import "testing"

// CR 106.6, 601.2g-h, 602.2b: Base Camp's any-colour mana may be spent on a
// Wizard's activated ability, so it pays the {U} of Jalira, Master
// Polymorphist's "{2}{U}, {T}, Sacrifice another creature" beside two Swamps,
// with an Eldrazi Scion (a sacrifice-for-mana creature) as the only other
// creature (tail-29 + tail-36 focus pair 118, decision 646). The kernel
// checked the sacrifice part's candidates against the plain census, which
// holds no restricted source, and refused the activation gorge offered.
func TestMBCRestrictedManaSacCostReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "restricted_mana_pays_sac_cost_activation")
}
