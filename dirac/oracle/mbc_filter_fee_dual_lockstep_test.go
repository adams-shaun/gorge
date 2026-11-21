package oracle

import "testing"

// A filter land's {B/R} fee funded by an untapped dual land lets an X spell's
// {B}{B} be paid (Mind Shatter beside Swamp, Graven Cairns and Molten
// Tributary): gorge offers the cast and the kernel did not.
func TestMBCFilterFeeDualLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_land_fee_funded_by_dual_land_xspell")
}
