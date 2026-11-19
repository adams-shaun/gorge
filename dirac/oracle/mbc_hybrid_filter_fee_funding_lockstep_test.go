package oracle

import (
	"testing"
)

// CR 601.2g-h and 605.3b: Fetid Heath's second mana ability ("{W/B}, {T}: Add
// {W}{W}, {W}{B} or {B}{B}") carries a hybrid mana fee, so it can make the
// {B} for Pride of the Perfect's {3}{B} only after another source has floated
// a {W} to spend on the fee. Gorge's census left that fee unfunded -- the
// prefix search could activate no covered source -- so the cast was withheld
// (census_incomplete) while the kernel offered it (tail-01 pair 6, decision
// 644).
func TestMBCPrideOfThePerfectReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hybrid_filter_fee_funding")
}
