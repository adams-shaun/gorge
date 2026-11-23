package oracle

import "testing"

// CR 605.3b with CR 118.12: the affordability walk of a pay-or-else resolution
// (windowAffordsTake) listed Bog Initiate ("{1}: Add {B}", a repeatable fee-only
// filter, neither tapped nor marked used) again and again once the B in the pool
// paid its fee and its output was that same B. The pool never changed, so the walk
// never ended and decision 691 never returned. The recording is the whole
// RandomPolicy game of life_split_face_cast_payment_trigger's pair, 751 decisions.
func TestMBCBogInitiateFilterFixedPointLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "bog_initiate_filter_fixed_point")
}
