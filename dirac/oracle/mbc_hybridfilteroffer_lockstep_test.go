package oracle

import "testing"

// Soliton's {U}: Untap with only Gnottvold Slumbermound's {R} and Cascade
// Bluffs' hybrid-fee filter to make the {U}: gorge offers the activation at
// decision 661 of the recording and dirac did not.
func TestMBCSolitonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hybrid_filter_fee_untap_offer")
}
