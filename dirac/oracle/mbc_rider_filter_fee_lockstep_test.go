package oracle

import "testing"

// CR 605.3a, 601.2g-h: p1's untapped Peat Bog ("{T}, remove a depletion
// counter: Add {B}{B}") and Boros Signet ("{1}, {T}: Add {R}{W}") pay Hand
// of Death's {2}{B}: one {B} pays the Signet's {1}, the other {B} and the
// {R}{W} pay the spell. gorge's prefix search offered the cast; dirac's did
// not (tail-55 pair 0, decision 378).
func TestMBCHandOfDeathReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "rider_land_funds_filter_fee")
}
