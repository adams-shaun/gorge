package oracle

import "testing"

// Soliton's {U}: Untap with only Gnottvold Slumbermound's {R} and Cascade
// Bluffs' hybrid-fee filter to make the {U}: gorge offers the activation at
// decision 661 of the recording and dirac did not.
func TestMBCSolitonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hybrid_filter_fee_untap_offer")
	// CR 106.7 with CR 605.3b: Incubation Druid's "any type a land you control
	// could produce" names {U} off Cascade Bluffs' {U/R} filter whether or not
	// that fee can be paid, so Soliton's {U} untap is offered with no {U} or
	// {R} on the board. The kernel offered it and gorge's potential bound read
	// the Druid as {C} alone (kfuzz tail-55-c000 pair 21, decision 941).
	stdReproNamed(t, "soliton_untap_offer_beside_hybrid_filter")
}
