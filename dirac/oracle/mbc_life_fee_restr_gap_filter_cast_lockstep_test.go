package oracle

import "testing"

// CR 605.3a with CR 601.2g: p1's Shadowblood Ridge ("{1}, {T}: Add {B}{R}")
// is a census gap whose fee the floating pool pays, and p1's Untaidake, the
// Cloud Keeper ("{T}, Pay 2 life: Add {C}{C}. Spend this mana only to cast
// legendary spells") is a second, live census gap beside it (life 5 > 2, its
// production restricted to legendary spells). Gorge's one prefix search
// activates every census gap in battlefield order and prices the plain cast
// of Geosurge ({R}{R}{R}{R}) with the Ridge alone: three Mountains and two
// Forests fund, one mana pays the Ridge's {1}, the Ridge's {B} and {R} join
// the pool, and the four {R} pips pay. The kernel's route table held the
// filter search off the board (the filter search runs beside other census
// gaps only as mixBoard's otherGap steps, and no step modelled a life-fee
// restricted production), so it offered pass and the land alone. The fix
// models the life-fee restricted gap as an otherGap step (otherRestrLife):
// paying the life, tapping, and floating the restricted production.
func TestMBCGeosurgeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_fee_restr_gap_filter_cast")
}
