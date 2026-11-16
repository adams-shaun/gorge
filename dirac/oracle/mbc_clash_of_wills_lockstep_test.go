package oracle

import "testing"

// CR 601.2b with CR 605.3b: Clash of Wills ({X}{U}) is cast with a single U
// floating and an Orzhov Signet ({1},{T}: add {W}{B}) the only other source.
// Feeding the Signet spends the floating U, so no X above 0 can be paid and
// gorge asks only X = 0. The kernel once kept the spent U in its ceiling and
// also offered X = 1 (decision 385 of pool tail-19, pair 41).
func TestMBCClashOfWillsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_fee_spends_pool_colour_in_x_ceiling")
}
