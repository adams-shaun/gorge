package oracle

import "testing"

// Arctic Merfolk's kicker returns a creature you control (CR 702.33a). The
// only creature is an Eldrazi Scion whose sacrifice pays the {1} of the spell
// (mana abilities are activated in CR 601.2g, before the kicker's cost is
// paid in CR 601.2h), so no creature is left to return and the kicked cast is
// not an option. The kernel offered it beside the plain cast (decision 237;
// tail-27-c000 pair 60, seed 976989).
func TestMBCArcticMerfolkReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kicker_return_after_sac_mana")
}
