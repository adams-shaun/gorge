package oracle

import "testing"

// Prying Blade's equip {2} is payable through a first-only filter's fee
// (Viridescent Bog's "{1},{T}: Add {B}{G}"): the Prism token's tap-sacrifice
// makes the 1 the Bog's fee needs, the Bog's {B}{G} then pays the equip with
// the Prism's mana (CR 605.3a, 605.3b). gorge listed the activation at p0's
// priority and the kernel withheld it: its activation-offer affordability
// priced the plain census alone, which skips the filter gap, where the cast
// offer's offerSourcesAfford falls back to the gaps census. Recorded as
// tail-12 pair 6 (surface full, mulligans 0, seed 796960).
func TestMBCPryingBladeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "act_offer_funded_by_filter_fee_chain")
}
