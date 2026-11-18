package oracle

import "testing"

// CR 601.2b, 601.2g, 605.3b: Repeal's X-bound target is offered only while
// some announcement both pays and leaves a target. A seat holding an untapped
// Island and a Golgari Signet ({1},{T}: Add {B}{G}) makes {U} alone, since the
// Signet's {1} fee must come from a source other than itself: X=0 has no
// mana value 0 permanent to return, X=1 (Island for U, the fee unfunded) has
// no payable plan and X=2, the Signet's own mana value, needs three mana the
// board cannot form. gorge withholds the cast; dirac priced the filter as a
// free {B}{G} and offered it (decision 78 of 78, tail-45 pair 55).
func TestMBCRepealReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "xbound_filter_fee_unfundable")
}
