package oracle

import (
	"testing"
)

// CR 605.3b with CR 601.2g-h: a mana ability's own cost is paid from mana
// floated first. Sunscorched Divide's mana ability costs {1} in addition to
// its {T}; Fetid Heath's second ability costs a {W/B} pip in addition to its
// {T}. With a Swamp, the Divide and the Heath the board makes {R}{B}{B}: tap
// the Swamp for {B}, spend it on the Divide's {1}, take {R}{W}, then pay the
// Heath's {W/B} fee with the floating {W} for {B}{B}. Both Cadaver Imp
// ({1}{B}{B}) and Walk the Plank ({B}{B}) are therefore castable. Gorge's
// planner census missed the Heath's hybrid-fee gap and could not relax it (a
// hybrid pip has no free-face relaxation), so the gap carried no fee and the
// prefix search never floated one; the casts were withheld as
// census_incomplete while the kernel offered them (tail-01-c001, pair 73,
// seed 18376685987216178564). The census now funds the once-only fee like any
// other paid gap, and the prefix search floats it before activating the filter.
func TestMBCCadaverImpReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mana_ability_cost_not_paid")
}
