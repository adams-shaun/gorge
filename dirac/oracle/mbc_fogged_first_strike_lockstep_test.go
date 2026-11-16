package oracle

import "testing"

// CR 510.4 with CR 615: a first striker blocked by two creatures is dealt its
// damage division in the first-strike step even when a Fog (Moment's Peace)
// prevents all of it, and having had first strike as the step began it takes
// no part in the regular step. The kernel skipped the whole damage pass under
// the Fog, so it never marked the first striker as having dealt damage and
// asked its controller for a second division in the regular damage step
// (Silver Knight, decision 327 of dr-mbc-babb pair 291).
func TestMBCSilverKnightReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "first_strike_fogged_not_dealt_again")
}
