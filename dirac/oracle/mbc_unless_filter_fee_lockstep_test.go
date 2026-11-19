package oracle

import "testing"

// CR 605.3b with CR 117.12: a "{1}, {T}: Add {W}{B}" filter land beside an
// untapped Island can pay a {2} unless cost (Island's {U} funds the land's
// fee). Nakaya Shade's opponent faced that choice with the land listed before
// the Island; gorge's reach search only let a fee be funded by an EARLIER unit,
// so it offered only "don't pay" while dirac offered both (pair 32 of
// tail-75-c001, decision 328).
func TestMBCNakayaShadeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_filter_land_fee_funded_by_later_source")
}
