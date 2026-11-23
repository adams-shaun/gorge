package oracle

import "testing"

// CR 702.16b: Tattoo Ward's "This effect doesn't remove Tattoo Ward" spares the
// Aura at the state-based check only; the enchanted Thassa's Devourer is an
// enchantment creature with protection from enchantments, so Tattoo Ward's own
// "destroy target enchantment" ability can't target it. gorge's exception for
// the host (Card.CardUID_<host>) once spared the Aura from every protection
// check and offered the Devourer as a target the kernel did not (decision 810
// of tail-04 pair 2).
func TestMBCThassaSDevourerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "protection_exception_not_for_targeting")
}
