package oracle

import "testing"

// A filter land's activation cost can be a hybrid pip (Cascade Bluffs's
// "{U/R}, {T}: Add {U}{U}, {U}{R}, or {R}{R}"). The planner census cannot
// relax that fee, but it still has to fund the prefix search that pays it:
// with the fee read as zero the search had no non-gap activation to tap for
// the {U/R}, so gorge priced every blue cast off a single Cascade Bluffs as
// census_incomplete and posed pass while the kernel offered the cast
// (tail-55 focus game, decision 158: Perilous Voyage).
func TestMBCPerilousVoyageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hybrid_filter_fee")
}
